package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

type insightFixture struct {
	handler *Handler
	db      *sql.DB
	owner   *auth.User
	other   *auth.User
	cookie  *http.Cookie
	seq     int
}

func setupInsightFixture(t *testing.T) *insightFixture {
	t.Helper()
	ctx := context.Background()
	cfg := &config.Config{AppName: "The Shelf", BaseURL: "http://localhost:6270", SessionTTL: time.Hour, Timezone: "UTC"}
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	manager := auth.New(database, cfg)
	owner, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.UpsertUser(ctx, "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := manager.StartSession(rec, req, owner.ID); err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie was not set")
	}
	h := &Handler{Config: cfg, Auth: manager, Account: account.New(database),
		Games: collection.NewRepo(database), Plays: plays.NewRepo(database), DB: database}
	return &insightFixture{handler: h, db: database, owner: owner, other: other, cookie: cookie}
}

func (f *insightFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(f.cookie)
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

// shelfGame inserts a game and puts it on a user's shelf, returning its id.
func (f *insightFixture) shelfGame(t *testing.T, userID, name string, minPlaytime, maxPlaytime, bggNumPlays int) int64 {
	t.Helper()
	now := db.Now()
	var bggID int
	if err := f.db.QueryRow(`SELECT COALESCE(MAX(bgg_id),0)+1 FROM games`).Scan(&bggID); err != nil {
		t.Fatal(err)
	}
	res, err := f.db.Exec(`INSERT INTO games(bgg_id,name,sort_name,min_playtime,max_playtime,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?)`, bggID, name, strings.ToLower(name), nullableIntValue(minPlaytime), nullableIntValue(maxPlaytime), now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := res.LastInsertId()
	if _, err := f.db.Exec(`INSERT INTO user_games(user_id,game_id,owned,bgg_num_plays,added_at,updated_at)
		VALUES(?,?,1,?,?,?)`, userID, gameID, bggNumPlays, now, now); err != nil {
		t.Fatal(err)
	}
	return gameID
}

func (f *insightFixture) play(t *testing.T, userID string, gameID int64, duration int) {
	t.Helper()
	f.seq++
	now := db.Now()
	id := "insight-" + strconv.Itoa(f.seq)
	if _, err := f.db.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,duration_minutes,quantity,created_at,updated_at)
		VALUES(?,?,?,'2026-03-04',?,1,?,?)`, id, userID, gameID, nullableIntValue(duration), now, now); err != nil {
		t.Fatal(err)
	}
}

func nullableIntValue(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}

func TestGamePageShowsPlaytimeCalibration(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "Calibration", 60, 90, 0)
	for i := 0; i < 3; i++ {
		f.play(t, f.owner.ID, gameID, 95)
	}
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	for _, want := range []string{"You average 95 minutes", "the box says 60–90 min", "That runs long."} {
		if !strings.Contains(body, want) {
			t.Errorf("game page missing %q", want)
		}
	}
}

func TestGamePageHidesCalibrationBelowThreshold(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "Sparse", 60, 90, 0)
	for i := 0; i < 2; i++ {
		f.play(t, f.owner.ID, gameID, 120)
	}
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	if strings.Contains(body, "You average") {
		t.Fatal("calibration line shown with fewer than three timed plays")
	}
}

func TestGamePageCalibrationIgnoresAnotherUsersPlays(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "Shared shelf", 60, 90, 0)
	// The other user has plenty of timed plays; the owner has two, so the
	// owner must still see nothing.
	for i := 0; i < 5; i++ {
		f.play(t, f.other.ID, gameID, 120)
	}
	for i := 0; i < 2; i++ {
		f.play(t, f.owner.ID, gameID, 150)
	}
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	if strings.Contains(body, "You average") {
		t.Fatal("another user's plays leaked into the owner's calibration")
	}
}

func TestGamePageShowsBGGPlayGap(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "Reconcile", 0, 0, 14)
	for i := 0; i < 9; i++ {
		f.play(t, f.owner.ID, gameID, 0)
	}
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	if !strings.Contains(body, "BGG says 14 plays; you") || !strings.Contains(body, "logged 9.") {
		t.Fatalf("game page missing the BGG play-count gap, got:\n%s", body)
	}
	if !strings.Contains(body, "/plays/new?game="+strconv.FormatInt(gameID, 10)) {
		t.Error("game page missing the prefilled log-a-play link")
	}
}

func TestGamePageHidesBGGGapWhenCaughtUp(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "Caught up", 0, 0, 4)
	for i := 0; i < 4; i++ {
		f.play(t, f.owner.ID, gameID, 0)
	}
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	if strings.Contains(body, "BGG says") {
		t.Fatal("BGG gap shown when the local log has caught up")
	}
}

func TestGamePageHidesBGGGapWithoutBGGCount(t *testing.T) {
	f := setupInsightFixture(t)
	gameID := f.shelfGame(t, f.owner.ID, "No BGG count", 0, 0, 0)
	f.play(t, f.owner.ID, gameID, 0)
	body := f.get(t, "/games/"+strconv.FormatInt(gameID, 10)).Body.String()
	if strings.Contains(body, "BGG says") {
		t.Fatal("BGG gap shown without a BGG count")
	}
}

func TestStatsPageShowsGroupInsights(t *testing.T) {
	f := setupInsightFixture(t)
	// Box midpoints 75 and 45; six plays of 90 and 60 minutes give
	// 450 logged against 360 box minutes, i.e. 25% over.
	a := f.shelfGame(t, f.owner.ID, "Group A", 60, 90, 10)
	b := f.shelfGame(t, f.owner.ID, "Group B", 30, 60, 5)
	for i := 0; i < 3; i++ {
		f.play(t, f.owner.ID, a, 90)
		f.play(t, f.owner.ID, b, 60)
	}
	// BGG is ahead on both games: three plays of each are only on BGG.
	for i := 0; i < 4; i++ {
		f.play(t, f.owner.ID, a, 0)
	}
	f.play(t, f.owner.ID, b, 0)

	body := f.get(t, "/stats").Body.String()
	if !strings.Contains(body, "run 25% over the box") || !strings.Contains(body, "from 6 timed plays across 2 games") {
		t.Fatalf("stats page missing the overrun line, got:\n%s", body)
	}
	if !strings.Contains(body, "2 games have plays only BGG knows about") || !strings.Contains(body, "4 plays in all") {
		t.Fatalf("stats page missing the BGG catch-up line, got:\n%s", body)
	}
}

func TestStatsPageHidesGroupInsightsWhenNotEnoughData(t *testing.T) {
	f := setupInsightFixture(t)
	a := f.shelfGame(t, f.owner.ID, "Lonely", 60, 90, 3)
	for i := 0; i < 3; i++ {
		f.play(t, f.owner.ID, a, 90)
	}
	body := f.get(t, "/stats").Body.String()
	if strings.Contains(body, "On average your games") {
		t.Fatal("group overrun shown for a single qualified game")
	}
	if strings.Contains(body, "plays only BGG knows about") {
		t.Fatal("BGG catch-up shown when the local log has caught up")
	}
}

func TestStatsInsightsIgnoreAnotherUsersData(t *testing.T) {
	f := setupInsightFixture(t)
	a := f.shelfGame(t, f.owner.ID, "Owner game", 60, 90, 0)
	f.play(t, f.owner.ID, a, 95)
	// The other user's shelf rows and plays must never reach the owner's page.
	b := f.shelfGame(t, f.other.ID, "Other game", 60, 90, 20)
	for i := 0; i < 5; i++ {
		f.play(t, f.other.ID, b, 300)
	}
	body := f.get(t, "/stats").Body.String()
	if strings.Contains(body, "On average your games") {
		t.Fatal("another user's plays leaked into the owner's group calibration")
	}
	if strings.Contains(body, "plays only BGG knows about") {
		t.Fatal("another user's BGG counts leaked into the owner's stats")
	}
}
