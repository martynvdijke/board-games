package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"boardgames/internal/account"
	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
	"boardgames/internal/trmnl"
)

type fixture struct {
	handler *Handler
	db      *sql.DB
	user    *auth.User
	read    string
	write   string
	full    string
	gameID  int64
}

func setupAPI(t *testing.T) *fixture {
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
	t.Cleanup(func() { database.Close() })
	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	now := db.Now()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,year_published,min_players,max_players,min_playtime,max_playtime,weight,created_at,updated_at) VALUES(13,'Catan','catan',1995,3,4,60,120,2.3,?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := res.LastInsertId()
	if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`, user.ID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
	read, _, _ := manager.CreateToken(ctx, user.ID, "read", []string{"read"})
	write, _, _ := manager.CreateToken(ctx, user.ID, "write", []string{"read", "write"})
	full, _, _ := manager.CreateToken(ctx, user.ID, "full", []string{"read", "write", "trmnl"})
	games := collection.NewRepo(database)
	playRepo := plays.NewRepo(database)
	handler := &Handler{Config: cfg, Auth: manager, Games: games, Plays: playRepo, DB: database,
		Account: account.New(database), TRMNL: trmnl.NewService(database, games, playRepo, cfg)}
	return &fixture{handler: handler, db: database, user: user, read: read, write: write, full: full, gameID: gameID}
}

func (f *fixture) request(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.handler.Routes().ServeHTTP(rr, req)
	return rr
}

func decode(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, rr.Body.String())
	}
	return out
}

func TestAuthAndScopes(t *testing.T) {
	f := setupAPI(t)
	if rr := f.request(t, http.MethodGet, "/v1/me", "", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	rr := f.request(t, http.MethodGet, "/v1/me", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("me: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	data := body["data"].(map[string]any)
	if data["email"] != "owner@example.com" {
		t.Fatalf("wrong user: %v", data)
	}
	// Read-only token cannot write.
	rr = f.request(t, http.MethodPatch, "/v1/games/"+itoa(f.gameID), f.read, map[string]any{"favorite": true})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body)
	}
	// TRMNL scope is separate.
	if rr := f.request(t, http.MethodGet, "/v1/trmnl", f.read, nil); rr.Code != http.StatusForbidden {
		t.Fatalf("expected trmnl scope failure, got %d", rr.Code)
	}
	if rr := f.request(t, http.MethodGet, "/v1/trmnl", f.full, nil); rr.Code != http.StatusOK {
		t.Fatalf("trmnl: %d %s", rr.Code, rr.Body)
	}
}

func TestGamesAndCollectionPatch(t *testing.T) {
	f := setupAPI(t)
	rr := f.request(t, http.MethodGet, "/v1/games?limit=1&offset=0", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("games: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	meta := body["pagination"].(map[string]any)
	if meta["total"].(float64) != 1 || meta["has_more"].(bool) {
		t.Fatalf("pagination: %v", meta)
	}
	rr = f.request(t, http.MethodPatch, "/v1/games/"+itoa(f.gameID), f.write, map[string]any{"favorite": true, "user_rating": 9.5, "notes": "great"})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body)
	}
	var favorite int
	var notes string
	if err := f.db.QueryRow(`SELECT favorite,notes FROM user_games WHERE user_id=? AND game_id=?`, f.user.ID, f.gameID).Scan(&favorite, &notes); err != nil {
		t.Fatal(err)
	}
	if favorite != 1 || notes != "great" {
		t.Fatalf("patch not saved: %d %q", favorite, notes)
	}
	rr = f.request(t, http.MethodPatch, "/v1/games/"+itoa(f.gameID), f.write, map[string]any{"user_rating": 77})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for bad rating, got %d", rr.Code)
	}
}

func TestPlayLifecycleAndValidation(t *testing.T) {
	f := setupAPI(t)
	payload := map[string]any{"game_id": f.gameID, "played_at": "2026-05-01", "duration_minutes": 60, "quantity": 1,
		"players": []map[string]any{{"name": "Alex", "score": "10", "is_winner": true}}}
	rr := f.request(t, http.MethodPost, "/v1/plays", f.write, payload)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	play := body["data"].(map[string]any)
	id := play["id"].(string)
	if id == "" {
		t.Fatal("missing play id")
	}
	if rr := f.request(t, http.MethodPatch, "/v1/plays/"+id, f.write, map[string]any{"notes": "edited"}); rr.Code != http.StatusOK {
		t.Fatalf("patch play: %d %s", rr.Code, rr.Body)
	}
	// Cross-user access is a 404.
	other, _ := f.handler.Auth.UpsertUser(context.Background(), "other@example.com", "Other", "", true)
	otherToken, _, _ := f.handler.Auth.CreateToken(context.Background(), other.ID, "other", []string{"read", "write"})
	if rr := f.request(t, http.MethodGet, "/v1/plays/"+id, otherToken, nil); rr.Code != http.StatusNotFound {
		t.Fatalf("cross-user play read: %d", rr.Code)
	}
	if rr := f.request(t, http.MethodDelete, "/v1/plays/"+id, f.write, nil); rr.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rr.Code)
	}
	// Invalid payloads return field-level 422 envelope.
	bad := map[string]any{"game_id": 9999, "played_at": "not-a-date", "quantity": 1}
	rr = f.request(t, http.MethodPost, "/v1/plays", f.write, bad)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body)
	}
	env := decode(t, rr)
	if env["error"] == nil {
		t.Fatalf("missing error envelope: %v", env)
	}
}

func TestPaginationValidation(t *testing.T) {
	f := setupAPI(t)
	for _, q := range []string{"limit=0", "limit=1000", "offset=-1"} {
		rr := f.request(t, http.MethodGet, "/v1/games?"+q, f.read, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", q, rr.Code)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// addOwnedGame inserts a catalogue row and marks it owned for the given user,
// so tests never depend on seeded ids.
func (f *fixture) addOwnedGame(t *testing.T, userID string, bggID int, name string) int64 {
	t.Helper()
	now := db.Now()
	res, err := f.db.Exec(`INSERT INTO games(bgg_id,name,sort_name,year_published,min_players,max_players,min_playtime,max_playtime,weight,created_at,updated_at) VALUES(?,?,?,2000,2,4,30,60,2.0,?,?)`,
		bggID, name, name, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`, userID, id, now, now); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) patchShelf(t *testing.T, token string, gameID int64, body map[string]any) {
	t.Helper()
	rr := f.request(t, http.MethodPatch, "/v1/games/"+itoa(gameID), token, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch game %d: %d %s", gameID, rr.Code, rr.Body)
	}
}

func (f *fixture) gameDetail(t *testing.T, token string, gameID int64) map[string]any {
	t.Helper()
	rr := f.request(t, http.MethodGet, "/v1/games/"+itoa(gameID), token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get game %d: %d %s", gameID, rr.Code, rr.Body)
	}
	body := decode(t, rr)
	return body["data"].(map[string]any)["game"].(map[string]any)
}

func (f *fixture) addPlay(t *testing.T, token string, gameID int64, duration, quantity int) {
	t.Helper()
	payload := map[string]any{"game_id": gameID, "played_at": "2026-05-01", "quantity": quantity}
	if duration > 0 {
		payload["duration_minutes"] = duration
	}
	rr := f.request(t, http.MethodPost, "/v1/plays", token, payload)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create play: %d %s", rr.Code, rr.Body)
	}
}

func TestPipelineResource(t *testing.T) {
	f := setupAPI(t)
	rr := f.request(t, http.MethodGet, "/v1/pipeline", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("pipeline: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	buckets, ok := body["buckets"].([]any)
	if !ok || len(buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %v", body["buckets"])
	}
	wantStates := []string{"wishlist", "want_to_play", "preordered", "for_trade", "unplayed"}
	wantLabels := []string{"Wishlist", "Want to play", "Preordered", "For trade", "Owned, never played"}
	for i, raw := range buckets {
		b := raw.(map[string]any)
		if b["state"] != wantStates[i] {
			t.Errorf("bucket %d state = %v, want %q", i, b["state"], wantStates[i])
		}
		if b["label"] != wantLabels[i] {
			t.Errorf("bucket %d label = %v, want %q", i, b["label"], wantLabels[i])
		}
		if _, ok := b["games"].([]any); !ok {
			t.Errorf("bucket %d games is not an array: %v", i, b["games"])
		}
	}
	for i := 0; i < 4; i++ {
		if got := buckets[i].(map[string]any)["count"].(float64); got != 0 {
			t.Errorf("bucket %d count = %v, want 0", i, got)
		}
	}
	unplayed := buckets[4].(map[string]any)
	if got := unplayed["count"].(float64); got != 1 {
		t.Fatalf("unplayed count = %v, want 1", got)
	}
	games := unplayed["games"].([]any)
	if len(games) != 1 {
		t.Fatalf("unplayed games = %d, want 1", len(games))
	}
	if got := int64(games[0].(map[string]any)["id"].(float64)); got != f.gameID {
		t.Fatalf("unplayed game id = %d, want %d", got, f.gameID)
	}
}

func TestPipelineLimitValidation(t *testing.T) {
	f := setupAPI(t)
	for _, q := range []string{"limit=0", "limit=97", "limit=abc"} {
		rr := f.request(t, http.MethodGet, "/v1/pipeline?"+q, f.read, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", q, rr.Code)
			continue
		}
		body := decode(t, rr)
		env, ok := body["error"].(map[string]any)
		if !ok || env["code"] == "" {
			t.Errorf("%s: missing error envelope: %s", q, rr.Body)
		}
	}
}

func TestPipelineRequiresAuthentication(t *testing.T) {
	f := setupAPI(t)
	rr := f.request(t, http.MethodGet, "/v1/pipeline", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body)
	}
}

func TestGameValueFields(t *testing.T) {
	f := setupAPI(t)
	f.patchShelf(t, f.write, f.gameID, map[string]any{"price_paid_cents": 3000, "acquisition_date": "2026-01-15"})

	game := f.gameDetail(t, f.read, f.gameID)
	if game["acquisition_date"] != "2026-01-15" {
		t.Errorf("acquisition_date = %v", game["acquisition_date"])
	}
	if got := game["price_paid_cents"].(float64); got != 3000 {
		t.Errorf("price_paid_cents = %v, want 3000", got)
	}
	if game["price_per_play_cents"] != nil {
		t.Errorf("price_per_play_cents = %v, want null without plays", game["price_per_play_cents"])
	}
	if game["price_per_hour_cents"] != nil {
		t.Errorf("price_per_hour_cents = %v, want null without plays", game["price_per_hour_cents"])
	}

	f.addPlay(t, f.write, f.gameID, 120, 1)
	game = f.gameDetail(t, f.read, f.gameID)
	if got := game["price_per_play_cents"].(float64); got != 3000 {
		t.Errorf("price_per_play_cents = %v, want 3000", got)
	}
	if got := game["price_per_hour_cents"].(float64); got != 1500 {
		t.Errorf("price_per_hour_cents = %v, want 1500", got)
	}
}

func TestCostPerPlayWithoutDuration(t *testing.T) {
	f := setupAPI(t)
	f.patchShelf(t, f.write, f.gameID, map[string]any{"price_paid_cents": 2000})
	f.addPlay(t, f.write, f.gameID, 0, 4)

	game := f.gameDetail(t, f.read, f.gameID)
	if got := game["price_per_play_cents"].(float64); got != 500 {
		t.Errorf("price_per_play_cents = %v, want 500", got)
	}
	if game["price_per_hour_cents"] != nil {
		t.Errorf("price_per_hour_cents = %v, want null without durations", game["price_per_hour_cents"])
	}
	if game["acquisition_date"] != nil {
		t.Errorf("acquisition_date = %v, want null", game["acquisition_date"])
	}
}

func TestStatsCollectionValue(t *testing.T) {
	f := setupAPI(t)
	f.patchShelf(t, f.write, f.gameID, map[string]any{"price_paid_cents": 3000})
	f.addPlay(t, f.write, f.gameID, 60, 2)

	unplayedID := f.addOwnedGame(t, f.user.ID, 99, "Unplayed Purchase")
	f.patchShelf(t, f.write, unplayedID, map[string]any{"price_paid_cents": 2000})

	rr := f.request(t, http.MethodGet, "/v1/stats", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	cv, ok := body["data"].(map[string]any)["collection_value"].(map[string]any)
	if !ok {
		t.Fatalf("missing collection_value: %s", rr.Body)
	}
	if got := cv["total_paid_cents"].(float64); got != 5000 {
		t.Errorf("total_paid_cents = %v, want 5000", got)
	}
	if got := cv["paid_games"].(float64); got != 2 {
		t.Errorf("paid_games = %v, want 2", got)
	}
	if got := cv["unplayed_capital_cents"].(float64); got != 2000 {
		t.Errorf("unplayed_capital_cents = %v, want 2000", got)
	}
	if got := cv["median_cost_per_play_cents"].(float64); got != 1500 {
		t.Errorf("median_cost_per_play_cents = %v, want 1500", got)
	}
}

func TestValueDataIsUserScoped(t *testing.T) {
	f := setupAPI(t)
	f.patchShelf(t, f.write, f.gameID, map[string]any{"price_paid_cents": 3000})
	f.addPlay(t, f.write, f.gameID, 60, 1)

	other, err := f.handler.Auth.UpsertUser(context.Background(), "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, _, err := f.handler.Auth.CreateToken(context.Background(), other.ID, "other", []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	otherGame := f.addOwnedGame(t, other.ID, 77, "Other Shelf Game")
	f.patchShelf(t, otherToken, otherGame, map[string]any{"price_paid_cents": 9999})
	f.addPlay(t, otherToken, otherGame, 120, 1)

	rr := f.request(t, http.MethodGet, "/v1/pipeline", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("pipeline: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	for _, raw := range body["buckets"].([]any) {
		b := raw.(map[string]any)
		for _, rawGame := range b["games"].([]any) {
			if id := int64(rawGame.(map[string]any)["id"].(float64)); id == otherGame {
				t.Fatalf("other user's game %d leaked into bucket %v", id, b["state"])
			}
		}
	}

	rr = f.request(t, http.MethodGet, "/v1/stats", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rr.Code, rr.Body)
	}
	statsBody := decode(t, rr)
	cv := statsBody["data"].(map[string]any)["collection_value"].(map[string]any)
	if got := cv["total_paid_cents"].(float64); got != 3000 {
		t.Errorf("total_paid_cents = %v, want only the first user's 3000", got)
	}
	if got := cv["paid_games"].(float64); got != 1 {
		t.Errorf("paid_games = %v, want only the first user's 1", got)
	}
}

func TestUnknownEndpointUsesErrorEnvelope(t *testing.T) {
	f := setupAPI(t)
	rr := f.request(t, http.MethodGet, "/v1/does-not-exist", f.read, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	body := decode(t, rr)
	errBody, ok := body["error"].(map[string]any)
	if !ok || errBody["code"] != "not_found" {
		t.Fatalf("unexpected envelope: %s", rr.Body.String())
	}
}
