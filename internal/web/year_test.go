package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

// TestYearDataUsesUserTimezone proves the recap takes its year boundary from
// user_settings.timezone, not the process's local zone. A play at 23:30 on 31
// December in Los Angeles is still that year locally even though the instant
// falls on 1 January UTC; the empty and invalid zones must mirror the play
// log's UTC fallback.
func TestYearDataUsesUserTimezone(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	cfg := &config.Config{Env: "test", Timezone: "UTC"}
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := db.Now()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(13,'Catan','catan',?,?)`, now, now)
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	gameID, _ := res.LastInsertId()
	if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`, user.ID, gameID, now, now); err != nil {
		t.Fatalf("insert shelf row: %v", err)
	}

	year := time.Now().UTC().Year() - 1
	// A June play keeps both candidate years inside the allowed window.
	if _, err := database.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,quantity,created_at,updated_at) VALUES('p1',?,?,?,1,?,?)`,
		user.ID, gameID, time.Date(year, 6, 1, 12, 0, 0, 0, time.UTC).Format("2006-01-02"), now, now); err != nil {
		t.Fatalf("insert june play: %v", err)
	}
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	boundary := time.Date(year, 12, 31, 23, 30, 0, 0, la)
	if _, err := database.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,quantity,created_at,updated_at) VALUES('p2',?,?,?,1,?,?)`,
		user.ID, gameID, db.Timestamp(boundary), now, now); err != nil {
		t.Fatalf("insert boundary play: %v", err)
	}

	handler := &Handler{
		Config: cfg,
		Auth:   manager,
		Games:  collection.NewRepo(database),
		Plays:  plays.NewRepo(database),
		DB:     database,
	}

	cases := []struct {
		name     string
		timezone string
		want     int
	}{
		{"user zone keeps the local year", "America/Los_Angeles", 2},
		{"empty zone falls back to UTC", "", 1},
		{"invalid zone falls back to UTC", "Not/AZone", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := database.Exec(`UPDATE user_settings SET timezone=?, updated_at=? WHERE user_id=?`, tc.timezone, now, user.ID); err != nil {
				t.Fatalf("set timezone: %v", err)
			}
			data, err := handler.yearData(httptest.NewRequest("GET", "/year/"+time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006"), nil), user.ID, year)
			if err != nil {
				t.Fatalf("yearData: %v", err)
			}
			if data.Recap.Sessions != tc.want {
				t.Errorf("Sessions = %d, want %d for timezone %q", data.Recap.Sessions, tc.want, tc.timezone)
			}
		})
	}
}

// TestYearRecapSparseYear proves the sparse branch of the recap page renders
// through the real HTTP handler: an in-range year with no plays answers 200
// with the sparse copy and none of the populated-section headings, while the
// shelf of shame still reports a figure so the page is not empty.
func TestYearRecapSparseYear(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	cfg := &config.Config{AppName: "The Shelf", BaseURL: "http://localhost:6270", Env: "test",
		SessionTTL: time.Hour, Timezone: "UTC"}
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := db.Now()

	// Two owned games with acquisition dates: one played in 2024, one never
	// played. That gives the shelf section a real figure for 2025.
	insertGame := func(bggID int, name string) int64 {
		t.Helper()
		res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(?,?,?,?,?)`,
			bggID, name, strings.ToLower(name), now, now)
		if err != nil {
			t.Fatalf("insert game %s: %v", name, err)
		}
		id, _ := res.LastInsertId()
		if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,acquisition_date,added_at,updated_at) VALUES(?,?,1,'2020-01-15',?,?)`,
			user.ID, id, now, now); err != nil {
			t.Fatalf("insert shelf row for %s: %v", name, err)
		}
		return id
	}
	playedID := insertGame(13, "Catan")
	insertGame(14, "Unplayed")

	// The user's first play is in 2024, so 2025 is in range but has no plays.
	if _, err := database.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,quantity,created_at,updated_at) VALUES('p1',?,?,?,1,?,?)`,
		user.ID, playedID, "2024-06-01", now, now); err != nil {
		t.Fatalf("insert 2024 play: %v", err)
	}

	rr := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := manager.StartSession(rr, sessionReq, user.ID); err != nil {
		t.Fatalf("start session: %v", err)
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie was not set")
	}

	handler := &Handler{
		Config: cfg,
		Auth:   manager,
		Games:  collection.NewRepo(database),
		Plays:  plays.NewRepo(database),
		DB:     database,
	}

	req := httptest.NewRequest(http.MethodGet, "/year/2025", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /year/2025 = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "No plays were logged in 2025") {
		t.Error("sparse page is missing the sparse copy")
	}
	for _, heading := range []string{
		"Most played",
		"The heaviest thing you played",
		"First plays",
		"Your nemesis",
		"Biggest surprise",
		"Cost per hour",
	} {
		if strings.Contains(body, heading) {
			t.Errorf("sparse page unexpectedly contains section heading %q", heading)
		}
	}
	for _, want := range []string{"The shelf of shame", "1 owned game has never been played"} {
		if !strings.Contains(body, want) {
			t.Errorf("sparse page is missing the shelf figure %q", want)
		}
	}
}
