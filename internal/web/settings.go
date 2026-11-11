package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/bgg"
	"boardgames/internal/collection"
	"boardgames/internal/db"
	"boardgames/internal/yearreview"
)

// SessionView is one row of the "Where you're signed in" panel.
type SessionView struct {
	ID       string
	Device   string
	IP       string
	Created  string
	LastSeen string
	Current  bool
}

// sessionViews loads the caller's sessions and formats them for display. It
// never exposes the session token hash or CSRF secret.
func (h *Handler) sessionViews(r *http.Request, ctx context.Context, userID string) []SessionView {
	sessions, err := h.Auth.ListSessions(ctx, userID)
	if err != nil {
		slog.Warn("list sessions", "error", err)
		return nil
	}
	current := ""
	if a := auth.FromContext(r.Context()); a != nil && a.Session != nil {
		current = auth.SessionID(a.Session.TokenHash)
	}
	now := time.Now().UTC()
	views := make([]SessionView, 0, len(sessions))
	for _, s := range sessions {
		views = append(views, SessionView{
			ID:       s.ID,
			Device:   deviceLabel(s.UserAgent),
			IP:       s.IP,
			Created:  dateShort(s.CreatedAt),
			LastSeen: relativeTime(s.LastSeenAt, now),
			Current:  s.ID == current,
		})
	}
	return views
}

// deviceLabel turns a User-Agent into "Chrome on macOS"-style text. It is
// deliberately coarse: it names the family and platform, not a version.
func deviceLabel(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "Unknown device"
	}
	var browser string
	switch {
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "EdgA/") || strings.Contains(ua, "EdgiOS/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/") || strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/") || strings.Contains(ua, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	var platform string
	switch {
	case strings.Contains(ua, "iPhone"):
		platform = "iPhone"
	case strings.Contains(ua, "iPad"):
		platform = "iPad"
	case strings.Contains(ua, "Android"):
		platform = "Android"
	case strings.Contains(ua, "CrOS"):
		platform = "ChromeOS"
	case strings.Contains(ua, "Windows"):
		platform = "Windows"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh"):
		platform = "macOS"
	case strings.Contains(ua, "Linux"):
		platform = "Linux"
	}
	switch {
	case browser != "" && platform != "":
		return browser + " on " + platform
	case browser != "":
		return browser
	case platform != "":
		return platform
	default:
		return "Unknown device"
	}
}

// relativeTime renders a stored RFC3339 timestamp as "just now", "2 hours
// ago", and so on. Anything older than a week falls back to a short date.
func relativeTime(ts string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return dateShort(ts)
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < 2*time.Minute:
		return "1 minute ago"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d/time.Minute))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d/time.Hour))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d/(24*time.Hour)))
	default:
		return "on " + dateShort(ts)
	}
}

func (h *Handler) settingsPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	settings, err := h.Auth.Settings(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your settings", err)
		return
	}
	runs, err := h.Sync.History(ctx, user.ID, 8)
	if err != nil && !errors.Is(err, collection.ErrNotFound) {
		slog.Warn("sync history", "error", err)
	}
	tokens, _ := h.Auth.ListTokens(ctx, user.ID)
	sessionViews := h.sessionViews(r, ctx, user.ID)
	token, source := h.Sync.APIToken(ctx)
	var shares []yearreview.Share
	if h.YearReview != nil {
		shares, _ = h.YearReview.List(ctx, user.ID)
	}
	var shareYears []int
	if h.Plays != nil {
		shareYears, _ = h.Plays.PlayYears(ctx, user.ID)
	}
	data := &SettingsData{
		Settings: settings, Runs: runs, Tokens: tokens, Sessions: sessionViews, DocsURL: "/api/docs",
		Shares: shares, ShareYears: shareYears,
		IsAdmin: user.Role == "admin", BGGTokenConfigured: token != "", BGGTokenSource: source,
	}
	p := h.page(r, "Settings", "settings", "")
	p.Data = data
	h.render(w, r, "settings.html", p)
}

// exportAccountData serves the signed-in user's JSON export through the web
// session, because the API route is bearer-authenticated only.
func (h *Handler) exportAccountData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 30*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if h.Account == nil {
		http.Error(w, "export is not configured", http.StatusInternalServerError)
		return
	}
	data, err := h.Account.Export(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't build your export", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="the-shelf-export.json"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (h *Handler) saveBGG(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 40*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	username := strings.TrimSpace(r.PostForm.Get("bgg_username"))
	enabled := r.PostForm.Has("bgg_sync_enabled")
	interval := parseInt(r.PostForm.Get("bgg_sync_interval"), 21600)
	if interval < 900 {
		interval = 900
	}
	if username == "" {
		_, err := h.DB.ExecContext(ctx, `UPDATE user_settings SET bgg_username='',bgg_username_verified_at=NULL,bgg_sync_enabled=0,
			bgg_sync_interval_seconds=?,updated_at=? WHERE user_id=?`, interval, db.Now(), user.ID)
		if err != nil {
			h.flashRedirect(w, r, "/settings", "Settings couldn't be saved", "error")
			return
		}
		h.flashRedirect(w, r, "/settings", "BGG username removed; scheduled sync is off", "success")
		return
	}
	token, _ := h.Sync.APIToken(ctx)
	if err := h.Sync.VerifyUsername(ctx, username, token); err != nil {
		_, _ = h.DB.ExecContext(ctx, `UPDATE user_settings SET bgg_username=?,bgg_username_verified_at=NULL,bgg_sync_enabled=0,
			bgg_sync_interval_seconds=?,updated_at=? WHERE user_id=?`, username, interval, db.Now(), user.ID)
		switch {
		case errors.Is(err, bgg.ErrUnauthorized) && user.Role == "admin":
			h.flashRedirect(w, r, "/settings", "BoardGameGeek rejected the API token. Add a valid token below, then save the username again.", "error")
		case errors.Is(err, bgg.ErrUnauthorized):
			h.flashRedirect(w, r, "/settings", "BoardGameGeek isn't configured yet. Ask an instance admin to add the API token, then try again.", "error")
		case errors.Is(err, bgg.ErrNotFound):
			h.flashRedirect(w, r, "/settings", "BGG didn't recognise that username. It's saved but sync is off until it verifies.", "error")
		default:
			slog.Warn("verify BGG username", "error", err)
			h.flashRedirect(w, r, "/settings", "BGG couldn't verify that username right now. It's saved but sync is off until it verifies.", "error")
		}
		return
	}
	now := db.Now()
	_, err := h.DB.ExecContext(ctx, `UPDATE user_settings SET bgg_username=?,bgg_username_verified_at=?,bgg_sync_enabled=?,
		bgg_sync_interval_seconds=?,updated_at=? WHERE user_id=?`, username, now, boolInt(enabled), interval, now, user.ID)
	if err != nil {
		h.flashRedirect(w, r, "/settings", "Settings couldn't be saved", "error")
		return
	}
	h.flashRedirect(w, r, "/settings", "BoardGameGeek account connected", "success")
}

// saveBGGToken stores an instance-wide BoardGameGeek application token.
func (h *Handler) saveBGGToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 40*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = r.ParseForm()
	token := strings.TrimSpace(r.PostForm.Get("bgg_api_token"))
	if token == "" {
		h.flashRedirect(w, r, "/settings", "Enter a BoardGameGeek API token", "error")
		return
	}
	switch err := h.Sync.ValidateAPIToken(ctx, token); {
	case errors.Is(err, bgg.ErrUnauthorized):
		h.flashRedirect(w, r, "/settings", "BGG rejected that API token", "error")
		return
	case err != nil:
		slog.Warn("validate BGG API token", "error", err)
		h.flashRedirect(w, r, "/settings", "The token couldn't be verified with BGG", "error")
		return
	}
	if err := h.Sync.SetAPIToken(ctx, token); err != nil {
		slog.Error("save BGG API token", "error", err)
		h.flashRedirect(w, r, "/settings", "The token couldn't be saved", "error")
		return
	}
	h.flashRedirect(w, r, "/settings", "BGG API token saved. Save the BoardGameGeek connection again to verify your username.", "success")
}

// removeBGGToken clears the instance-level token so the environment fallback
// (if any) applies again.
func (h *Handler) removeBGGToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := h.Sync.SetAPIToken(ctx, ""); err != nil {
		slog.Error("remove BGG API token", "error", err)
		h.flashRedirect(w, r, "/settings", "The token couldn't be removed", "error")
		return
	}
	h.flashRedirect(w, r, "/settings", "BGG API token removed", "success")
}

func (h *Handler) startSync(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	_, err := h.Sync.Start(user.ID, "manual")
	switch {
	case errors.Is(err, collection.ErrNoBGGUsername):
		h.flashRedirect(w, r, "/settings", "Add a BoardGameGeek username first", "error")
	case errors.Is(err, collection.ErrSyncRunning):
		h.flashRedirect(w, r, "/settings", "A sync is already running", "error")
	case err != nil:
		slog.Error("start sync", "error", err)
		h.flashRedirect(w, r, "/settings", "The sync couldn't be started", "error")
	default:
		h.flashRedirect(w, r, "/settings", "Sync started — this can take a few minutes", "success")
	}
}

func (h *Handler) syncStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	run, err := h.Sync.Latest(ctx, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "status unavailable"})
		return
	}
	if run == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "never", "running": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": run.ID, "status": run.Status, "running": run.Status == "running", "phase": run.Phase,
		"collection_count": run.CollectionCount, "things_count": run.ThingsCount, "plays_count": run.PlaysCount,
		"error": run.Error, "trigger": run.Trigger, "started_at": run.StartedAt, "finished_at": run.FinishedAt,
	})
}

func (h *Handler) saveTheme(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	theme := strings.TrimSpace(r.FormValue("theme"))
	switch theme {
	case "light", "dark", "system":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid theme"})
		return
	}
	_, err := h.DB.ExecContext(ctx, `UPDATE user_settings SET theme=?,updated_at=? WHERE user_id=?`, theme, db.Now(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save theme"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "shelf_theme", Value: theme, Path: "/", MaxAge: 31536000, SameSite: http.SameSiteLaxMode, HttpOnly: true, Secure: h.Config.SecureCookies()})
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		writeJSON(w, http.StatusOK, map[string]string{"theme": theme})
		return
	}
	h.flashRedirect(w, r, "/settings", "Appearance saved", "success")
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if name == "" {
		h.flashRedirect(w, r, "/settings", "Give the token a name", "error")
		return
	}
	var scopes []string
	for _, s := range []string{"read", "write", "trmnl"} {
		if r.PostForm.Has("scope_" + s) {
			scopes = append(scopes, s)
		}
	}
	if len(scopes) == 0 {
		h.flashRedirect(w, r, "/settings", "Choose at least one scope", "error")
		return
	}
	raw, _, err := h.Auth.CreateToken(ctx, user.ID, name, scopes)
	if err != nil {
		h.flashRedirect(w, r, "/settings", "The token couldn't be created", "error")
		return
	}
	settings, _ := h.Auth.Settings(ctx, user.ID)
	runs, _ := h.Sync.History(ctx, user.ID, 8)
	tokens, _ := h.Auth.ListTokens(ctx, user.ID)
	p := h.page(r, "Settings", "settings", "")
	p.Data = &SettingsData{Settings: settings, Runs: runs, Tokens: tokens, NewToken: raw, NewTokenName: name, DocsURL: "/api/docs"}
	h.render(w, r, "settings.html", p)
}

func (h *Handler) revokeToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if err := h.Auth.RevokeToken(ctx, user.ID, r.PathValue("id")); err != nil {
		h.flashRedirect(w, r, "/settings", "The token couldn't be revoked", "error")
		return
	}
	h.flashRedirect(w, r, "/settings", "Token revoked", "success")
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.Auth.RevokeSession(ctx, user.ID, id); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			h.errorPage(w, r, http.StatusNotFound, "That device has already been signed out", nil)
			return
		}
		slog.Error("revoke session", "error", err)
		h.flashRedirect(w, r, "/settings", "The device couldn't be signed out", "error")
		return
	}
	if a := auth.FromContext(r.Context()); a != nil && a.Session != nil && auth.SessionID(a.Session.TokenHash) == id {
		h.Auth.ClearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.flashRedirect(w, r, "/settings", "Signed out that device", "success")
}

func (h *Handler) trmnlPreview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	payload, err := h.TRMNL.Build(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "The preview couldn't be built", err)
		return
	}
	p := h.page(r, "TRMNL preview", "settings", "")
	p.Data = &TRMNLData{Payload: payload}
	h.render(w, r, "trmnl_preview.html", p)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
