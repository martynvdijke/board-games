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

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

type settingsFixture struct {
	handler *Handler
	db      *sql.DB
	auth    *auth.Manager
	owner   *auth.User
	cookie  *http.Cookie
	csrf    string
}

const (
	currentUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
	otherUA   = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
)

func setupSettings(t *testing.T) *settingsFixture {
	t.Helper()
	ctx := context.Background()
	cfg := &config.Config{AppName: "The Shelf", BaseURL: "http://localhost:6270", SessionTTL: time.Hour,
		Timezone: "UTC"}
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	manager := auth.New(database, cfg)
	owner, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpsertUser(ctx, "other@example.com", "Other", "", true); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", currentUA)
	req.RemoteAddr = "203.0.113.4:5000"
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
	var csrf string
	if err := database.QueryRow(`SELECT csrf_token FROM sessions WHERE user_id=? AND user_agent=?`, owner.ID, currentUA).Scan(&csrf); err != nil {
		t.Fatal(err)
	}

	playsRepo := plays.NewRepo(database)
	h := &Handler{
		Config: cfg,
		Auth:   manager,
		DB:     database,
		Sync:   collection.NewSyncService(database, nil, collection.NewRepo(database), playsRepo, cfg),
		Plays:  playsRepo,
	}
	return &settingsFixture{handler: h, db: database, auth: manager, owner: owner, cookie: cookie, csrf: csrf}
}

// addSession creates another browser session and returns its public ID and
// stored token hash so a test can assert the hash never reaches a page.
func (f *settingsFixture) addSession(t *testing.T, userID, ua, ip string) (string, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", ua)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	if err := f.auth.StartSession(rec, req, userID); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := f.db.QueryRow(`SELECT token_hash FROM sessions WHERE user_id=? AND user_agent=?`, userID, ua).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return auth.SessionID(hash), hash
}

func (f *settingsFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(f.cookie)
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func (f *settingsFixture) post(t *testing.T, path string, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(url.Values{}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(f.cookie)
	if csrf != "" {
		req.Header.Set(auth.CSRFHeader, csrf)
	}
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func TestSettingsShowsSignedInDevicesWithoutSecrets(t *testing.T) {
	f := setupSettings(t)
	_, otherHash := f.addSession(t, f.owner.ID, otherUA, "198.51.100.9:6000")

	rr := f.get(t, "/settings")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /settings = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Where you're signed in",
		"Chrome on macOS",
		"Safari on iPhone",
		"This device",
		"203.0.113.4:5000",
		"198.51.100.9:6000",
		"Last seen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings page missing %q", want)
		}
	}
	if strings.Contains(body, otherHash) {
		t.Error("settings page leaked a session token hash")
	}
	var otherCSRF string
	if err := f.db.QueryRow(`SELECT csrf_token FROM sessions WHERE user_id=? AND user_agent=?`, f.owner.ID, otherUA).Scan(&otherCSRF); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, otherCSRF) {
		t.Error("settings page leaked another session's CSRF token")
	}
}

func TestRevokeSessionRequiresCSRF(t *testing.T) {
	f := setupSettings(t)
	id, hash := f.addSession(t, f.owner.ID, otherUA, "198.51.100.9:6000")

	rr := f.post(t, "/settings/sessions/"+id+"/revoke", "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF = %d, want 403", rr.Code)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash=?`, hash).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("session was deleted without a CSRF token")
	}
}

func TestRevokeAnotherUsersSessionReturnsNotFound(t *testing.T) {
	f := setupSettings(t)
	var otherID string
	if err := f.db.QueryRow(`SELECT id FROM users WHERE email='other@example.com'`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	victimID, victimHash := f.addSession(t, otherID, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/127.0", "192.0.2.7:7000")

	rr := f.post(t, "/settings/sessions/"+victimID+"/revoke", f.csrf)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("revoking another user's session = %d, want 404", rr.Code)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash=?`, victimHash).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("another user's session was deleted")
	}
}

func TestRevokeOtherDeviceKeepsCurrentSession(t *testing.T) {
	f := setupSettings(t)
	id, _ := f.addSession(t, f.owner.ID, otherUA, "198.51.100.9:6000")

	rr := f.post(t, "/settings/sessions/"+id+"/revoke", f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("revoke = %d, want 303 (body %s)", rr.Code, rr.Body)
	}
	if loc := rr.Header().Get("Location"); !strings.HasPrefix(loc, "/settings?") {
		t.Fatalf("redirect = %q, want back to settings", loc)
	}
	if rr := f.get(t, "/settings"); rr.Code != http.StatusOK {
		t.Fatalf("current session no longer valid after revoking another device: %d", rr.Code)
	}
}

func TestRevokeCurrentSessionSignsOut(t *testing.T) {
	f := setupSettings(t)
	currentID := auth.SessionID(mustSessionHash(t, f.db, f.owner.ID, currentUA))

	rr := f.post(t, "/settings/sessions/"+currentID+"/revoke", f.csrf)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("revoke current = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Fatalf("redirect = %q, want /login", loc)
	}
}

func TestTokenListShowsPrefixAndLastUsed(t *testing.T) {
	f := setupSettings(t)
	ctx := context.Background()
	raw, token, err := f.auth.CreateToken(ctx, f.owner.ID, "TRMNL display", []string{"read", "trmnl"})
	if err != nil {
		t.Fatal(err)
	}
	body := f.get(t, "/settings").Body.String()
	if !strings.Contains(body, token.Prefix+"…") {
		t.Errorf("token list missing prefix %q", token.Prefix)
	}
	if !strings.Contains(body, "never used") {
		t.Error("token list missing a never-used label")
	}
	_ = raw

	if _, err := f.db.Exec(`UPDATE api_tokens SET last_used_at=? WHERE id=?`, "2026-03-02T10:00:00Z", token.ID); err != nil {
		t.Fatal(err)
	}
	body = f.get(t, "/settings").Body.String()
	if !strings.Contains(body, "used 2 Mar 2026") {
		t.Error("token list missing the last-used date")
	}
}

func mustSessionHash(t *testing.T, database *sql.DB, userID, ua string) string {
	t.Helper()
	var hash string
	if err := database.QueryRow(`SELECT token_hash FROM sessions WHERE user_id=? AND user_agent=?`, userID, ua).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash
}
