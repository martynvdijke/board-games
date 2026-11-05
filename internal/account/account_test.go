package account

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"boardgames/internal/db"
)

func accountFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	ctx := context.Background()
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	dir := t.TempDir()
	now := db.Now()
	if _, err := database.ExecContext(ctx, `INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('owner','owner@example.com','Owner','admin',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return NewWithMedia(database, dir), dir, "owner"
}

func insertSetupImage(t *testing.T, s *Service, userID, filename string) string {
	t.Helper()
	ctx := context.Background()
	now := db.Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(13,'Catan','catan',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO game_setup_images(id,user_id,game_id,filename,content_type,created_at)
		VALUES('img-1',?,?,'photo.jpg','image/jpeg',?)`, userID, gameID, now); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.MediaDir, filename)
	if err := os.WriteFile(path, []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeleteRemovesSetupImageFileAndRow(t *testing.T) {
	ctx := context.Background()
	s, _, userID := accountFixture(t)
	now := db.Now()
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO users(id,email,name,role,created_at,updated_at) VALUES('second','second@example.com','Second','admin',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	path := insertSetupImage(t, s, userID, "photo.jpg")

	if err := s.Delete(ctx, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("setup image still exists: %v", err)
	}
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM game_setup_images WHERE user_id=?`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("game_setup_images rows = %d, want 0", n)
	}
}

func TestDeleteLastAdminLeavesSetupImageIntact(t *testing.T) {
	ctx := context.Background()
	s, _, userID := accountFixture(t)
	path := insertSetupImage(t, s, userID, "photo.jpg")

	err := s.Delete(ctx, userID)
	if !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("delete = %v, want ErrLastAdmin", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("setup image was removed after a rolled-back delete: %v", err)
	}
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM game_setup_images WHERE user_id=?`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("game_setup_images rows = %d, want 1", n)
	}
}

func TestDeleteLastRealAdminIgnoresDemoAdmin(t *testing.T) {
	ctx := context.Background()
	s, _, userID := accountFixture(t)
	now := db.Now()
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO users(id,email,name,role,is_demo,created_at,updated_at)
		VALUES('demo','demo@theshelf.local','Shelf Demo','admin',1,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, userID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("delete = %v, want ErrLastAdmin", err)
	}
}
