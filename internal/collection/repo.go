package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	// Cost metrics are derived from a separate batched query so the
	// hand-maintained cardColumns/scanCard pair stays untouched.
	if err := r.applyCostMetrics(ctx, userID, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// playAgg is the recorded-play aggregate for one game.
type playAgg struct {
	Plays   int
	Minutes int
}

// valueMetrics returns the recorded play count and summed duration per game in
// one user-scoped query. The denominator is the recorded plays only: BGG's
// bgg_num_plays is deliberately not added, because synced plays already exist
// as rows with origin='bgg' and adding the counter would double-count them.
func (r *Repo) valueMetrics(ctx context.Context, userID string, gameIDs []int64) (map[int64]playAgg, error) {
	out := make(map[int64]playAgg, len(gameIDs))
	if len(gameIDs) == 0 {
		return out, nil
	}
	placeholders, args := idList(userID, gameIDs)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT game_id, COALESCE(SUM(quantity),0), COALESCE(SUM(COALESCE(duration_minutes,0)),0)
		 FROM plays WHERE user_id=? AND game_id IN (`+placeholders+`) GROUP BY game_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("value metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var agg playAgg
		if err := rows.Scan(&id, &agg.Plays, &agg.Minutes); err != nil {
			return nil, fmt.Errorf("value metrics: %w", err)
		}
		out[id] = agg
	}
	return out, rows.Err()
}

// pricesFor returns price_paid per game for the user's shelf rows. It is a
// separate small batched query so cardColumns/scanCard stay untouched.
func (r *Repo) pricesFor(ctx context.Context, userID string, gameIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(gameIDs))
	if len(gameIDs) == 0 {
		return out, nil
	}
	placeholders, args := idList(userID, gameIDs)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT game_id, price_paid FROM user_games WHERE user_id=? AND price_paid IS NOT NULL AND game_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("price metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var price int
		if err := rows.Scan(&id, &price); err != nil {
			return nil, fmt.Errorf("price metrics: %w", err)
		}
		out[id] = price
	}
	return out, rows.Err()
}

// idList builds the placeholder list for "IN (...)" and the matching arguments.
func idList(userID string, gameIDs []int64) (string, []any) {
	placeholders := make([]string, len(gameIDs))
	args := make([]any, 0, len(gameIDs)+1)
	args = append(args, userID)
	for i, id := range gameIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	return strings.Join(placeholders, ","), args
}

// applyCostMetrics fills PricePaid and the derived cost fields on cards after
// they have been scanned. Cost is undefined (nil) rather than zero when a
// denominator is missing, and when no price was recorded.
func (r *Repo) applyCostMetrics(ctx context.Context, userID string, cards []GameCard) error {
	if len(cards) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	aggs, err := r.valueMetrics(ctx, userID, ids)
	if err != nil {
		return err
	}
	prices, err := r.pricesFor(ctx, userID, ids)
	if err != nil {
		return err
	}
	for i := range cards {
		if price, ok := prices[cards[i].ID]; ok {
			p := price
			cards[i].PricePaid = &p
		}
		applyCost(&cards[i], aggs[cards[i].ID])
	}
	return nil
}

// applyCost sets the derived cost fields from an already-populated price.
func applyCost(g *GameCard, agg playAgg) {
	if g.PricePaid == nil || *g.PricePaid <= 0 {
		return
	}
	price := float64(*g.PricePaid)
	if agg.Plays > 0 {
		v := price / float64(agg.Plays)
		g.CostPerPlayPence = &v
	}
	if agg.Minutes > 0 {
		v := price / (float64(agg.Minutes) / 60)
		g.CostPerHourPence = &v
	}
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
	case "cost_play":
		expr := "CASE WHEN ug.price_paid IS NULL OR ug.price_paid<=0 THEN NULL ELSE 1.0*ug.price_paid/NULLIF((SELECT COALESCE(SUM(p.quantity),0) FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id),0) END"
		return "(" + expr + ") IS NULL, (" + expr + ") ASC, g.name COLLATE NOCASE ASC"
	case "cost_hour":
		// 1.0*price*60/minutes is the same ordering as the per-hour metric
		// (price / (minutes/60)); SUM ignores NULL durations.
		expr := "CASE WHEN ug.price_paid IS NULL OR ug.price_paid<=0 THEN NULL ELSE 1.0*ug.price_paid*60/NULLIF((SELECT SUM(p.duration_minutes) FROM plays p WHERE p.game_id=g.id AND p.user_id=ug.user_id),0) END"
		return "(" + expr + ") IS NULL, (" + expr + ") ASC, g.name COLLATE NOCASE ASC"
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
	var location, condition, description, setupNotes string
	if err := r.DB.QueryRowContext(ctx, `SELECT g.description,ug.location,ug.condition,ug.price_paid,ug.setup_notes FROM games g JOIN user_games ug ON ug.game_id=g.id WHERE g.id=? AND ug.user_id=?`, gameID, userID).
		Scan(&description, &location, &condition, &price, &setupNotes); err != nil {
		return nil, err
	}
	d.Description = description
	d.Location = location
	d.Condition = condition
	d.SetupNotes = setupNotes
	d.Notes = card.Notes
	if price.Valid {
		p := int(price.Int64)
		d.PricePaid = &p
	}
	// Keep the embedded card's value fields in step with the detail fields and
	// derive cost per play/hour from the recorded plays.
	d.GameCard.PricePaid = d.PricePaid
	aggs, err := r.valueMetrics(ctx, userID, []int64{gameID})
	if err != nil {
		return nil, err
	}
	applyCost(&d.GameCard, aggs[gameID])
	d.SetupImages, _ = r.setupImages(ctx, userID, gameID)
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
	// Value totals. "Unplayed" mirrors the shelf's own definition: owned, no
	// recorded plays and no BGG play count.
	if err := r.DB.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(price_paid),0),
		COALESCE(SUM(CASE WHEN price_paid IS NOT NULL THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN price_paid IS NOT NULL AND bgg_num_plays=0 AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.game_id=ug.game_id AND p.user_id=ug.user_id) THEN price_paid ELSE 0 END),0)
		FROM user_games ug WHERE ug.user_id=? AND ug.owned=1`, userID).
		Scan(&s.TotalPaidPence, &s.PaidGames, &s.UnplayedCapitalPence); err != nil {
		return s, err
	}
	median, err := r.medianCostPerPlay(ctx, userID)
	if err != nil {
		return s, err
	}
	s.MedianCostPerPlayPence = median
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

// medianCostPerPlay returns the median price per recorded play across owned
// games that have both a usable price and at least one recorded play. The two
// middle values are averaged for an even count.
func (r *Repo) medianCostPerPlay(ctx context.Context, userID string) (*float64, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT ug.price_paid,
		COALESCE((SELECT SUM(p.quantity) FROM plays p WHERE p.game_id=ug.game_id AND p.user_id=ug.user_id),0)
		FROM user_games ug
		WHERE ug.user_id=? AND ug.owned=1 AND ug.price_paid IS NOT NULL AND ug.price_paid>0`, userID)
	if err != nil {
		return nil, fmt.Errorf("median cost per play: %w", err)
	}
	defer rows.Close()
	var costs []float64
	for rows.Next() {
		var price, plays int
		if err := rows.Scan(&price, &plays); err != nil {
			return nil, fmt.Errorf("median cost per play: %w", err)
		}
		if plays == 0 {
			continue
		}
		costs = append(costs, float64(price)/float64(plays))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(costs) == 0 {
		return nil, nil
	}
	sort.Float64s(costs)
	var median float64
	if n := len(costs); n%2 == 1 {
		median = costs[n/2]
	} else {
		median = (costs[n/2-1] + costs[n/2]) / 2
	}
	return &median, nil
}

// pipelineBuckets fixes the presentation order and labels of the pipeline.
var pipelineBuckets = []struct{ state, label string }{
	{"wishlist", "Wishlist"},
	{"want_to_play", "Want to play"},
	{"preordered", "Preordered"},
	{"for_trade", "For trade"},
	{"unplayed", "Owned, never played"},
}

// Pipeline returns one bucket per acquisition state by reusing List, so
// filtering, counting and ownership rules stay in one place. A game flagged in
// several states appears in each matching bucket.
func (r *Repo) Pipeline(ctx context.Context, userID string, perBucket int, f Filters) ([]PipelineBucket, error) {
	if perBucket <= 0 {
		perBucket = 24
	}
	out := make([]PipelineBucket, 0, len(pipelineBuckets))
	for _, b := range pipelineBuckets {
		filters := f
		filters.Status = b.state
		filters.Page = 1
		filters.Offset = 0
		filters.PerPage = perBucket
		games, total, err := r.List(ctx, userID, filters)
		if err != nil {
			return nil, err
		}
		// The trade bucket is a ranked list, not a plain alphabetical one.
		// Count stays the pre-cap total from List; only the visible order
		// changes.
		if b.state == "for_trade" && len(games) > 1 {
			ranked := RankTradeCandidatesAt(games, time.Now())
			ordered := make([]GameCard, len(ranked))
			for i, c := range ranked {
				ordered[i] = c.Game
			}
			games = ordered
		}
		out = append(out, PipelineBucket{State: b.state, Label: b.label, Count: total, Games: games})
	}
	return out, nil
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
	if p.SetupNotes != nil {
		v := strings.TrimSpace(*p.SetupNotes)
		if len([]rune(v)) > 5000 {
			return errors.New("setup guide must be 5000 characters or fewer")
		}
		sets = append(sets, "setup_notes=?")
		args = append(args, v)
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
