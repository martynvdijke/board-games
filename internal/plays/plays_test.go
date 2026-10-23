package plays

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"boardgames/internal/db"
)

func setupPlays(t *testing.T) (*Repo, *sql.DB, string, int64) {
	t.Helper()
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := db.Now()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('u1','one@example.com','One','admin',?,?)`, []any{now, now}},
		{`INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('u2','two@example.com','Two','member',?,?)`, []any{now, now}},
		{`INSERT INTO games(id,bgg_id,name,sort_name,created_at,updated_at) VALUES(1,1,'Catan','catan',?,?)`, []any{now, now}},
		{`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES('u1',1,1,?,?)`, []any{now, now}},
	} {
		if _, err := database.Exec(q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	return NewRepo(database), database, "u1", 1
}

func TestCreateUpdateDelete(t *testing.T) {
	repo, database, userID, gameID := setupPlays(t)
	ctx := context.Background()
	p, err := repo.Create(ctx, userID, Input{GameID: gameID, PlayedAt: "2026-01-02", Duration: 60, Quantity: 1,
		Players: []Player{{Name: "Alex", Score: "10", IsWinner: true}, {Name: "Sam", Score: "8"}}}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" || len(p.Players) != 2 {
		t.Fatalf("unexpected play: %#v", p)
	}
	got, err := repo.Get(ctx, userID, p.ID)
	if err != nil || len(got.Players) != 2 {
		t.Fatalf("get: %v %#v", err, got)
	}
	if _, err := repo.Get(ctx, "u2", p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user read should be not found, got %v", err)
	}
	updated, err := repo.Update(ctx, userID, p.ID, Input{GameID: gameID, PlayedAt: "2026-01-03", Duration: 45, Quantity: 2, Players: []Player{{Name: "Alex"}}}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if updated.PlayedAt != "2026-01-03" || updated.Quantity != 2 || len(updated.Players) != 1 {
		t.Fatalf("update not applied: %#v", updated)
	}
	if _, err := repo.Update(ctx, "u2", p.ID, Input{GameID: gameID, PlayedAt: "2026-01-03", Quantity: 1}, "UTC"); err == nil {
		t.Fatal("cross-user update should fail")
	}
	if err := repo.Delete(ctx, "u2", p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user delete should fail, got %v", err)
	}
	if err := repo.Delete(ctx, userID, p.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM plays`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("play not deleted: %v %d", err, count)
	}
}

func TestValidation(t *testing.T) {
	repo, database, userID, gameID := setupPlays(t)
	ctx := context.Background()
	cases := []Input{
		{GameID: 999, PlayedAt: "2026-01-01", Quantity: 1},
		{GameID: gameID, PlayedAt: "01/01/2026", Quantity: 1},
		{GameID: gameID, PlayedAt: "2999-01-01", Quantity: 1},
		{GameID: gameID, PlayedAt: "2026-01-01", Quantity: 1, Duration: 5000},
		{GameID: gameID, PlayedAt: "2026-01-01", Quantity: 0},
		{GameID: gameID, PlayedAt: "2026-01-01", Quantity: 1, Players: []Player{{Name: "Alex"}, {Name: "Alex"}}},
	}
	for i, in := range cases {
		if _, err := repo.Create(ctx, userID, in, "UTC"); err == nil {
			t.Errorf("case %d should have failed", i)
		}
	}
	var count int
	_ = database.QueryRow(`SELECT COUNT(*) FROM plays`).Scan(&count)
	if count != 0 {
		t.Fatalf("invalid plays were created: %d", count)
	}
	// A game that is not on this user's shelf is rejected.
	if _, err := repo.Create(ctx, "u2", Input{GameID: gameID, PlayedAt: "2026-01-01", Quantity: 1}, "UTC"); err == nil {
		t.Fatal("expected shelf ownership validation")
	}
}

func TestStatsAndIdempotentImport(t *testing.T) {
	repo, _, userID, gameID := setupPlays(t)
	ctx := context.Background()
	if _, err := repo.Create(ctx, userID, Input{GameID: gameID, PlayedAt: "2026-04-02", Quantity: 2, Players: []Player{{Name: "Alex", IsWinner: true}}}, "UTC"); err != nil {
		t.Fatal(err)
	}
	bggID := int64(9001)
	p := Play{ID: "imported", GameID: gameID, PlayedAt: "2026-04-03", Quantity: 1, BGGPlayID: &bggID, Players: []Player{{Name: "Sam"}}}
	created, err := repo.UpsertImported(ctx, userID, p)
	if err != nil || !created {
		t.Fatalf("import: %v %v", created, err)
	}
	created, err = repo.UpsertImported(ctx, userID, p)
	if err != nil || created {
		t.Fatalf("re-import should update: %v %v", created, err)
	}
	stats, err := repo.Stats(ctx, userID, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPlays != 3 || stats.TotalSessions != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
