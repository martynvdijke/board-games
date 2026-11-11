package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"boardgames/internal/account"
	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/loans"
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
	handler := &Handler{Config: cfg, Auth: manager, Games: games, Plays: playRepo, Loans: loans.New(database), DB: database,
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

// lend creates a loan through the API and returns the decoded payload.
func (f *fixture) lend(t *testing.T, token string, gameID int64, borrower, lentAt string) map[string]any {
	t.Helper()
	payload := map[string]any{"game_id": gameID, "borrower": borrower}
	if lentAt != "" {
		payload["lent_at"] = lentAt
	}
	rr := f.request(t, http.MethodPost, "/v1/loans", token, payload)
	if rr.Code != http.StatusCreated {
		t.Fatalf("lend game %d: %d %s", gameID, rr.Code, rr.Body)
	}
	return decode(t, rr)["data"].(map[string]any)
}

// markReturned closes a loan; an empty date sends no body so the server default
// (today) applies.
func (f *fixture) markReturned(t *testing.T, token string, id int64, returnedAt string) map[string]any {
	t.Helper()
	var body any
	if returnedAt != "" {
		body = map[string]any{"returned_at": returnedAt}
	}
	rr := f.request(t, http.MethodPost, "/v1/loans/"+itoa(id)+"/return", token, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("return loan %d: %d %s", id, rr.Code, rr.Body)
	}
	return decode(t, rr)["data"].(map[string]any)
}

func (f *fixture) loanList(t *testing.T, token, query string) []any {
	t.Helper()
	rr := f.request(t, http.MethodGet, "/v1/loans"+query, token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list loans %q: %d %s", query, rr.Code, rr.Body)
	}
	return decode(t, rr)["loans"].([]any)
}

func loanID(t *testing.T, loan map[string]any) int64 {
	t.Helper()
	id, ok := loan["id"].(float64)
	if !ok {
		t.Fatalf("loan has no numeric id: %v", loan)
	}
	return int64(id)
}

func TestLoanCreateAndList(t *testing.T) {
	f := setupAPI(t)
	loan := f.lend(t, f.write, f.gameID, "Sam", "2026-08-01")

	for _, key := range []string{"id", "game", "borrower", "contact", "lent_at", "due_at", "returned_at",
		"notes", "closed_reason", "overdue", "days_out", "days_overdue", "notified_at"} {
		if _, ok := loan[key]; !ok {
			t.Errorf("loan is missing %q: %v", key, loan)
		}
	}
	if loan["borrower"] != "Sam" || loan["lent_at"] != "2026-08-01" {
		t.Fatalf("unexpected loan: %v", loan)
	}
	for _, key := range []string{"contact", "due_at", "returned_at", "notes", "closed_reason", "notified_at"} {
		if loan[key] != "" {
			t.Errorf("%s = %v, want empty", key, loan[key])
		}
	}
	if loan["overdue"] != false {
		t.Errorf("overdue = %v, want false without a due date", loan["overdue"])
	}
	if _, ok := loan["days_out"].(float64); !ok {
		t.Errorf("days_out is not numeric: %v", loan["days_out"])
	}
	if _, ok := loan["days_overdue"].(float64); !ok {
		t.Errorf("days_overdue is not numeric: %v", loan["days_overdue"])
	}
	game := loan["game"].(map[string]any)
	if int64(game["id"].(float64)) != f.gameID || game["name"] != "Catan" || int(game["bgg_id"].(float64)) != 13 {
		t.Errorf("unexpected game reference: %v", game)
	}
	if _, ok := game["thumbnail_url"].(string); !ok {
		t.Errorf("thumbnail_url is not a string: %v", game["thumbnail_url"])
	}

	list := f.loanList(t, f.read, "")
	if len(list) != 1 {
		t.Fatalf("open loans = %d, want 1: %v", len(list), list)
	}
	if got := loanID(t, list[0].(map[string]any)); got != loanID(t, loan) {
		t.Errorf("listed loan id = %d, want %d", got, loanID(t, loan))
	}
}

func TestLoanConflictLeavesFirstLoanUnchanged(t *testing.T) {
	f := setupAPI(t)
	first := f.lend(t, f.write, f.gameID, "Sam", "2026-08-01")

	rr := f.request(t, http.MethodPost, "/v1/loans", f.write, map[string]any{"game_id": f.gameID, "borrower": "Alex"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body)
	}
	env, ok := decode(t, rr)["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %s", rr.Body)
	}
	if env["code"] != "conflict" {
		t.Errorf("code = %v, want conflict", env["code"])
	}
	if msg, _ := env["message"].(string); !strings.Contains(msg, "Catan") {
		t.Errorf("conflict message should name the game, got %q", msg)
	}

	rr = f.request(t, http.MethodGet, "/v1/loans/"+itoa(loanID(t, first)), f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("re-read loan: %d %s", rr.Code, rr.Body)
	}
	again := decode(t, rr)["data"].(map[string]any)
	if loanID(t, again) != loanID(t, first) || again["borrower"] != "Sam" {
		t.Errorf("first loan changed: %v", again)
	}
}

func TestLoanReturnMovesToClosed(t *testing.T) {
	f := setupAPI(t)
	loan := f.lend(t, f.write, f.gameID, "Sam", "2026-08-01")

	closed := f.markReturned(t, f.write, loanID(t, loan), "2026-09-01")
	if closed["returned_at"] != "2026-09-01" || closed["closed_reason"] != "returned" {
		t.Fatalf("unexpected closed loan: %v", closed)
	}
	if closed["overdue"] != false {
		t.Errorf("a returned loan cannot be overdue: %v", closed)
	}
	if open := f.loanList(t, f.read, "?status=open"); len(open) != 0 {
		t.Errorf("open loans = %d, want 0", len(open))
	}
	history := f.loanList(t, f.read, "?status=closed")
	if len(history) != 1 || loanID(t, history[0].(map[string]any)) != loanID(t, loan) {
		t.Errorf("closed history = %v", history)
	}

	// An omitted body closes the loan today.
	second := f.lend(t, f.write, f.addOwnedGame(t, f.user.ID, 501, "Second Game"), "Alex", "2026-09-01")
	defaulted := f.markReturned(t, f.write, loanID(t, second), "")
	if defaulted["returned_at"] == "" || defaulted["closed_reason"] != "returned" {
		t.Errorf("omitted returned_at should default to today: %v", defaulted)
	}
}

func TestLoanCrossUserIsNotFound(t *testing.T) {
	f := setupAPI(t)
	loan := f.lend(t, f.write, f.gameID, "Sam", "2026-08-01")

	other, err := f.handler.Auth.UpsertUser(context.Background(), "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, _, err := f.handler.Auth.CreateToken(context.Background(), other.ID, "other", []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/v1/loans/" + itoa(loanID(t, loan)), "/v1/loans/" + itoa(loanID(t, loan)) + "/return"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "/return") {
			method = http.MethodPost
		}
		rr := f.request(t, method, path, otherToken, nil)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s %s: expected 404, got %d: %s", method, path, rr.Code, rr.Body)
		}
	}
	if list := f.loanList(t, otherToken, ""); len(list) != 0 {
		t.Errorf("other user sees %d loans, want 0", len(list))
	}
}

func TestLoanRequiresOwnedGame(t *testing.T) {
	f := setupAPI(t)
	other, err := f.handler.Auth.UpsertUser(context.Background(), "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	otherGame := f.addOwnedGame(t, other.ID, 502, "Not Yours")

	rr := f.request(t, http.MethodPost, "/v1/loans", f.write, map[string]any{"game_id": otherGame, "borrower": "Sam"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body)
	}
	env := decode(t, rr)["error"].(map[string]any)
	fields, ok := env["fields"].(map[string]any)
	if !ok || fields["game_id"] == nil {
		t.Errorf("422 should name game_id: %s", rr.Body)
	}

	// Missing borrower is attributed to that field.
	rr = f.request(t, http.MethodPost, "/v1/loans", f.write, map[string]any{"game_id": f.gameID})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing borrower, got %d: %s", rr.Code, rr.Body)
	}
	env = decode(t, rr)["error"].(map[string]any)
	if fields, ok := env["fields"].(map[string]any); !ok || fields["borrower"] == nil {
		t.Errorf("422 should name borrower: %s", rr.Body)
	}
}

func TestLoanListOrderingAndLimit(t *testing.T) {
	f := setupAPI(t)
	closedA := f.lend(t, f.write, f.addOwnedGame(t, f.user.ID, 601, "Closed A"), "A", "2026-01-01")
	f.markReturned(t, f.write, loanID(t, closedA), "2026-02-01")
	openB := f.lend(t, f.write, f.addOwnedGame(t, f.user.ID, 602, "Open B"), "B", "2026-03-01")
	openC := f.lend(t, f.write, f.addOwnedGame(t, f.user.ID, 603, "Open C"), "C", "2026-04-01")
	closedD := f.lend(t, f.write, f.addOwnedGame(t, f.user.ID, 604, "Closed D"), "D", "2026-04-15")
	f.markReturned(t, f.write, loanID(t, closedD), "2026-05-01")

	all := f.loanList(t, f.read, "?status=all")
	if len(all) != 4 {
		t.Fatalf("status=all returned %d loans, want 4: %v", len(all), all)
	}
	wantOrder := []int64{loanID(t, openB), loanID(t, openC), loanID(t, closedD), loanID(t, closedA)}
	for i, want := range wantOrder {
		if got := loanID(t, all[i].(map[string]any)); got != want {
			t.Errorf("status=all[%d] = %d, want %d (open first, then newest closed)", i, got, want)
		}
	}
	for i, raw := range all {
		loan := raw.(map[string]any)
		isOpen := loan["returned_at"] == ""
		if i < 2 && !isOpen {
			t.Errorf("loan %d should be open: %v", i, loan)
		}
		if i >= 2 && isOpen {
			t.Errorf("loan %d should be closed: %v", i, loan)
		}
	}

	// limit applies only to the closed section, and numeric outliers clamp.
	clampedLow := f.loanList(t, f.read, "?status=closed&limit=0")
	if len(clampedLow) != 1 {
		t.Errorf("closed&limit=0 returned %d, want 1 after clamping", len(clampedLow))
	}
	clampedHigh := f.loanList(t, f.read, "?status=closed&limit=1000")
	if len(clampedHigh) != 2 {
		t.Errorf("closed&limit=1000 returned %d, want both loans", len(clampedHigh))
	}
	mixed := f.loanList(t, f.read, "?status=all&limit=0")
	if len(mixed) != 3 {
		t.Errorf("all&limit=0 returned %d, want 2 open + 1 closed", len(mixed))
	}

	for _, query := range []string{"?status=sideways", "?limit=abc"} {
		rr := f.request(t, http.MethodGet, "/v1/loans"+query, f.read, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", query, rr.Code, rr.Body)
		}
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
