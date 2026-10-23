package trmnl

import (
	"bytes"
	"context"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"boardgames/internal/collection"
	"boardgames/internal/plays"
)

func candidates() []collection.GameCard {
	return []collection.GameCard{
		{ID: 1, BGGID: 101, Name: "Wingspan", Year: 2019, TotalPlays: 3},
		{ID: 2, BGGID: 102, Name: "Brass: Birmingham", Year: 2018},
		{ID: 3, BGGID: 103, Name: "Patchwork", Year: 2014},
	}
}

func TestRecommendIsStableForADate(t *testing.T) {
	first := Recommend("user-1", "2026-10-23", candidates())
	second := Recommend("user-1", "2026-10-23", candidates())
	if first == nil || second == nil {
		t.Fatal("expected a pick")
	}
	if first.ID != second.ID {
		t.Fatalf("pick changed within a day: %d vs %d", first.ID, second.ID)
	}
}

func TestRecommendVariesByUserAndDate(t *testing.T) {
	seen := map[int64]bool{}
	for _, user := range []string{"a", "b", "c", "d", "e"} {
		for _, date := range []string{"2026-10-23", "2026-10-24", "2026-10-25"} {
			p := Recommend(user, date, candidates())
			if p == nil {
				t.Fatal("expected a pick")
			}
			seen[p.ID] = true
		}
	}
	if len(seen) < 2 {
		t.Fatalf("recommendation never varies: %v", seen)
	}
}

func TestRecommendEmptyShelf(t *testing.T) {
	if p := Recommend("user-1", "2026-10-23", nil); p != nil {
		t.Fatalf("expected nil, got %+v", p)
	}
}

func TestPlaceholderIsDeterministicJPEG(t *testing.T) {
	var a, b bytes.Buffer
	if err := jpeg.Encode(&a, placeholder(101), &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&b, placeholder(101), &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("placeholder is not deterministic")
	}
	img, err := jpeg.Decode(bytes.NewReader(a.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 300 || img.Bounds().Dy() != 450 {
		t.Fatalf("unexpected placeholder size %v", img.Bounds())
	}
}

func TestBuildEmptyShelf(t *testing.T) {
	database := testDB(t)
	svc := NewService(database, collection.NewRepo(database), plays.NewRepo(database), testConfig(t))
	payload, err := svc.Build(context.Background(), "missing-user")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Pick != nil {
		t.Fatalf("expected no pick, got %+v", payload.Pick)
	}
	if payload.Date == "" || payload.GeneratedAt.IsZero() {
		t.Fatal("missing generated metadata")
	}
}

func TestImageServiceFallsBackToJPEG(t *testing.T) {
	database := testDB(t)
	svc := NewImageService(database, "")
	req := httptest.NewRequest(http.MethodGet, "/media/bgg/999.jpg", nil)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("unexpected content type %q", ct)
	}
	if _, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("fallback is not a JPEG: %v", err)
	}
}
