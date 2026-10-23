// Package trmnl builds the e-ink feed, deterministic daily recommendation,
// and proxied cover images.
package trmnl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/plays"
)

type Service struct {
	DB     *sql.DB
	Games  *collection.Repo
	Plays  *plays.Repo
	Config *config.Config
}

func NewService(database *sql.DB, games *collection.Repo, playRepo *plays.Repo, cfg *config.Config) *Service {
	return &Service{DB: database, Games: games, Plays: playRepo, Config: cfg}
}

type Stats struct {
	Owned         int `json:"owned"`
	Unplayed      int `json:"unplayed"`
	PlaysThisYear int `json:"plays_this_year"`
	Wishlist      int `json:"wishlist"`
	TotalPlays    int `json:"total_plays"`
}

type Pick struct {
	GameID   int64  `json:"game_id"`
	BGGID    int    `json:"bgg_id"`
	Name     string `json:"name"`
	Year     int    `json:"year,omitempty"`
	Players  string `json:"players,omitempty"`
	Playtime string `json:"playtime,omitempty"`
	Weight   string `json:"weight,omitempty"`
	Rating   string `json:"rating,omitempty"`
	Reason   string `json:"reason"`
	CoverURL string `json:"cover_url"`
	URL      string `json:"url"`
	Unplayed bool   `json:"unplayed"`
}

type Recent struct {
	Name     string `json:"name"`
	PlayedAt string `json:"played_at"`
	Players  int    `json:"players,omitempty"`
	URL      string `json:"url"`
}

type ShelfOfShame struct {
	Count           int    `json:"count"`
	LongestWaiting  string `json:"longest_waiting,omitempty"`
	LongestWaitDays int    `json:"longest_wait_days,omitempty"`
}

type Payload struct {
	GeneratedAt  time.Time    `json:"generated_at"`
	Date         string       `json:"date"`
	Stats        Stats        `json:"stats"`
	Pick         *Pick        `json:"pick"`
	Recent       []Recent     `json:"recent"`
	ShelfOfShame ShelfOfShame `json:"shelf_of_shame"`
}

func (s *Service) userLocation(ctx context.Context, userID string) *time.Location {
	var tz string
	if err := s.DB.QueryRowContext(ctx, `SELECT timezone FROM user_settings WHERE user_id=?`, userID).Scan(&tz); err != nil || tz == "" {
		tz = s.Config.Timezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (s *Service) Build(ctx context.Context, userID string) (*Payload, error) {
	stats, err := s.Games.Stats(ctx, userID)
	if err != nil {
		return nil, err
	}
	loc := s.userLocation(ctx, userID)
	now := time.Now().In(loc)
	p := &Payload{
		GeneratedAt:  time.Now().UTC(),
		Date:         now.Format("2006-01-02"),
		Stats:        Stats{Owned: stats.Owned, Unplayed: stats.Unplayed, PlaysThisYear: stats.PlaysThisYear, Wishlist: stats.Wishlist, TotalPlays: stats.TotalPlays},
		Recent:       []Recent{},
		ShelfOfShame: ShelfOfShame{Count: stats.Unplayed},
	}
	if stats.LongestUnplayed != nil {
		p.ShelfOfShame.LongestWaiting = stats.LongestUnplayed.Name
		p.ShelfOfShame.LongestWaitDays = stats.LongestWaitDays
	}
	candidates, err := s.Games.RecommendationCandidates(ctx, userID)
	if err != nil {
		return nil, err
	}
	best := Recommend(userID, p.Date, candidates)
	if best != nil {
		g := *best
		p.Pick = &Pick{
			GameID: g.ID, BGGID: g.BGGID, Name: g.Name, Year: g.Year,
			Players: g.PlayerLabel(), Playtime: g.TimeLabel(), Weight: g.WeightLabel(), Rating: g.RatingLabel(),
			Reason: reason(g), CoverURL: s.CoverURL(g.BGGID), URL: s.Config.BaseURL + "/games/" + itoa(g.ID),
			Unplayed: g.TotalPlays == 0,
		}
	}
	recent, err := s.Plays.Recent(ctx, userID, 5)
	if err == nil {
		for _, pl := range recent {
			p.Recent = append(p.Recent, Recent{Name: pl.GameName, PlayedAt: pl.PlayedAt, Players: len(pl.Players), URL: s.Config.BaseURL + "/games/" + itoa(pl.GameID)})
		}
	}
	return p, nil
}

// Recommend chooses a stable game for a user and calendar day. Weighting
// favours unplayed games, short playtimes, and high personal ratings.
func Recommend(userID, date string, candidates []collection.GameCard) *collection.GameCard {
	if len(candidates) == 0 {
		return nil
	}
	var best *collection.GameCard
	bestScore := -1.0
	for i := range candidates {
		g := candidates[i]
		score := 0.0
		if g.TotalPlays == 0 {
			score += 55
		}
		if g.MaxPlaytime > 0 && g.MaxPlaytime <= 30 {
			score += 22
		} else if g.MaxPlaytime > 0 && g.MaxPlaytime <= 60 {
			score += 12
		}
		if g.UserRating != nil {
			score += (*g.UserRating) * 2.2
		} else {
			score += g.BGGRating * 1.4
		}
		// A stable per-day shuffle keeps ties from always favouring the same shelf slot.
		h := fnv.New32a()
		_, _ = h.Write([]byte(userID + "|" + date + "|" + itoa(g.ID)))
		score += float64(h.Sum32()%1000) / 40.0
		if score > bestScore {
			bestScore = score
			copy := g
			best = &copy
		}
	}
	return best
}

func reason(g collection.GameCard) string {
	switch {
	case g.TotalPlays == 0 && g.MaxPlaytime > 0 && g.MaxPlaytime <= 45:
		return "Unplayed and short enough for a weeknight."
	case g.TotalPlays == 0:
		return "It has been waiting on your shelf — give it a first play."
	case g.UserRating != nil && *g.UserRating >= 8:
		return "One of your highest-rated games."
	case g.MaxPlaytime > 0 && g.MaxPlaytime <= 30:
		return "A quick game that fits a spare half hour."
	default:
		return "A dependable pick from your shelf."
	}
}

func (s *Service) CoverURL(bggID int) string {
	return s.Config.BaseURL + "/media/bgg/" + itoa(bggID) + ".jpg"
}

func itoa[T ~int | ~int64](n T) string {
	return fmt.Sprintf("%d", n)
}

// ErrNoCover indicates that a game has no image URL and should use a placeholder.
var ErrNoCover = errors.New("no cover available")
