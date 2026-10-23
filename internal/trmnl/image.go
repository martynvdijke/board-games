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
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ImageService struct {
	DB      *sql.DB
	Dir     string
	HTTP    *http.Client
	BaseURL string
}

func NewImageService(db *sql.DB, dir string) *ImageService {
	return &ImageService{DB: db, Dir: dir, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// Serve streams a cached cover, fetching it from BGG once. Failures produce a
// deterministic generated placeholder, never an error page.
func (s *ImageService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/media/bgg/"), ".jpg")
	bggID, err := strconv.Atoi(idStr)
	if err != nil || bggID <= 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if s.Dir != "" {
		path := filepath.Join(s.Dir, fmt.Sprintf("%d.jpg", bggID))
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(data)
			return
		}
		var source string
		err := s.DB.QueryRowContext(r.Context(), `SELECT CASE WHEN thumbnail_url<>'' THEN thumbnail_url ELSE image_url END FROM games WHERE bgg_id=?`, bggID).Scan(&source)
		if err == nil && source != "" {
			if data, err := s.fetch(r.Context(), source); err == nil {
				_ = os.MkdirAll(s.Dir, 0o750)
				tmp := path + ".tmp"
				if os.WriteFile(tmp, data, 0o640) == nil {
					_ = os.Rename(tmp, path)
				}
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write(data)
				return
			}
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_ = jpeg.Encode(w, placeholder(bggID), &jpeg.Options{Quality: 82})
}

func (s *ImageService) fetch(ctx context.Context, source string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TheShelf/1.0 (cover cache)")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch: HTTP %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "image/") {
		return nil, errors.New("cover response is not an image")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty cover")
	}
	// Verify the bytes really decode before caching.
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, err
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
