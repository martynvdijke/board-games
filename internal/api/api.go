// Package api exposes the versioned JSON API used by the Android companion,
// scripts, and TRMNL. Authentication is bearer-only and scopes are enforced
// per route.
package api

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/loans"
	"boardgames/internal/observability"
	"boardgames/internal/plays"
	"boardgames/internal/trmnl"
	"boardgames/internal/venues"
)

//go:embed openapi.json docs.html
var assets embed.FS

type Handler struct {
	Config  *config.Config
	Auth    *auth.Manager
	Account *account.Service
	Games   *collection.Repo
	Plays   *plays.Repo
	Loans   *loans.Repo
	Sync    *collection.SyncService
	TRMNL   *trmnl.Service
	Venues  *venues.Repo
	DB      *sql.DB
}

// NewHandler assembles an API handler from its dependencies. cmd/server and
// the tests both build the handler here, so a dependency cannot be wired into
// one composition path and silently left nil in the other.
func NewHandler(cfg *config.Config, authManager *auth.Manager, accountService *account.Service, games *collection.Repo, playRepo *plays.Repo, loanRepo *loans.Repo, syncService *collection.SyncService, trmnlService *trmnl.Service, database *sql.DB) *Handler {
	venueRepo := venues.NewRepo(database)
	playRepo.SetVenueResolver(venueRepo)
	return &Handler{
		Config:  cfg,
		Auth:    authManager,
		Account: accountService,
		Games:   games,
		Plays:   playRepo,
		Loans:   loanRepo,
		Sync:    syncService,
		TRMNL:   trmnlService,
		Venues:  venueRepo,
		DB:      database,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /docs", h.docs)
	mux.HandleFunc("GET /openapi.json", h.openapi)
	// Public discovery endpoint for companion apps; it deliberately bypasses
	// bearer authentication so a fresh install can learn the demo credentials.
	mux.Handle("GET /v1/auth/config", h.rateLimit(http.HandlerFunc(h.authConfig)))
	// Public telemetry discovery so the companion app can resolve its OTLP
	// exporter at runtime instead of at build time. Exposure is limited to an
	// endpoint URL and feature flags; the endpoint itself must be reachable
	// without a token, so this leaks no credential.
	mux.Handle("GET /v1/otel-settings", h.rateLimit(http.HandlerFunc(h.otelSettings)))

	v1 := http.NewServeMux()
	v1.Handle("GET /me", auth.RequireScope("read", http.HandlerFunc(h.me)))
	v1.Handle("GET /me/export", auth.RequireScope("read", http.HandlerFunc(h.exportMe)))
	v1.Handle("DELETE /players", auth.RequireScope("write", http.HandlerFunc(h.removePlayer)))
	v1.Handle("GET /games", auth.RequireScope("read", http.HandlerFunc(h.listGames)))
	v1.Handle("GET /games/{id}", auth.RequireScope("read", http.HandlerFunc(h.getGame)))
	v1.Handle("PATCH /games/{id}", auth.RequireScope("write", http.HandlerFunc(h.patchGame)))
	v1.Handle("GET /pipeline", auth.RequireScope("read", http.HandlerFunc(h.pipeline)))
	v1.Handle("GET /plays", auth.RequireScope("read", http.HandlerFunc(h.listPlays)))
	v1.Handle("POST /plays", auth.RequireScope("write", http.HandlerFunc(h.createPlay)))
	v1.Handle("GET /plays/{id}", auth.RequireScope("read", http.HandlerFunc(h.getPlay)))
	v1.Handle("PATCH /plays/{id}", auth.RequireScope("write", http.HandlerFunc(h.patchPlay)))
	v1.Handle("DELETE /plays/{id}", auth.RequireScope("write", http.HandlerFunc(h.deletePlay)))
	v1.Handle("GET /loans", auth.RequireScope("read", http.HandlerFunc(h.listLoans)))
	v1.Handle("POST /loans", auth.RequireScope("write", http.HandlerFunc(h.createLoan)))
	v1.Handle("GET /loans/{id}", auth.RequireScope("read", http.HandlerFunc(h.getLoan)))
	v1.Handle("POST /loans/{id}/return", auth.RequireScope("write", http.HandlerFunc(h.returnLoan)))
	v1.Handle("GET /stats", auth.RequireScope("read", http.HandlerFunc(h.stats)))
	v1.Handle("GET /venues", auth.RequireScope("read", http.HandlerFunc(h.listVenues)))
	v1.Handle("GET /sync", auth.RequireScope("read", http.HandlerFunc(h.syncStatus)))
	v1.Handle("POST /sync", auth.RequireScope("write", http.HandlerFunc(h.startSync)))
	v1.Handle("GET /trmnl", auth.RequireScope("trmnl", http.HandlerFunc(h.trmnlFeed)))
	v1.HandleFunc("/", h.notFound)

	protected := h.Auth.AuthenticateBearer(h.rateLimit(v1))
	mux.Handle("/v1/", http.StripPrefix("/v1", protected))
	// Account deletion accepts either a personal token or a browser session.
	// The more specific pattern wins over the /v1/ subtree.
	mux.Handle("DELETE /v1/me", h.rateLimit(http.HandlerFunc(h.deleteMe)))
	mux.HandleFunc("/", h.notFound)
	return h.securityHeaders(mux)
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "The requested API resource does not exist.", nil)
}

func (h *Handler) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) openapi(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("openapi.json")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "The specification is unavailable.", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) docs(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("docs.html")
	if err != nil {
		http.Error(w, "docs unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	a := auth.FromContext(r.Context())
	if a == nil || a.User == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"id": a.User.ID, "email": a.User.Email, "name": a.User.Name, "avatar_url": a.User.AvatarURL,
		"role": a.User.Role, "scopes": a.Scopes,
	}})
}

type gameJSON struct {
	ID          int64    `json:"id"`
	BGGID       int      `json:"bgg_id"`
	Name        string   `json:"name"`
	Year        int      `json:"year,omitempty"`
	ImageURL    string   `json:"image_url"`
	BGGURL      string   `json:"bgg_url"`
	MinPlayers  int      `json:"min_players,omitempty"`
	MaxPlayers  int      `json:"max_players,omitempty"`
	MinPlaytime int      `json:"min_playtime,omitempty"`
	MaxPlaytime int      `json:"max_playtime,omitempty"`
	MinAge      int      `json:"min_age,omitempty"`
	Weight      float64  `json:"weight,omitempty"`
	BGGRating   float64  `json:"bgg_rating,omitempty"`
	BGGRank     int      `json:"bgg_rank,omitempty"`
	UserRating  *float64 `json:"user_rating"`
	Owned       bool     `json:"owned"`
	Wishlist    bool     `json:"wishlist"`
	WantToPlay  bool     `json:"want_to_play"`
	Preordered  bool     `json:"preordered"`
	ForTrade    bool     `json:"for_trade"`
	Favorite    bool     `json:"favorite"`
	BGGNumPlays int      `json:"bgg_num_plays"`
	LocalPlays  int      `json:"local_plays"`
	TotalPlays  int      `json:"total_plays"`
	AddedAt     string   `json:"added_at,omitempty"`
	IsExpansion bool     `json:"is_expansion"`
	Notes       string   `json:"notes,omitempty"`

	AcquisitionDate   *string  `json:"acquisition_date"`
	PricePaidCents    *int     `json:"price_paid_cents"`
	PricePerPlayCents *float64 `json:"price_per_play_cents"`
	PricePerHourCents *float64 `json:"price_per_hour_cents"`
}

func (h *Handler) gameJSON(g collection.GameCard) gameJSON {
	var acquisitionDate *string
	if g.AcquisitionDate != "" {
		v := g.AcquisitionDate
		acquisitionDate = &v
	}
	return gameJSON{ID: g.ID, BGGID: g.BGGID, Name: g.Name, Year: g.Year, ImageURL: h.absolute(g.CoverURL()),
		BGGURL: g.BGGURL(), MinPlayers: g.MinPlayers, MaxPlayers: g.MaxPlayers, MinPlaytime: g.MinPlaytime,
		MaxPlaytime: g.MaxPlaytime, MinAge: g.MinAge, Weight: round1(g.Weight), BGGRating: round1(g.BGGRating), BGGRank: g.BGGRank,
		UserRating: g.UserRating, Owned: g.Owned, Wishlist: g.Wishlist, WantToPlay: g.WantToPlay, Preordered: g.Preordered,
		ForTrade: g.ForTrade, Favorite: g.Favorite, BGGNumPlays: g.BGGNumPlays, LocalPlays: g.LocalPlays,
		TotalPlays: g.TotalPlays, AddedAt: g.AddedAt, IsExpansion: g.IsExpansion, Notes: g.Notes,
		AcquisitionDate: acquisitionDate, PricePaidCents: g.PricePaid,
		PricePerPlayCents: round2Ptr(g.CostPerPlayPence), PricePerHourCents: round2Ptr(g.CostPerHourPence)}
}

func (h *Handler) listGames(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	limit, offset, fields := paginationParams(q)
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "Pagination is invalid.", fields)
		return
	}
	f := collection.Filters{
		Query: strings.TrimSpace(q.Get("q")), Players: intParam(q.Get("players"), 0),
		MaxPlaytime: intParam(q.Get("max_playtime"), 0), MinWeight: floatParam(q.Get("min_weight")),
		MaxWeight: floatParam(q.Get("max_weight")), Category: q.Get("category"), Mechanic: q.Get("mechanic"),
		Status: q.Get("status"), Sort: q.Get("sort"), Page: 1, PerPage: limit, Offset: offset,
	}
	games, total, err := h.Games.List(ctx, user.ID, f)
	if err != nil {
		serverError(w, "list games", err)
		return
	}
	out := make([]gameJSON, 0, len(games))
	for _, g := range games {
		out = append(out, h.gameJSON(g))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "pagination": map[string]any{
		"limit": limit, "offset": offset, "total": total, "has_more": offset+len(out) < total,
	}})
}

func (h *Handler) getGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Game not found.", nil)
		return
	}
	game, err := h.Games.Get(ctx, user.ID, id)
	if err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Game not found.", nil)
			return
		}
		serverError(w, "get game", err)
		return
	}
	card := game.GameCard
	payload := h.gameJSON(card)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"game": payload, "description": game.Description, "categories": game.Categories, "mechanics": game.Mechanics,
		"designers": game.Designers, "publishers": game.Publishers, "location": game.Location,
		"condition": game.Condition, "price_paid_cents": game.PricePaid, "setup_notes": game.SetupNotes,
	}})
}

// pipeline returns the five acquisition states in a fixed order. Games are
// grouped from the existing user_games flags, so a game can appear in more
// than one bucket; count is the full total while games is capped at limit.
func (h *Handler) pipeline(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	limit := 24
	fields := map[string]string{}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 96 {
			fields["limit"] = "must be between 1 and 96"
		} else {
			limit = n
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "Pagination is invalid.", fields)
		return
	}
	buckets, err := h.Games.Pipeline(ctx, user.ID, limit, collection.Filters{})
	if err != nil {
		serverError(w, "pipeline", err)
		return
	}
	out := make([]map[string]any, 0, len(buckets))
	for _, b := range buckets {
		games := make([]gameJSON, 0, len(b.Games))
		for _, g := range b.Games {
			games = append(games, h.gameJSON(g))
		}
		out = append(out, map[string]any{"state": b.State, "label": b.Label, "count": b.Count, "games": games})
	}
	writeJSON(w, http.StatusOK, map[string]any{"buckets": out})
}

type collectionPatch struct {
	Owned           *bool    `json:"owned"`
	Wishlist        *bool    `json:"wishlist"`
	WantToPlay      *bool    `json:"want_to_play"`
	Preordered      *bool    `json:"preordered"`
	ForTrade        *bool    `json:"for_trade"`
	Favorite        *bool    `json:"favorite"`
	UserRating      *float64 `json:"user_rating"`
	ClearRating     bool     `json:"clear_rating"`
	Notes           *string  `json:"notes"`
	SetupNotes      *string  `json:"setup_notes"`
	AcquisitionDate *string  `json:"acquisition_date"`
	PricePaidCents  *int     `json:"price_paid_cents"`
	ClearPrice      bool     `json:"clear_price"`
	Location        *string  `json:"location"`
	Condition       *string  `json:"condition"`
}

func (h *Handler) patchGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Game not found.", nil)
		return
	}
	var body collectionPatch
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	patch := collection.CollectionPatch{Owned: body.Owned, Wishlist: body.Wishlist, WantToPlay: body.WantToPlay,
		Preordered: body.Preordered, ForTrade: body.ForTrade, Favorite: body.Favorite, UserRating: body.UserRating,
		ClearRating: body.ClearRating, Notes: body.Notes, SetupNotes: body.SetupNotes, AcquisitionDate: body.AcquisitionDate,
		PricePaid: body.PricePaidCents, ClearPrice: body.ClearPrice, Location: body.Location, Condition: body.Condition}
	if err := h.Games.Apply(ctx, user.ID, id, patch); err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Game not found.", nil)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error(), nil)
		return
	}
	game, err := h.Games.Get(ctx, user.ID, id)
	if err != nil {
		serverError(w, "reload game", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": h.gameJSON(game.GameCard)})
}

type playPlayerJSON struct {
	Name     string `json:"name"`
	Score    string `json:"score,omitempty"`
	IsWinner bool   `json:"is_winner"`
	IsFirst  bool   `json:"is_first_play"`
}

type playJSON struct {
	ID              string           `json:"id"`
	GameID          int64            `json:"game_id"`
	GameName        string           `json:"game_name"`
	ImageURL        string           `json:"image_url,omitempty"`
	PlayedAt        string           `json:"played_at"`
	Location        string           `json:"location,omitempty"`
	DurationMinutes int              `json:"duration_minutes,omitempty"`
	Quantity        int              `json:"quantity"`
	Notes           string           `json:"notes,omitempty"`
	Origin          string           `json:"origin"`
	BGGPlayID       *int64           `json:"bgg_play_id,omitempty"`
	Players         []playPlayerJSON `json:"players"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

func (h *Handler) playJSON(p plays.Play) playJSON {
	out := playJSON{ID: p.ID, GameID: p.GameID, GameName: p.GameName, PlayedAt: p.PlayedAt, Location: p.Location,
		DurationMinutes: p.Duration, Quantity: p.Quantity, Notes: p.Notes, Origin: p.Origin, BGGPlayID: p.BGGPlayID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, Players: make([]playPlayerJSON, 0, len(p.Players))}
	if p.ThumbnailURL != "" {
		out.ImageURL = h.absolute("/media/bgg/" + strconv.Itoa(p.BGGID) + ".jpg")
	}
	for _, p := range p.Players {
		out.Players = append(out.Players, playPlayerJSON{Name: p.Name, Score: p.Score, IsWinner: p.IsWinner, IsFirst: p.IsFirst})
	}
	return out
}

func (h *Handler) listPlays(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	limit, offset, fields := paginationParams(q)
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "Pagination is invalid.", fields)
		return
	}
	list, total, err := h.Plays.List(ctx, user.ID, plays.ListOptions{GameID: int64(intParam(q.Get("game_id"), 0)),
		Year: intParam(q.Get("year"), 0), Limit: limit, Offset: offset})
	if err != nil {
		serverError(w, "list plays", err)
		return
	}
	out := make([]playJSON, 0, len(list))
	for _, p := range list {
		out = append(out, h.playJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "pagination": map[string]any{
		"limit": limit, "offset": offset, "total": total, "has_more": offset+len(out) < total,
	}})
}

type playInputJSON struct {
	GameID          int64            `json:"game_id"`
	PlayedAt        string           `json:"played_at"`
	Location        string           `json:"location"`
	DurationMinutes int              `json:"duration_minutes"`
	Quantity        int              `json:"quantity"`
	Notes           string           `json:"notes"`
	Players         []playPlayerJSON `json:"players"`
	ClientPlayID    string           `json:"client_play_id"`
}

func (h *Handler) createPlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	var body playInputJSON
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	in := plays.Input{GameID: body.GameID, PlayedAt: body.PlayedAt, Location: body.Location, Duration: body.DurationMinutes,
		Quantity: body.Quantity, Notes: body.Notes, Players: toPlayers(body.Players), ClientID: body.ClientPlayID}
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	settings, _ := h.Auth.Settings(ctx, user.ID)
	play, err := h.Plays.Create(ctx, user.ID, in, settings.Timezone)
	if err != nil {
		validationError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/plays/%s", play.ID))
	writeJSON(w, http.StatusCreated, map[string]any{"data": h.playJSON(*play)})
}

func (h *Handler) getPlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	play, err := h.Plays.Get(ctx, user.ID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, plays.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Play not found.", nil)
			return
		}
		serverError(w, "get play", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": h.playJSON(*play)})
}

type playPatchJSON struct {
	GameID          *int64            `json:"game_id"`
	PlayedAt        *string           `json:"played_at"`
	Location        *string           `json:"location"`
	DurationMinutes *int              `json:"duration_minutes"`
	Quantity        *int              `json:"quantity"`
	Notes           *string           `json:"notes"`
	Players         *[]playPlayerJSON `json:"players"`
}

func (h *Handler) patchPlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	current, err := h.Plays.Get(ctx, user.ID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, plays.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Play not found.", nil)
			return
		}
		serverError(w, "get play", err)
		return
	}
	if current.Origin != "local" {
		writeError(w, http.StatusForbidden, "read_only", "Plays imported from BoardGameGeek cannot be edited here.", nil)
		return
	}
	var body playPatchJSON
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	in := plays.Input{GameID: current.GameID, PlayedAt: current.PlayedAt, Location: current.Location,
		Duration: current.Duration, Quantity: current.Quantity, Notes: current.Notes, Players: current.Players}
	if body.GameID != nil {
		in.GameID = *body.GameID
	}
	if body.PlayedAt != nil {
		in.PlayedAt = *body.PlayedAt
	}
	if body.Location != nil {
		in.Location = *body.Location
	}
	if body.DurationMinutes != nil {
		in.Duration = *body.DurationMinutes
	}
	if body.Quantity != nil {
		in.Quantity = *body.Quantity
	}
	if body.Notes != nil {
		in.Notes = *body.Notes
	}
	if body.Players != nil {
		in.Players = toPlayers(*body.Players)
	}
	settings, _ := h.Auth.Settings(ctx, user.ID)
	updated, err := h.Plays.Update(ctx, user.ID, current.ID, in, settings.Timezone)
	if err != nil {
		validationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": h.playJSON(*updated)})
}

func (h *Handler) deletePlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if err := h.Plays.Delete(ctx, user.ID, r.PathValue("id")); err != nil {
		if errors.Is(err, plays.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Play not found or imported from BoardGameGeek.", nil)
			return
		}
		serverError(w, "delete play", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loanGameJSON is the game reference embedded in a loan. Only the fields a
// client needs to show the borrowed title are included.
type loanGameJSON struct {
	ID           int64  `json:"id"`
	BGGID        int    `json:"bgg_id"`
	Name         string `json:"name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// loanJSON is the wire form of a loan. Dates are calendar days (YYYY-MM-DD)
// and stay as empty strings until set; every field is always present so
// clients see a stable schema.
type loanJSON struct {
	ID           int64        `json:"id"`
	Game         loanGameJSON `json:"game"`
	Borrower     string       `json:"borrower"`
	Contact      string       `json:"contact"`
	LentAt       string       `json:"lent_at"`
	DueAt        string       `json:"due_at"`
	ReturnedAt   string       `json:"returned_at"`
	Notes        string       `json:"notes"`
	ClosedReason string       `json:"closed_reason"`
	Overdue      bool         `json:"overdue"`
	DaysOut      int          `json:"days_out"`
	DaysOverdue  int          `json:"days_overdue"`
	NotifiedAt   string       `json:"notified_at"`
}

func (h *Handler) loanJSON(l loans.Loan) loanJSON {
	out := loanJSON{
		ID: l.ID, Borrower: l.Borrower, Contact: l.Contact, LentAt: l.LentAt, DueAt: l.DueAt,
		ReturnedAt: l.ReturnedAt, Notes: l.Notes, ClosedReason: l.ClosedReason,
		Overdue: l.Overdue, DaysOut: l.DaysOut, DaysOverdue: l.DaysOverdue, NotifiedAt: l.NotifiedAt,
		Game: loanGameJSON{ID: l.GameID, BGGID: l.BGGID, Name: l.GameName},
	}
	if l.ThumbnailURL != "" {
		out.Game.ThumbnailURL = h.absolute("/media/bgg/" + strconv.Itoa(l.BGGID) + ".jpg")
	}
	return out
}

// listLoans returns open loans by default, closed history on request, or both
// with open first. limit bounds only the closed section, which is history; the
// open section is always short because a game can be out at most once.
func (h *Handler) listLoans(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "closed" && status != "all" {
		writeError(w, http.StatusBadRequest, "invalid_request", "The status filter must be open, closed, or all.",
			map[string]string{"status": "must be open, closed, or all"})
		return
	}
	// limit is 1-100 with 24 as the default. Numeric values outside the range
	// are clamped rather than rejected, because a client asking for "more" is
	// not making a mistake; non-numeric values are still refused.
	limit := 24
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "The limit must be a number.",
				map[string]string{"limit": "must be a number"})
			return
		}
		switch {
		case n < 1:
			limit = 1
		case n > 100:
			limit = 100
		default:
			limit = n
		}
	}
	out := make([]loanJSON, 0)
	if status == "open" || status == "all" {
		open, err := h.Loans.Open(ctx, user.ID)
		if err != nil {
			serverError(w, "list open loans", err)
			return
		}
		for _, l := range open {
			out = append(out, h.loanJSON(l))
		}
	}
	if status == "closed" || status == "all" {
		closed, err := h.Loans.Closed(ctx, user.ID, limit)
		if err != nil {
			serverError(w, "list closed loans", err)
			return
		}
		for _, l := range closed {
			out = append(out, h.loanJSON(l))
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"loans": out})
}

type loanCreateJSON struct {
	GameID   int64  `json:"game_id"`
	Borrower string `json:"borrower"`
	Contact  string `json:"contact"`
	LentAt   string `json:"lent_at"`
	DueAt    string `json:"due_at"`
	Notes    string `json:"notes"`
}

func (h *Handler) createLoan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	var body loanCreateJSON
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	loan, err := h.Loans.Lend(ctx, user.ID, loans.NewLoan{GameID: body.GameID, Borrower: body.Borrower,
		Contact: body.Contact, LentAt: body.LentAt, DueAt: body.DueAt, Notes: body.Notes})
	if err != nil {
		h.loanError(ctx, w, user.ID, err, body.GameID)
		return
	}
	w.Header().Set("Location", "/api/v1/loans/"+strconv.FormatInt(loan.ID, 10))
	writeJSON(w, http.StatusCreated, map[string]any{"data": h.loanJSON(*loan)})
}

func (h *Handler) getLoan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Loan not found.", nil)
		return
	}
	loan, err := h.Loans.Get(ctx, user.ID, id)
	if err != nil {
		h.loanError(ctx, w, user.ID, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": h.loanJSON(*loan)})
}

type loanReturnJSON struct {
	ReturnedAt string `json:"returned_at"`
}

func (h *Handler) returnLoan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Loan not found.", nil)
		return
	}
	var body loanReturnJSON
	if err := decodeOptionalJSON(w, r, &body); err != nil {
		return
	}
	loan, err := h.Loans.Return(ctx, user.ID, id, body.ReturnedAt)
	if err != nil {
		h.loanError(ctx, w, user.ID, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": h.loanJSON(*loan)})
}

// loanError maps the loans package sentinels onto the shared envelope. The
// conflict message names the game when the open loan is still readable, which
// is the one extra lookup paid only on the failure path.
func (h *Handler) loanError(ctx context.Context, w http.ResponseWriter, userID string, err error, gameID int64) {
	switch {
	case errors.Is(err, loans.ErrOpenLoanExists):
		message := "That game already has an open loan. Mark it returned first."
		existing, lookupErr := h.Loans.OpenByGame(ctx, userID, gameID)
		if lookupErr == nil && existing != nil && existing.GameName != "" {
			message = fmt.Sprintf("%s already has an open loan to %s. Mark it returned first.", existing.GameName, existing.Borrower)
		}
		writeError(w, http.StatusConflict, "conflict", message, nil)
	case errors.Is(err, loans.ErrNotOwned):
		message := "The game must be on your owned shelf."
		if gameID <= 0 {
			message = "A game_id is required."
		}
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", message,
			map[string]string{"game_id": "must be a game you own"})
	case errors.Is(err, loans.ErrInvalid):
		message := loanMessage(err)
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", message, loanFields(message))
	case errors.Is(err, loans.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Loan not found.", nil)
	default:
		serverError(w, "loan", err)
	}
}

// loanMessage strips the package sentinel prefix so the envelope reads as a
// sentence about the request rather than an internal error string.
func loanMessage(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"loans: invalid loan: ", "loans: "} {
		if strings.HasPrefix(msg, prefix) {
			return strings.TrimPrefix(msg, prefix)
		}
	}
	return msg
}

// loanFields attributes a validation message to the field that caused it, so
// clients can highlight the right input.
func loanFields(message string) map[string]string {
	for _, pair := range []struct{ needle, field string }{
		{"borrower", "borrower"}, {"contact", "contact"}, {"notes", "notes"},
		{"lend date", "lent_at"}, {"due date", "due_at"}, {"return date", "returned_at"},
	} {
		if strings.Contains(message, pair.needle) {
			return map[string]string{pair.field: message}
		}
	}
	return nil
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	year := intParam(r.URL.Query().Get("year"), time.Now().Year())
	stats, err := h.Plays.Stats(ctx, user.ID, year)
	if err != nil {
		serverError(w, "stats", err)
		return
	}
	shelf, err := h.Games.Stats(ctx, user.ID)
	if err != nil {
		serverError(w, "shelf stats", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"year": year, "monthly": stats.Monthly, "total_plays": stats.TotalPlays, "total_sessions": stats.TotalSessions,
		"top_games": stats.TopGames, "leaderboard": stats.Leaderboard, "weight_bands": stats.WeightBands,
		"shelf": map[string]any{"owned": shelf.Owned, "unplayed": shelf.Unplayed, "wishlist": shelf.Wishlist,
			"plays_this_year": shelf.PlaysThisYear, "total_plays": shelf.TotalPlays,
			"longest_unplayed": shelf.LongestUnplayed, "longest_wait_days": shelf.LongestWaitDays},
		"collection_value": map[string]any{
			"total_paid_cents":           shelf.TotalPaidPence,
			"paid_games":                 shelf.PaidGames,
			"unplayed_capital_cents":     shelf.UnplayedCapitalPence,
			"median_cost_per_play_cents": shelf.MedianCostPerPlayPence,
		},
	}})
}

func (h *Handler) listVenues(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	stats, err := h.Venues.Stats(ctx, user.ID)
	if err != nil {
		serverError(w, "list venues", err)
		return
	}
	type venueJSON struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Plays         int    `json:"plays"`
		DistinctGames int    `json:"distinct_games"`
		LastPlayedAt  string `json:"last_played_at"`
	}
	out := make([]venueJSON, 0, len(stats))
	for _, v := range stats {
		out = append(out, venueJSON{ID: v.LocationID, Name: v.Name, Plays: v.Plays,
			DistinctGames: v.DistinctGames, LastPlayedAt: v.LastPlayedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"venues": out})
}

func (h *Handler) syncStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	run, err := h.Sync.Latest(ctx, user.ID)
	if err != nil {
		serverError(w, "sync status", err)
		return
	}
	if run == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": run})
}

func (h *Handler) startSync(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	run, err := h.Sync.Start(user.ID, "api")
	switch {
	case errors.Is(err, collection.ErrNoBGGUsername):
		writeError(w, http.StatusConflict, "bgg_not_connected", "Connect a BoardGameGeek username in settings first.", nil)
	case errors.Is(err, collection.ErrSyncRunning):
		writeError(w, http.StatusConflict, "sync_running", "A sync is already running.", nil)
	case err != nil:
		serverError(w, "start sync", err)
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"data": run})
	}
}

func (h *Handler) trmnlFeed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	payload, err := h.TRMNL.Build(ctx, user.ID)
	if err != nil {
		serverError(w, "build TRMNL feed", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) absolute(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(h.Config.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func toPlayers(in []playPlayerJSON) []plays.Player {
	out := make([]plays.Player, 0, len(in))
	for _, p := range in {
		out = append(out, plays.Player{Name: p.Name, Score: p.Score, IsWinner: p.IsWinner, IsFirst: p.IsFirst})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	payload := map[string]any{"error": map[string]any{"code": code, "message": message}}
	if len(fields) > 0 {
		payload["error"].(map[string]any)["fields"] = fields
	}
	writeJSON(w, status, payload)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body must be a valid JSON object.", nil)
		return err
	}
	return nil
}

// decodeOptionalJSON decodes a body only when the client sent one, so an
// endpoint with an optional body does not reject an absent body as bad JSON.
func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return nil
	}
	return decodeJSON(w, r, dst)
}

func validationError(w http.ResponseWriter, err error) {
	if errors.Is(err, plays.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Play not found.", nil)
		return
	}
	if errors.Is(err, plays.ErrInvalid) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error(), nil)
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error(), nil)
}

func serverError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side.", nil)
}

// paginationParams parses the documented limit/offset pair. It rejects
// nonsensical values instead of silently clamping them.
func paginationParams(q url.Values) (int, int, map[string]string) {
	limit, offset := 24, 0
	fields := map[string]string{}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			fields["limit"] = "must be between 1 and 100"
		} else {
			limit = n
		}
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			fields["offset"] = "must be zero or greater"
		} else {
			offset = n
		}
	}
	return limit, offset, fields
}

func intParam(v string, fallback int) int {
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}

func floatParam(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func round2Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := round2(*v)
	return &r
}

// limiter is a small per-user fixed-window limiter. It is intentionally
// in-memory: one instance serves this single-user deployment behind a proxy.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count int
	reset time.Time
}

func newLimiter() *limiter { return &limiter{buckets: map[string]*bucket{}} }

func (l *limiter) allow(key string, limit int) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &bucket{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

func (h *Handler) rateLimit(next http.Handler) http.Handler {
	l := newLimiter()
	trusted := observability.ParseTrustedProxies(h.Config.TrustedProxies)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := auth.FromContext(r.Context())
		key := observability.RealIP(r, trusted)
		if a != nil && a.User != nil {
			key = a.User.ID
		}
		if !l.allow(key, 120) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests. Try again shortly.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
