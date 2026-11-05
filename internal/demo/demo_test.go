package demo_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"boardgames/internal/auth"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/demo"
)

func setupDemo(t *testing.T) (*demo.Service, *sql.DB, *auth.Manager, *config.Config) {
	t.Helper()
	ctx := context.Background()
	cfg := &config.Config{
		AppName: "The Shelf", BaseURL: "http://localhost:6270", Timezone: "UTC",
		SessionSecret: "test-secret", DemoUserEnabled: true, DemoUserEmail: "demo@theshelf.local",
		DemoUserName: "Shelf Demo", DemoUserToken: "bg_demo_test_token_1234567890",
	}
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	manager := auth.New(database, cfg)
	service := demo.New(database, manager, cfg)
	return service, database, manager, cfg
}

func TestEnsureCreatesWorkingDemoAccount(t *testing.T) {
	ctx := context.Background()
	service, database, manager, cfg := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	var id, role string
	var isDemo int
	if err := database.QueryRow(`SELECT id,role,is_demo FROM users WHERE email=?`, cfg.DemoUserEmail).Scan(&id, &role, &isDemo); err != nil {
		t.Fatal(err)
	}
	if role != "member" {
		t.Errorf("demo role = %q, want member", role)
	}
	if isDemo != 1 {
		t.Errorf("is_demo = %d, want 1", isDemo)
	}

	ac, err := manager.TokenAuth(ctx, cfg.DemoUserToken)
	if err != nil {
		t.Fatalf("demo token did not authenticate: %v", err)
	}
	if ac.User.ID != id {
		t.Errorf("token user = %q, want %q", ac.User.ID, id)
	}
	if !ac.User.IsDemo {
		t.Error("token user was not marked as demo")
	}

	var settings int
	if err := database.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id=?`, id).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if settings != 1 {
		t.Errorf("settings rows = %d, want 1", settings)
	}

	var shelf, plays int
	if err := database.QueryRow(`SELECT COUNT(*) FROM user_games WHERE user_id=?`, id).Scan(&shelf); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, id).Scan(&plays); err != nil {
		t.Fatal(err)
	}
	if shelf == 0 {
		t.Error("demo shelf is empty")
	}
	if plays == 0 {
		t.Error("demo play log is empty")
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	ctx := context.Background()
	service, database, _, cfg := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	var firstID string
	if err := database.QueryRow(`SELECT id FROM users WHERE email=?`, cfg.DemoUserEmail).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	if err := service.Ensure(ctx); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	var secondID string
	if err := database.QueryRow(`SELECT id FROM users WHERE email=?`, cfg.DemoUserEmail).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	if firstID != secondID {
		t.Errorf("demo user id changed from %q to %q", firstID, secondID)
	}
	var tokens int
	if err := database.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE user_id=? AND name='Play Store demo'`, firstID).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 1 {
		t.Errorf("demo tokens = %d, want 1", tokens)
	}
	var shelf int
	if err := database.QueryRow(`SELECT COUNT(*) FROM user_games WHERE user_id=?`, firstID).Scan(&shelf); err != nil {
		t.Fatal(err)
	}
	if shelf == 0 {
		t.Error("second ensure removed the shelf")
	}
}

func TestEnsureRefusesNonDemoEmailTakeover(t *testing.T) {
	ctx := context.Background()
	service, _, manager, cfg := setupDemo(t)
	if _, err := manager.UpsertUser(ctx, cfg.DemoUserEmail, "Real Person", "", true); err != nil {
		t.Fatal(err)
	}
	err := service.Ensure(ctx)
	if err == nil {
		t.Fatal("expected Ensure to refuse an existing non-demo email")
	}
	var isDemo int
	if err := service.DB.QueryRow(`SELECT is_demo FROM users WHERE email=?`, cfg.DemoUserEmail).Scan(&isDemo); err != nil {
		t.Fatal(err)
	}
	if isDemo != 0 {
		t.Error("Ensure converted the real account into a demo account")
	}
}

func TestEnsureDoesNotOverwriteExistingGame(t *testing.T) {
	ctx := context.Background()
	service, database, _, _ := setupDemo(t)
	now := db.Now()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,description,created_at,updated_at)
		VALUES(13,'Catan','catan','Distinct real Catan description.',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO categories(name) VALUES('Custom Category')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO game_categories(game_id,category_id)
		SELECT ?,id FROM categories WHERE name='Custom Category'`, gameID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO designers(name) VALUES('Custom Designer')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO game_designers(game_id,designer_id)
		SELECT ?,id FROM designers WHERE name='Custom Designer'`, gameID); err != nil {
		t.Fatal(err)
	}

	if err := service.Ensure(ctx); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	var description string
	if err := database.QueryRow(`SELECT description FROM games WHERE bgg_id=13`).Scan(&description); err != nil {
		t.Fatal(err)
	}
	if description != "Distinct real Catan description." {
		t.Errorf("description = %q, shared game row was overwritten", description)
	}
	var category string
	if err := database.QueryRow(`SELECT c.name FROM categories c JOIN game_categories gc ON gc.category_id=c.id WHERE gc.game_id=?`, gameID).Scan(&category); err != nil {
		t.Fatal(err)
	}
	if category != "Custom Category" {
		t.Errorf("category = %q, shared links were replaced", category)
	}
	var designer string
	if err := database.QueryRow(`SELECT d.name FROM designers d JOIN game_designers gd ON gd.designer_id=d.id WHERE gd.game_id=?`, gameID).Scan(&designer); err != nil {
		t.Fatal(err)
	}
	if designer != "Custom Designer" {
		t.Errorf("designer = %q, shared links were replaced", designer)
	}
	var shelf int
	if err := database.QueryRow(`SELECT COUNT(*) FROM user_games ug JOIN users u ON u.id=ug.user_id
		WHERE u.is_demo=1 AND ug.game_id=?`, gameID).Scan(&shelf); err != nil {
		t.Fatal(err)
	}
	if shelf != 1 {
		t.Errorf("demo user_games rows for Catan = %d, want 1", shelf)
	}
}

func TestEnsureDisablingRevokesAccess(t *testing.T) {
	ctx := context.Background()
	service, database, manager, cfg := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	var userID string
	if err := database.QueryRow(`SELECT id FROM users WHERE is_demo=1`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := manager.StartSession(rr, req, userID); err != nil {
		t.Fatal(err)
	}
	var session *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("session cookie was not set")
	}
	if _, err := manager.TokenAuth(ctx, cfg.DemoUserToken); err != nil {
		t.Fatalf("token should work before disabling: %v", err)
	}

	cfg.DemoUserEnabled = false
	if err := service.Ensure(ctx); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := manager.TokenAuth(ctx, cfg.DemoUserToken); err == nil {
		t.Error("demo token still authenticates after disabling")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(session)
	if _, err := manager.SessionFromRequest(req); err == nil {
		t.Error("demo session still authenticates after disabling")
	}
	var sessions, tokens int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=?`, userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE user_id=?`, userID).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || tokens != 0 {
		t.Errorf("credential rows remain: sessions=%d tokens=%d", sessions, tokens)
	}
}

func TestEnsureUpdatesDemoEmailSafely(t *testing.T) {
	ctx := context.Background()
	service, database, manager, cfg := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	var originalID string
	if err := database.QueryRow(`SELECT id FROM users WHERE is_demo=1`).Scan(&originalID); err != nil {
		t.Fatal(err)
	}

	cfg.DemoUserEmail = "renamed@theshelf.local"
	if err := service.Ensure(ctx); err != nil {
		t.Fatalf("rename: %v", err)
	}
	var id, email string
	if err := database.QueryRow(`SELECT id,email FROM users WHERE is_demo=1`).Scan(&id, &email); err != nil {
		t.Fatal(err)
	}
	if id != originalID || email != cfg.DemoUserEmail {
		t.Errorf("canonical demo = %q/%q, want %q/%q", id, email, originalID, cfg.DemoUserEmail)
	}
	var demos int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users WHERE is_demo=1`).Scan(&demos); err != nil {
		t.Fatal(err)
	}
	if demos != 1 {
		t.Errorf("demo users = %d, want 1", demos)
	}
	if _, err := manager.TokenAuth(ctx, cfg.DemoUserToken); err != nil {
		t.Errorf("token stopped working after email change: %v", err)
	}

	other, err := manager.UpsertUser(ctx, "taken@example.com", "Real Person", "", true)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DemoUserEmail = "taken@example.com"
	if err := service.Ensure(ctx); err == nil {
		t.Fatal("expected Ensure to refuse a non-demo email collision")
	}
	var collisionID string
	var isDemo int
	if err := database.QueryRow(`SELECT id,is_demo FROM users WHERE email='taken@example.com'`).Scan(&collisionID, &isDemo); err != nil {
		t.Fatal(err)
	}
	if collisionID != other.ID || isDemo != 0 {
		t.Error("real user was modified by the demo collision")
	}
}

func TestDemoTokenIsStoredHashed(t *testing.T) {
	ctx := context.Background()
	service, database, _, cfg := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := database.QueryRow(`SELECT t.token_hash FROM api_tokens t JOIN users u ON u.id=t.user_id
		WHERE u.is_demo=1 AND t.name='Play Store demo'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(cfg.DemoUserToken))
	want := hex.EncodeToString(sum[:])
	if stored == cfg.DemoUserToken {
		t.Error("raw demo token was stored in api_tokens")
	}
	if stored != want {
		t.Errorf("stored hash = %q, want %q", stored, want)
	}
}

func TestDemoOnlyInstanceMakesFirstRealUserAdmin(t *testing.T) {
	ctx := context.Background()
	service, _, manager, _ := setupDemo(t)
	if err := service.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != "admin" {
		t.Errorf("first real user role = %q, want admin", user.Role)
	}
}
