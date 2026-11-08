package trmnl

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// jpegBytes produces a small but valid, uniquely coloured JPEG.
func jpegBytes(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{shade, 120, 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func placeholderBytes(t *testing.T, bggID int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, placeholder(bggID), &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// imageServer serves the given bodies for the thumbnail and image paths.
func imageServer(t *testing.T, thumb, full []byte, thumbStatus, fullStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body []byte, status int) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("/thumb.jpg", func(w http.ResponseWriter, r *http.Request) { write(w, thumb, thumbStatus) })
	mux.HandleFunc("/image.jpg", func(w http.ResponseWriter, r *http.Request) { write(w, full, fullStatus) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func insertGame(t *testing.T, svc *ImageService, bggID int, thumb, imageURL string) {
	t.Helper()
	_, err := svc.DB.ExecContext(context.Background(),
		`INSERT INTO games(bgg_id,name,image_url,thumbnail_url,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		bggID, "Game", imageURL, thumb, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, svc *ImageService, bggID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/media/bgg/"+bggID+".jpg", nil)
	rr := httptest.NewRecorder()
	svc.ServeHTTP(rr, req)
	return rr
}

func TestImageFallsBackToImageURL(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	thumb := jpegBytes(t, 10)
	full := jpegBytes(t, 200)
	srv := imageServer(t, thumb, full, http.StatusInternalServerError, http.StatusOK)
	insertGame(t, svc, 13, srv.URL+"/thumb.jpg", srv.URL+"/image.jpg")

	rr := get(t, svc, "13")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != successCacheControl {
		t.Fatalf("cache-control = %q", got)
	}
	body := rr.Body.Bytes()
	if bytes.Equal(body, placeholderBytes(t, 13)) {
		t.Fatal("served the placeholder instead of the fallback image")
	}
	if !bytes.Equal(body, full) {
		t.Fatal("did not serve the image_url bytes")
	}
	// Success is cached to disk.
	cached, err := os.ReadFile(filepath.Join(dir, "13.jpg"))
	if err != nil {
		t.Fatalf("cover was not cached: %v", err)
	}
	if !bytes.Equal(cached, full) {
		t.Fatal("cached bytes differ from fetched bytes")
	}
}

func TestImagePrefersThumbnail(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	thumb := jpegBytes(t, 10)
	full := jpegBytes(t, 200)
	srv := imageServer(t, thumb, full, http.StatusOK, http.StatusOK)
	insertGame(t, svc, 21, srv.URL+"/thumb.jpg", srv.URL+"/image.jpg")

	body := get(t, svc, "21").Body.Bytes()
	if !bytes.Equal(body, thumb) {
		t.Fatal("thumbnail was not tried first")
	}
}

func TestImagePlaceholderIsNotCached(t *testing.T) {
	cases := []struct {
		name  string
		thumb string
		full  string
	}{
		{name: "empty urls"},
		{name: "both fail", thumb: "FAIL", full: "FAIL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			svc := NewImageService(testDB(t), dir)
			thumb, full := "", ""
			if tc.thumb != "" || tc.full != "" {
				srv := imageServer(t, nil, nil, http.StatusInternalServerError, http.StatusInternalServerError)
				thumb, full = srv.URL+"/thumb.jpg", srv.URL+"/image.jpg"
			}
			insertGame(t, svc, 174430, thumb, full)

			rr := get(t, svc, "174430")
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d", rr.Code)
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache-control = %q, want no-store", got)
			}
			if !bytes.Equal(rr.Body.Bytes(), placeholderBytes(t, 174430)) {
				t.Fatal("expected the placeholder")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("placeholder was written to disk: %v", entries)
			}
		})
	}
}

func TestImageMissingRowPlaceholder(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)

	rr := get(t, svc, "999999")
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
	if !bytes.Equal(rr.Body.Bytes(), placeholderBytes(t, 999999)) {
		t.Fatal("expected the placeholder")
	}
}

func TestImageDiskCacheSurvivesSourceDown(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	full := jpegBytes(t, 90)
	srv := imageServer(t, nil, full, http.StatusInternalServerError, http.StatusOK)
	insertGame(t, svc, 7, "", srv.URL+"/image.jpg")

	first := get(t, svc, "7").Body.Bytes()
	if !bytes.Equal(first, full) {
		t.Fatal("first request did not serve the image")
	}
	srv.Close()

	second := get(t, svc, "7")
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d", second.Code)
	}
	if got := second.Header().Get("Cache-Control"); got != successCacheControl {
		t.Fatalf("cache-control = %q", got)
	}
	if !bytes.Equal(second.Body.Bytes(), full) {
		t.Fatal("cached bytes were not served after the source went down")
	}
}

// countingServer counts every cover request it receives.
func countingServer(t *testing.T, status int, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	serve := func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write(body)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/thumb.jpg", serve)
	mux.HandleFunc("/image.jpg", serve)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &count
}

func TestNegativeCacheSkipsRefetch(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	srv, count := countingServer(t, http.StatusInternalServerError, nil)
	insertGame(t, svc, 55, "", srv.URL+"/image.jpg")

	for i := 0; i < 2; i++ {
		rr := get(t, svc, "55")
		if got := rr.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("request %d: cache-control = %q, want no-store", i+1, got)
		}
		if !bytes.Equal(rr.Body.Bytes(), placeholderBytes(t, 55)) {
			t.Fatalf("request %d: expected the placeholder", i+1)
		}
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("source requests = %d, want 1 (negative cache should skip the second)", got)
	}
}

func TestNegativeCacheExpires(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	svc.negativeTTL = 10 * time.Millisecond
	srv, count := countingServer(t, http.StatusInternalServerError, nil)
	insertGame(t, svc, 56, "", srv.URL+"/image.jpg")

	get(t, svc, "56")
	if got := count.Load(); got != 1 {
		t.Fatalf("source requests after first = %d, want 1", got)
	}
	time.Sleep(25 * time.Millisecond)
	rr := get(t, svc, "56")
	if got := count.Load(); got != 2 {
		t.Fatalf("source requests after expiry = %d, want 2", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestNegativeCacheClearedOnSuccess(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(testDB(t), dir)
	svc.negativeTTL = 0 // cached failures are immediately stale for this test
	full := jpegBytes(t, 77)
	var healthy atomic.Bool
	var count atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/image.jpg", func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if !healthy.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(full)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	insertGame(t, svc, 66, "", srv.URL+"/image.jpg")

	first := get(t, svc, "66")
	if !bytes.Equal(first.Body.Bytes(), placeholderBytes(t, 66)) {
		t.Fatal("expected the placeholder while the source is broken")
	}

	healthy.Store(true)
	second := get(t, svc, "66")
	if got := second.Header().Get("Cache-Control"); got != successCacheControl {
		t.Fatalf("cache-control = %q, want success caching", got)
	}
	if !bytes.Equal(second.Body.Bytes(), full) {
		t.Fatal("repaired source was not served")
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("source requests = %d, want 2", got)
	}

	svc.mu.Lock()
	_, present := svc.negatives[66]
	svc.mu.Unlock()
	if present {
		t.Fatal("negative entry was not cleared after a successful load")
	}
}
