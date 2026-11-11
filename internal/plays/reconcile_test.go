package plays

import "testing"

func TestPlayCountGap(t *testing.T) {
	cases := []struct {
		name        string
		bgg, logged int
		wantCatchUp bool
		wantMissing int
	}{
		{"bgg ahead", 14, 9, true, 5},
		{"equal", 9, 9, false, 0},
		{"log ahead of bgg", 9, 12, false, 0},
		{"bgg never reported", 0, 4, false, 0},
		{"negative bgg is no signal", -1, 4, false, 0},
		{"nothing logged yet", 6, 0, true, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := PlayCountGap{BGG: tc.bgg, Logged: tc.logged}
			if got := g.NeedsCatchUp(); got != tc.wantCatchUp {
				t.Fatalf("NeedsCatchUp = %v, want %v", got, tc.wantCatchUp)
			}
			if got := g.Missing(); got != tc.wantMissing {
				t.Fatalf("Missing = %d, want %d", got, tc.wantMissing)
			}
		})
	}
}

func TestSummariseReconcile(t *testing.T) {
	cases := []struct {
		name        string
		gaps        []PlayCountGap
		wantOK      bool
		wantGames   int
		wantMissing int
	}{
		{
			name:        "two games behind",
			gaps:        []PlayCountGap{{14, 9}, {6, 0}, {5, 5}},
			wantOK:      true,
			wantGames:   2,
			wantMissing: 11,
		},
		{
			name:   "everything logged",
			gaps:   []PlayCountGap{{5, 5}, {3, 8}},
			wantOK: false,
		},
		{
			name:   "no bgg counts",
			gaps:   []PlayCountGap{{0, 0}},
			wantOK: false,
		},
		{
			name:   "empty",
			gaps:   nil,
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SummariseReconcile(tc.gaps)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Games != tc.wantGames || got.Missing != tc.wantMissing {
				t.Fatalf("got %+v, want games=%d missing=%d", got, tc.wantGames, tc.wantMissing)
			}
		})
	}
}
