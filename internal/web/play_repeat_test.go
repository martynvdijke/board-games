package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"boardgames/internal/venues"
)

type playFormFixture struct {
	handler *Handler
	db      *sql.DB
	repo    *plays.Repo
	owner   *auth.User
	other   *auth.User
	gameID  int64
	cookie  *http.Cookie
	csrf    string
}

func setupPlayForm(t *testing.T) *playFormFixture {
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
	now := db.Now()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(13,'Catan','catan',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := res.LastInsertId()
	for _, userID := range []string{owner.ID, other.ID} {
		if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`, userID, gameID, now, now); err != nil {
			t.Fatal(err)
		}
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
	var csrf string
	if err := database.QueryRow(`SELECT csrf_token FROM sessions WHERE user_id=?`, owner.ID).Scan(&csrf); err != nil {
		t.Fatal(err)
	}
	games := collection.NewRepo(database)
	repo := plays.NewRepo(database)
	venueRepo := venues.NewRepo(database)
	repo.SetVenueResolver(venueRepo)
	h := &Handler{Config: cfg, Auth: manager, Account: account.New(database), Games: games, Plays: repo, Venues: venueRepo, DB: database}
	return &playFormFixture{handler: h, db: database, repo: repo, owner: owner, other: other, gameID: gameID, cookie: cookie, csrf: csrf}
}

func (f *playFormFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(f.cookie)
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func (f *playFormFixture) post(t *testing.T, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(auth.CSRFHeader, f.csrf)
	req.AddCookie(f.cookie)
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func TestRepeatPlayPrefillsPlayers(t *testing.T) {
	f := setupPlayForm(t)
	ctx := context.Background()
	source, err := f.repo.Create(ctx, f.owner.ID, plays.Input{GameID: f.gameID, PlayedAt: "2026-03-04", Quantity: 2,
		Players: []plays.Player{{Name: "Alex", Score: "12", IsWinner: true, IsFirst: true}, {Name: "Sam", Score: "9"}}}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	rr := f.get(t, "/plays/new?from="+source.ID)
	if rr.Code != http.StatusOK {
		t.Fatalf("repeat form: %d %s", rr.Code, rr.Body)
	}
	body := rr.Body.String()
	for _, want := range []string{"Alex", "Sam", "value=\"12\"", "value=\"9\"", "name=\"client_id\""} {
		if !strings.Contains(body, want) {
			t.Errorf("prefilled form missing %q", want)
		}
	}
	// The repeat resets the first-play flag; it is a new session.
	if strings.Contains(body, "name=\"player_first\" value=\"0\" checked") {
		t.Error("first-play flag should reset on a repeated lineup")
	}
}

func TestRepeatPlayFromAnotherUserIsNotFound(t *testing.T) {
	f := setupPlayForm(t)
	ctx := context.Background()
	theirs, err := f.repo.Create(ctx, f.other.ID, plays.Input{GameID: f.gameID, PlayedAt: "2026-03-05", Quantity: 1,
		Players: []plays.Player{{Name: "Mallory"}}}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	rr := f.get(t, "/plays/new?from="+theirs.ID)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("cross-user repeat: got %d, want 404", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "Mallory") {
		t.Fatal("another user's player names leaked into the response")
	}
}

func TestPlayFormDoubleSubmitCreatesOnePlay(t *testing.T) {
	f := setupPlayForm(t)
	values := url.Values{
		"game_id":     {strconv.FormatInt(f.gameID, 10)},
		"played_at":   {"2026-03-06"},
		"quantity":    {"1"},
		"player_name": {"Alex"},
		"client_id":   {"double-submit-key"},
	}
	for i := 0; i < 2; i++ {
		if rr := f.post(t, "/plays", values); rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
			t.Fatalf("submit %d: %d %s", i, rr.Code, rr.Body)
		}
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, f.owner.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one play after a double submit, got %d", count)
	}
}
