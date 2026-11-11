package loans

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"boardgames/internal/db"
)

type fixture struct {
	t                   *testing.T
	repo                *Repo
	db                  *sql.DB
	userA, userB        string
	gameA, gameB, gameC int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := db.Now()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES(?,?,?,?,?,?)`, []any{"u1", "one@example.com", "One", "admin", now, now}},
		{`INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES(?,?,?,?,?,?)`, []any{"u2", "two@example.com", "Two", "member", now, now}},
	} {
		if _, err := database.Exec(q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	f := &fixture{t: t, db: database, repo: New(database), userA: "u1", userB: "u2"}
	f.gameA = f.addGame(1, "Patchwork")
	f.gameB = f.addGame(2, "Catan")
	f.gameC = f.addGame(3, "Wingspan")
	for _, gameID := range []int64{f.gameA, f.gameB} {
		if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`,
			f.userA, gameID, now, now); err != nil {
			t.Fatal(err)
		}
	}
	// gameC exists but userA does not own it.
	if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,0,?,?)`,
		f.userA, f.gameC, now, now); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) addGame(bggID int, name string) int64 {
	f.t.Helper()
	now := db.Now()
	res, err := f.db.Exec(`INSERT INTO games(bgg_id,name,sort_name,thumbnail_url,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		bggID, name, strings.ToLower(name), "https://example.com/"+name+".jpg", now, now)
	if err != nil {
		f.t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

// daysAgo returns a YYYY-MM-DD date n days before today in UTC.
func daysAgo(n int) string {
	return time.Now().UTC().AddDate(0, 0, -n).Format("2006-01-02")
}

type fakeNotifier struct {
	mu     sync.Mutex
	titles []string
	bodies []string
	err    error
}

func (n *fakeNotifier) Notify(_ context.Context, title, body string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil {
		return n.err
	}
	n.titles = append(n.titles, title)
	n.bodies = append(n.bodies, body)
	return nil
}

func (n *fakeNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.titles)
}

func TestLendCreatesOpenLoan(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	lentAt := daysAgo(10)
	loan, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "  Sam  ", LentAt: lentAt, Notes: "box has a dent"})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	if loan.ID == 0 || loan.Borrower != "Sam" || loan.GameName != "Patchwork" || loan.BGGID != 1 {
		t.Fatalf("unexpected loan: %#v", loan)
	}
	if loan.ReturnedAt != "" || loan.ClosedReason != "" || loan.DueAt != "" {
		t.Fatalf("new loan should be open and undated: %#v", loan)
	}
	if loan.Overdue {
		t.Fatalf("loan without a due date must never be overdue")
	}
	if loan.DaysOut != 10 {
		t.Fatalf("DaysOut = %d, want 10", loan.DaysOut)
	}

	open, err := f.repo.Open(ctx, f.userA)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(open) != 1 || open[0].ID != loan.ID {
		t.Fatalf("open loans = %#v", open)
	}
	byGame, err := f.repo.OpenByGame(ctx, f.userA, f.gameA)
	if err != nil || byGame == nil || byGame.ID != loan.ID {
		t.Fatalf("OpenByGame: %v %#v", err, byGame)
	}
	missing, err := f.repo.OpenByGame(ctx, f.userA, f.gameB)
	if err != nil || missing != nil {
		t.Fatalf("OpenByGame without a loan = %v %#v", err, missing)
	}
	many, err := f.repo.OpenByGameIDs(ctx, f.userA, []int64{f.gameA, f.gameB})
	if err != nil {
		t.Fatalf("OpenByGameIDs: %v", err)
	}
	if len(many) != 1 || many[f.gameA].ID != loan.ID {
		t.Fatalf("OpenByGameIDs = %#v", many)
	}
	if empty, err := f.repo.OpenByGameIDs(ctx, f.userA, nil); err != nil || len(empty) != 0 {
		t.Fatalf("OpenByGameIDs(nil) = %#v, %v", empty, err)
	}
	names, err := f.repo.RecentBorrowers(ctx, f.userA, 5)
	if err != nil || len(names) != 1 || names[0] != "Sam" {
		t.Fatalf("RecentBorrowers = %#v, %v", names, err)
	}
}

func TestSecondOpenLoanRefusedAndFirstUnchanged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	lentAt := daysAgo(4)
	first, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: lentAt})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Alex", LentAt: daysAgo(1)}); !errors.Is(err, ErrOpenLoanExists) {
		t.Fatalf("second Lend = %v, want ErrOpenLoanExists", err)
	}
	got, err := f.repo.Get(ctx, f.userA, first.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != first.ID || got.Borrower != "Sam" || got.LentAt != lentAt {
		t.Fatalf("first loan changed: %#v", got)
	}
	open, err := f.repo.Open(ctx, f.userA)
	if err != nil || len(open) != 1 {
		t.Fatalf("Open after refused lend = %#v, %v", open, err)
	}
}

func TestLendRequiresOwnership(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// gameC exists but its user_games row is not owned.
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameC, Borrower: "Sam"}); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("unowned game = %v, want ErrNotOwned", err)
	}
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: 999999, Borrower: "Sam"}); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("nonexistent game = %v, want ErrNotOwned", err)
	}
	// Another user cannot lend a game they do not own either.
	if _, err := f.repo.Lend(ctx, f.userB, NewLoan{GameID: f.gameA, Borrower: "Sam"}); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("other user's game = %v, want ErrNotOwned", err)
	}
}

func TestReturnMovesLoanToHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	loan, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: daysAgo(12)})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	closed, err := f.repo.Return(ctx, f.userA, loan.ID, daysAgo(2))
	if err != nil {
		t.Fatalf("Return: %v", err)
	}
	if closed.ReturnedAt != daysAgo(2) || closed.ClosedReason != "returned" || closed.Overdue {
		t.Fatalf("unexpected returned loan: %#v", closed)
	}
	if closed.DaysOut != 10 {
		t.Fatalf("DaysOut after return = %d, want 10", closed.DaysOut)
	}
	if open, err := f.repo.Open(ctx, f.userA); err != nil || len(open) != 0 {
		t.Fatalf("Open after return = %#v, %v", open, err)
	}
	history, err := f.repo.Closed(ctx, f.userA, 10)
	if err != nil || len(history) != 1 || history[0].ID != loan.ID {
		t.Fatalf("Closed = %#v, %v", history, err)
	}
	if _, err := f.repo.Return(ctx, f.userA, loan.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Return = %v, want ErrNotFound", err)
	}
}

func TestOverdueIsDerived(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	overdue, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: daysAgo(20), DueAt: daysAgo(5)})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	got, err := f.repo.Get(ctx, f.userA, overdue.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Overdue {
		t.Fatalf("loan past its due date should be overdue: %#v", got)
	}
	if got.DaysOverdue != 5 {
		t.Fatalf("DaysOverdue = %d, want 5", got.DaysOverdue)
	}
	if got.DaysOut != 20 {
		t.Fatalf("DaysOut = %d, want 20", got.DaysOut)
	}
	idle, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameB, Borrower: "Alex", LentAt: daysAgo(90)})
	if err != nil {
		t.Fatalf("Lend idle: %v", err)
	}
	if idle.Overdue || idle.DaysOverdue != 0 {
		t.Fatalf("loan without a due date must never be overdue: %#v", idle)
	}
}

func TestCrossUserIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	loan, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam"})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	if _, err := f.repo.Get(ctx, f.userB, loan.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user Get = %v, want ErrNotFound", err)
	}
	open, err := f.repo.Open(ctx, f.userB)
	if err != nil || len(open) != 0 {
		t.Fatalf("cross-user Open = %#v, %v", open, err)
	}
	history, err := f.repo.Closed(ctx, f.userB, 10)
	if err != nil || len(history) != 0 {
		t.Fatalf("cross-user Closed = %#v, %v", history, err)
	}
}

func TestDueForReminderHonoursInterval(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: daysAgo(3), DueAt: daysAgo(2)}); err != nil {
		t.Fatalf("Lend: %v", err)
	}
	interval := 14 * 24 * time.Hour
	now := time.Now()

	due, err := f.repo.DueForReminder(ctx, now, interval)
	if err != nil || len(due) != 1 {
		t.Fatalf("fresh overdue loan should be due: %#v, %v", due, err)
	}
	if err := f.repo.MarkNotified(ctx, due[0].ID, now); err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}
	if due, err := f.repo.DueForReminder(ctx, now, interval); err != nil || len(due) != 0 {
		t.Fatalf("loan just stamped must not be due: %#v, %v", due, err)
	}
	// A fresh repository over the same database sees the stamp: the guard is in
	// the database, so a restart does not repeat the reminder.
	restarted := New(f.db)
	if due, err := restarted.DueForReminder(ctx, now, interval); err != nil || len(due) != 0 {
		t.Fatalf("after restart the stamped loan must not be due: %#v, %v", due, err)
	}
	// Once the interval elapses it is due again.
	if due, err := f.repo.DueForReminder(ctx, now.Add(interval+time.Minute), interval); err != nil || len(due) != 1 {
		t.Fatalf("stale stamp should be due again: %#v, %v", due, err)
	}

	// A loan that is not overdue and has not reached the idle period is skipped.
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameB, Borrower: "Alex", LentAt: daysAgo(5)}); err != nil {
		t.Fatalf("Lend idle: %v", err)
	}
	if due, err := f.repo.DueForReminder(ctx, now.Add(interval+time.Minute), interval); err != nil || len(due) != 1 {
		t.Fatalf("young idle loan must not be due: %#v, %v", due, err)
	}
}

func TestRemindSendsOnceAndRetriesFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: daysAgo(30), DueAt: daysAgo(9)}); err != nil {
		t.Fatalf("Lend: %v", err)
	}
	interval := 14 * 24 * time.Hour

	if n, err := f.repo.Remind(ctx, nil, interval); n != 0 || err != nil {
		t.Fatalf("Remind(nil) = %d, %v; want 0, nil", n, err)
	}

	notifier := &fakeNotifier{}
	n, err := f.repo.Remind(ctx, notifier, interval)
	if err != nil || n != 1 || notifier.count() != 1 {
		t.Fatalf("Remind = %d, %v, notifications %d", n, err, notifier.count())
	}
	if title := notifier.titles[0]; title != "Game still out" {
		t.Fatalf("title = %q", title)
	}
	body := notifier.bodies[0]
	for _, want := range []string{"Patchwork", "Sam", daysAgo(30), "due " + daysAgo(9)} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q does not mention %q", body, want)
		}
	}
	if n, err := f.repo.Remind(ctx, notifier, interval); n != 0 || err != nil || notifier.count() != 1 {
		t.Fatalf("second Remind = %d, %v, notifications %d", n, err, notifier.count())
	}

	// A failing notifier must not stamp, so the next pass retries.
	failing := &fakeNotifier{err: errors.New("telegram down")}
	if n, err := f.repo.Remind(ctx, failing, interval); n != 0 || err != nil {
		t.Fatalf("Remind with failing notifier = %d, %v", n, err)
	}
	if due, err := f.repo.DueForReminder(ctx, time.Now(), interval); err != nil || len(due) != 0 {
		t.Fatalf("already-notified loan should not be due yet: %#v, %v", due, err)
	}
	// Stamp is still the successful send's timestamp: rewind it to prove retry.
	due, err := f.repo.DueForReminder(ctx, time.Now().Add(interval+time.Minute), interval)
	if err != nil || len(due) != 1 {
		t.Fatalf("loan should be due after the interval: %#v, %v", due, err)
	}
	if err := f.repo.MarkNotified(ctx, due[0].ID, time.Now().Add(-interval-time.Hour)); err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}
	if n, err := f.repo.Remind(ctx, failing, interval); n != 0 || err != nil {
		t.Fatalf("Remind with failing notifier = %d, %v", n, err)
	}
	if due, err := f.repo.DueForReminder(ctx, time.Now(), interval); err != nil || len(due) != 1 {
		t.Fatalf("failed send must remain due: %#v, %v", due, err)
	}
}

func TestCloseForOwnershipChange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	loan, err := f.repo.Lend(ctx, f.userA, NewLoan{GameID: f.gameA, Borrower: "Sam", LentAt: daysAgo(40)})
	if err != nil {
		t.Fatalf("Lend: %v", err)
	}
	if due, err := f.repo.DueForReminder(ctx, time.Now(), 14*24*time.Hour); err != nil || len(due) != 1 {
		t.Fatalf("idle loan should be due before the ownership change: %#v, %v", due, err)
	}
	if _, err := f.db.Exec(`UPDATE user_games SET owned=0 WHERE user_id=? AND game_id=?`, f.userA, f.gameA); err != nil {
		t.Fatalf("drop ownership: %v", err)
	}
	closedAt := daysAgo(0)
	n, err := f.repo.CloseForOwnershipChangeAt(ctx, f.userA, closedAt)
	if err != nil || n != 1 {
		t.Fatalf("CloseForOwnershipChangeAt = %d, %v", n, err)
	}
	got, err := f.repo.Get(ctx, f.userA, loan.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ReturnedAt != closedAt || got.ClosedReason != "ownership" {
		t.Fatalf("loan not closed for ownership: %#v", got)
	}
	if !strings.Contains(got.Notes, "Closed automatically: removed from collection on "+closedAt) {
		t.Fatalf("explanatory note missing: %q", got.Notes)
	}
	if due, err := f.repo.DueForReminder(ctx, time.Now(), 14*24*time.Hour); err != nil || len(due) != 0 {
		t.Fatalf("closed loan must not be reminded: %#v, %v", due, err)
	}
	// Nothing left to close on a second pass.
	if n, err := f.repo.CloseForOwnershipChange(ctx, f.userA); err != nil || n != 0 {
		t.Fatalf("second pass = %d, %v", n, err)
	}
}

// TestMigrationCreatesLoansAndUniqueIndex proves 0007 applies cleanly and that
// the partial index enforces at most one open loan per user and game.
func TestMigrationCreatesLoansAndUniqueIndex(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var name string
	if err := f.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='loans'`).Scan(&name); err != nil {
		t.Fatalf("loans table missing: %v", err)
	}
	if err := f.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_loans_open_unique'`).Scan(&name); err != nil {
		t.Fatalf("partial unique index missing: %v", err)
	}
	now := db.Now()
	if _, err := f.db.ExecContext(ctx, `INSERT INTO loans(user_id,game_id,borrower,lent_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		f.userA, f.gameA, "Sam", daysAgo(2), now, now); err != nil {
		t.Fatalf("first open loan: %v", err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO loans(user_id,game_id,borrower,lent_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		f.userA, f.gameA, "Alex", daysAgo(1), now, now); err == nil {
		t.Fatal("second open loan for the same game was accepted")
	}
	// A closed row does not block a new open loan.
	if _, err := f.db.ExecContext(ctx, `INSERT INTO loans(user_id,game_id,borrower,lent_at,returned_at,closed_reason,created_at,updated_at) VALUES(?,?,?,?,?,'returned',?,?)`,
		f.userA, f.gameA, "Alex", daysAgo(20), daysAgo(10), now, now); err != nil {
		t.Fatalf("closed loan should be allowed: %v", err)
	}
}
