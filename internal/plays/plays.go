// Package plays implements local play logging, editing, timelines, and
// aggregate statistics. Imported BGG plays share the same storage.
package plays

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"boardgames/internal/db"
	"boardgames/internal/observability"
)

var (
	ErrNotFound = errors.New("play not found")
	ErrInvalid  = errors.New("invalid play")
)

type Player struct {
	Name     string `json:"name"`
	Score    string `json:"score"`
	IsWinner bool   `json:"is_winner"`
	IsFirst  bool   `json:"is_first_play"`
}

type Play struct {
	ID           string
	UserID       string
	GameID       int64
	BGGID        int
	GameName     string
	ThumbnailURL string
	PlayedAt     string
	Location     string
	Duration     int
	Quantity     int
	Notes        string
	Origin       string
	BGGPlayID    *int64
	CreatedAt    string
	UpdatedAt    string
	Players      []Player
}

type Input struct {
	GameID   int64
	PlayedAt string
	Location string
	Duration int
	Quantity int
	Notes    string
	Players  []Player
}

type Repo struct{ DB *sql.DB }

func NewRepo(database *sql.DB) *Repo { return &Repo{DB: database} }

// Validate enforces the same rules for web and API writes.
func (r *Repo) Validate(ctx context.Context, userID string, in Input, timezone string) error {
	if in.GameID <= 0 {
		return fmt.Errorf("%w: choose a game", ErrInvalid)
	}
	if in.PlayedAt == "" {
		return fmt.Errorf("%w: a play date is required", ErrInvalid)
	}
	date, err := time.Parse("2006-01-02", in.PlayedAt)
	if err != nil {
		return fmt.Errorf("%w: the play date must be YYYY-MM-DD", ErrInvalid)
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc).Format("2006-01-02")
	if in.PlayedAt > today {
		return fmt.Errorf("%w: a play cannot be in the future", ErrInvalid)
	}
	_ = date
	if in.Duration < 0 {
		return fmt.Errorf("%w: duration cannot be negative", ErrInvalid)
	}
	if in.Duration > 24*60 {
		return fmt.Errorf("%w: duration must be under 24 hours", ErrInvalid)
	}
	if in.Quantity < 1 {
		in.Quantity = 1
	}
	if in.Quantity > 50 {
		return fmt.Errorf("%w: quantity is too large", ErrInvalid)
	}
	if len(in.Players) > 20 {
		return fmt.Errorf("%w: at most 20 players are supported", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, p := range in.Players {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return fmt.Errorf("%w: every player needs a name", ErrInvalid)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("%w: duplicate player %q", ErrInvalid, name)
		}
		seen[key] = true
	}
	var exists int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_games WHERE user_id=? AND game_id=?`, userID, in.GameID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("%w: that game is not on your shelf", ErrInvalid)
	}
	return nil
}

func (r *Repo) Create(ctx context.Context, userID string, in Input, timezone string) (*Play, error) {
	if err := r.Validate(ctx, userID, in, timezone); err != nil {
		return nil, err
	}
	id := newID()
	now := db.Now()
	err := db.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,location,duration_minutes,quantity,notes,origin,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,'local',?,?)`, id, userID, in.GameID, in.PlayedAt, strings.TrimSpace(in.Location), nullableInt(in.Duration), in.Quantity, strings.TrimSpace(in.Notes), now, now)
		if err != nil {
			return err
		}
		return insertPlayers(ctx, tx, id, in.Players)
	})
	if err != nil {
		return nil, err
	}
	observability.RecordPlayCreated(ctx, "local")
	return r.Get(ctx, userID, id)
}

func nullableInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}

func insertPlayers(ctx context.Context, tx *sql.Tx, playID string, players []Player) error {
	for i, p := range players {
		_, err := tx.ExecContext(ctx, `INSERT INTO play_players(play_id,name,score,is_winner,is_first_play,position) VALUES(?,?,?,?,?,?)`,
			playID, strings.TrimSpace(p.Name), strings.TrimSpace(p.Score), boolInt(p.IsWinner), boolInt(p.IsFirst), i)
		if err != nil {
			return err
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newID() string {
	b := make([]byte, 16)
	_ = fillRandom(b)
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&15]
	}
	return string(out)
}

func (r *Repo) Get(ctx context.Context, userID, id string) (*Play, error) {
	var p Play
	var duration, bggPlayID sql.NullInt64
	err := r.DB.QueryRowContext(ctx, `SELECT p.id,p.user_id,p.game_id,g.bgg_id,g.name,g.thumbnail_url,p.played_at,p.location,
		coalesce(p.duration_minutes,0),p.quantity,p.notes,p.origin,p.bgg_play_id,p.created_at,p.updated_at
		FROM plays p JOIN games g ON g.id=p.game_id WHERE p.id=? AND p.user_id=?`, id, userID).
		Scan(&p.ID, &p.UserID, &p.GameID, &p.BGGID, &p.GameName, &p.ThumbnailURL, &p.PlayedAt, &p.Location,
			&duration, &p.Quantity, &p.Notes, &p.Origin, &bggPlayID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Duration = int(duration.Int64)
	if bggPlayID.Valid {
		v := bggPlayID.Int64
		p.BGGPlayID = &v
	}
	p.Players = r.players(ctx, p.ID)
	return &p, nil
}

func (r *Repo) players(ctx context.Context, playID string) []Player {
	rows, err := r.DB.QueryContext(ctx, `SELECT name,score,is_winner,is_first_play FROM play_players WHERE play_id=? ORDER BY position,name`, playID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Player
	for rows.Next() {
		var p Player
		var win, first int
		if rows.Scan(&p.Name, &p.Score, &win, &first) == nil {
			p.IsWinner = win == 1
			p.IsFirst = first == 1
			out = append(out, p)
		}
	}
	return out
}

func (r *Repo) Update(ctx context.Context, userID, id string, in Input, timezone string) (*Play, error) {
	if err := r.Validate(ctx, userID, in, timezone); err != nil {
		return nil, err
	}
	now := db.Now()
	err := db.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE plays SET game_id=?,played_at=?,location=?,duration_minutes=?,quantity=?,notes=?,updated_at=?
			WHERE id=? AND user_id=? AND origin='local'`, in.GameID, in.PlayedAt, strings.TrimSpace(in.Location), nullableInt(in.Duration), in.Quantity, strings.TrimSpace(in.Notes), now, id, userID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM play_players WHERE play_id=?`, id); err != nil {
			return err
		}
		return insertPlayers(ctx, tx, id, in.Players)
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, userID, id)
}

func (r *Repo) Delete(ctx context.Context, userID, id string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM plays WHERE id=? AND user_id=? AND origin='local'`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type ListOptions struct {
	GameID int64
	Year   int
	Limit  int
	Offset int
}

func (r *Repo) List(ctx context.Context, userID string, opt ListOptions) ([]Play, int, error) {
	if opt.Limit <= 0 {
		opt.Limit = 50
	}
	if opt.Limit > 200 {
		opt.Limit = 200
	}
	where := " WHERE p.user_id=?"
	args := []any{userID}
	if opt.GameID > 0 {
		where += " AND p.game_id=?"
		args = append(args, opt.GameID)
	}
	if opt.Year > 0 {
		where += " AND substr(p.played_at,1,4)=?"
		args = append(args, fmt.Sprintf("%04d", opt.Year))
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM plays p"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opt.Limit, opt.Offset)
	rows, err := r.DB.QueryContext(ctx, `SELECT p.id,p.user_id,p.game_id,g.bgg_id,g.name,g.thumbnail_url,p.played_at,p.location,
		coalesce(p.duration_minutes,0),p.quantity,p.notes,p.origin,p.bgg_play_id,p.created_at,p.updated_at
		FROM plays p JOIN games g ON g.id=p.game_id`+where+` ORDER BY p.played_at DESC, p.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	ids := []string{}
	var out []Play
	for rows.Next() {
		var p Play
		var duration, bggPlayID sql.NullInt64
		if err := rows.Scan(&p.ID, &p.UserID, &p.GameID, &p.BGGID, &p.GameName, &p.ThumbnailURL, &p.PlayedAt, &p.Location,
			&duration, &p.Quantity, &p.Notes, &p.Origin, &bggPlayID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		p.Duration = int(duration.Int64)
		if bggPlayID.Valid {
			v := bggPlayID.Int64
			p.BGGPlayID = &v
		}
		out = append(out, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	byPlay := r.playersPage(ctx, ids)
	for i := range out {
		out[i].Players = byPlay[out[i].ID]
	}
	return out, total, nil
}

func (r *Repo) playersPage(ctx context.Context, ids []string) map[string][]Player {
	out := map[string][]Player{}
	if len(ids) == 0 {
		return out
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT play_id,name,score,is_winner,is_first_play FROM play_players WHERE play_id IN (`+placeholders+`) ORDER BY position,name`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var playID string
		var p Player
		var win, first int
		if rows.Scan(&playID, &p.Name, &p.Score, &win, &first) == nil {
			p.IsWinner = win == 1
			p.IsFirst = first == 1
			out[playID] = append(out[playID], p)
		}
	}
	return out
}

func (r *Repo) Recent(ctx context.Context, userID string, limit int) ([]Play, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT p.id,p.user_id,p.game_id,g.bgg_id,g.name,g.thumbnail_url,p.played_at,p.location,
		coalesce(p.duration_minutes,0),p.quantity,p.notes,p.origin,p.bgg_play_id,p.created_at,p.updated_at
		FROM plays p JOIN games g ON g.id=p.game_id WHERE p.user_id=? ORDER BY p.played_at DESC, p.created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Play
	for rows.Next() {
		var p Play
		var duration, bggPlayID sql.NullInt64
		if err := rows.Scan(&p.ID, &p.UserID, &p.GameID, &p.BGGID, &p.GameName, &p.ThumbnailURL, &p.PlayedAt, &p.Location,
			&duration, &p.Quantity, &p.Notes, &p.Origin, &bggPlayID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Duration = int(duration.Int64)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Stats builds the statistics page from the user's data only.
func (r *Repo) Stats(ctx context.Context, userID string, year int) (Statistics, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	s := Statistics{Year: year}
	s.Monthly = make([]MonthCount, 12)
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for i := range s.Monthly {
		s.Monthly[i] = MonthCount{Month: i + 1, Label: monthNames[i]}
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT CAST(substr(played_at,6,2) AS INTEGER), COALESCE(SUM(quantity),0) FROM plays
		WHERE user_id=? AND substr(played_at,1,4)=? GROUP BY 1`, userID, fmt.Sprintf("%04d", year))
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var m, n int
		if rows.Scan(&m, &n) == nil && m >= 1 && m <= 12 {
			s.Monthly[m-1].Count = n
		}
	}
	rows.Close()
	_ = r.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity),0), COUNT(*) FROM plays WHERE user_id=?`, userID).Scan(&s.TotalPlays, &s.TotalSessions)

	rows, err = r.DB.QueryContext(ctx, `SELECT g.name, COALESCE(SUM(p.quantity),0) FROM plays p JOIN games g ON g.id=p.game_id
		WHERE p.user_id=? GROUP BY p.game_id ORDER BY 2 DESC, g.name LIMIT 8`, userID)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var c CountItem
		if rows.Scan(&c.Name, &c.Count) == nil {
			s.TopGames = append(s.TopGames, c)
		}
	}
	rows.Close()

	rows, err = r.DB.QueryContext(ctx, `SELECT MIN(pp.name), COUNT(*), COALESCE(SUM(pp.is_winner),0)
		FROM play_players pp JOIN plays p ON p.id=pp.play_id WHERE p.user_id=?
		GROUP BY lower(pp.name) ORDER BY 3 DESC, 2 DESC, 1 COLLATE NOCASE LIMIT 12`, userID)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var e LeaderEntry
		if rows.Scan(&e.Name, &e.Plays, &e.Wins) == nil {
			s.Leaderboard = append(s.Leaderboard, e)
		}
	}
	rows.Close()

	bands := map[string]int{}
	rows, err = r.DB.QueryContext(ctx, `SELECT g.weight FROM games g JOIN user_games ug ON ug.game_id=g.id WHERE ug.user_id=? AND ug.owned=1`, userID)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var w sql.NullFloat64
		if rows.Scan(&w) == nil {
			label := "Unknown"
			if w.Valid {
				switch {
				case w.Float64 < 1.8:
					label = "Light"
				case w.Float64 < 2.6:
					label = "Medium-light"
				case w.Float64 < 3.4:
					label = "Medium"
				case w.Float64 < 4.2:
					label = "Medium-heavy"
				default:
					label = "Heavy"
				}
			}
			bands[label]++
		}
	}
	rows.Close()
	for _, label := range []string{"Light", "Medium-light", "Medium", "Medium-heavy", "Heavy", "Unknown"} {
		if bands[label] > 0 {
			s.WeightBands = append(s.WeightBands, BandCount{Label: label, Count: bands[label]})
		}
	}
	return s, nil
}

// UpsertImported inserts or updates a BGG play by (user, bgg_play_id).
func (r *Repo) UpsertImported(ctx context.Context, userID string, p Play) (bool, error) {
	var created bool
	err := db.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		var err error
		created, err = r.UpsertImportedTx(ctx, tx, userID, p)
		return err
	})
	return created, err
}

// UpsertImportedTx imports a BGG play inside the caller's transaction. It is
// idempotent on (user_id, bgg_play_id) so a failed page can be retried safely.
func (r *Repo) UpsertImportedTx(ctx context.Context, tx *sql.Tx, userID string, p Play) (bool, error) {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT id FROM plays WHERE user_id=? AND bgg_play_id=?`, userID, *p.BGGPlayID).Scan(&existing)
	switch {
	case err == nil:
		_, err = tx.ExecContext(ctx, `UPDATE plays SET game_id=?,played_at=?,location=?,duration_minutes=?,quantity=?,notes=?,updated_at=? WHERE id=?`,
			p.GameID, p.PlayedAt, p.Location, nullableInt(p.Duration), p.Quantity, p.Notes, db.Now(), existing)
		if err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM play_players WHERE play_id=?`, existing); err != nil {
			return false, err
		}
		if err := insertPlayers(ctx, tx, existing, p.Players); err != nil {
			return false, err
		}
		return false, nil
	case errors.Is(err, sql.ErrNoRows):
		now := db.Now()
		if _, err := tx.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,location,duration_minutes,quantity,notes,origin,bgg_play_id,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,'bgg',?,?,?)`, p.ID, userID, p.GameID, p.PlayedAt, p.Location, nullableInt(p.Duration), p.Quantity, p.Notes, *p.BGGPlayID, now, now); err != nil {
			return false, err
		}
		if err := insertPlayers(ctx, tx, p.ID, p.Players); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, err
	}
}

// GameOption is a lightweight choice for play forms and filters.
type GameOption struct {
	ID   int64
	Name string
}

// ShelfGames lists every game on the user's shelf for the play form.
func (r *Repo) ShelfGames(ctx context.Context, userID string) ([]GameOption, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT g.id,g.name FROM user_games ug JOIN games g ON g.id=ug.game_id
		WHERE ug.user_id=? AND (ug.owned=1 OR ug.want_to_play=1 OR ug.preordered=1) ORDER BY g.sort_name,g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameOption
	for rows.Next() {
		var g GameOption
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// PlayedGames lists only games with logged plays, for timeline filters.
func (r *Repo) PlayedGames(ctx context.Context, userID string) ([]GameOption, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT DISTINCT g.id,g.name FROM plays p JOIN games g ON g.id=p.game_id
		WHERE p.user_id=? ORDER BY g.sort_name,g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameOption
	for rows.Next() {
		var g GameOption
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// PlayYears lists years in which the user logged plays, newest first.
func (r *Repo) PlayYears(ctx context.Context, userID string) ([]int, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT DISTINCT CAST(substr(played_at,1,4) AS INTEGER) FROM plays
		WHERE user_id=? ORDER BY 1 DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}
