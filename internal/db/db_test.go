package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestMigrateAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	database, err := OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var version string
	if err := database.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "0003_shelf" {
		t.Fatalf("expected migrations through 0003_shelf, got %q", version)
	}
	if _, err := database.Exec(`INSERT INTO sessions(token_hash,user_id,csrf_token,created_at,expires_at) VALUES('x','missing','y',?,?)`, Now(), Now()); err == nil {
		t.Fatal("expected foreign key violation")
	}
}

func TestFTS(t *testing.T) {
	ctx := context.Background()
	database, err := OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	res, err := database.Exec(`INSERT INTO games(bgg_id,name,sort_name,description,created_at,updated_at) VALUES(1,'Wingspan','wingspan','A bird engine builder',?,?)`, Now(), Now())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := database.Exec(`INSERT INTO games_fts(rowid,name,description) VALUES(?,?,?)`, id, "Wingspan", "A bird engine builder"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM games_fts WHERE games_fts MATCH 'win*'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one FTS match, got %d", count)
	}
}

func TestPartialUniqueBGGPlay(t *testing.T) {
	ctx := context.Background()
	database, err := OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mustExec(t, database, `INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('u','a@example.com','A','member',?,?)`, Now(), Now())
	mustExec(t, database, `INSERT INTO games(id,bgg_id,name,sort_name,created_at,updated_at) VALUES(1,1,'Game','game',?,?)`, Now(), Now())
	mustExec(t, database, `INSERT INTO plays(id,user_id,game_id,played_at,origin,bgg_play_id,created_at,updated_at) VALUES('p1','u',1,'2026-01-01','bgg','99',?,?)`, Now(), Now())
	if _, err := database.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,origin,bgg_play_id,created_at,updated_at) VALUES('p2','u',1,'2026-01-01','bgg','99',?,?)`, Now(), Now()); err == nil {
		t.Fatal("expected duplicate bgg play to fail")
	}
	mustExec(t, database, `INSERT INTO plays(id,user_id,game_id,played_at,origin,created_at,updated_at) VALUES('p3','u',1,'2026-01-01','local',?,?)`, Now(), Now())
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
