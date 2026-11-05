package collection

import (
	"context"
	"errors"
	"strings"
	"testing"

	"boardgames/internal/db"
)

type setupFixture struct {
	repo   *Repo
	user   string
	other  string
	gameID int64
}

func newSetupFixture(t *testing.T) *setupFixture {
	t.Helper()
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := db.Now()
	for _, id := range []string{"user-a", "user-b"} {
		if _, err := database.Exec(`INSERT INTO users (id,email,name,created_at,updated_at) VALUES (?,?,?,?,?)`, id, id+"@example.com", id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	res, err := database.Exec(`INSERT INTO games (bgg_id,name,created_at,updated_at) VALUES (?,?,?,?)`, 266192, "Wingspan", now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO user_games (user_id,game_id,owned,added_at,updated_at) VALUES (?,?,1,?,?)`, "user-a", gameID, now, now); err != nil {
		t.Fatal(err)
	}
	return &setupFixture{repo: NewRepo(database), user: "user-a", other: "user-b", gameID: gameID}
}

func TestSetupNotesPersist(t *testing.T) {
	f := newSetupFixture(t)
	ctx := context.Background()

	notes := "  - Place the board\n- Shuffle the bird deck  "
	if err := f.repo.Apply(ctx, f.user, f.gameID, CollectionPatch{SetupNotes: &notes}); err != nil {
		t.Fatal(err)
	}
	detail, err := f.repo.Get(ctx, f.user, f.gameID)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(notes); detail.SetupNotes != want {
		t.Fatalf("setup notes = %q, want %q", detail.SetupNotes, want)
	}
}

func TestSetupNotesTooLong(t *testing.T) {
	f := newSetupFixture(t)
	ctx := context.Background()
	long := strings.Repeat("a", 5001)
	err := f.repo.Apply(ctx, f.user, f.gameID, CollectionPatch{SetupNotes: &long})
	if err == nil {
		t.Fatal("expected an error for an oversized setup guide")
	}
	exact := strings.Repeat("a", 5000)
	if err := f.repo.Apply(ctx, f.user, f.gameID, CollectionPatch{SetupNotes: &exact}); err != nil {
		t.Fatalf("5000 runes should be accepted: %v", err)
	}
}

func TestSetupImages(t *testing.T) {
	f := newSetupFixture(t)
	ctx := context.Background()

	if err := f.repo.AddSetupImage(ctx, f.user, f.gameID, "img-1", "img-1.jpg", "image/jpeg", "First"); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.AddSetupImage(ctx, f.user, f.gameID, "img-2", "img-2.png", "image/png", ""); err != nil {
		t.Fatal(err)
	}
	count, err := f.repo.CountSetupImages(ctx, f.user, f.gameID)
	if err != nil || count != 2 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
	img, err := f.repo.SetupImage(ctx, f.user, "img-1")
	if err != nil {
		t.Fatal(err)
	}
	if img.URL != "/media/setup/img-1" || img.Caption != "First" || img.ContentType != "image/jpeg" {
		t.Fatalf("unexpected image: %+v", img)
	}
	detail, err := f.repo.Get(ctx, f.user, f.gameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.SetupImages) != 2 || detail.SetupImages[0].ID != "img-1" || detail.SetupImages[1].ID != "img-2" {
		t.Fatalf("unexpected image order: %+v", detail.SetupImages)
	}

	filename, err := f.repo.DeleteSetupImage(ctx, f.user, "img-1")
	if err != nil {
		t.Fatal(err)
	}
	if filename != "img-1.jpg" {
		t.Fatalf("filename = %q", filename)
	}
	if _, err := f.repo.SetupImage(ctx, f.user, "img-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetupImagesAreScopedToUser(t *testing.T) {
	f := newSetupFixture(t)
	ctx := context.Background()
	if err := f.repo.AddSetupImage(ctx, f.user, f.gameID, "img-1", "img-1.jpg", "image/jpeg", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := f.repo.SetupImage(ctx, f.other, "img-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user SetupImage = %v, want ErrNotFound", err)
	}
	if _, err := f.repo.DeleteSetupImage(ctx, f.other, "img-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user DeleteSetupImage = %v, want ErrNotFound", err)
	}
	if err := f.repo.AddSetupImage(ctx, f.other, f.gameID, "img-2", "img-2.jpg", "image/jpeg", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user AddSetupImage = %v, want ErrNotFound", err)
	}
}
