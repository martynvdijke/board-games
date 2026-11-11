package yearreview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"boardgames/internal/db"
)

// testYear is a completed calendar year relative to the real clock, so the
// fixtures never depend on the machine's date while still exercising the
// "past year" path.
func testYear() int { return time.Now().UTC().Year() - 1 }

func date(year, month, day int) string {
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func createUser(t *testing.T, database *sql.DB, id string) {
	t.Helper()
	mustExec(t, database, `INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES(?,?,?,'member',?,?)`,
		id, id+"@example.com", strings.ToUpper(id), db.Now(), db.Now())
}

func createGame(t *testing.T, database *sql.DB, bggID int, name string, weight *float64) int64 {
	t.Helper()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,thumbnail_url,weight,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?)`, bggID, name, strings.ToLower(name), "thumb-"+name, weight, db.Now(), db.Now())
	if err != nil {
		t.Fatalf("insert game %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("game id: %v", err)
	}
	return id
}

// ownedGame is a shelf row. acquisition is stored verbatim; an empty string
// means "no acquisition date recorded".
type shelfRow struct {
	owned       bool
	acquisition string
	price       *int
	rating      *float64
	addedAt     string
}

func addShelfGame(t *testing.T, database *sql.DB, userID string, gameID int64, row shelfRow) {
	t.Helper()
	addedAt := row.addedAt
	if addedAt == "" {
		addedAt = "2020-01-01T00:00:00Z"
	}
	var acquisition any
	if row.acquisition != "" {
		acquisition = row.acquisition
	}
	owned := 0
	if row.owned {
		owned = 1
	}
	mustExec(t, database, `INSERT INTO user_games(user_id,game_id,owned,acquisition_date,price_paid,user_rating,added_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		userID, gameID, owned, acquisition, nullableInt(row.price), nullableFloat(row.rating), addedAt, db.Now())
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func ptrInt(v int) *int           { return &v }
func ptrFloat(v float64) *float64 { return &v }

func addPlay(t *testing.T, database *sql.DB, id, userID string, gameID int64, playedAt string, minutes *int, quantity int) {
	t.Helper()
	var duration any
	if minutes != nil {
		duration = *minutes
	}
	mustExec(t, database, `INSERT INTO plays(id,user_id,game_id,played_at,duration_minutes,quantity,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, id, userID, gameID, playedAt, duration, quantity, db.Now(), db.Now())
}

func addPlayer(t *testing.T, database *sql.DB, playID, name string, winner bool) {
	t.Helper()
	win := 0
	if winner {
		win = 1
	}
	mustExec(t, database, `INSERT INTO play_players(play_id,name,is_winner,is_first_play,position) VALUES(?,?,?,0,0)`,
		playID, name, win)
}

func TestComputeFullYearTotals(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	year := testYear()
	a := createGame(t, database, 1, "Alpha", nil)
	b := createGame(t, database, 2, "Beta", nil)

	addPlay(t, database, "p1", "u1", a, date(year, 3, 1), ptrInt(60), 2)
	addPlay(t, database, "p2", "u1", a, date(year, 4, 1), nil, 1) // no duration
	addPlay(t, database, "p3", "u1", b, date(year, 5, 1), ptrInt(90), 1)
	addPlay(t, database, "p4", "u1", b, date(year-1, 12, 31), ptrInt(30), 3) // previous year

	recap, err := Compute(ctx, database, "u1", year)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if recap.Plays != 4 {
		t.Errorf("Plays = %d, want 4", recap.Plays)
	}
	if recap.Sessions != 3 {
		t.Errorf("Sessions = %d, want 3", recap.Sessions)
	}
	if recap.DistinctGames != 2 {
		t.Errorf("DistinctGames = %d, want 2", recap.DistinctGames)
	}
	if recap.Minutes != 150 {
		t.Errorf("Minutes = %d, want 150 (only plays with a duration)", recap.Minutes)
	}
	if recap.PlaysWithDuration != 2 {
		t.Errorf("PlaysWithDuration = %d, want 2", recap.PlaysWithDuration)
	}
	if recap.Sparse {
		t.Error("Sparse = true for a year with plays")
	}
	if recap.Year != year {
		t.Errorf("Year = %d, want %d", recap.Year, year)
	}
}

func TestComputeMostPlayedAndHeaviest(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	year := testYear()

	a := createGame(t, database, 1, "Alpha", ptrFloat(3.5))
	b := createGame(t, database, 2, "Beta", ptrFloat(4.2))
	addPlay(t, database, "p1", "u1", a, date(year, 2, 1), nil, 5)
	addPlay(t, database, "p2", "u1", b, date(year, 2, 2), nil, 2)

	recap, err := Compute(ctx, database, "u1", year)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if recap.MostPlayed == nil || recap.MostPlayed.ID != a {
		t.Fatalf("MostPlayed = %+v, want Alpha", recap.MostPlayed)
	}
	if recap.MostPlayedPlays != 5 {
		t.Errorf("MostPlayedPlays = %d, want 5", recap.MostPlayedPlays)
	}
	if recap.Heaviest == nil || recap.Heaviest.ID != b {
		t.Fatalf("Heaviest = %+v, want Beta", recap.Heaviest)
	}
	if recap.HeaviestWeight != 4.2 {
		t.Errorf("HeaviestWeight = %v, want 4.2", recap.HeaviestWeight)
	}

	t.Run("no played game has a weight", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		g := createGame(t, database, 1, "Weightless", nil)
		addPlay(t, database, "p1", "u1", g, date(year, 6, 1), nil, 1)
		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Heaviest != nil {
			t.Errorf("Heaviest = %+v, want nil", recap.Heaviest)
		}
		if recap.MostPlayed == nil {
			t.Error("MostPlayed = nil, want the played game")
		}
	})
}

func TestComputeSurprise(t *testing.T) {
	ctx := context.Background()
	year := testYear()

	t.Run("three rated first plays", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		b := createGame(t, database, 2, "Beta", nil)
		c := createGame(t, database, 3, "Gamma", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true, rating: ptrFloat(6)})
		addShelfGame(t, database, "u1", b, shelfRow{owned: true, rating: ptrFloat(7)})
		addShelfGame(t, database, "u1", c, shelfRow{owned: true, rating: ptrFloat(9)})
		addPlay(t, database, "p1", "u1", a, date(year, 1, 5), nil, 1)
		addPlay(t, database, "p2", "u1", b, date(year, 1, 6), nil, 1)
		addPlay(t, database, "p3", "u1", c, date(year, 1, 7), nil, 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if len(recap.FirstPlays) != 3 {
			t.Fatalf("FirstPlays = %d, want 3", len(recap.FirstPlays))
		}
		if recap.Surprise == nil {
			t.Fatal("Surprise = nil, want a surprise with three rated first plays")
		}
		if recap.Surprise.Game.ID != c {
			t.Errorf("Surprise game = %q, want Gamma (furthest above the median)", recap.Surprise.Game.Name)
		}
		if recap.Surprise.Median != 7 {
			t.Errorf("Surprise median = %v, want 7", recap.Surprise.Median)
		}
		if recap.Surprise.Delta != 2 {
			t.Errorf("Surprise delta = %v, want 2", recap.Surprise.Delta)
		}
		if recap.Surprise.UserRating != 9 {
			t.Errorf("Surprise rating = %v, want 9", recap.Surprise.UserRating)
		}
	})

	t.Run("two rated first plays", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		b := createGame(t, database, 2, "Beta", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true, rating: ptrFloat(9)})
		addShelfGame(t, database, "u1", b, shelfRow{owned: true, rating: ptrFloat(2)})
		addPlay(t, database, "p1", "u1", a, date(year, 1, 5), nil, 1)
		addPlay(t, database, "p2", "u1", b, date(year, 1, 6), nil, 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Surprise != nil {
			t.Errorf("Surprise = %+v, want nil below three rated first plays", recap.Surprise)
		}
	})
}

func TestComputeNemesis(t *testing.T) {
	ctx := context.Background()
	year := testYear()

	t.Run("most frequent winner", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		g := createGame(t, database, 1, "Alpha", nil)
		addPlay(t, database, "p1", "u1", g, date(year, 2, 1), nil, 1)
		addPlayer(t, database, "p1", "Alice", true)
		addPlayer(t, database, "p1", "Bob", false)
		addPlay(t, database, "p2", "u1", g, date(year, 2, 2), nil, 1)
		addPlayer(t, database, "p2", "Alice", true)
		addPlayer(t, database, "p2", "Bob", false)
		addPlay(t, database, "p3", "u1", g, date(year, 2, 3), nil, 1)
		addPlayer(t, database, "p3", "Bob", true)
		addPlayer(t, database, "p3", "Alice", false)
		// A play with no recorded winner contributes nothing.
		addPlay(t, database, "p4", "u1", g, date(year, 2, 4), nil, 1)
		addPlayer(t, database, "p4", "Alice", false)
		addPlayer(t, database, "p4", "Bob", false)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Nemesis == nil {
			t.Fatal("Nemesis = nil, want Alice")
		}
		if recap.Nemesis.Name != "Alice" {
			t.Errorf("Nemesis name = %q, want Alice", recap.Nemesis.Name)
		}
		if recap.Nemesis.Wins != 2 {
			t.Errorf("Nemesis wins = %d, want 2", recap.Nemesis.Wins)
		}
		if recap.Nemesis.Meetings != 4 {
			t.Errorf("Nemesis meetings = %d, want 4", recap.Nemesis.Meetings)
		}
	})

	t.Run("no recorded winners", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		g := createGame(t, database, 1, "Alpha", nil)
		addPlay(t, database, "p1", "u1", g, date(year, 2, 1), nil, 1)
		addPlayer(t, database, "p1", "Alice", false)
		addPlayer(t, database, "p1", "Bob", false)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Nemesis != nil {
			t.Errorf("Nemesis = %+v, want nil", recap.Nemesis)
		}
	})
}

func TestComputeCostPerHour(t *testing.T) {
	ctx := context.Background()
	year := testYear()

	t.Run("hours and prices recorded", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		b := createGame(t, database, 2, "Beta", nil)
		unplayed := createGame(t, database, 3, "Unplayed", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true, price: ptrInt(2400)})
		addShelfGame(t, database, "u1", b, shelfRow{owned: true, price: ptrInt(1000)})
		addShelfGame(t, database, "u1", unplayed, shelfRow{owned: true, price: ptrInt(5000)})
		addPlay(t, database, "p1", "u1", a, date(year, 4, 1), ptrInt(60), 1)
		addPlay(t, database, "p2", "u1", b, date(year, 4, 2), ptrInt(60), 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.CostPerHourPence == nil {
			t.Fatal("CostPerHourPence = nil, want a value")
		}
		if got := *recap.CostPerHourPence; got != 1700 {
			t.Errorf("CostPerHourPence = %v, want 1700 (3400 pence over 2 hours)", got)
		}
	})

	t.Run("no durations", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true, price: ptrInt(2400)})
		addPlay(t, database, "p1", "u1", a, date(year, 4, 1), nil, 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.CostPerHourPence != nil {
			t.Errorf("CostPerHourPence = %v, want nil without durations", *recap.CostPerHourPence)
		}
	})

	t.Run("no prices", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true})
		addPlay(t, database, "p1", "u1", a, date(year, 4, 1), ptrInt(60), 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.CostPerHourPence != nil {
			t.Errorf("CostPerHourPence = %v, want nil without prices", *recap.CostPerHourPence)
		}
	})
}

func TestComputeShelfProgress(t *testing.T) {
	ctx := context.Background()
	year := testYear()

	t.Run("acquisition fallback and play during the year", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		keptUnplayed := createGame(t, database, 1, "Kept Unplayed", nil)
		playedThisYear := createGame(t, database, 2, "Played This Year", nil)
		noAcquisition := createGame(t, database, 3, "No Acquisition", nil)

		addShelfGame(t, database, "u1", keptUnplayed, shelfRow{owned: true, acquisition: date(year-2, 1, 15)})
		addShelfGame(t, database, "u1", playedThisYear, shelfRow{owned: true, acquisition: date(year-2, 2, 1)})
		addShelfGame(t, database, "u1", noAcquisition, shelfRow{owned: true, addedAt: "2020-01-01T00:00:00Z"})
		addPlay(t, database, "p1", "u1", playedThisYear, date(year, 3, 1), nil, 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Shelf.UnplayedThen != 3 {
			t.Errorf("UnplayedThen = %d, want 3", recap.Shelf.UnplayedThen)
		}
		if recap.Shelf.UnplayedNow != 2 {
			t.Errorf("UnplayedNow = %d, want 2 (the played game left the pile)", recap.Shelf.UnplayedNow)
		}
		if recap.Shelf.Delta != -1 {
			t.Errorf("Delta = %d, want -1", recap.Shelf.Delta)
		}
		if !recap.Shelf.UsedFallback {
			t.Error("UsedFallback = false, want true when acquisition_date is missing")
		}
	})

	t.Run("no fallback when every acquisition date is present", func(t *testing.T) {
		database := newTestDB(t)
		createUser(t, database, "u1")
		a := createGame(t, database, 1, "Alpha", nil)
		b := createGame(t, database, 2, "Beta", nil)
		addShelfGame(t, database, "u1", a, shelfRow{owned: true, acquisition: date(year-2, 1, 15)})
		addShelfGame(t, database, "u1", b, shelfRow{owned: true, acquisition: date(year-2, 2, 1)})
		addPlay(t, database, "p1", "u1", b, date(year, 3, 1), nil, 1)

		recap, err := Compute(ctx, database, "u1", year)
		if err != nil {
			t.Fatalf("compute: %v", err)
		}
		if recap.Shelf.UsedFallback {
			t.Error("UsedFallback = true, want false")
		}
		if recap.Shelf.UnplayedThen != 2 || recap.Shelf.UnplayedNow != 1 {
			t.Errorf("shelf = %+v, want then 2 / now 1", recap.Shelf)
		}
	})
}

func TestComputeSparseYear(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	a := createGame(t, database, 1, "Alpha", nil)
	addShelfGame(t, database, "u1", a, shelfRow{owned: true, acquisition: date(testYear()-3, 1, 1)})

	recap, err := Compute(ctx, database, "u1", time.Now().UTC().Year())
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !recap.Sparse {
		t.Error("Sparse = false for a year with no plays")
	}
	if recap.MostPlayed != nil || recap.Heaviest != nil || recap.Surprise != nil || recap.Nemesis != nil {
		t.Errorf("sections should be nil for a sparse year: %+v", recap)
	}
	if recap.Plays != 0 || recap.Sessions != 0 || recap.DistinctGames != 0 || recap.Minutes != 0 {
		t.Errorf("counts should be zero: %+v", recap)
	}
	if len(recap.FirstPlays) != 0 {
		t.Errorf("FirstPlays = %d, want 0", len(recap.FirstPlays))
	}
	if recap.CostPerHourPence != nil {
		t.Errorf("CostPerHourPence = %v, want nil", *recap.CostPerHourPence)
	}
}

func TestComputeYearBounds(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	year := testYear()
	g := createGame(t, database, 1, "Alpha", nil)
	addPlay(t, database, "p1", "u1", g, date(year, 5, 1), nil, 1)

	if _, err := Compute(ctx, database, "u1", time.Now().UTC().Year()+1); !errors.Is(err, ErrYearOutOfRange) {
		t.Errorf("year after current = %v, want ErrYearOutOfRange", err)
	}
	if _, err := Compute(ctx, database, "u1", year-2); !errors.Is(err, ErrYearOutOfRange) {
		t.Errorf("year before first play = %v, want ErrYearOutOfRange", err)
	}

	noPlays := newTestDB(t)
	createUser(t, noPlays, "u1")
	if _, err := Compute(ctx, noPlays, "u1", time.Now().UTC().Year()); err != nil {
		t.Errorf("current year with no plays = %v, want a sparse recap", err)
	}
	if _, err := Compute(ctx, noPlays, "u1", time.Now().UTC().Year()-1); !errors.Is(err, ErrYearOutOfRange) {
		t.Errorf("past year with no plays = %v, want ErrYearOutOfRange", err)
	}
}

// TestLocationFallback mirrors the play log: an empty or unknown zone is UTC,
// a valid IANA name is honoured.
func TestLocationFallback(t *testing.T) {
	cases := []struct {
		name     string
		timezone string
		want     string
	}{
		{"valid", "Pacific/Auckland", "Pacific/Auckland"},
		{"empty", "", "UTC"},
		{"invalid", "Not/AZone", "UTC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Location(tc.timezone).String(); got != tc.want {
				t.Errorf("Location(%q) = %q, want %q", tc.timezone, got, tc.want)
			}
		})
	}
}

// TestComputeBoundaryUsesLocation proves the year boundary comes from loc and
// not from SQLite: the same instant belongs to different years depending on the
// zone the caller passed.
func TestComputeBoundaryUsesLocation(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	year := testYear()
	g := createGame(t, database, 1, "Alpha", nil)

	// A normal play in June, so both years are inside the allowed window.
	addPlay(t, database, "p1", "u1", g, date(year, 6, 1), nil, 1)

	// 23:59:30 on 31 December in a zone two hours behind UTC: the instant is
	// 01:59:30 UTC on 1 January of the following year.
	loc := time.FixedZone("UTC-2", -2*60*60)
	newYearEve := time.Date(year, 12, 31, 23, 59, 30, 0, loc)
	addPlay(t, database, "p2", "u1", g, db.Timestamp(newYearEve), nil, 1)

	inZone, err := ComputeAt(ctx, database, "u1", year, loc)
	if err != nil {
		t.Fatalf("compute in zone: %v", err)
	}
	if inZone.Sessions != 2 {
		t.Errorf("Sessions in %s = %d, want 2 (the play is still %d locally)", loc, inZone.Sessions, year)
	}

	utc, err := Compute(ctx, database, "u1", year)
	if err != nil {
		t.Fatalf("compute in UTC: %v", err)
	}
	if utc.Sessions != 1 {
		t.Errorf("Sessions in UTC = %d, want 1 (the play landed in %d UTC)", utc.Sessions, year+1)
	}

	nextYear, err := Compute(ctx, database, "u1", year+1)
	if err != nil {
		t.Fatalf("compute next year in UTC: %v", err)
	}
	if nextYear.Sessions != 1 {
		t.Errorf("Sessions in %d UTC = %d, want 1", year+1, nextYear.Sessions)
	}
}

func TestShareLifecycle(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	createUser(t, database, "u1")
	createUser(t, database, "u2")
	store := NewStore(database)

	share, err := store.Create(ctx, "u1", 2025, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(share.Token) != 22 {
		t.Errorf("token length = %d, want 22", len(share.Token))
	}
	if strings.ContainsAny(share.Token, "+/=") {
		t.Errorf("token %q is not URL-safe", share.Token)
	}
	if share.IncludePlayers {
		t.Error("IncludePlayers = true, want false by default")
	}
	if share.RevokedAt != "" {
		t.Errorf("RevokedAt = %q, want empty", share.RevokedAt)
	}

	found, err := store.ByToken(ctx, share.Token)
	if err != nil {
		t.Fatalf("by token: %v", err)
	}
	if found == nil || found.ID != share.ID {
		t.Fatalf("ByToken = %+v, want the created share", found)
	}

	if _, err := store.ByToken(ctx, strings.Repeat("x", 22)); err != nil {
		t.Fatalf("unknown token: %v", err)
	} else if found, _ := store.ByToken(ctx, strings.Repeat("x", 22)); found != nil {
		t.Errorf("unknown token resolved to %+v, want nil", found)
	}

	// A second share for the same year coexists and becomes the active one.
	if _, err := database.Exec(`UPDATE year_review_shares SET created_at='2020-01-01T00:00:00Z' WHERE id=?`, share.ID); err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, "u1", 2025, true)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	active, err := store.ActiveFor(ctx, "u1", 2025)
	if err != nil {
		t.Fatalf("active for: %v", err)
	}
	if active == nil || active.ID != second.ID {
		t.Fatalf("ActiveFor = %+v, want the newest share", active)
	}

	if err := store.Revoke(ctx, "u2", share.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke another user's share = %v, want ErrNotFound", err)
	}
	if err := store.Revoke(ctx, "u1", share.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, err := store.ByToken(ctx, share.Token)
	if err != nil {
		t.Fatalf("by token after revoke: %v", err)
	}
	if revoked != nil {
		t.Errorf("ByToken after revoke = %+v, want nil", revoked)
	}
	// Revoking twice is not an error.
	if err := store.Revoke(ctx, "u1", share.ID); err != nil {
		t.Errorf("second revoke = %v, want nil", err)
	}

	active, err = store.ActiveFor(ctx, "u1", 2025)
	if err != nil {
		t.Fatalf("active for after revoke: %v", err)
	}
	if active == nil || active.ID != second.ID {
		t.Errorf("ActiveFor after revoke = %+v, want the surviving share", active)
	}

	mine, err := store.List(ctx, "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("List = %d shares, want 2 (revoked included)", len(mine))
	}
	if mine[0].ID != second.ID {
		t.Errorf("List first = %d, want newest %d", mine[0].ID, second.ID)
	}
	theirs, err := store.List(ctx, "u2")
	if err != nil {
		t.Fatalf("list other: %v", err)
	}
	if len(theirs) != 0 {
		t.Errorf("List for another user = %d, want 0", len(theirs))
	}
}

func TestMigrationCreatesShareTable(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	var table string
	if err := database.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='year_review_shares'`).Scan(&table); err != nil {
		t.Fatalf("year_review_shares table missing: %v", err)
	}

	rows, err := database.QueryContext(ctx, `SELECT name FROM pragma_index_list('year_review_shares') WHERE "unique"=1`)
	if err != nil {
		t.Fatalf("index list: %v", err)
	}
	defer rows.Close()
	unique := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("index scan: %v", err)
		}
		unique[name] = true
	}
	if !unique["sqlite_autoindex_year_review_shares_1"] {
		t.Fatalf("unique token index missing, have %v", unique)
	}

	// The unique index is real: the same token cannot be inserted twice.
	createUser(t, database, "u1")
	mustExec(t, database, `INSERT INTO year_review_shares(user_id,year,token,include_players,created_at,revoked_at)
		VALUES('u1',2025,'duplicate-token',0,?,'')`, db.Now())
	if _, err := database.Exec(`INSERT INTO year_review_shares(user_id,year,token,include_players,created_at,revoked_at)
		VALUES('u1',2025,'duplicate-token',0,?,'')`, db.Now()); err == nil {
		t.Fatal("expected duplicate token to fail")
	}
}
