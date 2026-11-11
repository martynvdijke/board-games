package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/observability"
	"boardgames/internal/yearreview"
)

// yearRecap renders the private annual recap at /year/{year}.
func (h *Handler) yearRecap(w http.ResponseWriter, r *http.Request) {
	year, ok := parseRecapYear(r.PathValue("year"))
	if !ok {
		h.errorPage(w, r, http.StatusNotFound, "That year isn't here", nil)
		return
	}
	user := auth.UserFromContext(r.Context())
	data, err := h.yearData(r, user.ID, year)
	if err != nil {
		if errors.Is(err, yearreview.ErrYearOutOfRange) {
			h.errorPage(w, r, http.StatusNotFound, "That year isn't here", err)
			return
		}
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't build your year", err)
		return
	}
	if h.YearReview != nil {
		if share, err := h.YearReview.ActiveFor(r.Context(), user.ID, year); err == nil {
			data.ActiveShare = share
		}
	}
	// The owner's own page always shows the name.
	data.NemesisNamed = true
	p := h.page(r, fmt.Sprintf("%d in review", year), "stats", "")
	p.Data = data
	h.render(w, r, "year.html", p)
}

// yearSharePreview renders the public page exactly as it would be published,
// without creating anything. It is the confirm step before a link exists.
func (h *Handler) yearSharePreview(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	year, ok := parseRecapYear(r.URL.Query().Get("year"))
	if !ok {
		h.flashRedirect(w, r, "/settings#year-review", "Choose a year that has a recap", "error")
		return
	}
	includePlayers := r.URL.Query().Get("include_players") != ""
	data, err := h.yearData(r, user.ID, year)
	if err != nil {
		if errors.Is(err, yearreview.ErrYearOutOfRange) {
			h.flashRedirect(w, r, "/settings#year-review", "That year has no recap, so there is nothing to share", "error")
			return
		}
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't build your year", err)
		return
	}
	data.Preview = true
	data.IncludePlayers = includePlayers
	data.NemesisNamed = includePlayers
	p := h.page(r, fmt.Sprintf("Preview of %d in review", year), "settings", "")
	// The preview must not be indexed either: it is not the published page.
	p.NoIndex = true
	p.Data = data
	h.render(w, r, "year_share.html", p)
}

// yearShareCreate mints the public link after the user has seen the preview.
func (h *Handler) yearShareCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	year, ok := parseRecapYear(r.PostForm.Get("year"))
	if !ok {
		h.flashRedirect(w, r, "/settings#year-review", "Choose a year that has a recap", "error")
		return
	}
	includePlayers := r.PostForm.Get("include_players") != ""
	if h.YearReview == nil {
		h.errorPage(w, r, http.StatusInternalServerError, "Sharing is not available", nil)
		return
	}
	// Refuse a year the recap page cannot render, so a share link never points
	// at a page that answers not found.
	if _, err := h.yearData(r, user.ID, year); err != nil {
		if errors.Is(err, yearreview.ErrYearOutOfRange) {
			h.flashRedirect(w, r, "/settings#year-review", "That year has no recap, so there is nothing to share", "error")
			return
		}
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't build your year", err)
		return
	}
	share, err := h.YearReview.Create(r.Context(), user.ID, year, includePlayers)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't create that link", err)
		return
	}
	h.flashRedirect(w, r, "/settings#year-review", "Public link ready: "+shareURL(h.Config.BaseURL, share.Token), "success")
}

// yearShareRevoke kills one link. The public route then answers not found.
func (h *Handler) yearShareRevoke(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 || h.YearReview == nil {
		h.flashRedirect(w, r, "/settings#year-review", "That link isn't here", "error")
		return
	}
	if err := h.YearReview.Revoke(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, yearreview.ErrNotFound) {
			h.flashRedirect(w, r, "/settings#year-review", "That link isn't here", "error")
			return
		}
		h.flashRedirect(w, r, "/settings#year-review", "We couldn't revoke that link", "error")
		return
	}
	h.flashRedirect(w, r, "/settings#year-review", "Link revoked. It now answers not found", "success")
}

// publicYearShare serves /s/{token}. It is registered outside the session and
// CSRF wrappers, so it never reads a session cookie, never sets one, and has no
// CSRF token to leak. The share store already filters revoked rows and compares
// tokens in constant time; an unknown token and a revoked token both look like
// "not found" here.
func (h *Handler) publicYearShare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if !publicShareLimits.allow(h.clientIP(r)) {
		w.Header().Set("Retry-After", strconv.Itoa(int(publicShareWindow.Seconds())))
		http.Error(w, "Too many requests. Try again shortly.", http.StatusTooManyRequests)
		return
	}
	if h.YearReview == nil {
		h.shareNotFound(w, r)
		return
	}
	token := strings.TrimSpace(r.PathValue("token"))
	share, err := h.YearReview.ByToken(r.Context(), token)
	if err != nil {
		slog.Warn("load year share", "error", err)
		h.shareNotFound(w, r)
		return
	}
	if share == nil {
		h.shareNotFound(w, r)
		return
	}
	data, err := h.yearData(r, share.UserID, share.Year)
	if err != nil {
		// A share can outlive the year's data (an emptied play log). Answering
		// not found keeps the link honest rather than publishing an empty page.
		slog.Warn("build shared year", "year", share.Year, "error", err)
		h.shareNotFound(w, r)
		return
	}
	data.Public = true
	data.IncludePlayers = share.IncludePlayers
	data.NemesisNamed = share.IncludePlayers
	p := h.publicPage(r, fmt.Sprintf("%d in review", share.Year), data)
	h.render(w, r, "year_share.html", p)
}

// yearData loads one recap plus the years the page can move between.
func (h *Handler) yearData(r *http.Request, userID string, year int) (*YearData, error) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	// A play belongs to the year the user lived in, not the UTC year. Use the
	// same user timezone the play log validates against (with the same UTC
	// fallback), so /year/{year} cannot disagree with /plays and /stats.
	settings := h.settingsFor(r, userID)
	recap, err := yearreview.ComputeAt(ctx, h.DB, userID, year, yearreview.Location(settings.Timezone))
	if err != nil {
		return nil, err
	}
	var years []int
	if h.Plays != nil {
		years, _ = h.Plays.PlayYears(ctx, userID)
	}
	older, newer := yearNeighbours(years, year)
	return &YearData{
		Recap:    recap,
		Years:    years,
		Hours:    formatHours(recap.Minutes),
		PrevYear: older,
		NextYear: newer,
	}, nil
}

// shareNotFound answers the public share route with the app's not-found page,
// minus anything that would leave the instance or leak the query string.
func (h *Handler) shareNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	p := h.publicPage(r, "404", map[string]any{"Status": http.StatusNotFound, "Message": "That recap isn't here"})
	h.render(w, r, "error.html", p)
}

// publicPage builds a page model with nothing that reaches outside the
// instance: no analytics, no flash text from the query string, and noindex.
func (h *Handler) publicPage(r *http.Request, title string, data any) Page {
	p := h.page(r, title, "", nil)
	p.Data = data
	p.UmamiURL, p.UmamiWebsiteID = "", ""
	p.AnalyticsOptOut = true
	p.Flash, p.FlashKind = "", ""
	p.NoIndex = true
	return p
}

// clientIP resolves the caller behind the configured proxies, the same way the
// rate limiters in internal/api and the demo sign-in do.
func (h *Handler) clientIP(r *http.Request) string {
	return observability.RealIP(r, observability.ParseTrustedProxies(h.Config.TrustedProxies))
}

// parseRecapYear accepts only a four-digit year, so a malformed path answers
// not found instead of reaching the aggregate with a nonsense value.
func parseRecapYear(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) != 4 {
		return 0, false
	}
	year, err := strconv.Atoi(raw)
	if err != nil || year < 1970 {
		return 0, false
	}
	return year, true
}

// yearNeighbours returns the nearest years with plays on either side of year.
func yearNeighbours(years []int, year int) (older, newer int) {
	for _, y := range years {
		if y < year && y > older {
			older = y
		}
		if y > year && (newer == 0 || y < newer) {
			newer = y
		}
	}
	return older, newer
}

// formatHours turns recorded minutes into the hours figure the letter prints,
// keeping one decimal and dropping a bare ".0".
func formatHours(minutes int) string {
	if minutes <= 0 {
		return "0"
	}
	return strings.TrimSuffix(strconv.FormatFloat(float64(minutes)/60, 'f', 1, 64), ".0")
}

// shareURL builds the public address of a share from the configured base URL.
func shareURL(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/s/" + token
}

// shareLimiter is a per-caller fixed-window limiter for the public share route.
// internal/api's limiter is unexported, so this mirrors the demo sign-in
// limiter already in this package rather than inventing a second shape.
type shareLimiter struct {
	mu      sync.Mutex
	buckets map[string]*shareBucket
}

type shareBucket struct {
	count int
	reset time.Time
}

const (
	publicShareLimit  = 60
	publicShareWindow = time.Minute
)

var publicShareLimits = &shareLimiter{buckets: map[string]*shareBucket{}}

func (l *shareLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &shareBucket{count: 1, reset: now.Add(publicShareWindow)}
		return true
	}
	if b.count >= publicShareLimit {
		return false
	}
	b.count++
	return true
}
