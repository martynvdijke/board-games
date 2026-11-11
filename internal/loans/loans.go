// Package loans records which owned games are out on loan, derives overdue
// state at read time, and dispatches reminders through internal/notify.
//
// Dates (lent, due, returned) are stored as bare YYYY-MM-DD strings because a
// loan is a calendar-day concept, not an instant. Every comparison and every
// derived day count in this package is performed in UTC: "today" means the
// current date in UTC. Timestamps (created_at, updated_at, notified_at) use
// the canonical RFC3339 UTC form from db.Now.
package loans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"boardgames/internal/db"
	"boardgames/internal/notify"
)

// dateLayout is the stored format for calendar-day fields.
const dateLayout = "2006-01-02"

// defaultIdleDays is the first reminder delay for a loan without a due date.
const defaultIdleDays = 30

var (
	// ErrOpenLoanExists reports that the game already has an open loan (HTTP 409).
	ErrOpenLoanExists = errors.New("loans: game already has an open loan")
	// ErrNotOwned reports that the game is not on the user's owned shelf (HTTP 422).
	ErrNotOwned = errors.New("loans: game is not owned by user")
	// ErrNotFound reports a loan that does not exist for this user (HTTP 404).
	ErrNotFound = errors.New("loans: not found")
	// ErrInvalid reports a rejected input, such as a missing borrower or a
	// malformed date (HTTP 422). Additive to the agreed contract so callers can
	// distinguish bad input from an internal failure.
	ErrInvalid = errors.New("loans: invalid loan")
)

// Loan is one lending event. ReturnedAt and ClosedReason stay empty while the
// loan is open; Overdue, DaysOut and DaysOverdue are derived on every read.
type Loan struct {
	ID           int64
	UserID       string
	GameID       int64
	GameName     string
	BGGID        int
	ThumbnailURL string
	Borrower     string
	Contact      string
	LentAt       string
	DueAt        string
	ReturnedAt   string
	Notes        string
	ClosedReason string
	NotifiedAt   string
	Overdue      bool
	DaysOut      int
	DaysOverdue  int
}

// NewLoan is the input for Lend. LentAt defaults to today when empty.
type NewLoan struct {
	GameID   int64
	Borrower string
	Contact  string
	LentAt   string
	DueAt    string
	Notes    string
}

// Repo persists loans. It is safe for concurrent use.
type Repo struct {
	db       *sql.DB
	idleDays int
}

// New returns a repository backed by database. The idle reminder period
// defaults to 30 days; use SetIdleDays to match configuration.
func New(database *sql.DB) *Repo {
	return &Repo{db: database, idleDays: defaultIdleDays}
}

// SetIdleDays changes how long a loan without a due date may sit before the
// first reminder. Non-positive values are ignored so the default survives.
func (r *Repo) SetIdleDays(days int) {
	if days > 0 {
		r.idleDays = days
	}
}

const loanColumns = `l.id, l.user_id, l.game_id, g.name, g.bgg_id, g.thumbnail_url,
	l.borrower, l.contact, l.lent_at, l.due_at, l.returned_at, l.notes,
	l.closed_reason, l.notified_at`

const loanFrom = ` FROM loans l JOIN games g ON g.id = l.game_id`

// Lend records a new open loan for an owned game.
func (r *Repo) Lend(ctx context.Context, userID string, in NewLoan) (*Loan, error) {
	borrower := strings.TrimSpace(in.Borrower)
	if borrower == "" {
		return nil, fmt.Errorf("%w: a borrower name is required", ErrInvalid)
	}
	if utf8.RuneCountInString(borrower) > 120 {
		return nil, fmt.Errorf("%w: the borrower name is too long", ErrInvalid)
	}
	contact := strings.TrimSpace(in.Contact)
	if utf8.RuneCountInString(contact) > 200 {
		return nil, fmt.Errorf("%w: the contact is too long", ErrInvalid)
	}
	notes := strings.TrimSpace(in.Notes)
	if utf8.RuneCountInString(notes) > 2000 {
		return nil, fmt.Errorf("%w: the notes are too long", ErrInvalid)
	}
	lentAt := strings.TrimSpace(in.LentAt)
	if lentAt == "" {
		lentAt = todayUTC()
	} else if _, err := time.Parse(dateLayout, lentAt); err != nil {
		return nil, fmt.Errorf("%w: the lend date must be YYYY-MM-DD", ErrInvalid)
	}
	dueAt := strings.TrimSpace(in.DueAt)
	if dueAt != "" {
		if _, err := time.Parse(dateLayout, dueAt); err != nil {
			return nil, fmt.Errorf("%w: the due date must be YYYY-MM-DD", ErrInvalid)
		}
		if dueAt < lentAt {
			return nil, fmt.Errorf("%w: the due date cannot be before the lend date", ErrInvalid)
		}
	}
	if in.GameID <= 0 {
		return nil, ErrNotOwned
	}

	now := db.Now()
	var id int64
	err := db.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var owned int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM user_games WHERE user_id=? AND game_id=? AND owned=1`,
			userID, in.GameID).Scan(&owned); err != nil {
			return err
		}
		if owned == 0 {
			return ErrNotOwned
		}
		var existing int64
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM loans WHERE user_id=? AND game_id=? AND returned_at=''`,
			userID, in.GameID).Scan(&existing)
		switch {
		case err == nil:
			return ErrOpenLoanExists
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO loans(user_id,game_id,borrower,contact,lent_at,due_at,returned_at,notes,closed_reason,notified_at,created_at,updated_at)
			VALUES(?,?,?,?,?,?,'',?,'','',?,?)`,
			userID, in.GameID, borrower, contact, lentAt, dueAt, notes, now, now)
		if err != nil {
			// The partial unique index is the race-proof arbiter; map its
			// violation onto the same sentinel the pre-check returns.
			if isUniqueViolation(err) {
				return ErrOpenLoanExists
			}
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, userID, id)
}

// Return closes an open loan. returnedAt defaults to today when empty.
func (r *Repo) Return(ctx context.Context, userID string, loanID int64, returnedAt string) (*Loan, error) {
	returnedAt = strings.TrimSpace(returnedAt)
	if returnedAt == "" {
		returnedAt = todayUTC()
	} else if _, err := time.Parse(dateLayout, returnedAt); err != nil {
		return nil, fmt.Errorf("%w: the return date must be YYYY-MM-DD", ErrInvalid)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE loans SET returned_at=?, closed_reason='returned', updated_at=?
		WHERE id=? AND user_id=? AND returned_at=''`,
		returnedAt, db.Now(), loanID, userID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, userID, loanID)
}

// Get returns one loan belonging to userID, or ErrNotFound.
func (r *Repo) Get(ctx context.Context, userID string, loanID int64) (*Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+loanFrom+` WHERE l.id=? AND l.user_id=?`, loanID, userID)
	loan, err := scanLoan(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	derive(loan, todayUTC())
	return loan, nil
}

// Open lists open loans oldest lend date first.
func (r *Repo) Open(ctx context.Context, userID string) ([]Loan, error) {
	return r.query(ctx, `SELECT `+loanColumns+loanFrom+
		` WHERE l.user_id=? AND l.returned_at='' ORDER BY l.lent_at, l.id`, userID)
}

// Closed lists recently returned loans, newest return date first.
func (r *Repo) Closed(ctx context.Context, userID string, limit int) ([]Loan, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.query(ctx, `SELECT `+loanColumns+loanFrom+
		` WHERE l.user_id=? AND l.returned_at<>'' ORDER BY l.returned_at DESC, l.id DESC LIMIT ?`, userID, limit)
}

// OpenByGame returns the open loan for one game, or nil when there is none.
func (r *Repo) OpenByGame(ctx context.Context, userID string, gameID int64) (*Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+loanFrom+
		` WHERE l.user_id=? AND l.game_id=? AND l.returned_at='' LIMIT 1`, userID, gameID)
	loan, err := scanLoan(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	derive(loan, todayUTC())
	return loan, nil
}

// OpenByGameIDs returns the open loans for a set of games in one query. Games
// without an open loan are simply absent from the map.
func (r *Repo) OpenByGameIDs(ctx context.Context, userID string, gameIDs []int64) (map[int64]*Loan, error) {
	out := make(map[int64]*Loan, len(gameIDs))
	if len(gameIDs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(gameIDs)+1)
	args = append(args, userID)
	placeholders := make([]string, len(gameIDs))
	for i, id := range gameIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := `SELECT ` + loanColumns + loanFrom +
		` WHERE l.user_id=? AND l.returned_at='' AND l.game_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY l.lent_at, l.id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := todayUTC()
	for rows.Next() {
		loan, err := scanLoan(rows.Scan)
		if err != nil {
			return nil, err
		}
		derive(loan, today)
		out[loan.GameID] = loan
	}
	return out, rows.Err()
}

// RecentBorrowers returns distinct borrower names, most recently used first.
func (r *Repo) RecentBorrowers(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT borrower FROM loans WHERE user_id=? GROUP BY borrower COLLATE NOCASE ORDER BY MAX(created_at) DESC LIMIT ?`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// DueForReminder selects open loans that warrant a nudge and have not been
// notified within interval. A loan qualifies when it is overdue, or when it
// has no due date and has been out for at least the configured idle period.
func (r *Repo) DueForReminder(ctx context.Context, now time.Time, interval time.Duration) ([]Loan, error) {
	if interval < 0 {
		interval = 0
	}
	now = now.UTC()
	today := now.Format(dateLayout)
	idleCutoff := now.AddDate(0, 0, -r.idleDays).Format(dateLayout)
	notifiedCutoff := db.Timestamp(now.Add(-interval))
	rows, err := r.db.QueryContext(ctx, `SELECT `+loanColumns+loanFrom+
		` WHERE l.returned_at=''
		AND ((l.due_at<>'' AND l.due_at < ?) OR (l.due_at='' AND l.lent_at <= ?))
		AND (l.notified_at='' OR l.notified_at < ?)
		ORDER BY l.lent_at, l.id`, today, idleCutoff, notifiedCutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Loan
	for rows.Next() {
		loan, err := scanLoan(rows.Scan)
		if err != nil {
			return nil, err
		}
		derive(loan, today)
		out = append(out, *loan)
	}
	return out, rows.Err()
}

// MarkNotified records when a reminder for loanID was sent.
func (r *Repo) MarkNotified(ctx context.Context, loanID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE loans SET notified_at=? WHERE id=?`, db.Timestamp(at), loanID)
	return err
}

// Remind sends at most one notification per due loan and stamps each success.
// A nil notifier is a no-op. Send and stamp failures are logged, never fatal,
// and a failed send leaves the loan unstamped so the next pass retries it.
func (r *Repo) Remind(ctx context.Context, n notify.Notifier, interval time.Duration) (int, error) {
	if n == nil {
		return 0, nil
	}
	now := time.Now()
	due, err := r.DueForReminder(ctx, now, interval)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, loan := range due {
		title, body := reminderText(loan)
		if err := n.Notify(ctx, title, body); err != nil {
			slog.Warn("loan reminder failed", "loan", loan.ID, "error", err)
			continue
		}
		if err := r.MarkNotified(ctx, loan.ID, now); err != nil {
			slog.Warn("record loan reminder", "loan", loan.ID, "error", err)
			continue
		}
		sent++
	}
	return sent, nil
}

// CloseForOwnershipChange closes every open loan for userID whose game is no
// longer owned. It returns the number of loans closed.
func (r *Repo) CloseForOwnershipChange(ctx context.Context, userID string) (int, error) {
	return r.CloseForOwnershipChangeAt(ctx, userID, todayUTC())
}

// CloseForOwnershipChangeAt is CloseForOwnershipChange with an explicit close
// date, so tests and backfills are deterministic.
func (r *Repo) CloseForOwnershipChangeAt(ctx context.Context, userID, today string) (int, error) {
	if _, err := time.Parse(dateLayout, today); err != nil {
		return 0, fmt.Errorf("%w: the close date must be YYYY-MM-DD", ErrInvalid)
	}
	note := fmt.Sprintf("Closed automatically: removed from collection on %s", today)
	res, err := r.db.ExecContext(ctx, `UPDATE loans SET
			returned_at=?, closed_reason='ownership',
			notes = CASE WHEN notes='' THEN ? ELSE notes || char(10) || ? END,
			updated_at=?
		WHERE user_id=? AND returned_at='' AND NOT EXISTS (
			SELECT 1 FROM user_games ug
			WHERE ug.user_id=loans.user_id AND ug.game_id=loans.game_id AND ug.owned=1
		)`, today, note, note, db.Now(), userID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// query runs a loan listing query and derives each row.
func (r *Repo) query(ctx context.Context, query string, args ...any) ([]Loan, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := todayUTC()
	var out []Loan
	for rows.Next() {
		loan, err := scanLoan(rows.Scan)
		if err != nil {
			return nil, err
		}
		derive(loan, today)
		out = append(out, *loan)
	}
	return out, rows.Err()
}

func scanLoan(scan func(dest ...any) error) (*Loan, error) {
	var l Loan
	if err := scan(&l.ID, &l.UserID, &l.GameID, &l.GameName, &l.BGGID, &l.ThumbnailURL,
		&l.Borrower, &l.Contact, &l.LentAt, &l.DueAt, &l.ReturnedAt, &l.Notes,
		&l.ClosedReason, &l.NotifiedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

// derive fills the read-time fields. Overdue is never stored: an open loan is
// overdue when its due date is before today (UTC, date-only comparison).
func derive(l *Loan, today string) {
	end := l.ReturnedAt
	if end == "" {
		end = today
	}
	l.DaysOut = dayCount(l.LentAt, end)
	l.Overdue = false
	l.DaysOverdue = 0
	if l.ReturnedAt == "" && l.DueAt != "" && l.DueAt < today {
		l.Overdue = true
		l.DaysOverdue = dayCount(l.DueAt, today)
	}
}

// dayCount counts whole calendar days from one YYYY-MM-DD date to another.
func dayCount(from, to string) int {
	a, err := time.Parse(dateLayout, from)
	if err != nil {
		return 0
	}
	b, err := time.Parse(dateLayout, to)
	if err != nil {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}

func todayUTC() string { return time.Now().UTC().Format(dateLayout) }

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

// reminderText builds the notification for a due loan.
func reminderText(l Loan) (string, string) {
	name := strings.TrimSpace(l.GameName)
	if name == "" {
		name = "A game"
	}
	borrower := strings.TrimSpace(l.Borrower)
	if borrower == "" {
		borrower = "someone"
	}
	body := fmt.Sprintf("%s has been with %s since %s — %d days", name, borrower, l.LentAt, l.DaysOut)
	if l.DueAt != "" {
		body += fmt.Sprintf(", due %s.", l.DueAt)
	} else {
		body += "."
	}
	return "Game still out", body
}
