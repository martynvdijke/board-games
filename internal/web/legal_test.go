package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/auth"
	"boardgames/internal/config"
	"boardgames/internal/db"
)

type legalFixture struct {
	handler *Handler
	db      *sql.DB
	auth    *auth.Manager
	user    *auth.User
	cookie  *http.Cookie
	csrf    string
}

func setupLegal(t *testing.T) *legalFixture {
	t.Helper()
	ctx := context.Background()
	cfg := &config.Config{AppName: "The Shelf", BaseURL: "http://localhost:6270", SessionTTL: time.Hour,
		Timezone: "UTC", LegalHostingRegion: "the European Union"}
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := manager.StartSession(rr, req, user.ID); err != nil {
		t.Fatal(err)
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
	var csrf string
	if err := database.QueryRow(`SELECT csrf_token FROM sessions WHERE user_id = ?`, user.ID).Scan(&csrf); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Config: cfg, Auth: manager, Account: account.New(database), DB: database}
	return &legalFixture{handler: h, db: database, auth: manager, user: user, cookie: cookie, csrf: csrf}
}

func (f *legalFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(f.cookie)
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func (f *legalFixture) post(t *testing.T, path string, values url.Values, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(f.cookie)
	if csrf != "" {
		req.Header.Set(auth.CSRFHeader, csrf)
	}
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func TestPublicLegalPagesAreReadableWithoutAuth(t *testing.T) {
	f := setupLegal(t)
	for _, path := range []string{"/privacy", "/terms", "/account/delete"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		f.handler.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body)
		}
		if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s: not HTML", path)
		}
	}
	body := func(path string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		f.handler.Routes().ServeHTTP(rr, req)
		return rr.Body.String()
	}("/privacy")
	for _, want := range []string{"OpenID Connect", "Authelia", "BoardGameGeek", "Article 13", "under 16", "backups"} {
		if !strings.Contains(body, want) {
			t.Errorf("privacy page missing %q", want)
		}
	}
}

func TestLegalPagesUseConfiguredController(t *testing.T) {
	f := setupLegal(t)
	f.handler.Config.LegalControllerName = "Shelf Collective"
	f.handler.Config.LegalContactEmail = "privacy@example.com"
	f.handler.Config.LegalContactAddress = "1 Example Street"
	f.handler.Config.LegalHostingRegion = "Germany"
	rr := f.get(t, "/privacy")
	body := rr.Body.String()
	for _, want := range []string{"Shelf Collective", "privacy@example.com", "1 Example Street", "Germany"} {
		if !strings.Contains(body, want) {
			t.Errorf("privacy page missing %q", want)
		}
	}
}

func TestDeleteAccountRequiresCSRF(t *testing.T) {
	f := setupLegal(t)
	rr := f.post(t, "/settings/account/delete", url.Values{"confirm": {"DELETE"}}, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", rr.Code)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, f.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("account was deleted without a CSRF token")
	}
}

func TestDeleteAccountRequiresTypedConfirmationAndRemovesEverything(t *testing.T) {
	f := setupLegal(t)
	ctx := context.Background()
	now := db.Now()
	res, err := f.db.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(1,'Catan','catan',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := res.LastInsertId()
	if _, err := f.db.ExecContext(ctx, `INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('second-admin','second@example.com','Second','admin',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`, f.user.ID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,origin,created_at,updated_at) VALUES('p1',?,?,'2026-01-01','local',?,?)`, f.user.ID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO play_players(play_id,name) VALUES('p1','Alex')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.auth.CreateToken(ctx, f.user.ID, "phone", []string{"read"}); err != nil {
		t.Fatal(err)
	}

	rr := f.post(t, "/settings/account/delete", url.Values{"confirm": {"yes"}}, f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("wrong confirmation should redirect, got %d", rr.Code)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, f.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("wrong confirmation deleted the account")
	}

	rr = f.post(t, "/settings/account/delete", url.Values{"confirm": {"DELETE"}}, f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "/account/delete") {
		t.Fatalf("unexpected redirect %q", loc)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM users WHERE id = ?`,
		`SELECT COUNT(*) FROM sessions WHERE user_id = ?`,
		`SELECT COUNT(*) FROM api_tokens WHERE user_id = ?`,
		`SELECT COUNT(*) FROM user_settings WHERE user_id = ?`,
		`SELECT COUNT(*) FROM user_games WHERE user_id = ?`,
		`SELECT COUNT(*) FROM plays WHERE user_id = ?`,
		`SELECT COUNT(*) FROM play_players WHERE play_id = 'p1'`,
	} {
		var count int
		if err := f.db.QueryRow(q, f.user.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%q left %d rows", q, count)
		}
	}
	// The browser session cookie should be expired.
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookie && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("session cookie was not cleared")
	}
}

func TestDeleteAccountRefusesLastAdmin(t *testing.T) {
	f := setupLegal(t)
	rr := f.post(t, "/settings/account/delete", url.Values{"confirm": {"DELETE"}}, f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, f.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("last admin was deleted")
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "flash=") {
		t.Fatalf("expected a flash error redirect, got %q", loc)
	}
}

func TestPlayerRemovalWebIsScopedToOwnPlays(t *testing.T) {
	f := setupLegal(t)
	ctx := context.Background()
	if _, err := f.db.ExecContext(ctx, `UPDATE users SET role='member' WHERE id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	other, err := f.auth.UpsertUser(ctx, "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	now := db.Now()
	res, err := f.db.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(1,'Catan','catan',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := res.LastInsertId()
	for _, row := range []struct{ id, user string }{{"mine", f.user.ID}, {"theirs", other.ID}} {
		if _, err := f.db.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,origin,created_at,updated_at)
			VALUES(?,?,?,'2026-01-01','local',?,?)`, row.id, row.user, gameID, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(ctx, `INSERT INTO play_players(play_id,name) VALUES(?,'Taylor')`, row.id); err != nil {
			t.Fatal(err)
		}
	}
	rr := f.post(t, "/settings/players/remove", url.Values{"name": {"Taylor"}, "mode": {"remove"}}, f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("remove player: %d %s", rr.Code, rr.Body)
	}
	for id, want := range map[string]int{"mine": 0, "theirs": 1} {
		var n int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM play_players WHERE play_id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("play %s has %d player rows, want %d", id, n, want)
		}
	}
}
