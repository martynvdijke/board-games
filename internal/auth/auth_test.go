package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"boardgames/internal/config"
	"boardgames/internal/db"
)

func testManager(t *testing.T) (*Manager, *User) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Env = "test"
	cfg.SessionSecret = "test-secret"
	database, err := db.OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	m := New(database, cfg)
	u, err := m.UpsertUser(context.Background(), "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	return m, u
}

func TestUpsertFirstUserIsAdmin(t *testing.T) {
	m, u := testManager(t)
	if u.Role != "admin" {
		t.Fatalf("expected admin, got %s", u.Role)
	}
	u2, err := m.UpsertUser(context.Background(), "second@example.com", "Second", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if u2.Role != "member" {
		t.Fatalf("expected member, got %s", u2.Role)
	}
}

func TestAllowlist(t *testing.T) {
	cfg, _ := config.Load()
	cfg.AllowedEmails = []string{"owner@example.com"}
	database, _ := db.OpenMemory(context.Background())
	defer database.Close()
	m := New(database, cfg)
	if _, err := m.UpsertUser(context.Background(), "other@example.com", "Other", "", true); err != ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if _, err := m.UpsertUser(context.Background(), "OWNER@example.com", "Owner", "", true); err != nil {
		t.Fatalf("expected allowed, got %v", err)
	}
}

func TestSessionAndCSRF(t *testing.T) {
	m, u := testManager(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := m.StartSession(rec, req, u.ID); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}

	protected := m.LoadSession(m.RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	// Missing CSRF token on an unsafe request.
	bad := httptest.NewRequest(http.MethodPost, "http://localhost/plays", nil)
	bad.AddCookie(cookie)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, bad)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", rr.Code)
	}

	// Load a valid CSRF token through a GET.
	get := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	get.AddCookie(cookie)
	gr := httptest.NewRecorder()
	var csrf string
	reader := m.LoadSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := FromContext(r.Context()); a != nil {
			csrf = a.CSRFToken
		}
	}))
	reader.ServeHTTP(gr, get)
	if csrf == "" {
		t.Fatal("no CSRF token in session")
	}

	ok := httptest.NewRequest(http.MethodPost, "http://localhost/plays", strings.NewReader(url.Values{"csrf_token": {csrf}}.Encode()))
	ok.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ok.AddCookie(cookie)
	orr := httptest.NewRecorder()
	protected.ServeHTTP(orr, ok)
	if orr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 with CSRF, got %d", orr.Code)
	}
}

func TestBearerBypassesCSRF(t *testing.T) {
	m, u := testManager(t)
	raw, _, err := m.CreateToken(context.Background(), u.ID, "test", []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := m.AuthenticateBearer(m.LoadSession(m.RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if !HasScope(r.Context(), "write") {
			t.Error("write scope missing")
		}
		w.WriteHeader(http.StatusOK)
	}))))
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/plays", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if !called || rr.Code != http.StatusOK {
		t.Fatalf("bearer request failed: called=%v status=%d body=%s", called, rr.Code, rr.Body.String())
	}
}

func TestExpiredSession(t *testing.T) {
	m, u := testManager(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := m.StartSession(rec, req, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.Exec(`UPDATE sessions SET expires_at=?`, db.Now()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(rec.Result().Cookies()[0])
	rr := httptest.NewRecorder()
	m.LoadSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) != nil {
			t.Error("expired session should not authenticate")
		}
	})).ServeHTTP(rr, r)
}

func TestRevokedToken(t *testing.T) {
	m, u := testManager(t)
	raw, token, err := m.CreateToken(context.Background(), u.ID, "test", []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RevokeToken(context.Background(), u.ID, token.ID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	m.AuthenticateBearer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("revoked token authenticated")
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCallbackRejectsMissingState(t *testing.T) {
	m, _ := testManager(t)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=forged", nil)
	rec := httptest.NewRecorder()
	m.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackRejectsMismatchedState(t *testing.T) {
	m, _ := testManager(t)
	stateRec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	if err := m.writeStateCookie(stateRec, statePayload{State: "expected", Verifier: "verifier", Nonce: "nonce", Expires: time.Now().Add(time.Minute).Unix()}); err != nil {
		t.Fatal(err)
	}
	cookies := stateRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no state cookie")
	}
	req.AddCookie(cookies[0])
	req.URL = mustURL(t, "/auth/callback?code=abc&state=forged")
	rec := httptest.NewRecorder()
	m.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestFirstUserOnboardingCreatesAdminSettings(t *testing.T) {
	m, u := testManager(t)
	if u.Role != "admin" {
		t.Fatalf("expected first user to be admin, got %s", u.Role)
	}
	if u.LastLoginAt == "" {
		t.Fatal("first sign-in should set last_login_at")
	}

	var theme, timezone string
	var enabled, interval int
	if err := m.db.QueryRow(`SELECT theme, timezone, bgg_sync_enabled, bgg_sync_interval_seconds FROM user_settings WHERE user_id=?`, u.ID).
		Scan(&theme, &timezone, &enabled, &interval); err != nil {
		t.Fatalf("onboarding should create user_settings: %v", err)
	}
	if theme != "system" {
		t.Errorf("expected system theme, got %q", theme)
	}
	if timezone == "" {
		t.Error("expected a timezone default")
	}
	if enabled != 1 || interval != 21600 {
		t.Errorf("expected sync enabled every 6h, got enabled=%d interval=%d", enabled, interval)
	}

	settings, err := m.Settings(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings.BGGUsername != "" || settings.Theme != "system" || !settings.SyncEnabled {
		t.Fatalf("unexpected onboarding defaults: %+v", settings)
	}
}

func TestSecondUserOnboardingIsMemberWithOwnSettings(t *testing.T) {
	m, first := testManager(t)
	second, err := m.UpsertUser(context.Background(), "Partner@Example.com", "Sam", "https://example.com/sam.png", true)
	if err != nil {
		t.Fatal(err)
	}
	if second.Role != "member" {
		t.Fatalf("expected member, got %s", second.Role)
	}
	if second.Email != "partner@example.com" {
		t.Fatalf("expected normalized email, got %s", second.Email)
	}
	if second.ID == first.ID {
		t.Fatal("second user reused the first user's ID")
	}
	var count int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id=?`, second.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one settings row for second user, got %d", count)
	}
	var firstSettings int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id=?`, first.ID).Scan(&firstSettings); err != nil {
		t.Fatal(err)
	}
	if firstSettings != 1 {
		t.Fatalf("first user settings changed: %d", firstSettings)
	}
}

func TestRepeatSignInPreservesRoleAndPreferences(t *testing.T) {
	m, u := testManager(t)
	if _, err := m.db.Exec(`UPDATE user_settings SET theme='dark', bgg_username='alexplays' WHERE user_id=?`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.Exec(`UPDATE users SET avatar_url='https://example.com/owner.png' WHERE id=?`, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureSettings(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}

	again, err := m.UpsertUser(context.Background(), "OWNER@example.com", "Alex Updated", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != u.ID {
		t.Fatal("repeat sign-in created a new account")
	}
	if again.Role != "admin" {
		t.Fatalf("repeat sign-in changed role to %s", again.Role)
	}
	if again.Name != "Alex Updated" {
		t.Fatalf("expected updated display name, got %q", again.Name)
	}
	if again.AvatarURL != "https://example.com/owner.png" {
		t.Fatalf("empty repeat-login avatar erased the existing one: %q", again.AvatarURL)
	}

	settings, err := m.Settings(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Theme != "dark" || settings.BGGUsername != "alexplays" {
		t.Fatalf("repeat sign-in overwrote preferences: %+v", settings)
	}
	var users, settingsRows int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM user_settings`).Scan(&settingsRows); err != nil {
		t.Fatal(err)
	}
	if users != 1 || settingsRows != 1 {
		t.Fatalf("expected one user and one settings row, got users=%d settings=%d", users, settingsRows)
	}
}

func TestAllowlistRequiresVerifiedDomainEmail(t *testing.T) {
	cfg, _ := config.Load()
	cfg.AllowedDomains = []string{"@example.com"}
	database, _ := db.OpenMemory(context.Background())
	defer database.Close()
	m := New(database, cfg)

	if _, err := m.UpsertUser(context.Background(), "person@example.com", "Person", "", false); err != ErrForbidden {
		t.Fatalf("expected unverified domain email to be rejected, got %v", err)
	}
	if _, err := m.UpsertUser(context.Background(), "person@example.com", "Person", "", true); err != nil {
		t.Fatalf("expected verified domain email to be allowed, got %v", err)
	}
	if _, err := m.UpsertUser(context.Background(), "person@other.test", "Person", "", true); err != ErrForbidden {
		t.Fatalf("expected outside domain to be rejected, got %v", err)
	}
}

func TestFakeAuthLoginOnboardsAdmin(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Env = "test"
	cfg.SessionSecret = "test-secret"
	cfg.DevFakeAuth = true
	cfg.DevFakeEmail = "new-owner@example.com"
	cfg.DevFakeName = "New Owner"
	database, err := db.OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	m := New(database, cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login?next=/settings", nil)
	m.Login(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/settings" {
		t.Fatalf("expected safe return-to, got %q", loc)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not create a session")
	}

	var got *User
	load := m.LoadSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserFromContext(r.Context())
	}))
	sessionReq := httptest.NewRequest(http.MethodGet, "/", nil)
	sessionReq.AddCookie(cookies[0])
	load.ServeHTTP(httptest.NewRecorder(), sessionReq)
	if got == nil {
		t.Fatal("login session did not authenticate")
	}
	if got.Email != "new-owner@example.com" || got.Role != "admin" {
		t.Fatalf("unexpected onboarded user: %+v", got)
	}
	if _, err := m.Settings(context.Background(), got.ID); err != nil {
		t.Fatalf("onboarded user has no usable settings: %v", err)
	}
}
