package plays

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"boardgames/internal/db"
)

// ownGame makes a game visible to a user so plays can be logged for them.
func ownGame(t *testing.T, database *sql.DB, userID string, gameID int64) {
	t.Helper()
	now := db.Now()
	if _, err := database.Exec(`INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at) VALUES(?,?,1,?,?)`,
		userID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
}

func logPlayWith(t *testing.T, repo *Repo, userID string, gameID int64, playedAt string, names ...string) {
	t.Helper()
	players := make([]Player, 0, len(names))
	for _, name := range names {
		players = append(players, Player{Name: name})
	}
	if _, err := repo.Create(context.Background(), userID, Input{
		GameID: gameID, PlayedAt: playedAt, Quantity: 1, Players: players,
	}, "UTC"); err != nil {
		t.Fatalf("create play on %s: %v", playedAt, err)
	}
}

func fetchRecent(t *testing.T, repo *Repo, userID string, limit int) []string {
	t.Helper()
	names, err := repo.RecentPlayers(context.Background(), userID, limit)
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestRecentPlayersScopesToUser(t *testing.T) {
	repo, database, userID, gameID := setupPlays(t)
	ownGame(t, database, "u2", gameID)

	logPlayWith(t, repo, userID, gameID, "2026-01-01", "Alex")
	logPlayWith(t, repo, "u2", gameID, "2026-01-02", "Bea")

	if got := fetchRecent(t, repo, userID, 12); len(got) != 1 || got[0] != "Alex" {
		t.Fatalf("u1 should only see their own players, got %#v", got)
	}
	if got := fetchRecent(t, repo, "u2", 12); len(got) != 1 || got[0] != "Bea" {
		t.Fatalf("u2 should only see their own players, got %#v", got)
	}
	if got := fetchRecent(t, repo, "nobody", 12); len(got) != 0 {
		t.Fatalf("unknown user should see nothing, got %#v", got)
	}
}

func TestRecentPlayersDedupesCaseInsensitively(t *testing.T) {
	repo, _, userID, gameID := setupPlays(t)

	logPlayWith(t, repo, userID, gameID, "2026-01-01", "Alex")
	logPlayWith(t, repo, userID, gameID, "2026-02-01", " alex ")
	logPlayWith(t, repo, userID, gameID, "2026-03-01", "ALEX")

	got := fetchRecent(t, repo, userID, 12)
	if len(got) != 1 {
		t.Fatalf("expected one deduped name, got %#v", got)
	}
	if strings.ToLower(got[0]) != "alex" {
		t.Fatalf("expected alex, got %q", got[0])
	}
}

func TestRecentPlayersOrdersByRecency(t *testing.T) {
	repo, _, userID, gameID := setupPlays(t)

	logPlayWith(t, repo, userID, gameID, "2026-01-01", "Old")
	logPlayWith(t, repo, userID, gameID, "2026-02-01", "Mid")
	logPlayWith(t, repo, userID, gameID, "2026-03-01", "New")
	// Same person, played again later: they move to the front.
	logPlayWith(t, repo, userID, gameID, "2026-04-01", "Old")
	// Same play, same date: alphabetical tie-break.
	logPlayWith(t, repo, userID, gameID, "2026-05-01", "Zed", "Amy")

	got := fetchRecent(t, repo, userID, 12)
	want := []string{"Amy", "Zed", "Old", "New", "Mid"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestRecentPlayersAppliesLimit(t *testing.T) {
	repo, _, userID, gameID := setupPlays(t)

	for i, name := range []string{"P1", "P2", "P3", "P4", "P5"} {
		logPlayWith(t, repo, userID, gameID, "2026-01-0"+string(rune('1'+i)), name)
	}

	got := fetchRecent(t, repo, userID, 2)
	if len(got) != 2 || got[0] != "P5" || got[1] != "P4" {
		t.Fatalf("limit 2 should keep the two most recent, got %#v", got)
	}

	// Zero (and negative) means the default of 12, which covers all five here.
	if got := fetchRecent(t, repo, userID, 0); len(got) != 5 {
		t.Fatalf("default limit should return all known players, got %#v", got)
	}
	if got := fetchRecent(t, repo, userID, -3); len(got) != 5 {
		t.Fatalf("negative limit should fall back to the default, got %#v", got)
	}
	// An oversized limit is clamped, not rejected.
	if got := fetchRecent(t, repo, userID, 500); len(got) != 5 {
		t.Fatalf("clamped limit should still return the known players, got %#v", got)
	}
}

func TestRecentPlayersIgnoresBlankNames(t *testing.T) {
	repo, database, userID, gameID := setupPlays(t)

	now := db.Now()
	if _, err := database.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,quantity,created_at,updated_at)
		VALUES('p-blank',?,?,'2026-01-01',1,?,?)`, userID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO play_players(play_id,name) VALUES('p-blank','   ')`); err != nil {
		t.Fatal(err)
	}

	if got := fetchRecent(t, repo, userID, 12); len(got) != 0 {
		t.Fatalf("blank player names should be ignored, got %#v", got)
	}
}
