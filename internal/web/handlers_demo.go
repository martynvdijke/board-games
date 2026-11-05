package web

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"boardgames/internal/observability"
)

type demoLoginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*demoLoginBucket
}

type demoLoginBucket struct {
	count int
	reset time.Time
}

var demoLogins = &demoLoginLimiter{buckets: map[string]*demoLoginBucket{}}

func (l *demoLoginLimiter) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &demoLoginBucket{count: 1, reset: now.Add(window)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

// demoLogin starts a session for the review account. It is only available when
// the demo account is configured, and it never accepts an external next URL.
func (h *Handler) demoLogin(w http.ResponseWriter, r *http.Request) {
	if !h.Config.DemoUserEnabled {
		http.NotFound(w, r)
		return
	}
	trusted := observability.ParseTrustedProxies(h.Config.TrustedProxies)
	if !demoLogins.allow(observability.RealIP(r, trusted), 30, time.Minute) {
		w.Header().Set("Retry-After", strconv.Itoa(60))
		http.Error(w, "too many demo sign-in attempts", http.StatusTooManyRequests)
		return
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		u, err := url.Parse(origin)
		base, _ := url.Parse(h.Config.BaseURL)
		if err != nil || base == nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
	}
	_ = r.ParseForm()
	next := strings.TrimSpace(r.FormValue("next"))
	if next == "" {
		next = strings.TrimSpace(r.URL.Query().Get("next"))
	}
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsAny(next, "\\\r\n\t") {
		next = "/"
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	var userID string
	err := h.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE email=? AND is_demo=1`, h.Config.DemoUserEmail).Scan(&userID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slog.Warn("demo login requested before the account exists")
		http.NotFound(w, r)
		return
	case err != nil:
		slog.Error("look up demo account", "error", err)
		http.Error(w, "demo sign-in is unavailable", http.StatusInternalServerError)
		return
	}
	if err := h.Auth.StartSession(w, r, userID); err != nil {
		slog.Error("start demo session", "error", err)
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}
