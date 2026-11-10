package collection

import (
	"strings"
	"testing"
	"time"
)

func pence(v int) *int { return &v }

func TestRankTradeCandidatesFavouriteScoresZero(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	games := []GameCard{
		{ID: 1, Name: "Beloved", Favorite: true, PricePaid: pence(5000), AcquisitionDate: "2020-01-01", TotalPlays: 0},
	}
	got := RankTradeCandidatesAt(games, now)
	if len(got) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(got))
	}
	if got[0].Score != 0 {
		t.Errorf("favourite score = %v, want 0", got[0].Score)
	}
	if !strings.Contains(got[0].Reason, "Favourite") {
		t.Errorf("reason = %q, want favourite reason", got[0].Reason)
	}
}

func TestRankTradeCandidatesUnplayedLongOwnedBeatsPlayedCheap(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	games := []GameCard{
		{ID: 1, Name: "Cheap and Played", PricePaid: pence(500), AcquisitionDate: "2025-11-10", TotalPlays: 10},
		{ID: 2, Name: "Expensive and Ignored", PricePaid: pence(4500), AcquisitionDate: "2024-11-10", TotalPlays: 0},
	}
	got := RankTradeCandidatesAt(games, now)
	if len(got) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(got))
	}
	if got[0].Game.ID != 2 {
		t.Errorf("top candidate id = %d, want 2 (expensive/unplayed)", got[0].Game.ID)
	}
	if !(got[0].Score > got[1].Score) {
		t.Errorf("scores not ordered: %v then %v", got[0].Score, got[1].Score)
	}
	// Both ignore-and-value signals fired: price and zero plays.
	if !strings.Contains(got[0].Reason, "shelf-warming") {
		t.Errorf("reason = %q, want a shelf-warming phrase", got[0].Reason)
	}
}

func TestRankTradeCandidatesDeterministicTieBreak(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	games := []GameCard{
		{ID: 9, Name: "Zeta", AcquisitionDate: "2020-01-01"},
		{ID: 3, Name: "Alpha", AcquisitionDate: "2020-01-01"},
		{ID: 2, Name: "Alpha", AcquisitionDate: "2020-01-01"},
	}
	got := RankTradeCandidatesAt(games, now)
	if len(got) != 3 {
		t.Fatalf("want 3 candidates, got %d", len(got))
	}
	// Equal scores: name ascending, then id ascending.
	if got[0].Game.ID != 2 || got[1].Game.ID != 3 || got[2].Game.ID != 9 {
		t.Errorf("tie order = [%d %d %d], want [2 3 9]", got[0].Game.ID, got[1].Game.ID, got[2].Game.ID)
	}
}

func TestRankTradeCandidatesAtFixedNow(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	games := []GameCard{
		{ID: 1, Name: "Two Years", PricePaid: pence(4500), AcquisitionDate: "2024-10-10", TotalPlays: 2},
		{ID: 2, Name: "Fresh", AcquisitionDate: "2026-11-10", TotalPlays: 0},
	}
	got := RankTradeCandidatesAt(games, now)
	byID := map[int64]TradeCandidate{}
	for _, c := range got {
		byID[c.Game.ID] = c
	}
	two := byID[1]
	if !strings.Contains(two.Reason, "2 years") {
		t.Errorf("reason = %q, want it to mention 2 years", two.Reason)
	}
	if !strings.Contains(two.Reason, "played twice") {
		t.Errorf("reason = %q, want it to mention played twice", two.Reason)
	}
	// Same function, same input, same now -> identical output.
	again := RankTradeCandidatesAt(games, now)
	for i := range got {
		if got[i].Score != again[i].Score || got[i].Reason != again[i].Reason {
			t.Errorf("non-deterministic at %d: %+v vs %+v", i, got[i], again[i])
		}
	}
}

func TestRankTradeCandidatesBadDatesDoNotPanic(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	games := []GameCard{
		{ID: 1, Name: "No Date", PricePaid: pence(1000), AcquisitionDate: "not-a-date", AddedAt: ""},
		{ID: 2, Name: "Empty Date", AcquisitionDate: "", AddedAt: ""},
	}
	got := RankTradeCandidatesAt(games, now)
	if len(got) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(got))
	}
	for _, c := range got {
		if c.Reason == "" {
			t.Errorf("game %d has an empty reason", c.Game.ID)
		}
	}
}

func TestRankTradeCandidatesEmpty(t *testing.T) {
	got := RankTradeCandidatesAt(nil, time.Now())
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}
