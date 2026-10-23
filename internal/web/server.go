// Package web serves the server-rendered shelf UI and its HTML fragments.
package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/plays"
	"boardgames/internal/trmnl"
)

type Handler struct {
	Config         *config.Config
	Auth           *auth.Manager
	Games          *collection.Repo
	Plays          *plays.Repo
	Sync           *collection.SyncService
	TRMNL          *trmnl.Service
	Images         *trmnl.ImageService
	DB             *sql.DB
	MetricsHandler http.Handler
}

type Page struct {
	Title          string
	AppName        string
	BaseURL        string
	User           *auth.User
	CSRF           string
	Active         string
	Theme          string
	Flash          string
	FlashKind      string
	Error          string
	Data           any
	Year           int
	UmamiURL       string
	UmamiWebsiteID string
}

type OverviewData struct {
	Filters   collection.Filters
	Games     []collection.GameCard
	Total     int
	Facets    collection.Facets
	Stats     collection.ShelfStats
	Pick      *trmnl.Pick
	Greeting  string
	FirstName string
	Pages     []int
	PageCount int
	View      string
}

func (d *OverviewData) queryURL(page int) string {
	v := url.Values{}
	if d.Filters.Query != "" {
		v.Set("q", d.Filters.Query)
	}
	if d.Filters.Players > 0 {
		v.Set("players", strconv.Itoa(d.Filters.Players))
	}
	if d.Filters.MaxPlaytime > 0 {
		v.Set("max_time", strconv.Itoa(d.Filters.MaxPlaytime))
	}
	if d.Filters.MinWeight > 0 {
		v.Set("min_weight", strconv.FormatFloat(d.Filters.MinWeight, 'f', 1, 64))
	}
	if d.Filters.MaxWeight > 0 {
		v.Set("max_weight", strconv.FormatFloat(d.Filters.MaxWeight, 'f', 1, 64))
	}
	if d.Filters.Category != "" {
		v.Set("category", d.Filters.Category)
	}
	if d.Filters.Mechanic != "" {
		v.Set("mechanic", d.Filters.Mechanic)
	}
	if d.Filters.Status != "" {
		v.Set("status", d.Filters.Status)
	}
	if d.Filters.Sort != "" && d.Filters.Sort != "name" {
		v.Set("sort", d.Filters.Sort)
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	v.Set("view", d.View)
	if enc := v.Encode(); enc != "" {
		return "/?" + enc
	}
	return "/"
}

type GameData struct {
	Game      *collection.GameDetail
	PlayToday string
}

type PlaysData struct {
	Groups    []PlayGroup
	Total     int
	Year      int
	GameID    int64
	Years     []int
	Games     []plays.GameOption
	Page      int
	PageCount int
	PrevURL   string
	NextURL   string
}

type PlayFormData struct {
	Play           *plays.Play
	Attempt        playAttempt
	Games          []plays.GameOption
	Today          string
	Editing        bool
	SelectedGameID int64
}

type playAttempt struct {
	GameID   int64
	PlayedAt string
	Location string
	Duration int
	Quantity int
	Notes    string
	Players  []plays.Player
}

type StatsData struct {
	Stats    plays.Statistics
	Shelf    collection.ShelfStats
	Years    []int
	ChartMax int
	TopMax   int
}

type SettingsData struct {
	Settings     auth.UserSettings
	Runs         []collection.SyncRun
	Tokens       []auth.Token
	NewToken     string
	NewTokenName string
	BGGError     string
	DocsURL      string
}

type TRMNLData struct {
	Payload *trmnl.Payload
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	if h.Config.MetricsEnabled {
		mux.Handle("GET /metrics", h.metricsHandler())
	} else {
		mux.HandleFunc("GET /metrics", http.NotFound)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServer(http.FS(StaticFS())))))
	mux.Handle("GET /media/bgg/", h.Images)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("GET /auth/login", h.Auth.Login)
	mux.HandleFunc("GET /auth/callback", h.Auth.Callback)
	mux.HandleFunc("POST /logout", h.Auth.Logout)

	mux.HandleFunc("GET /{$}", h.requireUser(h.overview))
	mux.HandleFunc("GET /games/{id}", h.requireUser(h.gameDetail))
	mux.HandleFunc("POST /games/{id}/collection", h.requireUser(h.updateCollection))
	mux.HandleFunc("GET /partials/games", h.requireUser(h.gamesPartial))
	mux.HandleFunc("GET /plays", h.requireUser(h.playsPage))
	mux.HandleFunc("GET /plays/new", h.requireUser(h.newPlayPage))
	mux.HandleFunc("POST /plays", h.requireUser(h.createPlay))
	mux.HandleFunc("GET /plays/{id}/edit", h.requireUser(h.editPlayPage))
	mux.HandleFunc("POST /plays/{id}", h.requireUser(h.updatePlay))
	mux.HandleFunc("POST /plays/{id}/delete", h.requireUser(h.deletePlay))
	mux.HandleFunc("GET /stats", h.requireUser(h.statsPage))
	mux.HandleFunc("GET /settings", h.requireUser(h.settingsPage))
	mux.HandleFunc("POST /settings/bgg", h.requireUser(h.saveBGG))
	mux.HandleFunc("POST /settings/sync", h.requireUser(h.startSync))
	mux.HandleFunc("GET /settings/sync/status", h.requireUser(h.syncStatus))
	mux.HandleFunc("POST /settings/theme", h.requireUser(h.saveTheme))
	mux.HandleFunc("POST /settings/tokens", h.requireUser(h.createToken))
	mux.HandleFunc("POST /settings/tokens/{id}/revoke", h.requireUser(h.revokeToken))
	mux.HandleFunc("GET /trmnl/preview", h.requireUser(h.trmnlPreview))

	return h.Auth.LoadSession(h.securityHeaders(h.Auth.RequireCSRF(mux)))
}

func (h *Handler) metricsHandler() http.Handler {
	if h.MetricsHandler != nil {
		return h.MetricsHandler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 2*time.Second)
	defer cancel()
	if err := h.DB.PingContext(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable","database":"unreachable"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ready","database":"ok"}`))
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if a := auth.FromContext(r.Context()); a != nil && a.User != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	theme := cookieTheme(r)
	p := h.page(r, "Sign in", "", "")
	p.Theme = theme
	p.Data = map[string]string{"Next": r.URL.Query().Get("next"), "Error": r.URL.Query().Get("error")}
	h.render(w, r, "login.html", p)
}

func (h *Handler) page(r *http.Request, title, active string, data any) Page {
	p := Page{Title: title, AppName: h.Config.AppName, BaseURL: h.Config.BaseURL, Active: active, Year: time.Now().Year(),
		UmamiURL: h.Config.UmamiURL, UmamiWebsiteID: h.Config.UmamiWebsiteID}
	if a := auth.FromContext(r.Context()); a != nil && a.User != nil {
		p.User = a.User
		p.CSRF = a.CSRFToken
	}
	p.Theme = cookieTheme(r)
	if p.User != nil {
		if s, err := h.Auth.Settings(r.Context(), p.User.ID); err == nil {
			p.Theme = s.Theme
		}
	}
	p.Flash = r.URL.Query().Get("flash")
	p.FlashKind = r.URL.Query().Get("kind")
	if p.FlashKind == "" {
		p.FlashKind = "success"
	}
	return p
}

func cookieTheme(r *http.Request) string {
	if c, err := r.Cookie("shelf_theme"); err == nil {
		switch c.Value {
		case "light", "dark", "system":
			return c.Value
		}
	}
	return "system"
}

func (h *Handler) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := auth.FromContext(r.Context())
		if a == nil || a.User == nil {
			if r.Method == http.MethodGet {
				next := url.QueryEscape(r.URL.RequestURI())
				http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (h *Handler) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		script := "'self' 'unsafe-inline'"
		connect := "'self'"
		if h.Config.UmamiURL != "" {
			if u, err := url.Parse(h.Config.UmamiURL); err == nil && u.Host != "" {
				origin := u.Scheme + "://" + u.Host
				script += " " + origin
				connect += " " + origin
			}
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src "+script+"; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src "+connect+"; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if h.Config.SecureCookies() && h.Config.Env == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}

func parseInt(v string, fallback int) int {
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}

func parseFloat(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

func (h *Handler) flashRedirect(w http.ResponseWriter, r *http.Request, path, message, kind string) {
	v := url.Values{}
	v.Set("flash", message)
	if kind != "" {
		v.Set("kind", kind)
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	http.Redirect(w, r, path+sep+v.Encode(), http.StatusSeeOther)
}

func (h *Handler) settingsFor(r *http.Request, userID string) auth.UserSettings {
	s, err := h.Auth.Settings(r.Context(), userID)
	if err != nil {
		slog.Warn("load settings", "error", err)
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func contextWithTimeout(r *http.Request, d time.Duration) (ctx context.Context, cancel context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}
