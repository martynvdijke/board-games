package plays

import (
	"context"
	"database/sql"
	"math"
)

// BoxRange is a game's advertised playtime in minutes. Either bound may be
// zero when BGG does not publish it.
type BoxRange struct {
	Min int
	Max int
}

// Valid reports whether the box advertises any playtime at all.
func (b BoxRange) Valid() bool { return b.Min > 0 || b.Max > 0 }

// Midpoint is the comparison value for the box: the midpoint when both bounds
// are present, the single bound when only one is, and zero when neither is.
func (b BoxRange) Midpoint() float64 {
	switch {
	case b.Min > 0 && b.Max > 0:
		return float64(b.Min+b.Max) / 2
	case b.Min > 0:
		return float64(b.Min)
	case b.Max > 0:
		return float64(b.Max)
	default:
		return 0
	}
}

// PlaytimeVerdict classifies an average logged duration against a box range.
type PlaytimeVerdict string

const (
	VerdictWithin PlaytimeVerdict = "within"
	VerdictLong   PlaytimeVerdict = "long"
	VerdictShort  PlaytimeVerdict = "short"
)

// MinTimedPlays is how many timed plays one game needs before its average is
// shown. Below it a single long session would read as a trend.
const MinTimedPlays = 3

// MinGamesForGroup is how many qualified games the library-wide sentence needs
// before it is worth stating.
const MinGamesForGroup = 2

// Classify compares an average logged duration with the box range. A missing
// bound never implies a verdict on that side: with only a minimum a game can
// run short but not long, and with only a maximum it can run long but not
// short. An invalid range is always "within", because there is nothing to
// compare against.
func Classify(average float64, box BoxRange) PlaytimeVerdict {
	switch {
	case !box.Valid():
		return VerdictWithin
	case box.Max > 0 && average > float64(box.Max):
		return VerdictLong
	case box.Min > 0 && average < float64(box.Min):
		return VerdictShort
	default:
		return VerdictWithin
	}
}

// DurationTotal sums the positive durations and counts them. Zero and negative
// values mean "not recorded" and are skipped.
func DurationTotal(durations []int) (total, count int) {
	for _, d := range durations {
		if d > 0 {
			total += d
			count++
		}
	}
	return total, count
}

// AverageDuration averages the positive durations and reports how many timed
// plays contributed. It returns (0, 0) when no play carries a duration.
func AverageDuration(durations []int) (float64, int) {
	total, n := DurationTotal(durations)
	if n == 0 {
		return 0, 0
	}
	return float64(total) / float64(n), n
}

// DurationSample is one game's timed plays and its box range.
type DurationSample struct {
	Durations []int
	Box       BoxRange
}

// GameCalibration is the per-game reading.
type GameCalibration struct {
	Plays   int
	Average int // rounded to the nearest minute
	Box     BoxRange
	Verdict PlaytimeVerdict
}

// CalibrateGame reads one game's timed plays against its box. It reports
// false when the box advertises no playtime or when fewer than minPlays plays
// carry a duration.
func CalibrateGame(durations []int, box BoxRange, minPlays int) (GameCalibration, bool) {
	if !box.Valid() {
		return GameCalibration{}, false
	}
	avg, n := AverageDuration(durations)
	if n == 0 || n < minPlays {
		return GameCalibration{}, false
	}
	return GameCalibration{
		Plays:   n,
		Average: int(math.Round(avg)),
		Box:     box,
		Verdict: Classify(avg, box),
	}, true
}

// GroupCalibration is the library-wide overrun.
type GroupCalibration struct {
	Games   int
	Plays   int
	Logged  int     // total logged minutes across qualifying games
	Box     int     // total box minutes: midpoint × timed plays, per game
	Percent float64 // (Logged-Box)/Box × 100; positive means over the box
}

// CalibrateGroup weights each game by its number of timed plays: the box
// minutes for a game are its midpoint multiplied by its timed plays, so an
// often-played game contributes proportionally rather than equally. Games
// below minPlays or without a box range are skipped, and fewer than minGames
// qualifying games means there is no sentence to state.
func CalibrateGroup(samples []DurationSample, minPlays, minGames int) (GroupCalibration, bool) {
	var g GroupCalibration
	for _, s := range samples {
		total, n := DurationTotal(s.Durations)
		if n < minPlays || !s.Box.Valid() {
			continue
		}
		g.Games++
		g.Plays += n
		g.Logged += total
		g.Box += int(math.Round(s.Box.Midpoint() * float64(n)))
	}
	if g.Box <= 0 || g.Games < minGames {
		return GroupCalibration{}, false
	}
	g.Percent = float64(g.Logged-g.Box) / float64(g.Box) * 100
	return g, true
}

// GameDurations returns the positive logged durations for one user's plays of
// one game. It never reads another user's rows.
func (r *Repo) GameDurations(ctx context.Context, userID string, gameID int64) ([]int, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT duration_minutes FROM plays
		WHERE user_id=? AND game_id=? AND duration_minutes IS NOT NULL AND duration_minutes>0
		ORDER BY played_at`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var d int
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CalibrationSamples returns one sample per game on the user's shelf with its
// box range and the user's timed play durations. A game with no timed plays
// still appears, with an empty slice, so callers can decide what to hide.
func (r *Repo) CalibrationSamples(ctx context.Context, userID string) ([]DurationSample, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT g.id, coalesce(g.min_playtime,0), coalesce(g.max_playtime,0), p.duration_minutes
		FROM user_games ug
		JOIN games g ON g.id=ug.game_id
		LEFT JOIN plays p ON p.game_id=ug.game_id AND p.user_id=ug.user_id AND p.duration_minutes>0
		WHERE ug.user_id=?
		ORDER BY g.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DurationSample
	index := map[int64]int{}
	for rows.Next() {
		var gameID int64
		var box BoxRange
		var duration sql.NullInt64
		if err := rows.Scan(&gameID, &box.Min, &box.Max, &duration); err != nil {
			return nil, err
		}
		at, ok := index[gameID]
		if !ok {
			at = len(out)
			index[gameID] = at
			out = append(out, DurationSample{Box: box})
		}
		if duration.Valid {
			out[at].Durations = append(out[at].Durations, int(duration.Int64))
		}
	}
	return out, rows.Err()
}
