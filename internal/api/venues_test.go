package api

import (
	"context"
	"net/http"
	"testing"

	"boardgames/internal/db"
)

func TestVenuesEndpointResolvesAndScopes(t *testing.T) {
	f := setupAPI(t)

	for _, location := range []string{"The Crown", "the crown"} {
		rr := f.request(t, http.MethodPost, "/v1/plays", f.write, map[string]any{
			"game_id":   f.gameID,
			"played_at": "2026-01-02",
			"location":  location,
			"quantity":  1,
		})
		if rr.Code != http.StatusCreated {
			t.Fatalf("create play with %q: %d %s", location, rr.Code, rr.Body)
		}
	}

	var venues int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id=?`, f.user.ID).Scan(&venues); err != nil {
		t.Fatal(err)
	}
	if venues != 1 {
		t.Fatalf("API writes should collapse spellings, got %d venues", venues)
	}

	// Another user's venue must not leak into the response.
	ctx := context.Background()
	other, err := f.handler.Auth.UpsertUser(ctx, "other@example.com", "Other", "", false)
	if err != nil {
		t.Fatal(err)
	}
	now := db.Now()
	res, err := f.db.Exec(`INSERT INTO locations(user_id,name,normalized_name,created_at,updated_at) VALUES(?,?,?,?,?)`,
		other.ID, "Other Place", "other place", now, now)
	if err != nil {
		t.Fatal(err)
	}
	otherVenue, _ := res.LastInsertId()
	if _, err := f.db.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,location,location_id,quantity,created_at,updated_at)
		VALUES('other-play',?,?,'2026-01-03','Other Place',?,1,?,?)`, other.ID, f.gameID, otherVenue, now, now); err != nil {
		t.Fatal(err)
	}

	rr := f.request(t, http.MethodGet, "/v1/venues", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/venues: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	rows, ok := body["venues"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("expected only the caller's venue, got %#v", body["venues"])
	}
	venue, _ := rows[0].(map[string]any)
	if venue["name"] != "The Crown" {
		t.Fatalf("unexpected venue name: %#v", venue)
	}
	if venue["plays"] != float64(2) || venue["distinct_games"] != float64(1) {
		t.Fatalf("unexpected venue counts: %#v", venue)
	}
	if venue["last_played_at"] != "2026-01-02" {
		t.Fatalf("unexpected last played: %#v", venue)
	}

	if rr := f.request(t, http.MethodGet, "/v1/venues", "", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /v1/venues = %d, want 401", rr.Code)
	}
}
