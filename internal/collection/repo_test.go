package collection

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"testing"

	"boardgames/internal/db"
)

type repoFixture struct {
	db     *sql.DB
	repo   *Repo
	userID string
	nextID int
}

func setupRepo(t *testing.T) *repoFixture {
	t.Helper()
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return &repoFixture{db: database, repo: NewRepo(database), userID: addUser(t, database, "owner@example.com")}
}

func addUser(t *testing.T, database *sql.DB, email string) string {
	t.Helper()
	id := "user-" + email
	now := db.Now()
	if _, err := database.Exec(`INSERT INTO users(id,email,name,created_at,updated_at) VALUES(?,?,?,?,?)`,
		id, email, email, now, now); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

// addGame inserts a catalogue row and returns its generated id (never hard-coded,
// because AUTOINCREMENT advances on upsert conflicts in seeded databases).
func (f *repoFixture) addGame(t *testing.T, bggID int, name string) int64 {
	t.Helper()
	now := db.Now()
	res, err := f.db.Exec(`INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(?,?,?,?,?)`,
		bggID, name, name, now, now)
	if err != nil {
		t.Fatalf("insert game %s: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type shelfRow struct {
	Owned       bool
	Wishlist    bool
	WantToPlay  bool
	Preordered  bool
	ForTrade    bool
	BGGNumPlays int
	PricePaid   *int
}

func (f *repoFixture) addShelf(t *testing.T, userID string, gameID int64, row shelfRow) {
	t.Helper()
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	now := db.Now()
	var price any
	if row.PricePaid != nil {
		price = *row.PricePaid
	}
	if _, err := f.db.Exec(`INSERT INTO user_games(user_id,game_id,owned,wishlist,want_to_play,preordered,for_trade,bgg_num_plays,price_paid,added_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		userID, gameID, bit(row.Owned), bit(row.Wishlist), bit(row.WantToPlay), bit(row.Preordered), bit(row.ForTrade),
		row.BGGNumPlays, price, now, now); err != nil {
		t.Fatalf("insert shelf row: %v", err)
	}
}

func (f *repoFixture) addPlay(t *testing.T, userID string, gameID int64, quantity int, duration *int) {
	t.Helper()
	f.nextID++
	id := fmt.Sprintf("play-%s-%d", userID, f.nextID)
	now := db.Now()
	var dur any
	if duration != nil {
		dur = *duration
	}
	if _, err := f.db.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,duration_minutes,quantity,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		id, userID, gameID, "2026-01-01", dur, quantity, now, now); err != nil {
		t.Fatalf("insert play: %v", err)
	}
}

func intPtr(v int) *int { return &v }

func nearlyEqual(a, b float64) bool { return math.Abs(a-b) < 0.001 }

func cardByID(t *testing.T, cards []GameCard, id int64) GameCard {
	t.Helper()
	for _, c := range cards {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("game %d not in result", id)
	return GameCard{}
}

func TestCostPerPlayAndHour(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()

	// Price, plays and durations recorded: 30.00 over 4 plays (2.5 hours).
	timed := f.addGame(t, 1001, "Timed")
	f.addShelf(t, f.userID, timed, shelfRow{Owned: true, PricePaid: intPtr(3000)})
	f.addPlay(t, f.userID, timed, 2, intPtr(60))
	f.addPlay(t, f.userID, timed, 2, intPtr(90))

	// Price but no plays: both metrics undefined, price still reported.
	unplayed := f.addGame(t, 1002, "Unplayed")
	f.addShelf(t, f.userID, unplayed, shelfRow{Owned: true, PricePaid: intPtr(2000)})

	// Plays without durations: cost per play defined, cost per hour undefined.
	untimed := f.addGame(t, 1003, "Untimed")
	f.addShelf(t, f.userID, untimed, shelfRow{Owned: true, PricePaid: intPtr(1000)})
	f.addPlay(t, f.userID, untimed, 2, nil)

	// No price: no cost metrics at all.
	free := f.addGame(t, 1004, "No price")
	f.addShelf(t, f.userID, free, shelfRow{Owned: true})
	f.addPlay(t, f.userID, free, 1, intPtr(60))

	cards, total, err := f.repo.List(ctx, f.userID, Filters{PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(cards) != 4 {
		t.Fatalf("expected 4 cards, got %d/%d", len(cards), total)
	}

	timedCard := cardByID(t, cards, timed)
	if timedCard.PricePaid == nil || *timedCard.PricePaid != 3000 {
		t.Fatalf("price = %v, want 3000", timedCard.PricePaid)
	}
	if timedCard.CostPerPlayPence == nil || !nearlyEqual(*timedCard.CostPerPlayPence, 750) {
		t.Fatalf("cost per play = %v, want 750", timedCard.CostPerPlayPence)
	}
	if timedCard.CostPerHourPence == nil || !nearlyEqual(*timedCard.CostPerHourPence, 1200) {
		t.Fatalf("cost per hour = %v, want 1200", timedCard.CostPerHourPence)
	}

	unplayedCard := cardByID(t, cards, unplayed)
	if unplayedCard.PricePaid == nil || *unplayedCard.PricePaid != 2000 {
		t.Fatalf("unplayed price = %v, want 2000", unplayedCard.PricePaid)
	}
	if unplayedCard.CostPerPlayPence != nil || unplayedCard.CostPerHourPence != nil {
		t.Fatalf("expected undefined costs for no plays, got %v/%v", unplayedCard.CostPerPlayPence, unplayedCard.CostPerHourPence)
	}

	untimedCard := cardByID(t, cards, untimed)
	if untimedCard.CostPerPlayPence == nil || !nearlyEqual(*untimedCard.CostPerPlayPence, 500) {
		t.Fatalf("untimed cost per play = %v, want 500", untimedCard.CostPerPlayPence)
	}
	if untimedCard.CostPerHourPence != nil {
		t.Fatalf("expected undefined cost per hour without durations, got %v", *untimedCard.CostPerHourPence)
	}

	freeCard := cardByID(t, cards, free)
	if freeCard.PricePaid != nil || freeCard.CostPerPlayPence != nil || freeCard.CostPerHourPence != nil {
		t.Fatalf("expected no metrics without a price, got %v/%v/%v", freeCard.PricePaid, freeCard.CostPerPlayPence, freeCard.CostPerHourPence)
	}
}

func TestGetIncludesCostMetrics(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()
	game := f.addGame(t, 1100, "Detailed")
	f.addShelf(t, f.userID, game, shelfRow{Owned: true, PricePaid: intPtr(2400)})
	f.addPlay(t, f.userID, game, 3, intPtr(120))

	detail, err := f.repo.Get(ctx, f.userID, game)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PricePaid == nil || *detail.PricePaid != 2400 {
		t.Fatalf("detail price = %v, want 2400", detail.PricePaid)
	}
	if detail.CostPerPlayPence == nil || !nearlyEqual(*detail.CostPerPlayPence, 800) {
		t.Fatalf("detail cost per play = %v, want 800", detail.CostPerPlayPence)
	}
	if detail.CostPerHourPence == nil || !nearlyEqual(*detail.CostPerHourPence, 1200) {
		t.Fatalf("detail cost per hour = %v, want 1200", detail.CostPerHourPence)
	}
}

func TestCostPlaySortPutsUndefinedLast(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()

	expensive := f.addGame(t, 1201, "A expensive")
	f.addShelf(t, f.userID, expensive, shelfRow{Owned: true, PricePaid: intPtr(1000)})
	f.addPlay(t, f.userID, expensive, 1, intPtr(60))

	cheap := f.addGame(t, 1202, "B cheap")
	f.addShelf(t, f.userID, cheap, shelfRow{Owned: true, PricePaid: intPtr(500)})
	f.addPlay(t, f.userID, cheap, 1, intPtr(60))

	noPlays := f.addGame(t, 1203, "C no plays")
	f.addShelf(t, f.userID, noPlays, shelfRow{Owned: true, PricePaid: intPtr(1000)})

	noPrice := f.addGame(t, 1204, "D no price")
	f.addShelf(t, f.userID, noPrice, shelfRow{Owned: true})

	cards, _, err := f.repo.List(ctx, f.userID, Filters{Sort: "cost_play", PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 4 {
		t.Fatalf("expected 4 cards, got %d", len(cards))
	}
	if cards[0].ID != cheap || cards[1].ID != expensive {
		t.Fatalf("defined costs not ascending: got %d then %d", cards[0].ID, cards[1].ID)
	}
	if cards[2].CostPerPlayPence != nil || cards[3].CostPerPlayPence != nil {
		t.Fatalf("undefined costs should sort last, got %v then %v", cards[2].CostPerPlayPence, cards[3].CostPerPlayPence)
	}

	// cost_hour uses summed durations; the untimed game is undefined there.
	hourCards, _, err := f.repo.List(ctx, f.userID, Filters{Sort: "cost_hour", PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if hourCards[0].CostPerHourPence == nil || hourCards[1].CostPerHourPence == nil {
		t.Fatalf("expected two defined per-hour costs first")
	}
	for _, c := range hourCards[2:] {
		if c.CostPerHourPence != nil {
			t.Fatalf("undefined per-hour cost sorted early for game %d", c.ID)
		}
	}
}

func TestPipelineBuckets(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()

	wishlist := f.addGame(t, 1301, "Wishlist only")
	f.addShelf(t, f.userID, wishlist, shelfRow{Wishlist: true})

	both := f.addGame(t, 1302, "Wishlist and preordered")
	f.addShelf(t, f.userID, both, shelfRow{Wishlist: true, Preordered: true})

	unplayed := f.addGame(t, 1303, "Owned unplayed")
	f.addShelf(t, f.userID, unplayed, shelfRow{Owned: true})

	forTrade := f.addGame(t, 1304, "For trade")
	f.addShelf(t, f.userID, forTrade, shelfRow{Owned: true, ForTrade: true})

	wantToPlay := f.addGame(t, 1305, "Want to play")
	f.addShelf(t, f.userID, wantToPlay, shelfRow{WantToPlay: true})

	played := f.addGame(t, 1306, "Owned and played")
	f.addShelf(t, f.userID, played, shelfRow{Owned: true})
	f.addPlay(t, f.userID, played, 1, intPtr(60))

	buckets, err := f.repo.Pipeline(ctx, f.userID, 0, Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(buckets))
	}
	want := []struct {
		state, label string
		count        int
	}{
		{"wishlist", "Wishlist", 2},
		{"want_to_play", "Want to play", 1},
		{"preordered", "Preordered", 1},
		{"for_trade", "For trade", 1},
		// The owned for-trade game has no plays either, so it also lands here.
		{"unplayed", "Owned, never played", 2},
	}
	for i, w := range want {
		if buckets[i].State != w.state || buckets[i].Label != w.label || buckets[i].Count != w.count {
			t.Fatalf("bucket %d = %+v, want %s/%s/%d", i, buckets[i], w.state, w.label, w.count)
		}
		if len(buckets[i].Games) != w.count {
			t.Fatalf("bucket %s returned %d games, want %d", w.state, len(buckets[i].Games), w.count)
		}
	}
	// The doubly flagged game appears in both buckets rather than being exclusive.
	if buckets[0].Games[0].ID != wishlist && buckets[0].Games[1].ID != wishlist {
		t.Fatalf("wishlist bucket missing the wishlist game")
	}
	foundPreordered := false
	for _, g := range buckets[2].Games {
		if g.ID == both {
			foundPreordered = true
		}
	}
	if !foundPreordered {
		t.Fatalf("doubly flagged game missing from preordered bucket")
	}

	// The unplayed bucket holds owned games without plays and excludes the
	// owned game that has one.
	unplayedSeen := false
	for _, g := range buckets[4].Games {
		if g.ID == played {
			t.Fatalf("played game appeared in the unplayed bucket")
		}
		if g.ID == unplayed {
			unplayedSeen = true
		}
	}
	if !unplayedSeen {
		t.Fatalf("unplayed game missing from the unplayed bucket")
	}

	// perBucket caps the returned games without changing the counts.
	capped, err := f.repo.Pipeline(ctx, f.userID, 1, Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if capped[0].Count != 2 || len(capped[0].Games) != 1 {
		t.Fatalf("perBucket cap failed: count=%d games=%d", capped[0].Count, len(capped[0].Games))
	}
}

func TestValueMetricsAreUserScoped(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()
	otherUser := addUser(t, f.db, "other@example.com")
	game := f.addGame(t, 1401, "Shared")

	f.addShelf(t, f.userID, game, shelfRow{Owned: true, PricePaid: intPtr(1000)})
	f.addPlay(t, f.userID, game, 1, intPtr(60))

	// The second user owns the same catalogue game with a very different price
	// and play history; none of it may leak into the first user's result.
	f.addShelf(t, otherUser, game, shelfRow{Owned: true, PricePaid: intPtr(9999)})
	f.addPlay(t, otherUser, game, 5, intPtr(600))

	cards, total, err := f.repo.List(ctx, f.userID, Filters{PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(cards) != 1 {
		t.Fatalf("expected one card, got %d/%d", len(cards), total)
	}
	card := cards[0]
	if card.PricePaid == nil || *card.PricePaid != 1000 {
		t.Fatalf("price = %v, want the first user's 1000", card.PricePaid)
	}
	if card.CostPerPlayPence == nil || !nearlyEqual(*card.CostPerPlayPence, 1000) {
		t.Fatalf("cost per play = %v, want 1000", card.CostPerPlayPence)
	}

	stats, err := f.repo.Stats(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPaidPence != 1000 || stats.PaidGames != 1 {
		t.Fatalf("value totals leaked: total=%d games=%d", stats.TotalPaidPence, stats.PaidGames)
	}
	if stats.MedianCostPerPlayPence == nil || !nearlyEqual(*stats.MedianCostPerPlayPence, 1000) {
		t.Fatalf("median = %v, want 1000", stats.MedianCostPerPlayPence)
	}

	// The second user's own view sees their own figures and nothing of the first.
	otherStats, err := f.repo.Stats(ctx, otherUser)
	if err != nil {
		t.Fatal(err)
	}
	if otherStats.TotalPaidPence != 9999 || otherStats.PaidGames != 1 {
		t.Fatalf("other user totals wrong: %+v", otherStats)
	}
}

func TestStatsValueTotals(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()

	// Two priced, played games plus one priced unplayed game and one played
	// game with no price.
	first := f.addGame(t, 1501, "First")
	f.addShelf(t, f.userID, first, shelfRow{Owned: true, PricePaid: intPtr(3000)})
	f.addPlay(t, f.userID, first, 2, intPtr(60))

	second := f.addGame(t, 1502, "Second")
	f.addShelf(t, f.userID, second, shelfRow{Owned: true, PricePaid: intPtr(1000)})
	f.addPlay(t, f.userID, second, 4, intPtr(60))

	unsold := f.addGame(t, 1503, "Unplayed with price")
	f.addShelf(t, f.userID, unsold, shelfRow{Owned: true, PricePaid: intPtr(2000)})

	unpriced := f.addGame(t, 1504, "Played without price")
	f.addShelf(t, f.userID, unpriced, shelfRow{Owned: true})
	f.addPlay(t, f.userID, unpriced, 3, intPtr(30))

	stats, err := f.repo.Stats(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPaidPence != 6000 {
		t.Fatalf("TotalPaidPence = %d, want 6000", stats.TotalPaidPence)
	}
	if stats.PaidGames != 3 {
		t.Fatalf("PaidGames = %d, want 3", stats.PaidGames)
	}
	if stats.UnplayedCapitalPence != 2000 {
		t.Fatalf("UnplayedCapitalPence = %d, want 2000", stats.UnplayedCapitalPence)
	}
	// 3000/2 = 1500 and 1000/4 = 250; the unplayed and unpriced games are
	// skipped, so the median is the average of the two.
	if stats.MedianCostPerPlayPence == nil || !nearlyEqual(*stats.MedianCostPerPlayPence, 875) {
		t.Fatalf("MedianCostPerPlayPence = %v, want 875", stats.MedianCostPerPlayPence)
	}
}

func TestMedianCostPerPlayUndefinedWithoutEligibleGames(t *testing.T) {
	f := setupRepo(t)
	ctx := context.Background()

	unplayed := f.addGame(t, 1601, "Priced but unplayed")
	f.addShelf(t, f.userID, unplayed, shelfRow{Owned: true, PricePaid: intPtr(1500)})

	unpriced := f.addGame(t, 1602, "Played but unpriced")
	f.addShelf(t, f.userID, unpriced, shelfRow{Owned: true})
	f.addPlay(t, f.userID, unpriced, 2, intPtr(60))

	stats, err := f.repo.Stats(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.MedianCostPerPlayPence != nil {
		t.Fatalf("median = %v, want nil", *stats.MedianCostPerPlayPence)
	}
}
