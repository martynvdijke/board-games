package plays

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
}
