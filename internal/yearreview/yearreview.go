// Package yearreview builds a per-user annual recap from existing play, shelf
// and collection data, and stores the opt-in public share tokens for it.
//
// Everything here is read-only apart from the share store. Every query is
// parameterised and scoped to the requesting user; no figure is ever derived
// from another user's rows.
package yearreview

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"boardgames/internal/db"
)

// GameRef identifies a game well enough to render a card: the internal id, the
// display name, the BGG id and the proxied thumbnail.
type GameRef struct {
	ID           int64
	Name         string
	BGGID        int
	ThumbnailURL string
}

// Surprise is the game the user rated furthest above the median rating of the
// games they first played that year.
type Surprise struct {
	Game       GameRef
	UserRating float64
	Median     float64
	Delta      float64
}

// Nemesis is the opponent who spent the most plays on the winning side.
type Nemesis struct {
	Name     string // "" when player labels are excluded
	Wins     int    // times this player finished on the winning side
	Meetings int    // times they appeared in the user's plays that year
}

// ShelfProgress compares the unplayed-owned count at the end of the year with
// the same count one year earlier.
type ShelfProgress struct {
	UnplayedThen int
	UnplayedNow  int
	Delta        int
	UsedFallback bool // true when any acquisition_date was missing and added_at was used
}

// Recap is the assembled annual recap. Pointer fields are nil when the section
// has no honest answer, and the template omits them rather than inventing one.
type Recap struct {
	Year              int
	Plays             int // SUM(quantity)
	Sessions          int // COUNT(*) over plays
	DistinctGames     int
	Minutes           int // SUM(duration_minutes) over plays that have one
	PlaysWithDuration int
	MostPlayed        *GameRef
	MostPlayedPlays   int
	Heaviest          *GameRef
	HeaviestWeight    float64
	FirstPlays        []GameRef
	FirstPlayNames    []string // parallel to FirstPlays, for the template
	Surprise          *Surprise
	Nemesis           *Nemesis
	CostPerHourPence  *float64 // nil when hours or prices are missing
	Shelf             ShelfProgress
	Sparse            bool // no plays in the year
}

var (
	// ErrYearOutOfRange reports a year after the current one, or before the
	// user's first play. The web layer maps it to 404.
	ErrYearOutOfRange = errors.New("yearreview: year out of range")
	// ErrNotFound reports a share that does not exist for the caller.
	ErrNotFound = errors.New("yearreview: share not found")
)

// Location resolves a user's stored IANA timezone. An empty or unknown zone
// falls back to UTC, exactly as play validation does, so the year recap and the
// play log agree on which day "today" is.
func Location(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Compute builds the recap for year in UTC. Use ComputeAt to pin the timezone
// that decides the year boundary and "today".
func Compute(ctx context.Context, database *sql.DB, userID string, year int) (*Recap, error) {
	return ComputeAt(ctx, database, userID, year, time.UTC)
}

// ComputeAt builds the recap for year using loc for both the year boundaries
// and the current date. The boundaries are converted to the canonical UTC
// representation before they reach SQL: every stored played_at is either a date
// (YYYY-MM-DD) or a canonical UTC RFC3339 string, and string comparison is only
// sound when both sides carry the same offset. No SQLite date functions are
// involved, so timezone rules live in exactly one place: here.
func ComputeAt(ctx context.Context, database *sql.DB, userID string, year int, loc *time.Location) (*Recap, error) {
	if loc == nil {
		loc = time.UTC
	}
	if err := checkYear(ctx, database, userID, year, loc); err != nil {
		return nil, err
	}
	start := db.Timestamp(time.Date(year, 1, 1, 0, 0, 0, 0, loc))
	end := db.Timestamp(time.Date(year+1, 1, 1, 0, 0, 0, 0, loc))

	recap := &Recap{Year: year}
	if err := database.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(quantity),0),
		COUNT(*),
		COUNT(DISTINCT game_id),
		COALESCE(SUM(COALESCE(duration_minutes,0)),0),
		COALESCE(SUM(CASE WHEN duration_minutes IS NOT NULL THEN 1 ELSE 0 END),0)
		FROM plays WHERE user_id=? AND played_at>=? AND played_at<?`,
		userID, start, end).
		Scan(&recap.Plays, &recap.Sessions, &recap.DistinctGames, &recap.Minutes, &recap.PlaysWithDuration); err != nil {
		return nil, fmt.Errorf("year review totals: %w", err)
	}
	recap.Sparse = recap.Plays == 0

	mostPlayedRef, mostPlayedPlays, err := mostPlayed(ctx, database, userID, start, end)
	if err != nil {
		return nil, err
	}
	recap.MostPlayed = mostPlayedRef
	recap.MostPlayedPlays = mostPlayedPlays

	heaviestRef, heaviestWeight, err := heaviest(ctx, database, userID, start, end)
	if err != nil {
		return nil, err
	}
	recap.Heaviest = heaviestRef
	recap.HeaviestWeight = heaviestWeight

	firsts, err := firstPlays(ctx, database, userID, start, end)
	if err != nil {
		return nil, err
	}
	if len(firsts) > 0 {
		recap.FirstPlays = make([]GameRef, len(firsts))
		recap.FirstPlayNames = make([]string, len(firsts))
		for i, fp := range firsts {
			recap.FirstPlays[i] = fp.ref
			recap.FirstPlayNames[i] = fp.ref.Name
		}
	}

	if recap.Surprise, err = surprise(ctx, database, userID, firsts); err != nil {
		return nil, err
	}
	if recap.Nemesis, err = nemesis(ctx, database, userID, start, end); err != nil {
		return nil, err
	}
	if recap.CostPerHourPence, err = costPerHour(ctx, database, userID, start, end, recap.Minutes); err != nil {
		return nil, err
	}
	if recap.Shelf, err = shelfProgress(ctx, database, userID, start, end); err != nil {
		return nil, err
	}
	return recap, nil
}

// checkYear enforces the recap's year window. A year after the current one is
// always out of range. For a user with plays, the earliest play is the floor.
// A user with no plays at all can only view the current year, which renders the
// sparse variant.
func checkYear(ctx context.Context, database *sql.DB, userID string, year int, loc *time.Location) error {
	if year > time.Now().In(loc).Year() {
		return ErrYearOutOfRange
	}
	var earliest sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT MIN(played_at) FROM plays WHERE user_id=?`, userID).Scan(&earliest); err != nil {
		return fmt.Errorf("year review earliest play: %w", err)
	}
	if !earliest.Valid || strings.TrimSpace(earliest.String) == "" {
		if year < time.Now().In(loc).Year() {
			return ErrYearOutOfRange
		}
		return nil
	}
	if year < yearOf(earliest.String, loc) {
		return ErrYearOutOfRange
	}
	return nil
}

// yearOf reads the calendar year from a stored played_at, which is either a
// date (YYYY-MM-DD) or a canonical UTC RFC3339 timestamp.
func yearOf(value string, loc *time.Location) int {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.In(loc).Year()
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Year()
	}
	var year int
	if _, err := fmt.Sscanf(value[:min(len(value), 4)], "%4d", &year); err == nil {
		return year
	}
	return 0
}

// mostPlayed returns the game with the highest summed quantity in the window,
// breaking ties alphabetically.
func mostPlayed(ctx context.Context, database *sql.DB, userID, start, end string) (*GameRef, int, error) {
	var ref GameRef
	var plays int
	err := database.QueryRowContext(ctx, `SELECT g.id,g.name,g.bgg_id,g.thumbnail_url,SUM(p.quantity) AS qty
		FROM plays p JOIN games g ON g.id=p.game_id
		WHERE p.user_id=? AND p.played_at>=? AND p.played_at<?
		GROUP BY p.game_id
		ORDER BY qty DESC, g.name COLLATE NOCASE ASC
		LIMIT 1`, userID, start, end).
		Scan(&ref.ID, &ref.Name, &ref.BGGID, &ref.ThumbnailURL, &plays)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("year review most played: %w", err)
	}
	return &ref, plays, nil
}

// heaviest returns the heaviest game actually played in the window. Games with
// no recorded weight are skipped; when none remains the section is omitted.
func heaviest(ctx context.Context, database *sql.DB, userID, start, end string) (*GameRef, float64, error) {
	var ref GameRef
	var weight float64
	err := database.QueryRowContext(ctx, `SELECT g.id,g.name,g.bgg_id,g.thumbnail_url,g.weight
		FROM games g
		WHERE g.weight IS NOT NULL AND g.id IN (
			SELECT p.game_id FROM plays p WHERE p.user_id=? AND p.played_at>=? AND p.played_at<?
		)
		ORDER BY g.weight DESC, g.name COLLATE NOCASE ASC
		LIMIT 1`, userID, start, end).
		Scan(&ref.ID, &ref.Name, &ref.BGGID, &ref.ThumbnailURL, &weight)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("year review heaviest: %w", err)
	}
	return &ref, weight, nil
}

// firstPlay is a game first played inside the window, with the date of that
// first play.
type firstPlay struct {
	ref   GameRef
	first string
}

// firstPlays lists the games whose earliest play for this user falls inside the
// window, oldest first. It derives the first play from MIN(played_at) per game
// rather than trusting play_players.is_first_play: that column is only set when
// a user ticks the box or BGG marks the play new during a sync, so it is
// routinely absent on otherwise perfectly good first plays.
func firstPlays(ctx context.Context, database *sql.DB, userID, start, end string) ([]firstPlay, error) {
	rows, err := database.QueryContext(ctx, `SELECT g.id,g.name,g.bgg_id,g.thumbnail_url,MIN(p.played_at) AS first_played
		FROM plays p JOIN games g ON g.id=p.game_id
		WHERE p.user_id=?
		GROUP BY p.game_id
		HAVING MIN(p.played_at)>=? AND MIN(p.played_at)<?
		ORDER BY first_played ASC, g.name COLLATE NOCASE ASC`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("year review first plays: %w", err)
	}
	defer rows.Close()
	var out []firstPlay
	for rows.Next() {
		var fp firstPlay
		if err := rows.Scan(&fp.ref.ID, &fp.ref.Name, &fp.ref.BGGID, &fp.ref.ThumbnailURL, &fp.first); err != nil {
			return nil, fmt.Errorf("year review first plays: %w", err)
		}
		out = append(out, fp)
	}
	return out, rows.Err()
}

// surprise picks the first-played game whose user rating sits furthest above
// the median rating of the rated first plays. Three rated first plays is the
// floor: a superlative over two data points is an invention, not a finding.
func surprise(ctx context.Context, database *sql.DB, userID string, firsts []firstPlay) (*Surprise, error) {
	if len(firsts) < 3 {
		return nil, nil
	}
	ids := make([]any, 0, len(firsts)+1)
	ids = append(ids, userID)
	marks := make([]string, len(firsts))
	for i, fp := range firsts {
		marks[i] = "?"
		ids = append(ids, fp.ref.ID)
	}
	rows, err := database.QueryContext(ctx, `SELECT game_id,user_rating FROM user_games
		WHERE user_id=? AND user_rating IS NOT NULL AND game_id IN (`+strings.Join(marks, ",")+`)`, ids...)
	if err != nil {
		return nil, fmt.Errorf("year review surprise: %w", err)
	}
	defer rows.Close()
	ratings := make(map[int64]float64, len(firsts))
	values := make([]float64, 0, len(firsts))
	for rows.Next() {
		var id int64
		var rating float64
		if err := rows.Scan(&id, &rating); err != nil {
			return nil, fmt.Errorf("year review surprise: %w", err)
		}
		ratings[id] = rating
		values = append(values, rating)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("year review surprise: %w", err)
	}
	if len(values) < 3 {
		return nil, nil
	}
	sort.Float64s(values)
	median := values[len(values)/2]
	if len(values)%2 == 0 {
		median = (values[len(values)/2-1] + values[len(values)/2]) / 2
	}
	var best *Surprise
	for _, fp := range firsts {
		rating, ok := ratings[fp.ref.ID]
		if !ok {
			continue
		}
		delta := rating - median
		if best == nil || delta > best.Delta+1e-9 {
			best = &Surprise{Game: fp.ref, UserRating: rating, Median: median, Delta: delta}
		}
	}
	return best, nil
}

// nemesis finds the opponent who most often finished on the winning side.
//
// The plays table records no account holder, so "the user lost" cannot be read
// directly. This uses the honest approximation the schema supports: a player
// beats the user when they were on the winning side (is_winner=1) of one of the
// user's plays that also has at least one recorded loser. Plays with no winner
// are ignored, as are plays without a loser (solo and cooperative sessions),
// because a win with nobody to beat says nothing about a rivalry.
func nemesis(ctx context.Context, database *sql.DB, userID, start, end string) (*Nemesis, error) {
	var n Nemesis
	err := database.QueryRowContext(ctx, `SELECT MIN(pp.name) AS name,
			COUNT(DISTINCT p.id) AS meetings,
			SUM(CASE WHEN pp.is_winner=1 AND EXISTS (
				SELECT 1 FROM play_players l WHERE l.play_id=p.id AND l.is_winner=0
			) THEN 1 ELSE 0 END) AS wins
		FROM play_players pp
		JOIN plays p ON p.id=pp.play_id
		WHERE p.user_id=? AND p.played_at>=? AND p.played_at<?
		GROUP BY lower(pp.name)
		HAVING wins>0
		ORDER BY wins DESC, meetings DESC, name COLLATE NOCASE ASC
		LIMIT 1`, userID, start, end).
		Scan(&n.Name, &n.Meetings, &n.Wins)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("year review nemesis: %w", err)
	}
	return &n, nil
}

// costPerHour is the total price paid for the distinct games played that year,
// divided by the hours at the table.
//
// The denominator is every recorded minute in the year, including time spent on
// unpriced games: that is the same "hours at the table" figure the recap shows,
// so the ratio means what the heading claims. Dividing only by hours logged in
// priced games would divide by a number the reader cannot see, and would make
// the figure jump when an unowned game is played.
func costPerHour(ctx context.Context, database *sql.DB, userID, start, end string, minutes int) (*float64, error) {
	if minutes <= 0 {
		return nil, nil
	}
	var total, priced int
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(SUM(ug.price_paid),0), COUNT(*)
		FROM user_games ug
		WHERE ug.user_id=? AND ug.price_paid IS NOT NULL AND ug.price_paid>0
		  AND ug.game_id IN (
			SELECT p.game_id FROM plays p WHERE p.user_id=? AND p.played_at>=? AND p.played_at<?
		  )`, userID, userID, start, end).Scan(&total, &priced); err != nil {
		return nil, fmt.Errorf("year review cost per hour: %w", err)
	}
	if priced == 0 || total <= 0 {
		return nil, nil
	}
	value := float64(total) / (float64(minutes) / 60)
	return &value, nil
}

// shelfProgress compares the unplayed-owned count at the end of the year with
// the count one year earlier. A cut-off is the exclusive upper bound of the
// period: "on or before the last instant of the year" is "< start of next year".
func shelfProgress(ctx context.Context, database *sql.DB, userID, start, end string) (ShelfProgress, error) {
	var progress ShelfProgress
	thenCount, thenFallback, err := unplayedAsOf(ctx, database, userID, start)
	if err != nil {
		return progress, err
	}
	nowCount, nowFallback, err := unplayedAsOf(ctx, database, userID, end)
	if err != nil {
		return progress, err
	}
	progress.UnplayedThen = thenCount
	progress.UnplayedNow = nowCount
	progress.Delta = nowCount - thenCount
	progress.UsedFallback = thenFallback || nowFallback
	return progress, nil
}

// unplayedAsOf counts the user's owned games that had entered the shelf by the
// cut-off and had no play by then. Acquisition date is authoritative; when it is
// missing the shelf's added_at is the only evidence available, and the caller
// reports that fallback rather than pretending precision.
func unplayedAsOf(ctx context.Context, database *sql.DB, userID, cutoff string) (int, bool, error) {
	var unplayed, fallback int
	if err := database.QueryRowContext(ctx, `SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN COALESCE(ug.acquisition_date,'')='' THEN 1 ELSE 0 END),0)
		FROM user_games ug
		WHERE ug.user_id=? AND ug.owned=1
		  AND COALESCE(NULLIF(ug.acquisition_date,''), ug.added_at) < ?
		  AND NOT EXISTS (
			SELECT 1 FROM plays p
			WHERE p.user_id=ug.user_id AND p.game_id=ug.game_id AND p.played_at<?
		  )`, userID, cutoff, cutoff).Scan(&unplayed, &fallback); err != nil {
		return 0, false, fmt.Errorf("year review shelf: %w", err)
	}
	return unplayed, fallback > 0, nil
}

// Share is one public recap link. Revocation is a row update, so links die
// without rotating any secret.
type Share struct {
	ID             int64
	UserID         string
	Year           int
	Token          string
	IncludePlayers bool
	CreatedAt      string
	RevokedAt      string
}

// Store persists share tokens.
type Store struct {
	database *sql.DB
}

// NewStore returns a share store over database.
func NewStore(database *sql.DB) *Store { return &Store{database: database} }

const shareColumns = `id,user_id,year,token,include_players,created_at,revoked_at`

// Create mints a share for the user and year. Several shares for the same year
// may coexist: a user can publish one, revoke it, and publish another, and the
// old row is kept for the audit trail.
func (s *Store) Create(ctx context.Context, userID string, year int, includePlayers bool) (*Share, error) {
	now := db.Now()
	for attempt := 0; attempt < 5; attempt++ {
		token, err := newToken()
		if err != nil {
			return nil, err
		}
		res, err := s.database.ExecContext(ctx, `INSERT INTO year_review_shares(user_id,year,token,include_players,created_at,revoked_at)
			VALUES(?,?,?,?,?,'')`, userID, year, token, boolInt(includePlayers), now)
		if err == nil {
			id, _ := res.LastInsertId()
			return &Share{
				ID:             id,
				UserID:         userID,
				Year:           year,
				Token:          token,
				IncludePlayers: includePlayers,
				CreatedAt:      now,
			}, nil
		}
		// The only expected failure is the unique token index. Anything else,
		// or a collision that is somehow already gone, is a real error.
		var taken int
		if qerr := s.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM year_review_shares WHERE token=?`, token).Scan(&taken); qerr != nil {
			return nil, fmt.Errorf("year review create share: %w", err)
		}
		if taken == 0 {
			return nil, fmt.Errorf("year review create share: %w", err)
		}
	}
	return nil, errors.New("year review: could not allocate a unique share token")
}

// List returns the caller's shares, newest first, revoked ones included so the
// settings page can show history.
func (s *Store) List(ctx context.Context, userID string) ([]Share, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+shareColumns+` FROM year_review_shares
		WHERE user_id=? ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("year review list shares: %w", err)
	}
	defer rows.Close()
	var out []Share
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *share)
	}
	return out, rows.Err()
}

// Revoke kills a share owned by the caller. Revoking an already revoked share is
// a no-op; a row belonging to someone else is reported as ErrNotFound, never as
// a permission error, so the store does not confirm that the id exists.
func (s *Store) Revoke(ctx context.Context, userID string, id int64) error {
	res, err := s.database.ExecContext(ctx, `UPDATE year_review_shares SET revoked_at=?
		WHERE id=? AND user_id=? AND revoked_at=''`, db.Now(), id, userID)
	if err != nil {
		return fmt.Errorf("year review revoke share: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	var exists int
	if err := s.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM year_review_shares WHERE id=? AND user_id=?`, id, userID).Scan(&exists); err != nil {
		return fmt.Errorf("year review revoke share: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	return nil
}

// ByToken resolves a live share. Unknown and revoked tokens both return nil,
// nil. SQL filters on revoked_at; the token is then re-checked in constant time
// in Go, so a match never depends on how much of the value the database
// compared before answering.
func (s *Store) ByToken(ctx context.Context, token string) (*Share, error) {
	if token == "" {
		return nil, nil
	}
	row := s.database.QueryRowContext(ctx, `SELECT `+shareColumns+` FROM year_review_shares
		WHERE token=? AND revoked_at=''`, token)
	share, err := scanShare(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(share.Token), []byte(token)) != 1 {
		return nil, nil
	}
	return share, nil
}

// ActiveFor returns the newest live share for the user and year, or nil when
// there is none.
func (s *Store) ActiveFor(ctx context.Context, userID string, year int) (*Share, error) {
	row := s.database.QueryRowContext(ctx, `SELECT `+shareColumns+` FROM year_review_shares
		WHERE user_id=? AND year=? AND revoked_at='' ORDER BY created_at DESC, id DESC LIMIT 1`, userID, year)
	share, err := scanShare(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return share, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanShare(src scanner) (*Share, error) {
	var share Share
	var includePlayers int
	if err := src.Scan(&share.ID, &share.UserID, &share.Year, &share.Token, &includePlayers, &share.CreatedAt, &share.RevokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("year review share scan: %w", err)
	}
	share.IncludePlayers = includePlayers == 1
	return &share, nil
}

// newToken returns a 128-bit token, URL-safe and without padding: 22 characters.
func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("year review share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
