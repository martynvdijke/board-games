package trmnl

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	successCacheControl = "public, max-age=604800, stale-while-revalidate=86400"
	// defaultNegativeTTL bounds how long a known-bad cover is skipped in-process.
	defaultNegativeTTL = 5 * time.Minute
)

type ImageService struct {
	DB      *sql.DB
	Dir     string
	HTTP    *http.Client
	BaseURL string

	mu          sync.Mutex
	negatives   map[int]time.Time // bggID -> entry expiry
	negativeTTL time.Duration
}

func NewImageService(db *sql.DB, dir string) *ImageService {
	return &ImageService{
		DB:          db,
		Dir:         dir,
		HTTP:        &http.Client{Timeout: 20 * time.Second},
		negatives:   make(map[int]time.Time),
		negativeTTL: defaultNegativeTTL,
	}
}

// negativeCached reports whether a recent failure for bggID should be skipped.
// Expired entries are dropped lazily.
func (s *ImageService) negativeCached(bggID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiry, ok := s.negatives[bggID]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(s.negatives, bggID)
		return false
	}
	return true
}

func (s *ImageService) markNegative(bggID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.negatives == nil {
		s.negatives = make(map[int]time.Time)
	}
	s.negatives[bggID] = time.Now().Add(s.negativeTTL)
}

func (s *ImageService) clearNegative(bggID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.negatives, bggID)
}

// Serve streams a cached cover, fetching it from BGG once. Failures produce a
// deterministic generated placeholder, never an error page, and are never
// cached so the next request retries.
func (s *ImageService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/media/bgg/"), ".jpg")
	bggID, err := strconv.Atoi(idStr)
	if err != nil || bggID <= 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	var path string
	if s.Dir != "" {
		path = filepath.Join(s.Dir, fmt.Sprintf("%d.jpg", bggID))
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			writeImage(w, data)
			return
		}
	}

	if s.negativeCached(bggID) {
		servePlaceholder(w, bggID)
		return
	}

	data, reason := s.load(r.Context(), bggID)
	if data != nil {
		s.clearNegative(bggID)
		if path != "" {
			_ = os.MkdirAll(s.Dir, 0o750)
			tmp := path + ".tmp"
			if os.WriteFile(tmp, data, 0o640) == nil {
				_ = os.Rename(tmp, path)
			}
		}
		writeImage(w, data)
		return
	}

	s.markNegative(bggID)
	slog.Warn("bgg cover unavailable", "bgg_id", bggID, "reason", reason)
	servePlaceholder(w, bggID)
}

// servePlaceholder writes an uncacheable placeholder; browsers retry, while the
// in-process negative cache keeps the server from hitting BGG every time.
func servePlaceholder(w http.ResponseWriter, bggID int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	_ = jpeg.Encode(w, placeholder(bggID), &jpeg.Options{Quality: 82})
}

func writeImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Cache-Control", successCacheControl)
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(data)
}

// load fetches the first usable cover source for a BGG id. It returns the image
// bytes, or nil plus a short reason when nothing could be fetched.
func (s *ImageService) load(ctx context.Context, bggID int) ([]byte, string) {
	var thumb, imageURL string
	err := s.DB.QueryRowContext(ctx, `SELECT thumbnail_url, image_url FROM games WHERE bgg_id=?`, bggID).Scan(&thumb, &imageURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "missing row"
	}
	if err != nil {
		return nil, "query failed"
	}

	var candidates []string
	for _, source := range []string{thumb, imageURL} {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		duplicate := false
		for _, existing := range candidates {
			if existing == source {
				duplicate = true
				break
			}
		}
		if !duplicate {
			candidates = append(candidates, source)
		}
	}
	if len(candidates) == 0 {
		return nil, "no urls"
	}

	reason := "fetch failed"
	for _, source := range candidates {
		data, err := s.fetch(ctx, source)
		if err == nil {
			return data, ""
		}
		reason = err.Error()
	}
	return nil, reason
}

func (s *ImageService) fetch(ctx context.Context, source string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, errors.New("bad request")
	}
	req.Header.Set("User-Agent", "TheShelf/1.0 (cover cache)")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "image/") {
		return nil, errors.New("not an image")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil {
		return nil, errors.New("read failed")
	}
	if len(data) == 0 {
		return nil, errors.New("empty response")
	}
	// Verify the bytes really decode before caching.
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, errors.New("undecodable image")
	}
	return data, nil
}

// placeholder draws a warm, deterministic meeple-like silhouette.
func placeholder(bggID int) image.Image {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strconv.Itoa(bggID)))
	seed := h.Sum32()
	palette := []color.RGBA{
		{31, 90, 69, 255}, {201, 123, 60, 255}, {135, 74, 56, 255}, {67, 92, 110, 255}, {158, 132, 78, 255},
	}
	bg := palette[int(seed)%len(palette)]
	img := image.NewRGBA(image.Rect(0, 0, 300, 450))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{247, 242, 232, 255}}, image.Point{}, draw.Src)
	// Cover field.
	draw.Draw(img, image.Rect(18, 18, 282, 432), &image.Uniform{bg}, image.Point{}, draw.Src)
	cream := color.RGBA{247, 242, 232, 255}
	// Meeple body and head.
	draw.Draw(img, image.Rect(137, 130, 163, 162), &image.Uniform{cream}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(105, 162, 195, 235), &image.Uniform{cream}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(84, 205, 112, 300), &image.Uniform{cream}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(188, 205, 216, 300), &image.Uniform{cream}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(115, 235, 143, 350), &image.Uniform{cream}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(157, 235, 185, 350), &image.Uniform{cream}, image.Point{}, draw.Src)
	// Rounded look through small corner cuts.
	for _, r := range []image.Rectangle{image.Rect(18, 18, 40, 40), image.Rect(260, 18, 282, 40), image.Rect(18, 410, 40, 432), image.Rect(260, 410, 282, 432)} {
		draw.Draw(img, r, &image.Uniform{color.RGBA{247, 242, 232, 255}}, image.Point{}, draw.Src)
	}
	return img
}
