package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"boardgames/internal/db"
)

func TestDeleteMeRevokesCredentialsAndData(t *testing.T) {
	f := setupAPI(t)
	ctx := context.Background()
	now := db.Now()
	if _, err := f.db.ExecContext(ctx, `UPDATE users SET role='member' WHERE id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,location,notes,origin,created_at,updated_at)
		VALUES('p1',?,?,'2026-01-01','Home','private','local',?,?)`, f.user.ID, f.gameID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO play_players(play_id,name) VALUES('p1','Alex')`); err != nil {
		t.Fatal(err)
	}
	rr := f.request(t, http.MethodDelete, "/v1/me", f.write, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete me: %d %s", rr.Code, rr.Body)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM users WHERE id = ?`,
		`SELECT COUNT(*) FROM sessions WHERE user_id = ?`,
		`SELECT COUNT(*) FROM api_tokens WHERE user_id = ?`,
		`SELECT COUNT(*) FROM user_settings WHERE user_id = ?`,
		`SELECT COUNT(*) FROM plays WHERE user_id = ?`,
		`SELECT COUNT(*) FROM play_players WHERE play_id = 'p1'`,
	} {
		var n int
		if err := f.db.QueryRowContext(ctx, q, f.user.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%q left %d rows", q, n)
		}
	}
	// The token used for the request is now invalid.
	if rr := f.request(t, http.MethodGet, "/v1/me", f.read, nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to fail, got %d", rr.Code)
	}
}

func TestDeleteMeRefusesLastAdmin(t *testing.T) {
	f := setupAPI(t)
	rr := f.request(t, http.MethodDelete, "/v1/me", f.read, nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, f.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("last admin was deleted")
	}
}

func TestExportIncludesProfilePlaysAndTokenMetadata(t *testing.T) {
	f := setupAPI(t)
	ctx := context.Background()
	now := db.Now()
	if _, err := f.db.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,location,notes,origin,created_at,updated_at)
		VALUES('p1',?,?,'2026-02-03','Pub','good game','local',?,?)`, f.user.ID, f.gameID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO play_players(play_id,name,score,is_winner) VALUES('p1','Sam','10',1)`); err != nil {
		t.Fatal(err)
	}
	rr := f.request(t, http.MethodGet, "/v1/me/export", f.read, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rr.Code, rr.Body)
	}
	body := rr.Body.String()
	for _, want := range []string{`"profile"`, `"owner@example.com"`, `"plays"`, `"players"`, `"Sam"`, `"tokens"`, `"prefix"`} {
		if !strings.Contains(body, want) {
			t.Errorf("export missing %s: %s", want, body)
		}
	}
	for _, forbidden := range []string{"token_hash", f.read, f.write} {
		if strings.Contains(body, forbidden) {
			t.Errorf("export leaked %q", forbidden)
		}
	}
	if got := rr.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Errorf("export should be an attachment, got %q", got)
	}
}

func TestRemovePlayerIsScopedToOwnPlays(t *testing.T) {
	f := setupAPI(t)
	ctx := context.Background()
	if _, err := f.db.ExecContext(ctx, `UPDATE users SET role='member' WHERE id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	other, err := f.handler.Auth.UpsertUser(ctx, "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	now := db.Now()
	for _, row := range []struct{ id, user string }{{"mine", f.user.ID}, {"theirs", other.ID}} {
		if _, err := f.db.ExecContext(ctx, `INSERT INTO plays(id,user_id,game_id,played_at,origin,created_at,updated_at)
			VALUES(?,?,?,'2026-01-01','local',?,?)`, row.id, row.user, f.gameID, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(ctx, `INSERT INTO play_players(play_id,name) VALUES(?,'Taylor')`, row.id); err != nil {
			t.Fatal(err)
		}
	}
	rr := f.request(t, http.MethodDelete, "/v1/players?name=Taylor", f.write, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("remove player: %d %s", rr.Code, rr.Body)
	}
	for id, want := range map[string]int{"mine": 0, "theirs": 1} {
		var n int
		if err := f.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_players WHERE play_id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("play %s has %d player rows, want %d", id, n, want)
		}
	}
}
