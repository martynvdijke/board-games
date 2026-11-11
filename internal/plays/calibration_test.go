package plays

import (
	"math"
	"testing"
)

func TestAverageDurationIgnoresUntimedPlays(t *testing.T) {
	cases := []struct {
		name    string
		in      []int
		wantAvg float64
		wantN   int
	}{
		{"empty", nil, 0, 0},
		{"all untimed", []int{0, 0}, 0, 0},
		{"negative is not a duration", []int{-5, 60}, 60, 1},
		{"mixed", []int{0, 60, 90, 120}, 90, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			avg, n := AverageDuration(tc.in)
			if n != tc.wantN {
				t.Fatalf("count = %d, want %d", n, tc.wantN)
			}
			if math.Abs(avg-tc.wantAvg) > 1e-9 {
				t.Fatalf("average = %v, want %v", avg, tc.wantAvg)
			}
		})
	}
}

func TestBoxRangeMidpoint(t *testing.T) {
	cases := []struct {
		name string
		box  BoxRange
		want float64
	}{
		{"both bounds", BoxRange{Min: 60, Max: 90}, 75},
		{"min only", BoxRange{Min: 60}, 60},
		{"max only", BoxRange{Max: 90}, 90},
		{"no bounds", BoxRange{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.box.Midpoint(); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("Midpoint = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCalibrateGame(t *testing.T) {
	cases := []struct {
		name      string
		durations []int
		box       BoxRange
		minPlays  int
		wantOK    bool
		wantAvg   int
		wantCount int
		want      PlaytimeVerdict
	}{
		{"below threshold", []int{90, 90}, BoxRange{60, 90}, MinTimedPlays, false, 0, 0, ""},
		{"no box playtime", []int{90, 90, 90}, BoxRange{}, MinTimedPlays, false, 0, 0, ""},
		{"min and max, runs long", []int{95, 95, 95}, BoxRange{60, 90}, MinTimedPlays, true, 95, 3, VerdictLong},
		{"min and max, runs short", []int{45, 45, 45}, BoxRange{60, 90}, MinTimedPlays, true, 45, 3, VerdictShort},
		{"min and max, within", []int{70, 80, 90}, BoxRange{60, 90}, MinTimedPlays, true, 80, 3, VerdictWithin},
		{"min only never runs long", []int{120, 120, 120}, BoxRange{Min: 60}, MinTimedPlays, true, 120, 3, VerdictWithin},
		{"min only can run short", []int{30, 30, 30}, BoxRange{Min: 60}, MinTimedPlays, true, 30, 3, VerdictShort},
		{"max only never runs short", []int{20, 20, 20}, BoxRange{Max: 90}, MinTimedPlays, true, 20, 3, VerdictWithin},
		{"max only can run long", []int{120, 120, 120}, BoxRange{Max: 90}, MinTimedPlays, true, 120, 3, VerdictLong},
		{"untimed plays do not count", []int{0, 0, 90, 90, 90}, BoxRange{60, 90}, MinTimedPlays, true, 90, 3, VerdictWithin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cal, ok := CalibrateGame(tc.durations, tc.box, tc.minPlays)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if cal.Average != tc.wantAvg || cal.Plays != tc.wantCount || cal.Verdict != tc.want {
				t.Fatalf("got %+v, want average=%d plays=%d verdict=%q", cal, tc.wantAvg, tc.wantCount, tc.want)
			}
		})
	}
}

func TestCalibrateGroupWeightsByTimedPlays(t *testing.T) {
	samples := []DurationSample{
		// Box midpoint 75, three plays of 90: logged 270, box 225.
		{Durations: []int{90, 90, 90}, Box: BoxRange{Min: 60, Max: 90}},
		// Min-only box 30, three plays of 60: logged 180, box 90.
		{Durations: []int{60, 60, 60}, Box: BoxRange{Min: 30}},
		// Skipped: below the per-game threshold.
		{Durations: []int{200}, Box: BoxRange{60, 90}},
		// Skipped: no box playtime.
		{Durations: []int{45, 45, 45}, Box: BoxRange{}},
	}
	group, ok := CalibrateGroup(samples, MinTimedPlays, MinGamesForGroup)
	if !ok {
		t.Fatal("expected a group calibration")
	}
	if group.Games != 2 || group.Plays != 6 {
		t.Fatalf("games=%d plays=%d, want 2 and 6", group.Games, group.Plays)
	}
	if group.Logged != 450 || group.Box != 315 {
		t.Fatalf("logged=%d box=%d, want 450 and 315", group.Logged, group.Box)
	}
	want := float64(450-315) / 315 * 100
	if math.Abs(group.Percent-want) > 1e-9 {
		t.Fatalf("percent = %v, want %v", group.Percent, want)
	}
}

func TestCalibrateGroupNeedsEnoughGames(t *testing.T) {
	one := []DurationSample{{Durations: []int{90, 90, 90}, Box: BoxRange{60, 90}}}
	if _, ok := CalibrateGroup(one, MinTimedPlays, MinGamesForGroup); ok {
		t.Fatal("a single qualified game must not produce a group sentence")
	}
	none := []DurationSample{{Durations: []int{90}, Box: BoxRange{60, 90}}}
	if _, ok := CalibrateGroup(none, MinTimedPlays, MinGamesForGroup); ok {
		t.Fatal("no qualified games must not produce a group sentence")
	}
}

func TestCalibrateGroupBelowBoxIsNegative(t *testing.T) {
	samples := []DurationSample{
		{Durations: []int{30, 30, 30}, Box: BoxRange{60, 90}},
		{Durations: []int{20, 20, 20}, Box: BoxRange{60, 90}},
	}
	group, ok := CalibrateGroup(samples, MinTimedPlays, MinGamesForGroup)
	if !ok {
		t.Fatal("expected a group calibration")
	}
	if group.Percent >= 0 {
		t.Fatalf("percent = %v, want negative when finishing early", group.Percent)
	}
}
