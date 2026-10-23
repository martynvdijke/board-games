package collection

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/bgg"
	fakebgg "boardgames/internal/bgg/fake"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

type syncFixture struct {
	db      *dbHandle
	userID  string
	service *SyncService
	games   *Repo
	plays   *plays.Repo
}

type dbHandle = struct{ close func() }

func setupSync(t *testing.T) (*SyncService, *Repo, string, func()) {
	t.Helper()
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Env = "test"
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE user_settings SET bgg_username='alexplays', bgg_username_verified_at=? WHERE user_id=?`, db.Now(), user.ID); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(fakebgg.Handler())
	t.Cleanup(server.Close)
	client := bgg.NewClient(server.URL+"/xmlapi2", "TheShelfTest/1.0 (test@example.com)", 100)
	games := NewRepo(database)
	playRepo := plays.NewRepo(database)
	cfg.BGGRefreshDays = 30
	service := NewSyncService(database, client, games, playRepo, cfg)
	return service, games, user.ID, func() { database.Close() }
}

func waitForSync(t *testing.T, service *SyncService, userID string) *SyncRun {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, err := service.Latest(context.Background(), userID)
		if err == nil && run.Status != "running" {
			return run
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("sync did not finish")
	return nil
}

func TestSyncImportsAndIsIdempotent(t *testing.T) {
	service, games, userID, cleanup := setupSync(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := service.Start(userID, "manual"); err != nil {
		t.Fatal(err)
	}
	run := waitForSync(t, service, userID)
	if run.Status != "success" {
		t.Fatalf("sync failed: %s (%s)", run.Status, run.Error)
	}
	if run.CollectionCount != 3 || run.ThingsCount != 3 || run.PlaysCount != 2 {
		t.Fatalf("unexpected counts: %+v", run)
	}

	list, total, err := games.List(ctx, userID, Filters{Status: "owned", PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("expected 3 games, got %d/%d", len(list), total)
	}
	var gameID int64
	if err := games.DB.QueryRow(`SELECT id FROM games WHERE bgg_id=266192`).Scan(&gameID); err != nil {
		t.Fatal(err)
	}
	detail, err := games.Get(ctx, userID, gameID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Weight < 2.3 || detail.Weight > 2.5 {
		t.Fatalf("weight not parsed: %v", detail.Weight)
	}
	if len(detail.Designers) != 1 || detail.Designers[0] != "Elizabeth Hargrave" {
		t.Fatalf("designers not parsed: %#v", detail.Designers)
	}

	// Local edits must survive a re-sync.
	if _, err := service.DB.Exec(`UPDATE user_games SET notes='my note', user_rating=9, favorite=1 WHERE user_id=? AND game_id=?`, userID, gameID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(userID, "manual"); err != nil {
		t.Fatal(err)
	}
	run = waitForSync(t, service, userID)
	if run.Status != "success" {
		t.Fatalf("second sync failed: %s (%s)", run.Status, run.Error)
	}
	var notes string
	var rating *float64
	var favorite int
	if err := service.DB.QueryRow(`SELECT notes,user_rating,favorite FROM user_games WHERE user_id=? AND game_id=?`, userID, gameID).Scan(&notes, &rating, &favorite); err != nil {
		t.Fatal(err)
	}
	if notes != "my note" || rating == nil || *rating != 9 || favorite != 1 {
		t.Fatalf("local edits lost: notes=%q rating=%v favorite=%d", notes, rating, favorite)
	}
	var playCount int
	if err := service.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND origin='bgg'`, userID).Scan(&playCount); err != nil {
		t.Fatal(err)
	}
	if playCount != 2 {
		t.Fatalf("expected 2 imported plays after re-sync, got %d", playCount)
	}
}

func TestSearchAndIsolation(t *testing.T) {
	service, games, userID, cleanup := setupSync(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := service.Start(userID, "manual"); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, service, userID)

	list, total, err := games.List(ctx, userID, Filters{Query: "wingspan"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].Name != "Wingspan" {
		t.Fatalf("FTS search failed: %d %#v", total, list)
	}

	var other string
	if err := service.DB.QueryRow(`SELECT id FROM users WHERE id<>?`, userID).Scan(&other); err == nil {
		t.Fatal("unexpected second user")
	}
}

func TestSyncRequiresUsername(t *testing.T) {
	service, _, userID, cleanup := setupSync(t)
	defer cleanup()
	if _, err := service.DB.Exec(`UPDATE user_settings SET bgg_username='' WHERE user_id=?`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(userID, "manual"); err != ErrNoBGGUsername {
		t.Fatalf("expected ErrNoBGGUsername, got %v", err)
	}
}

func TestSyncSurvivesEnrichmentFailure(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Env = "test"
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE user_settings SET bgg_username='alexplays', bgg_username_verified_at=? WHERE user_id=?`, db.Now(), user.ID); err != nil {
		t.Fatal(err)
	}

	fake := httptest.NewServer(fakebgg.Handler())
	defer fake.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/thing") {
			http.Error(w, "enrichment unavailable", http.StatusBadRequest)
			return
		}
		resp, err := http.Get(fake.URL + r.URL.Path + "?" + r.URL.RawQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer proxy.Close()

	client := bgg.NewClient(proxy.URL+"/xmlapi2", "TheShelfTest/1.0 (test@example.com)", 100)
	service := NewSyncService(database, client, NewRepo(database), plays.NewRepo(database), cfg)
	if _, err := service.Start(user.ID, "manual"); err != nil {
		t.Fatal(err)
	}
	run := waitForSync(t, service, user.ID)
	if run.Status != "partial" {
		t.Fatalf("expected partial run, got %s (%s)", run.Status, run.Error)
	}
	if run.CollectionCount == 0 {
		t.Fatalf("collection phase did not complete: %+v", run)
	}
}

func TestMalformedCollectionXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<items><item><name>broken`))
	}))
	defer server.Close()
	client := bgg.NewClient(server.URL, "TheShelfTest/1.0 (test@example.com)", 100)
	if _, err := client.Collection(context.Background(), "someone"); err == nil {
		t.Fatal("expected malformed XML to fail")
	}
}
