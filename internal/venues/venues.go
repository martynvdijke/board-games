// Package venues resolves free-text play locations into a per-user normalised
// venue table. Resolution happens on write, inside the caller's transaction, so
// the same pattern can later be reused for player profiles.
package venues

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"boardgames/internal/db"
)

// Location is a normalised venue together with its play count.
type Location struct {
	ID    int64
	Name  string
	Plays int
}

// VenueStat aggregates a venue's plays for the current user.
type VenueStat struct {
	LocationID    int64
	Name          string
	Plays         int
	DistinctGames int
	LastPlayedAt  string
	// Wins counts plays at this venue won by the current user. Player
	// identity does not exist yet, so it stays 0; once player profiles
	// land, join play_players to the user's profile here.
	// TODO(player-profiles): populate Wins from the user's own winner rows.
	Wins int
	// DecidedPlays counts plays at this venue that produced any winner.
	DecidedPlays int
}

// Repo provides venue resolution and aggregates over *sql.DB.
type Repo struct {
	DB *sql.DB
}

func NewRepo(database *sql.DB) *Repo { return &Repo{DB: database} }

// Normalize folds a raw location into a stable lookup key.
//
// Rules, applied in order:
//   - trim leading and trailing whitespace;
//   - lowercase (Unicode-aware);
//   - fold apostrophe variants (’ ‘ ` ´ ʼ) to the ASCII apostrophe;
//   - replace "&" with " and ";
//   - collapse every run of internal whitespace to a single space;
//   - strip trailing periods.
//
// Articles ("the", "le", "la", "de") are deliberately kept: dropping them
// would merge genuinely distinct venues. Normalize is pure and idempotent.
func Normalize(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = strings.NewReplacer(
		"\u2019", "'", // right single quote
		"\u2018", "'", // left single quote
		"\u02bc", "'", // modifier letter apostrophe
		"`", "'",
		"\u00b4", "'", // acute accent
	).Replace(s)
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, ".")
	return s
}

// ResolveTx returns the venue id for raw, creating it on first use. It must be
// called inside an existing transaction; it never opens or nests one. The
// display name keeps the caller's capitalisation, while lookup keys on
// Normalize, and an existing row's name is never overwritten (first spelling
// wins). An empty raw value yields (0, "", nil), meaning "no venue".
func (r *Repo) ResolveTx(ctx context.Context, tx *sql.Tx, userID, raw string) (int64, string, error) {
	display := strings.TrimSpace(raw)
	normalized := Normalize(raw)
	if display == "" || normalized == "" {
		return 0, "", nil
	}

	var id int64
	var name string
	err := tx.QueryRowContext(ctx,
		`SELECT id, name FROM locations WHERE user_id = ? AND normalized_name = ?`,
		userID, normalized).Scan(&id, &name)
	if err == nil {
		return id, name, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, "", err
	}

	now := db.Now()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO locations(user_id, name, normalized_name, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, normalized_name) DO NOTHING`,
		userID, display, normalized, now, now)
	if err != nil {
		return 0, "", err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, "", err
	}
	if affected == 0 {
		// Another writer inserted the same venue concurrently; reuse it and
		// keep its stored spelling.
		if err := tx.QueryRowContext(ctx,
			`SELECT id, name FROM locations WHERE user_id = ? AND normalized_name = ?`,
			userID, normalized).Scan(&id, &name); err != nil {
			return 0, "", err
		}
		return id, name, nil
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	return id, display, nil
}

// Stats aggregates the user's plays per venue. Plays with an empty or zero
// timestamp are excluded. Results are ordered by play count, then name.
func (r *Repo) Stats(ctx context.Context, userID string) ([]VenueStat, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT l.id, l.name,
		        COUNT(DISTINCT p.id) AS plays,
		        COUNT(DISTINCT p.game_id) AS games,
		        MAX(p.played_at) AS last_played,
		        COUNT(DISTINCT CASE WHEN pp.id IS NOT NULL THEN p.id END) AS decided
		 FROM locations l
		 JOIN plays p ON p.location_id = l.id AND p.user_id = l.user_id
		 LEFT JOIN play_players pp ON pp.play_id = p.id AND pp.is_winner = 1
		 WHERE l.user_id = ?
		   AND p.played_at <> ''
		   AND p.played_at NOT LIKE '0001-01-01%'
		 GROUP BY l.id, l.name
		 ORDER BY plays DESC, l.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []VenueStat{}
	for rows.Next() {
		var s VenueStat
		if err := rows.Scan(&s.LocationID, &s.Name, &s.Plays, &s.DistinctGames, &s.LastPlayedAt, &s.DecidedPlays); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// List returns the user's venues with play counts for pickers and
// autocomplete, ordered by play count then name.
func (r *Repo) List(ctx context.Context, userID string) ([]Location, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT l.id, l.name, COUNT(p.id) AS plays
		 FROM locations l
		 LEFT JOIN plays p ON p.location_id = l.id AND p.user_id = l.user_id
		 WHERE l.user_id = ?
		 GROUP BY l.id, l.name
		 ORDER BY plays DESC, l.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := []Location{}
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Plays); err != nil {
			return nil, err
		}
		locations = append(locations, l)
	}
	return locations, rows.Err()
}

// HasUnlinked reports whether the user has legacy plays with free-text
// locations that Backfill could link. It is a single cheap query so callers
// can guard a backfill without paying for a transaction.
func (r *Repo) HasUnlinked(ctx context.Context, userID string) (bool, error) {
	var exists int
	err := r.DB.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM plays
			WHERE user_id = ? AND location_id IS NULL AND TRIM(location) <> ''
		)`, userID).Scan(&exists)
	return exists == 1, err
}

// Backfill resolves legacy free-text plays that have no location_id yet. Each
// distinct raw value is resolved once per run and applied in a single
// transaction. It is idempotent and returns the number of plays linked.
func (r *Repo) Backfill(ctx context.Context, userID string) (int, error) {
	linked := 0
	err := db.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT location FROM plays
			 WHERE user_id = ? AND location_id IS NULL AND TRIM(location) <> ''
			 GROUP BY location
			 ORDER BY MIN(rowid)`,
			userID)
		if err != nil {
			return err
		}
		var raws []string
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			raws = append(raws, raw)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		for _, raw := range raws {
			id, _, err := r.ResolveTx(ctx, tx, userID, raw)
			if err != nil {
				return err
			}
			if id == 0 {
				continue
			}
			res, err := tx.ExecContext(ctx,
				`UPDATE plays SET location_id = ?
				 WHERE user_id = ? AND location_id IS NULL AND location = ?`,
				id, userID, raw)
			if err != nil {
				return err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return err
			}
			linked += int(affected)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return linked, nil
}
