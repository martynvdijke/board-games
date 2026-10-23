package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"boardgames/internal/db"
)

var ErrNotFound = errors.New("game not found on this shelf")

type Repo struct{ DB *sql.DB }

func NewRepo(database *sql.DB) *Repo { return &Repo{DB: database} }

const cardColumns = `g.id,g.bgg_id,g.name,coalesce(g.year_published,0),g.thumbnail_url,g.image_url,
	coalesce(g.min_players,0),coalesce(g.max_players,0),coalesce(g.min_playtime,0),coalesce(g.max_playtime,0),
	coalesce(g.min_age,0),coalesce(g.weight,0),coalesce(g.bgg_rating,0),coalesce(g.bgg_rank,0),
	ug.user_rating,ug.owned,ug.wishlist,ug.want_to_play,ug.preordered,ug.for_trade,ug.favorite,
	ug.bgg_num_plays,ug.added_at,coalesce(ug.acquisition_date,''),g.is_expansion,ug.notes,
	(SELECT COALESCE(SUM(p.quantity),0) FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id) AS local_plays`

const cardFrom = ` FROM games g JOIN user_games ug ON ug.game_id=g.id`

func scanCard(s scanner) (GameCard, error) {
	var g GameCard
	var rating sql.NullFloat64
	var owned, wishlist, want, preorder, trade, favorite, expansion int
	err := s.Scan(&g.ID, &g.BGGID, &g.Name, &g.Year, &g.ThumbnailURL, &g.ImageURL,
		&g.MinPlayers, &g.MaxPlayers, &g.MinPlaytime, &g.MaxPlaytime,
		&g.MinAge, &g.Weight, &g.BGGRating, &g.BGGRank,
		&rating, &owned, &wishlist, &want, &preorder, &trade, &favorite,
		&g.BGGNumPlays, &g.AddedAt, &g.AcquisitionDate, &expansion, &g.Notes, &g.LocalPlays)
	if err != nil {
		return g, err
	}
	if rating.Valid {
		v := rating.Float64
		g.UserRating = &v
	}
	g.Owned = owned == 1
	g.Wishlist = wishlist == 1
	g.WantToPlay = want == 1
	g.Preordered = preorder == 1
	g.ForTrade = trade == 1
	g.Favorite = favorite == 1
	g.IsExpansion = expansion == 1
	g.TotalPlays = g.BGGNumPlays + g.LocalPlays
	return g, nil
}

type scanner interface{ Scan(dest ...any) error }

// List returns a filtered page of the user's shelf plus the total match count.
func (r *Repo) List(ctx context.Context, userID string, f Filters) ([]GameCard, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 48
	}
	if f.PerPage > 96 {
		f.PerPage = 96
	}
	var clauses []string
	var args []any
	clauses = append(clauses, "ug.user_id = ?")
	args = append(args, userID)

	if f.Query != "" {
		if q := FTSQuery(f.Query); q != "" {
			clauses = append(clauses, "g.id IN (SELECT rowid FROM games_fts WHERE games_fts MATCH ?)")
			args = append(args, q)
		}
	}
	if f.Players > 0 {
		clauses = append(clauses, "g.min_players <= ? AND g.max_players >= ?")
		args = append(args, f.Players, f.Players)
	}
	if f.MaxPlaytime > 0 {
		clauses = append(clauses, "g.max_playtime > 0 AND g.max_playtime <= ?")
		args = append(args, f.MaxPlaytime)
	}
	if f.MinWeight > 0 {
		clauses = append(clauses, "g.weight >= ?")
		args = append(args, f.MinWeight)
	}
	if f.MaxWeight > 0 {
		clauses = append(clauses, "g.weight <= ?")
		args = append(args, f.MaxWeight)
	}
	if f.Category != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM game_categories gc JOIN categories c ON c.id=gc.category_id WHERE gc.game_id=g.id AND c.name = ? COLLATE NOCASE)`)
		args = append(args, f.Category)
	}
	if f.Mechanic != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM game_mechanics gm JOIN mechanics m ON m.id=gm.mechanic_id WHERE gm.game_id=g.id AND m.name = ? COLLATE NOCASE)`)
		args = append(args, f.Mechanic)
	}
	switch f.Status {
	case "owned":
		clauses = append(clauses, "ug.owned=1")
	case "wishlist":
		clauses = append(clauses, "ug.wishlist=1")
	case "want_to_play":
		clauses = append(clauses, "ug.want_to_play=1")
	case "preordered":
		clauses = append(clauses, "ug.preordered=1")
	case "for_trade":
		clauses = append(clauses, "ug.for_trade=1")
	case "favorite":
		clauses = append(clauses, "ug.favorite=1")
	case "unplayed":
		clauses = append(clauses, "ug.owned=1 AND g.is_expansion=0 AND ug.bgg_num_plays=0 AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id)")
	}
	where := " WHERE " + strings.Join(clauses, " AND ")

	var total int
	if err := r.DB.QueryRowContext(ctx, "SELECT COUNT(*)"+cardFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count shelf: %w", err)
	}

	order := sortClause(f.Sort)
	perPage := f.PerPage
	if perPage <= 0 {
		perPage = 48
	}
	if perPage > 96 {
		perPage = 96
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	offset += (page - 1) * perPage
	rows, err := r.DB.QueryContext(ctx, "SELECT "+cardColumns+cardFrom+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", append(args, perPage, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list shelf: %w", err)
	}
	defer rows.Close()
	var out []GameCard
	for rows.Next() {
		g, err := scanCard(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

func sortClause(sort string) string {
	switch sort {
	case "year":
		return "g.year_published DESC NULLS LAST, g.name COLLATE NOCASE"
	case "rating":
		return "ug.user_rating DESC NULLS LAST, g.name COLLATE NOCASE"
	case "bgg_rating":
		return "g.bgg_rating DESC NULLS LAST, g.name COLLATE NOCASE"
	case "weight":
		return "CASE WHEN g.weight IS NULL THEN 1 ELSE 0 END, g.weight ASC, g.name COLLATE NOCASE"
	case "weight_desc":
		return "CASE WHEN g.weight IS NULL THEN 1 ELSE 0 END, g.weight DESC, g.name COLLATE NOCASE"
	case "added":
		return "ug.added_at DESC, g.name COLLATE NOCASE"
	case "plays":
		return "(ug.bgg_num_plays + (SELECT COALESCE(SUM(p.quantity),0) FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id)) DESC, g.name COLLATE NOCASE"
	default:
		return "g.name COLLATE NOCASE ASC"
	}
}

// FTSQuery sanitises user input into an FTS5 prefix query. Only letters and
// digits survive, so no FTS operators can be injected.
func FTSQuery(q string) string {
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, b.String()+"*")
			b.Reset()
		}
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if len([]rune(b.String())) < 40 {
				b.WriteRune(r)
			}
		} else {
			flush()
		}
	}
	flush()
	if len(tokens) > 8 {
		tokens = tokens[:8]
	}
	return strings.Join(tokens, " AND ")
}

func (r *Repo) Get(ctx context.Context, userID string, gameID int64) (*GameDetail, error) {
	row := r.DB.QueryRowContext(ctx, "SELECT "+cardColumns+cardFrom+" WHERE g.id=? AND ug.user_id=?", gameID, userID)
	card, err := scanCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := &GameDetail{GameCard: card}
	var price sql.NullInt64
	var location, condition, description string
	if err := r.DB.QueryRowContext(ctx, `SELECT g.description,ug.location,ug.condition,ug.price_paid FROM games g JOIN user_games ug ON ug.game_id=g.id WHERE g.id=? AND ug.user_id=?`, gameID, userID).
		Scan(&description, &location, &condition, &price); err != nil {
		return nil, err
	}
	d.Description = description
	d.Location = location
	d.Condition = condition
	d.Notes = card.Notes
	if price.Valid {
		p := int(price.Int64)
		d.PricePaid = &p
	}
	d.Categories = r.namesFor(ctx, "game_categories", "category_id", "categories", gameID)
	d.Mechanics = r.namesFor(ctx, "game_mechanics", "mechanic_id", "mechanics", gameID)
	d.Designers = r.namesFor(ctx, "game_designers", "designer_id", "designers", gameID)
	d.Publishers = r.namesFor(ctx, "game_publishers", "publisher_id", "publishers", gameID)
	d.RecentPlays, _ = r.recentPlays(ctx, userID, gameID, 5)
	return d, nil
}

func (r *Repo) namesFor(ctx context.Context, junction, col, table string, gameID int64) []string {
	q := fmt.Sprintf(`SELECT n.name FROM %s j JOIN %s n ON n.id=j.%s WHERE j.game_id=? ORDER BY n.name COLLATE NOCASE`, junction, table, col)
	rows, err := r.DB.QueryContext(ctx, q, gameID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return out
}

func (r *Repo) recentPlays(ctx context.Context, userID string, gameID int64, limit int) ([]PlaySummary, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,played_at,location,coalesce(duration_minutes,0),quantity,notes,origin FROM plays WHERE user_id=? AND game_id=? ORDER BY played_at DESC, created_at DESC LIMIT ?`, userID, gameID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlaySummary
	for rows.Next() {
		var p PlaySummary
		if err := rows.Scan(&p.ID, &p.PlayedAt, &p.Location, &p.Duration, &p.Quantity, &p.Notes, &p.Origin); err != nil {
			return nil, err
		}
		p.Players = r.playersFor(ctx, p.ID)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) playersFor(ctx context.Context, playID string) []PlayPlayer {
	rows, err := r.DB.QueryContext(ctx, `SELECT name,score,is_winner FROM play_players WHERE play_id=? ORDER BY position,name`, playID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PlayPlayer
	for rows.Next() {
		var p PlayPlayer
		var win int
		if rows.Scan(&p.Name, &p.Score, &win) == nil {
			p.IsWinner = win == 1
			out = append(out, p)
		}
	}
	return out
}

func (r *Repo) Facets(ctx context.Context, userID string) (Facets, error) {
	var f Facets
	f.Categories = r.facet(ctx, `SELECT DISTINCT c.name FROM categories c JOIN game_categories gc ON gc.category_id=c.id JOIN user_games ug ON ug.game_id=gc.game_id WHERE ug.user_id=? ORDER BY c.name COLLATE NOCASE`, userID)
	f.Mechanics = r.facet(ctx, `SELECT DISTINCT m.name FROM mechanics m JOIN game_mechanics gm ON gm.mechanic_id=m.id JOIN user_games ug ON ug.game_id=gm.game_id WHERE ug.user_id=? ORDER BY m.name COLLATE NOCASE`, userID)
	return f, nil
}

func (r *Repo) facet(ctx context.Context, query, userID string) []string {
	rows, err := r.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return out
}

// Stats computes the overview cards, including the longest-waiting unplayed game.
func (r *Repo) Stats(ctx context.Context, userID string) (ShelfStats, error) {
	var s ShelfStats
	year := time.Now().Format("2006")
	err := r.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM user_games WHERE user_id=? AND owned=1),
		(SELECT COUNT(*) FROM user_games ug JOIN games g ON g.id=ug.game_id WHERE ug.user_id=? AND ug.owned=1 AND g.is_expansion=0 AND ug.bgg_num_plays=0 AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.game_id=ug.game_id AND p.user_id=ug.user_id)),
		(SELECT COALESCE(SUM(quantity),0) FROM plays WHERE user_id=? AND substr(played_at,1,4)=?),
		(SELECT COUNT(*) FROM user_games WHERE user_id=? AND wishlist=1),
		(SELECT COALESCE(SUM(quantity),0) FROM plays WHERE user_id=?)`,
		userID, userID, userID, year, userID, userID).
		Scan(&s.Owned, &s.Unplayed, &s.PlaysThisYear, &s.Wishlist, &s.TotalPlays)
	if err != nil {
		return s, err
	}
	row := r.DB.QueryRowContext(ctx, "SELECT "+cardColumns+cardFrom+` WHERE ug.user_id=? AND ug.owned=1 AND g.is_expansion=0 AND ug.bgg_num_plays=0
		AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id)
		ORDER BY COALESCE(NULLIF(ug.acquisition_date,''), ug.added_at) ASC LIMIT 1`, userID)
	g, err := scanCard(row)
	if err == nil {
		s.LongestUnplayed = &g
		since := g.AcquisitionDate
		if since == "" {
			since = g.AddedAt
		}
		s.LongestWaitSince = since
		if t, ok := parseDate(since); ok {
			days := int(time.Since(t).Hours() / 24)
			if days < 0 {
				days = 0
			}
			s.LongestWaitDays = days
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	return s, nil
}

func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Apply updates only the supplied collection fields.
func (r *Repo) Apply(ctx context.Context, userID string, gameID int64, p CollectionPatch) error {
	if p.UserRating != nil && p.ClearRating {
		return errors.New("cannot set and clear a rating at once")
	}
	if p.UserRating != nil {
		v := *p.UserRating
		if v < 1 || v > 10 || v*2 != float64(int(v*2)) {
			return errors.New("rating must be between 1 and 10 in half-point steps")
		}
	}
	var sets []string
	var args []any
	boolField := func(name string, v *bool) {
		if v != nil {
			sets = append(sets, name+"=?")
			if *v {
				args = append(args, 1)
			} else {
				args = append(args, 0)
			}
		}
	}
	boolField("owned", p.Owned)
	boolField("wishlist", p.Wishlist)
	boolField("want_to_play", p.WantToPlay)
	boolField("preordered", p.Preordered)
	boolField("for_trade", p.ForTrade)
	boolField("favorite", p.Favorite)
	if p.UserRating != nil {
		sets = append(sets, "user_rating=?")
		args = append(args, *p.UserRating)
	}
	if p.ClearRating {
		sets = append(sets, "user_rating=NULL")
	}
	if p.Notes != nil {
		sets = append(sets, "notes=?")
		args = append(args, strings.TrimSpace(*p.Notes))
	}
	if p.AcquisitionDate != nil {
		v := strings.TrimSpace(*p.AcquisitionDate)
		if v != "" {
			if _, ok := parseDate(v); !ok {
				return errors.New("acquisition date must be YYYY-MM-DD")
			}
			sets = append(sets, "acquisition_date=?")
			args = append(args, v)
		} else {
			sets = append(sets, "acquisition_date=NULL")
		}
	}
	if p.PricePaid != nil && p.ClearPrice {
		return errors.New("cannot set and clear a price at once")
	}
	if p.PricePaid != nil {
		if *p.PricePaid < 0 {
			return errors.New("price cannot be negative")
		}
		sets = append(sets, "price_paid=?")
		args = append(args, *p.PricePaid)
	}
	if p.ClearPrice {
		sets = append(sets, "price_paid=NULL")
	}
	if p.Location != nil {
		sets = append(sets, "location=?")
		args = append(args, strings.TrimSpace(*p.Location))
	}
	if p.Condition != nil {
		sets = append(sets, "condition=?")
		args = append(args, strings.TrimSpace(*p.Condition))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at=?")
	args = append(args, db.Now(), userID, gameID)
	res, err := r.DB.ExecContext(ctx, "UPDATE user_games SET "+strings.Join(sets, ",")+" WHERE user_id=? AND game_id=?", args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RecommendationCandidates returns the owned, non-expansion shelf for the
// daily pick algorithm.
func (r *Repo) RecommendationCandidates(ctx context.Context, userID string) ([]GameCard, error) {
	rows, err := r.DB.QueryContext(ctx, "SELECT "+cardColumns+cardFrom+" WHERE ug.user_id=? AND ug.owned=1 AND g.is_expansion=0 ORDER BY g.name", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameCard
	for rows.Next() {
		g, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repo) PlayCount(ctx context.Context, userID string, gameID int64) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity),0) FROM plays WHERE user_id=? AND game_id=?`, userID, gameID).Scan(&n)
	return n, err
}
