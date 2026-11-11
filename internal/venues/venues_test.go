package venues

import (
	"context"
	"database/sql"
	"testing"

	"boardgames/internal/db"
)

func setup(t *testing.T) (*Repo, *sql.DB) {
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
		{`INSERT INTO games(id,bgg_id,name,sort_name,created_at,updated_at) VALUES(2,2,'Wingspan','wingspan',?,?)`, []any{now, now}},
	} {
		if _, err := database.Exec(q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	return NewRepo(database), database
}

func insertPlay(t *testing.T, database *sql.DB, id, userID string, gameID int64, playedAt, location string, locationID any) {
	t.Helper()
	now := db.Now()
	if _, err := database.Exec(
		`INSERT INTO plays(id,user_id,game_id,played_at,location,location_id,quantity,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,1,?,?)`,
		id, userID, gameID, playedAt, location, locationID, now, now); err != nil {
		t.Fatal(err)
	}
}

func addWinner(t *testing.T, database *sql.DB, playID, name string) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO play_players(play_id,name,is_winner) VALUES(?,?,1)`, playID, name); err != nil {
		t.Fatal(err)
	}
}

func resolve(t *testing.T, repo *Repo, database *sql.DB, userID, raw string) (int64, string) {
	t.Helper()
	var id int64
	var name string
	if err := db.WithTx(context.Background(), database, func(tx *sql.Tx) error {
		var err error
		id, name, err = repo.ResolveTx(context.Background(), tx, userID, raw)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id, name
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"The Crown", "the crown"},
		{"  The   Crown  ", "the crown"},
		{"the crown", "the crown"},
		{"The Crown.", "the crown"},
		{"Café & Bar", "café and bar"},
		{"A&B", "a and b"},
		{"Joe’s Place", "joe's place"},
		{"Joe`s Place", "joe's place"},
		{"Jose\u00b4s", "jose's"},
		{"Le Café", "le café"},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	for _, in := range []string{"", "  The   Crown ", "Café & Bar ", "Joe’s  Place.", "A  &  B"} {
		once := Normalize(in)
		if twice := Normalize(once); twice != once {
			t.Errorf("Normalize not idempotent for %q: %q != %q", in, once, twice)
		}
	}
}

func TestResolveTxCreatesAndReuses(t *testing.T) {
	repo, database := setup(t)

	id, name := resolve(t, repo, database, "u1", "  The Crown  ")
	if id == 0 || name != "The Crown" {
		t.Fatalf("unexpected resolve: %d %q", id, name)
	}

	againID, againName := resolve(t, repo, database, "u1", "the crown")
	if againID != id {
		t.Fatalf("different spelling should reuse venue: %d != %d", againID, id)
	}
	if againName != "The Crown" {
		t.Fatalf("first spelling should win, got %q", againName)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id='u1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one venue row, got %d", count)
	}
}

func TestResolveTxSeparatesUsersAndEmpty(t *testing.T) {
	repo, database := setup(t)

	u1ID, _ := resolve(t, repo, database, "u1", "The Crown")
	u2ID, _ := resolve(t, repo, database, "u2", "The Crown")
	if u1ID == u2ID {
		t.Fatalf("users must not share venue rows: both got %d", u1ID)
	}

	id, name := resolve(t, repo, database, "u1", "   ")
	if id != 0 || name != "" {
		t.Fatalf("empty location should yield zero values, got %d %q", id, name)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM locations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected two venue rows, got %d", count)
	}
}

func TestStats(t *testing.T) {
	repo, database := setup(t)
	ctx := context.Background()

	crownID, _ := resolve(t, repo, database, "u1", "The Crown")
	tavernID, _ := resolve(t, repo, database, "u1", "The Tavern")
	otherID, _ := resolve(t, repo, database, "u2", "Other Place")

	insertPlay(t, database, "p1", "u1", 1, "2026-01-02", "The Crown", crownID)
	insertPlay(t, database, "p2", "u1", 2, "2026-02-03", "the crown", crownID)
	insertPlay(t, database, "p3", "u1", 1, "2026-03-04", "The Tavern", tavernID)
	insertPlay(t, database, "p4", "u1", 1, "", "The Crown", crownID) // no timestamp: excluded
	insertPlay(t, database, "p5", "u2", 1, "2026-04-05", "Other Place", otherID)

	addWinner(t, database, "p1", "Alex")
	addWinner(t, database, "p3", "Sam")

	stats, err := repo.Stats(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected two venues, got %d: %#v", len(stats), stats)
	}
	crown, tavern := stats[0], stats[1]
	if crown.Name != "The Crown" || crown.Plays != 2 || crown.DistinctGames != 2 {
		t.Fatalf("unexpected crown stats: %#v", crown)
	}
	if crown.LastPlayedAt != "2026-02-03" || crown.DecidedPlays != 1 || crown.Wins != 0 {
		t.Fatalf("unexpected crown aggregates: %#v", crown)
	}
	if tavern.Plays != 1 || tavern.DistinctGames != 1 || tavern.DecidedPlays != 1 {
		t.Fatalf("unexpected tavern stats: %#v", tavern)
	}
}

func TestBackfill(t *testing.T) {
	repo, database := setup(t)
	ctx := context.Background()

	insertPlay(t, database, "p1", "u1", 1, "2026-01-01", "The Crown", nil)
	insertPlay(t, database, "p2", "u1", 2, "2026-01-02", "the crown", nil)
	insertPlay(t, database, "p3", "u1", 1, "2026-01-03", "  The   Tavern ", nil)
	insertPlay(t, database, "p4", "u1", 1, "2026-01-04", "", nil)
	insertPlay(t, database, "p5", "u2", 1, "2026-01-05", "The Crown", nil)

	linked, err := repo.Backfill(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if linked != 3 {
		t.Fatalf("expected three linked plays, got %d", linked)
	}

	var locationCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id='u1'`).Scan(&locationCount); err != nil {
		t.Fatal(err)
	}
	if locationCount != 2 {
		t.Fatalf("expected two venues for u1, got %d", locationCount)
	}

	var p1, p2 int64
	if err := database.QueryRow(`SELECT location_id FROM plays WHERE id='p1'`).Scan(&p1); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT location_id FROM plays WHERE id='p2'`).Scan(&p2); err != nil {
		t.Fatal(err)
	}
	if p1 == 0 || p1 != p2 {
		t.Fatalf("spellings should share one venue: %d %d", p1, p2)
	}

	// Empty location and other users' plays stay unlinked.
	for _, id := range []string{"p4", "p5"} {
		var locationID sql.NullInt64
		if err := database.QueryRow(`SELECT location_id FROM plays WHERE id=?`, id).Scan(&locationID); err != nil {
			t.Fatal(err)
		}
		if locationID.Valid {
			t.Fatalf("play %s should remain unlinked", id)
		}
	}

	linked, err = repo.Backfill(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if linked != 0 {
		t.Fatalf("backfill should be idempotent, linked %d", linked)
	}
}

func TestListOrdersByPlayCount(t *testing.T) {
	repo, database := setup(t)
	ctx := context.Background()

	quietID, _ := resolve(t, repo, database, "u1", "Quiet Pub")
	busyID, _ := resolve(t, repo, database, "u1", "Busy Hall")

	insertPlay(t, database, "p1", "u1", 1, "2026-01-01", "Busy Hall", busyID)
	insertPlay(t, database, "p2", "u1", 1, "2026-01-02", "busy hall", busyID)
	insertPlay(t, database, "p3", "u1", 1, "2026-01-03", "Quiet Pub", quietID)

	locations, err := repo.List(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 2 {
		t.Fatalf("expected two venues, got %d", len(locations))
	}
	if locations[0].Name != "Busy Hall" || locations[0].Plays != 2 {
		t.Fatalf("unexpected first venue: %#v", locations[0])
	}
	if locations[1].Name != "Quiet Pub" || locations[1].Plays != 1 {
		t.Fatalf("unexpected second venue: %#v", locations[1])
	}
}
