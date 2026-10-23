// Package collection owns the shared game catalogue, each user's shelf state,
// shelf queries, statistics, and BoardGameGeek sync orchestration.
package collection

import "time"

type GameCard struct {
	ID              int64
	BGGID           int
	Name            string
	Year            int
	ThumbnailURL    string
	ImageURL        string
	MinPlayers      int
	MaxPlayers      int
	MinPlaytime     int
	MaxPlaytime     int
	MinAge          int
	Weight          float64
	BGGRating       float64
	BGGRank         int
	UserRating      *float64
	Owned           bool
	Wishlist        bool
	WantToPlay      bool
	Preordered      bool
	ForTrade        bool
	Favorite        bool
	BGGNumPlays     int
	LocalPlays      int
	TotalPlays      int
	AddedAt         string
	AcquisitionDate string
	IsExpansion     bool
	Notes           string
}

func (g GameCard) PlayLabel() string {
	n := g.TotalPlays
	if n == 1 {
		return "1 play"
	}
	return itoa(n) + " plays"
}

func (g GameCard) PlayerLabel() string {
	if g.MinPlayers == 0 && g.MaxPlayers == 0 {
		return ""
	}
	if g.MaxPlayers == 0 || g.MinPlayers == g.MaxPlayers {
		return itoa(g.MinPlayers) + " players"
	}
	return itoa(g.MinPlayers) + "–" + itoa(g.MaxPlayers)
}

func (g GameCard) TimeLabel() string {
	if g.MaxPlaytime == 0 {
		return ""
	}
	if g.MinPlaytime == 0 || g.MinPlaytime == g.MaxPlaytime {
		return itoa(g.MaxPlaytime) + " min"
	}
	return itoa(g.MinPlaytime) + "–" + itoa(g.MaxPlaytime) + " min"
}

func (g GameCard) WeightLabel() string {
	switch {
	case g.Weight <= 0:
		return ""
	case g.Weight < 1.8:
		return "Light"
	case g.Weight < 2.6:
		return "Medium-light"
	case g.Weight < 3.4:
		return "Medium"
	case g.Weight < 4.2:
		return "Medium-heavy"
	default:
		return "Heavy"
	}
}

func (g GameCard) RatingLabel() string {
	if g.UserRating == nil {
		return ""
	}
	return trimFloat(*g.UserRating)
}

func (g GameCard) CoverURL() string {
	if g.ThumbnailURL != "" {
		return "/media/bgg/" + itoa(g.BGGID) + ".jpg"
	}
	return "/media/bgg/" + itoa(g.BGGID) + ".jpg"
}

func (g GameCard) BGGURL() string {
	return "https://boardgamegeek.com/boardgame/" + itoa(g.BGGID)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func trimFloat(f float64) string {
	whole := int(f)
	half := int(f*2) % 2
	if half == 1 {
		return itoa(whole) + ".5"
	}
	return itoa(whole)
}

type GameDetail struct {
	GameCard
	Description string
	Categories  []string
	Mechanics   []string
	Designers   []string
	Publishers  []string
	Notes       string
	Location    string
	Condition   string
	PricePaid   *int
	RecentPlays []PlaySummary
}

type PlaySummary struct {
	ID       string
	PlayedAt string
	Location string
	Duration int
	Quantity int
	Notes    string
	Origin   string
	Players  []PlayPlayer
}

type PlayPlayer struct {
	Name     string
	Score    string
	IsWinner bool
}

type Filters struct {
	Query       string
	Players     int
	MaxPlaytime int
	MinWeight   float64
	MaxWeight   float64
	Category    string
	Mechanic    string
	Status      string
	Sort        string
	Page        int
	PerPage     int
	Offset      int
	Density     string
}

func (f Filters) HasFilters() bool {
	return f.Query != "" || f.Players > 0 || f.MaxPlaytime > 0 || f.MinWeight > 0 || f.MaxWeight > 0 || f.Category != "" || f.Mechanic != "" || f.Status != ""
}

func (f Filters) ActiveFilterCount() int {
	n := 0
	if f.Query != "" {
		n++
	}
	if f.Players > 0 {
		n++
	}
	if f.MaxPlaytime > 0 {
		n++
	}
	if f.MinWeight > 0 || f.MaxWeight > 0 {
		n++
	}
	if f.Category != "" {
		n++
	}
	if f.Mechanic != "" {
		n++
	}
	if f.Status != "" {
		n++
	}
	return n
}

type ShelfStats struct {
	Owned            int
	Unplayed         int
	PlaysThisYear    int
	Wishlist         int
	TotalPlays       int
	LongestUnplayed  *GameCard
	LongestWaitDays  int
	LongestWaitSince string
}

type Facets struct {
	Categories []string
	Mechanics  []string
}

type CollectionPatch struct {
	Owned           *bool
	Wishlist        *bool
	WantToPlay      *bool
	Preordered      *bool
	ForTrade        *bool
	Favorite        *bool
	UserRating      *float64
	ClearRating     bool
	Notes           *string
	AcquisitionDate *string
	PricePaid       *int
	ClearPrice      bool
	Location        *string
	Condition       *string
}

type MonthCount struct {
	Month int
	Label string
	Count int
}

type CountItem struct {
	Name  string
	Count int
}

type BandCount struct {
	Label string
	Count int
}

type LeaderEntry struct {
	Name  string
	Wins  int
	Plays int
}

type Statistics struct {
	Year          int
	Monthly       []MonthCount
	TotalPlays    int
	TotalSessions int
	TopGames      []CountItem
	Leaderboard   []LeaderEntry
	WeightBands   []BandCount
	Shelf         ShelfStats
}

func InLastYear(t time.Time) bool {
	return time.Since(t) < 365*24*time.Hour
}
