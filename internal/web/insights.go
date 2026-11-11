package web

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"boardgames/internal/collection"
	"boardgames/internal/plays"
	"boardgames/internal/venues"
)

// GamePageData is the game detail view model: the existing GameData plus the
// two insight lines. It embeds *GameData so the template keeps reading
// .Data.Game, .Data.Loan and the lend form unchanged.
type GamePageData struct {
	*GameData
	Playtime  *PlaytimeInsight
	Reconcile *ReconcileInsight
}

// StatsPageData is the stats view model with the library-wide insight lines.
type StatsPageData struct {
	*StatsData
	Playtime  *GroupPlaytimeInsight
	Reconcile *ReconcileSummaryInsight
	Venues    []venues.VenueStat
	VenueLead string
}

// PlaytimeInsight is the per-game line comparing logged durations with the
// box. Line is the whole sentence, built here so the template stays logic-free.
type PlaytimeInsight struct {
	Average int
	Box     string
	Phrase  string
	Line    string
}

// ReconcileInsight is the per-game BGG play-count gap.
type ReconcileInsight struct {
	BGG     int
	Logged  int
	Missing int
	LogURL  string
	Line    string
}

// GroupPlaytimeInsight is the library-wide overrun sentence.
type GroupPlaytimeInsight struct {
	Games   int
	Plays   int
	Percent int // rounded absolute percentage
	Line    string
}

// ReconcileSummaryInsight is the library-wide catch-up sentence.
type ReconcileSummaryInsight struct {
	Games   int
	Missing int
	Line    string
}

// gamePageData loads a game with its loan state and both insight lines.
func (h *Handler) gamePageData(ctx context.Context, userID string, game *collection.GameDetail) *GamePageData {
	return h.gamePageDataWith(ctx, userID, h.gameData(ctx, userID, game))
}

// gamePageDataWith adds the insight lines to already-loaded game data, so a
// handler that re-renders the page (such as a refused lend) keeps them.
func (h *Handler) gamePageDataWith(ctx context.Context, userID string, data *GameData) *GamePageData {
	out := &GamePageData{GameData: data}
	if h.Plays == nil || data == nil || data.Game == nil {
		return out
	}
	game := data.Game
	if durations, err := h.Plays.GameDurations(ctx, userID, game.ID); err != nil {
		slog.Warn("load game durations", "game", game.ID, "error", err)
	} else if cal, ok := plays.CalibrateGame(durations, boxRange(game), plays.MinTimedPlays); ok {
		out.Playtime = playtimeInsight(cal)
	}
	if logged, err := h.Plays.LoggedPlayCount(ctx, userID, game.ID); err != nil {
		slog.Warn("count logged plays", "game", game.ID, "error", err)
	} else if gap := (plays.PlayCountGap{BGG: game.BGGNumPlays, Logged: logged}); gap.NeedsCatchUp() {
		out.Reconcile = reconcileInsight(game.ID, gap)
	}
	return out
}

// statsPageData adds the library-wide insight lines to the stats model.
func (h *Handler) statsPageData(ctx context.Context, userID string, data *StatsData) *StatsPageData {
	out := &StatsPageData{StatsData: data}
	if h.Plays == nil {
		return out
	}
	if samples, err := h.Plays.CalibrationSamples(ctx, userID); err != nil {
		slog.Warn("load calibration samples", "error", err)
	} else if group, ok := plays.CalibrateGroup(samples, plays.MinTimedPlays, plays.MinGamesForGroup); ok {
		out.Playtime = groupPlaytimeInsight(group)
	}
	if gaps, err := h.Plays.ReconcileGaps(ctx, userID); err != nil {
		slog.Warn("load play-count gaps", "error", err)
	} else if summary, ok := plays.SummariseReconcile(gaps); ok {
		out.Reconcile = reconcileSummaryInsight(summary)
	}
	if h.Venues != nil {
		if list, err := h.Venues.Stats(ctx, userID); err != nil {
			slog.Warn("load venue stats", "error", err)
		} else {
			out.Venues = list
			out.VenueLead = venueLead(list)
		}
	}
	return out
}

// backfillVenues links legacy free-text play locations on first visit. The
// guard costs one cheap query; a backfill transaction runs only when there is
// something to link. Names are user-visible on their own stats page, so only
// the count is logged.
func (h *Handler) backfillVenues(ctx context.Context, userID string) {
	if h.Venues == nil {
		return
	}
	unlinked, err := h.Venues.HasUnlinked(ctx, userID)
	if err != nil {
		slog.Warn("check unlinked play locations", "error", err)
		return
	}
	if !unlinked {
		return
	}
	linked, err := h.Venues.Backfill(ctx, userID)
	if err != nil {
		slog.Warn("backfill play locations", "error", err)
		return
	}
	if linked > 0 {
		slog.Info("linked legacy play locations", "plays", linked)
	}
}

// venueLead summarises the busiest venue for the stats page.
func venueLead(stats []venues.VenueStat) string {
	if len(stats) == 0 || stats[0].Plays == 0 {
		return ""
	}
	top := stats[0]
	playsWord, gamesWord := "plays", "games"
	if top.Plays == 1 {
		playsWord = "play"
	}
	if top.DistinctGames == 1 {
		gamesWord = "game"
	}
	return fmt.Sprintf("Most of your plays happen at %s (%d %s across %d %s).", top.Name, top.Plays, playsWord, top.DistinctGames, gamesWord)
}

// venueOptions lists known venues for the play form's autocomplete.
func (h *Handler) venueOptions(ctx context.Context, userID string) []venues.Location {
	if h.Venues == nil {
		return nil
	}
	list, err := h.Venues.List(ctx, userID)
	if err != nil {
		slog.Warn("load venues", "error", err)
		return nil
	}
	return list
}

func boxRange(game *collection.GameDetail) plays.BoxRange {
	return plays.BoxRange{Min: game.MinPlaytime, Max: game.MaxPlaytime}
}

// boxLabel describes the advertised range in the same words the rest of the
// page uses for playtime.
func boxLabel(box plays.BoxRange) string {
	switch {
	case box.Min > 0 && box.Max > 0 && box.Min == box.Max:
		return fmt.Sprintf("%d min", box.Max)
	case box.Min > 0 && box.Max > 0:
		return fmt.Sprintf("%d–%d min", box.Min, box.Max)
	case box.Min > 0:
		return fmt.Sprintf("%d min or more", box.Min)
	case box.Max > 0:
		return fmt.Sprintf("up to %d min", box.Max)
	default:
		return ""
	}
}

func playtimeInsight(cal plays.GameCalibration) *PlaytimeInsight {
	phrase := "That's within the box range."
	switch cal.Verdict {
	case plays.VerdictLong:
		phrase = "That runs long."
	case plays.VerdictShort:
		phrase = "That runs short."
	}
	box := boxLabel(cal.Box)
	return &PlaytimeInsight{
		Average: cal.Average,
		Box:     box,
		Phrase:  phrase,
		Line:    fmt.Sprintf("You average %d minutes; the box says %s. %s", cal.Average, box, phrase),
	}
}

func reconcileInsight(gameID int64, gap plays.PlayCountGap) *ReconcileInsight {
	logged := fmt.Sprintf("you've logged %d", gap.Logged)
	if gap.Logged == 0 {
		logged = "you haven't logged any"
	}
	return &ReconcileInsight{
		BGG:     gap.BGG,
		Logged:  gap.Logged,
		Missing: gap.Missing(),
		LogURL:  "/plays/new?game=" + strconv.FormatInt(gameID, 10),
		Line:    fmt.Sprintf("BGG says %d plays; %s.", gap.BGG, logged),
	}
}

func groupPlaytimeInsight(group plays.GroupCalibration) *GroupPlaytimeInsight {
	pct := int(math.Round(math.Abs(group.Percent)))
	var trend string
	switch {
	case pct == 0:
		trend = "finish right on the box"
	case group.Percent > 0:
		trend = fmt.Sprintf("run %d%% over the box", pct)
	default:
		trend = fmt.Sprintf("run %d%% under the box", pct)
	}
	return &GroupPlaytimeInsight{
		Games:   group.Games,
		Plays:   group.Plays,
		Percent: pct,
		Line:    fmt.Sprintf("On average your games %s, from %d timed plays across %d games.", trend, group.Plays, group.Games),
	}
}

func reconcileSummaryInsight(summary plays.ReconcileSummary) *ReconcileSummaryInsight {
	noun, verb := "games", "have"
	if summary.Games == 1 {
		noun, verb = "game", "has"
	}
	return &ReconcileSummaryInsight{
		Games:   summary.Games,
		Missing: summary.Missing,
		Line:    fmt.Sprintf("%d %s %s plays only BGG knows about — %d plays in all.", summary.Games, noun, verb, summary.Missing),
	}
}
