package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/auth"
)

// deleteMe closes the authenticated account. Personal tokens and browser
// sessions are both accepted because the Android app uses a token while the
// settings page uses a cookie; the response is JSON either way.
func (h *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	var a *auth.AuthContext
	var err error
	if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
		raw := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		a, err = h.Auth.TokenAuth(ctx, raw)
	} else {
		a, err = h.Auth.SessionFromRequest(r)
		if err == nil {
			if !h.validCookieCSRF(r, a) {
				writeError(w, http.StatusForbidden, "invalid_csrf", "A valid CSRF token is required for a browser session.", nil)
				return
			}
		}
	}
	if err != nil || a == nil || a.User == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", nil)
		return
	}
	if h.Account == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Account deletion is not configured.", nil)
		return
	}
	if err := h.Account.Delete(ctx, a.User.ID); err != nil {
		switch {
		case errors.Is(err, account.ErrLastAdmin):
			writeError(w, http.StatusConflict, "last_admin", "The last admin account cannot be deleted. Promote another admin first.", nil)
		case errors.Is(err, account.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "The account no longer exists.", nil)
		default:
			serverError(w, "delete account", err)
		}
		return
	}
	if a.Via == "cookie" {
		h.Auth.ClearSessionCookie(w)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

func (h *Handler) validCookieCSRF(r *http.Request, a *auth.AuthContext) bool {
	if a.Session == nil || a.CSRFToken == "" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		base, _ := url.Parse(h.Config.BaseURL)
		if err != nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
			return false
		}
	}
	token := r.Header.Get(auth.CSRFHeader)
	if token == "" {
		token = r.FormValue("csrf_token")
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.CSRFToken)) == 1
}

// exportMe returns the signed-in user's own data. Token hashes are never
// included; only token metadata is exported.
func (h *Handler) exportMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if h.Account == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Export is not configured.", nil)
		return
	}
	data, err := h.Account.Export(ctx, user.ID)
	if err != nil {
		serverError(w, "export account", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="the-shelf-export.json"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// removePlayer removes or anonymises a free-text player name. A non-admin is
// scoped to plays they own, so this route cannot read or change another
// account's play log.
func (h *Handler) removePlayer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "A player name is required.", map[string]string{"name": "Enter the exact name to remove."})
		return
	}
	if mode != "" && mode != "remove" && mode != "anonymise" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Mode must be remove or anonymise.", map[string]string{"mode": "Use remove or anonymise."})
		return
	}
	if h.Account == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Player removal is not configured.", nil)
		return
	}
	n, err := h.Account.RemovePlayerName(ctx, user.ID, user.Role == "admin", name, mode == "anonymise")
	if err != nil {
		if errors.Is(err, account.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), nil)
			return
		}
		serverError(w, "remove player name", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"changed": n, "name": name, "mode": modeOrDefault(mode)}})
}

func modeOrDefault(mode string) string {
	if mode == "" {
		return "remove"
	}
	return mode
}
