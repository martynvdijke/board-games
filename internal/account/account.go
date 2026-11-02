// Package account owns whole-account operations: exporting the data the
// instance holds about a user and closing the account. Both the HTML settings
// page and the JSON API call this service so their behaviour cannot drift.
package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"boardgames/internal/db"
)

var (
	// ErrLastAdmin means the account is the last admin and deleting it would
	// leave the instance without anyone able to administer it.
	ErrLastAdmin = errors.New("the last active admin cannot be deleted")
	// ErrNotFound means there is no account with the requested ID.
	ErrNotFound = errors.New("account not found")
	// ErrInvalid means the request was malformed before it reached the store.
	ErrInvalid = errors.New("invalid account request")
)

// Service performs whole-account operations. It deliberately uses database/sql
// directly because deletion spans every table that references a user.
type Service struct {
	DB *sql.DB
}

func New(database *sql.DB) *Service { return &Service{DB: database} }

// Delete removes the account and every row that belongs to it in one
// transaction. Plays are owned by one account and are not shared with other
// users, so they are deleted with their player-name rows rather than detached.
// The last-admin guard is part of the final DELETE, which keeps the invariant
// correct when two admins try to leave at the same time.
func (s *Service) Delete(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Revoke credentials and delete personal rows first. If the guard below
	// refuses the deletion the transaction rolls back, restoring all of them.
	for _, stmt := range []string{
		`DELETE FROM sessions WHERE user_id = ?`,
		`DELETE FROM api_tokens WHERE user_id = ?`,
		`DELETE FROM user_settings WHERE user_id = ?`,
		`DELETE FROM user_games WHERE user_id = ?`,
		`DELETE FROM play_players WHERE play_id IN (SELECT id FROM plays WHERE user_id = ?)`,
		`DELETE FROM plays WHERE user_id = ?`,
		`DELETE FROM sync_runs WHERE user_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, userID); err != nil {
			return err
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND (role <> 'admin' OR
		(SELECT COUNT(*) FROM users WHERE role = 'admin') > 1)`, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var role string
		err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, userID).Scan(&role)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if role == "admin" {
			return ErrLastAdmin
		}
		return ErrNotFound
	}
	return tx.Commit()
}

type exportProfile struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

type exportSettings struct {
	BGGUsername       string `json:"bgg_username"`
	BGGVerifiedAt     string `json:"bgg_username_verified_at,omitempty"`
	BGGSyncEnabled    bool   `json:"bgg_sync_enabled"`
	BGGSyncInterval   int    `json:"bgg_sync_interval_seconds"`
	Theme             string `json:"theme"`
	Timezone          string `json:"timezone"`
	SettingsUpdatedAt string `json:"updated_at"`
}

type exportShelf struct {
	GameID      int64    `json:"game_id"`
	Game        string   `json:"game"`
	Owned       bool     `json:"owned"`
	Wishlist    bool     `json:"wishlist"`
	WantToPlay  bool     `json:"want_to_play"`
	Preordered  bool     `json:"preordered"`
	ForTrade    bool     `json:"for_trade"`
	Favorite    bool     `json:"favorite"`
	UserRating  *float64 `json:"user_rating"`
	Notes       string   `json:"notes"`
	Location    string   `json:"location"`
	Condition   string   `json:"condition"`
	Acquisition string   `json:"acquisition_date,omitempty"`
	PricePaid   *int64   `json:"price_paid_cents"`
	BGGNumPlays int      `json:"bgg_num_plays"`
	AddedAt     string   `json:"added_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type exportPlayer struct {
	Name     string `json:"name"`
	Score    string `json:"score"`
	IsWinner bool   `json:"is_winner"`
	IsFirst  bool   `json:"is_first_play"`
}

type exportPlay struct {
	ID              string         `json:"id"`
	GameID          int64          `json:"game_id"`
	Game            string         `json:"game"`
	PlayedAt        string         `json:"played_at"`
	Location        string         `json:"location"`
	DurationMinutes int            `json:"duration_minutes"`
	Quantity        int            `json:"quantity"`
	Notes           string         `json:"notes"`
	Origin          string         `json:"origin"`
	BGGPlayID       *int64         `json:"bgg_play_id,omitempty"`
	Players         []exportPlayer `json:"players"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type exportToken struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
	RevokedAt  string   `json:"revoked_at,omitempty"`
}

type Export struct {
	ExportedAt string          `json:"exported_at"`
	Profile    exportProfile   `json:"profile"`
	Settings   *exportSettings `json:"settings,omitempty"`
	Shelf      []exportShelf   `json:"shelf"`
	Plays      []exportPlay    `json:"plays"`
	Tokens     []exportToken   `json:"tokens"`
}

// Export returns the user's own data as pretty-printed JSON. API token hashes
// are never included; only the metadata needed to understand the token list.
func (s *Service) Export(ctx context.Context, userID string) ([]byte, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalid
	}
	out := Export{ExportedAt: db.Now(), Shelf: []exportShelf{}, Plays: []exportPlay{}, Tokens: []exportToken{}}
	err := s.DB.QueryRowContext(ctx, `SELECT id,email,name,avatar_url,role,created_at,updated_at,coalesce(last_login_at,'')
		FROM users WHERE id = ?`, userID).
		Scan(&out.Profile.ID, &out.Profile.Email, &out.Profile.Name, &out.Profile.AvatarURL, &out.Profile.Role,
			&out.Profile.CreatedAt, &out.Profile.UpdatedAt, &out.Profile.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var st exportSettings
	var syncEnabled int
	err = s.DB.QueryRowContext(ctx, `SELECT bgg_username,coalesce(bgg_username_verified_at,''),bgg_sync_enabled,
		bgg_sync_interval_seconds,theme,timezone,updated_at FROM user_settings WHERE user_id = ?`, userID).
		Scan(&st.BGGUsername, &st.BGGVerifiedAt, &syncEnabled, &st.BGGSyncInterval, &st.Theme, &st.Timezone, &st.SettingsUpdatedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No settings row is normal for an account that never opened Settings.
	case err != nil:
		return nil, err
	default:
		st.BGGSyncEnabled = syncEnabled == 1
		out.Settings = &st
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT ug.game_id,g.name,ug.owned,ug.wishlist,ug.want_to_play,ug.preordered,
		ug.for_trade,ug.favorite,ug.user_rating,ug.notes,ug.location,ug.condition,coalesce(ug.acquisition_date,''),
		ug.price_paid,ug.bgg_num_plays,ug.added_at,ug.updated_at
		FROM user_games ug JOIN games g ON g.id = ug.game_id WHERE ug.user_id = ? ORDER BY g.sort_name`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e exportShelf
		var owned, wishlist, want, preordered, trade, favorite int
		if err := rows.Scan(&e.GameID, &e.Game, &owned, &wishlist, &want, &preordered, &trade, &favorite,
			&e.UserRating, &e.Notes, &e.Location, &e.Condition, &e.Acquisition, &e.PricePaid, &e.BGGNumPlays,
			&e.AddedAt, &e.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		e.Owned = owned == 1
		e.Wishlist = wishlist == 1
		e.WantToPlay = want == 1
		e.Preordered = preordered == 1
		e.ForTrade = trade == 1
		e.Favorite = favorite == 1
		out.Shelf = append(out.Shelf, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.DB.QueryContext(ctx, `SELECT p.id,p.game_id,g.name,p.played_at,p.location,coalesce(p.duration_minutes,0),
		p.quantity,p.notes,p.origin,p.bgg_play_id,p.created_at,p.updated_at
		FROM plays p JOIN games g ON g.id = p.game_id WHERE p.user_id = ? ORDER BY p.played_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	plays := []exportPlay{}
	ids := []string{}
	for rows.Next() {
		var p exportPlay
		var duration, bggPlayID sql.NullInt64
		if err := rows.Scan(&p.ID, &p.GameID, &p.Game, &p.PlayedAt, &p.Location, &duration, &p.Quantity,
			&p.Notes, &p.Origin, &bggPlayID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		p.DurationMinutes = int(duration.Int64)
		if bggPlayID.Valid {
			v := bggPlayID.Int64
			p.BGGPlayID = &v
		}
		p.Players = []exportPlayer{}
		plays = append(plays, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	players := map[string][]exportPlayer{}
	if len(ids) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		prows, err := s.DB.QueryContext(ctx, `SELECT play_id,name,score,is_winner,is_first_play FROM play_players
			WHERE play_id IN (`+placeholders+`) ORDER BY play_id,position,id`, args...)
		if err != nil {
			return nil, err
		}
		for prows.Next() {
			var playID string
			var p exportPlayer
			var winner, first int
			if err := prows.Scan(&playID, &p.Name, &p.Score, &winner, &first); err != nil {
				prows.Close()
				return nil, err
			}
			p.IsWinner = winner == 1
			p.IsFirst = first == 1
			players[playID] = append(players[playID], p)
		}
		if err := prows.Err(); err != nil {
			prows.Close()
			return nil, err
		}
		prows.Close()
	}
	for i := range plays {
		if list, ok := players[plays[i].ID]; ok {
			plays[i].Players = list
		}
	}
	out.Plays = plays

	rows, err = s.DB.QueryContext(ctx, `SELECT id,name,prefix,scopes,created_at,coalesce(last_used_at,''),coalesce(revoked_at,'')
		FROM api_tokens WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t exportToken
		var scopes string
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &scopes, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		t.Scopes = strings.Fields(scopes)
		out.Tokens = append(out.Tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(out, "", "  ")
}

// RemovePlayerName removes or anonymises every play_players row matching name.
// A non-admin may only touch rows belonging to their own plays, so the route
// cannot be used to reveal or alter another account's play log. It returns the
// number of rows changed.
func (s *Service) RemovePlayerName(ctx context.Context, actorID string, admin bool, name string, anonymise bool) (int64, error) {
	actorID = strings.TrimSpace(actorID)
	name = strings.TrimSpace(name)
	if actorID == "" || name == "" {
		return 0, ErrInvalid
	}
	if len(name) > 200 {
		return 0, fmt.Errorf("%w: player name is too long", ErrInvalid)
	}
	scope := ""
	args := []any{}
	if !admin {
		scope = ` AND play_id IN (SELECT id FROM plays WHERE user_id = ?)`
		args = append(args, actorID)
	}
	var res sql.Result
	var err error
	if anonymise {
		args = append([]any{"Removed player", name}, args...)
		res, err = s.DB.ExecContext(ctx, `UPDATE play_players SET name = ? WHERE name = ? COLLATE NOCASE`+scope, args...)
	} else {
		args = append([]any{name}, args...)
		res, err = s.DB.ExecContext(ctx, `DELETE FROM play_players WHERE name = ? COLLATE NOCASE`+scope, args...)
	}
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
