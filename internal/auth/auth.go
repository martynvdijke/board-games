// Package auth implements OIDC login, database-backed sessions, CSRF
// protection, and scoped personal API tokens.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"boardgames/internal/config"
	"boardgames/internal/db"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	SessionCookie = "shelf_session"
	OAuthCookie   = "shelf_oauth"
	CSRFHeader    = "X-CSRF-Token"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
)

type User struct {
	ID          string
	Email       string
	Name        string
	AvatarURL   string
	Role        string
	CreatedAt   string
	LastLoginAt string
}

type Session struct {
	TokenHash string
	UserID    string
	CSRFToken string
	ExpiresAt time.Time
}

type Token struct {
	ID         string
	UserID     string
	Name       string
	Prefix     string
	Scopes     []string
	CreatedAt  string
	LastUsedAt string
	RevokedAt  string
}

type AuthContext struct {
	User      *User
	Session   *Session
	Token     *Token
	Scopes    []string
	Via       string // cookie, token, jwt
	CSRFToken string
}

type ctxKey struct{}

func WithAuth(ctx context.Context, a *AuthContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

func FromContext(ctx context.Context) *AuthContext {
	a, _ := ctx.Value(ctxKey{}).(*AuthContext)
	return a
}

func UserFromContext(ctx context.Context) *User {
	if a := FromContext(ctx); a != nil {
		return a.User
	}
	return nil
}

func HasScope(ctx context.Context, scope string) bool {
	a := FromContext(ctx)
	if a == nil {
		return false
	}
	if a.Via == "cookie" {
		return true
	}
	for _, s := range a.Scopes {
		if s == scope || s == "write" && scope == "read" {
			return true
		}
	}
	return false
}

type Manager struct {
	db     *sql.DB
	cfg    *config.Config
	secure bool

	oidcMu      chan struct{}
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauth       *oauth2.Config
	discoverErr error
}

func New(db *sql.DB, cfg *config.Config) *Manager {
	m := &Manager{db: db, cfg: cfg, secure: cfg.SecureCookies(), oidcMu: make(chan struct{}, 1)}
	m.oidcMu <- struct{}{}
	return m
}

func (m *Manager) Config() *config.Config { return m.cfg }

// Discover resolves OIDC metadata once; failures are retried on the next call.
func (m *Manager) Discover(ctx context.Context) error {
	select {
	case <-m.oidcMu:
		defer func() { m.oidcMu <- struct{}{} }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if m.provider != nil {
		return nil
	}
	if m.cfg.OIDCIssuer == "" {
		m.discoverErr = errors.New("OIDC is not configured")
		return m.discoverErr
	}
	provider, err := oidc.NewProvider(ctx, m.cfg.OIDCIssuer)
	if err != nil {
		m.discoverErr = fmt.Errorf("discover OIDC provider: %w", err)
		return m.discoverErr
	}
	m.provider = provider
	m.verifier = provider.Verifier(&oidc.Config{ClientID: m.cfg.OIDCClientID})
	m.oauth = &oauth2.Config{
		ClientID:     m.cfg.OIDCClientID,
		ClientSecret: m.cfg.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  m.cfg.RedirectURI(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	m.discoverErr = nil
	return nil
}

func (m *Manager) OAuthConfig() *oauth2.Config { return m.oauth }

type statePayload struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	ReturnTo string `json:"return_to"`
	Expires  int64  `json:"expires"`
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (m *Manager) sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(m.cfg.SessionSecret))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) writeStateCookie(w http.ResponseWriter, p statePayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	value := base64.RawURLEncoding.EncodeToString(body) + "." + m.sign(body)
	http.SetCookie(w, &http.Cookie{
		Name: OAuthCookie, Value: value, Path: "/", HttpOnly: true, Secure: m.secure,
		SameSite: http.SameSiteLaxMode, MaxAge: 600, Expires: time.Now().Add(10 * time.Minute),
	})
	return nil
}

func (m *Manager) readStateCookie(r *http.Request) (statePayload, error) {
	var p statePayload
	c, err := r.Cookie(OAuthCookie)
	if err != nil {
		return p, errors.New("missing OAuth state")
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return p, errors.New("invalid OAuth state")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return p, errors.New("invalid OAuth state")
	}
	if !hmac.Equal([]byte(parts[1]), []byte(m.sign(body))) {
		return p, errors.New("invalid OAuth state signature")
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return p, errors.New("invalid OAuth state payload")
	}
	if time.Now().Unix() > p.Expires {
		return p, errors.New("OAuth state expired")
	}
	return p, nil
}

func (m *Manager) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: OAuthCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
}

// Login starts the Authorization Code + PKCE flow.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request) {
	returnTo := safeReturnTo(r.URL.Query().Get("next"))
	if m.cfg.DevFakeAuth {
		u, err := m.UpsertUser(r.Context(), m.cfg.DevFakeEmail, m.cfg.DevFakeName, "", true)
		if err != nil {
			http.Error(w, "could not sign in", http.StatusInternalServerError)
			return
		}
		if err := m.StartSession(w, r, u.ID); err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}
	if err := m.Discover(r.Context()); err != nil {
		slog.Error("OIDC discovery failed", "error", err)
		http.Error(w, "Sign-in is temporarily unavailable. Please try again.", http.StatusServiceUnavailable)
		return
	}
	state, err := randomString(24)
	if err != nil {
		http.Error(w, "could not start login", http.StatusInternalServerError)
		return
	}
	nonce, err := randomString(24)
	if err != nil {
		http.Error(w, "could not start login", http.StatusInternalServerError)
		return
	}
	verifier, err := randomString(48)
	if err != nil {
		http.Error(w, "could not start login", http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if err := m.writeStateCookie(w, statePayload{State: state, Nonce: nonce, Verifier: verifier, ReturnTo: returnTo, Expires: time.Now().Add(10 * time.Minute).Unix()}); err != nil {
		http.Error(w, "could not start login", http.StatusInternalServerError)
		return
	}
	url := m.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback completes the OIDC flow.
func (m *Manager) Callback(w http.ResponseWriter, r *http.Request) {
	if m.cfg.DevFakeAuth {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	state, err := m.readStateCookie(r)
	m.clearStateCookie(w)
	if err != nil {
		slog.Warn("OIDC callback rejected", "error", err)
		http.Error(w, "Sign-in could not be verified. Please start again.", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") == "" || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state.State)) != 1 {
		http.Error(w, "Sign-in could not be verified. Please start again.", http.StatusBadRequest)
		return
	}
	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		slog.Warn("OIDC provider error", "error", providerErr)
		http.Error(w, "The identity provider declined the sign-in. Please try again.", http.StatusUnauthorized)
		return
	}
	if err := m.Discover(r.Context()); err != nil {
		http.Error(w, "Sign-in is temporarily unavailable. Please try again.", http.StatusServiceUnavailable)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Sign-in could not be completed.", http.StatusBadRequest)
		return
	}
	token, err := m.oauth.Exchange(r.Context(), code, oauth2.SetAuthURLParam("code_verifier", state.Verifier))
	if err != nil {
		slog.Warn("OIDC token exchange failed", "error", err)
		http.Error(w, "Sign-in could not be completed. Please try again.", http.StatusBadGateway)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok || rawID == "" {
		http.Error(w, "The identity provider did not return an identity token.", http.StatusBadGateway)
		return
	}
	idToken, err := m.verifier.Verify(r.Context(), rawID)
	if err != nil {
		slog.Warn("ID token verification failed", "error", err)
		http.Error(w, "Sign-in could not be verified. Please try again.", http.StatusUnauthorized)
		return
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		Name          string `json:"name"`
		PreferredName string `json:"preferred_username"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Sign-in could not be completed.", http.StatusBadGateway)
		return
	}
	if !validNonce(idToken, state.Nonce) {
		http.Error(w, "Sign-in could not be verified. Please start again.", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		http.Error(w, "The identity provider did not share a verified email address.", http.StatusForbidden)
		return
	}
	verified := claims.EmailVerified == nil || *claims.EmailVerified
	if !m.Allowed(email, verified) {
		slog.Warn("login denied by allowlist", "email", email)
		http.Error(w, "This account is not allowed to access this shelf.", http.StatusForbidden)
		return
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredName
	}
	u, err := m.UpsertUser(r.Context(), email, name, claims.Picture, verified)
	if err != nil {
		slog.Error("upsert user failed", "error", err)
		http.Error(w, "Could not create your account.", http.StatusInternalServerError)
		return
	}
	if err := m.StartSession(w, r, u.ID); err != nil {
		slog.Error("create session failed", "error", err)
		http.Error(w, "Could not start your session.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, safeReturnTo(state.ReturnTo), http.StatusSeeOther)
}

func validNonce(token *oidc.IDToken, expected string) bool {
	if expected == "" {
		return true
	}
	// go-oidc does not expose nonce directly; it lives in the raw claims.
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return false
	}
	got, _ := claims["nonce"].(string)
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func safeReturnTo(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.ContainsAny(v, "\r\n") {
		return "/"
	}
	return v
}

func (m *Manager) Allowed(email string, verified bool) bool {
	if len(m.cfg.AllowedEmails) == 0 && len(m.cfg.AllowedDomains) == 0 {
		return true
	}
	if !verified {
		return false
	}
	for _, e := range m.cfg.AllowedEmails {
		if strings.EqualFold(e, email) {
			return true
		}
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := email[at+1:]
	for _, d := range m.cfg.AllowedDomains {
		if strings.EqualFold(strings.TrimPrefix(d, "@"), domain) {
			return true
		}
	}
	return false
}

// UpsertUser creates or refreshes a local user. The first user becomes admin.
func (m *Manager) UpsertUser(ctx context.Context, email, name, avatar string, emailVerified bool) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email is required")
	}
	if !m.Allowed(email, emailVerified) {
		return nil, ErrForbidden
	}
	now := db.Now()
	var u User
	err := db.WithTx(ctx, m.db, func(tx *sql.Tx) error {
		var existing int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&existing); err != nil {
			return err
		}
		role := "member"
		if existing == 0 {
			role = "admin"
		}
		var id string
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			id = newID()
			_, err = tx.ExecContext(ctx, `INSERT INTO users(id,email,name,avatar_url,role,created_at,updated_at,last_login_at) VALUES(?,?,?,?,?,?,?,?)`,
				id, email, name, avatar, role, now, now, now)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_settings(user_id, timezone, updated_at) VALUES(?,?,?)`, id, m.cfg.Timezone, now); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if _, err := tx.ExecContext(ctx, `UPDATE users SET email=?,
				name=CASE WHEN ?<>'' THEN ? ELSE name END,
				avatar_url=CASE WHEN ?<>'' THEN ? ELSE avatar_url END,
				last_login_at=?, updated_at=? WHERE id=?`,
				email, name, name, avatar, avatar, now, now, id); err != nil {
				return err
			}
		}
		return tx.QueryRowContext(ctx, `SELECT id,email,name,avatar_url,role,coalesce(last_login_at,'') FROM users WHERE id=?`, id).
			Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.Role, &u.LastLoginAt)
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// StartSession creates a fresh session and sets the cookie.
func (m *Manager) StartSession(w http.ResponseWriter, r *http.Request, userID string) error {
	raw, err := randomString(32)
	if err != nil {
		return err
	}
	csrf, err := randomString(24)
	if err != nil {
		return err
	}
	hash := hashToken(raw)
	now := time.Now().UTC()
	expires := now.Add(m.cfg.SessionTTL)
	if _, err := m.db.ExecContext(r.Context(), `INSERT INTO sessions(token_hash,user_id,csrf_token,created_at,expires_at,last_seen_at,ip,user_agent) VALUES(?,?,?,?,?,?,?,?)`,
		hash, userID, csrf, db.Timestamp(now), db.Timestamp(expires), db.Timestamp(now), r.RemoteAddr, truncate(r.UserAgent(), 300)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: raw, Path: "/", HttpOnly: true, Secure: m.secure,
		SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(m.cfg.SessionTTL.Seconds()),
	})
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// LoadSession reads the session cookie and attaches auth context when valid.
func (m *Manager) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err == nil && c.Value != "" {
			a, err := m.sessionAuth(r, c.Value)
			if err == nil {
				r = r.WithContext(WithAuth(r.Context(), a))
			} else if !errors.Is(err, ErrUnauthenticated) {
				slog.Warn("session lookup failed", "error", err)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) sessionAuth(r *http.Request, raw string) (*AuthContext, error) {
	hash := hashToken(raw)
	now := db.Now()
	var u User
	var s Session
	var expires string
	err := m.db.QueryRowContext(r.Context(), `SELECT u.id,u.email,u.name,u.avatar_url,u.role,coalesce(u.last_login_at,''), s.token_hash,s.user_id,s.csrf_token,s.expires_at
		FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?`, hash).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.Role, &u.LastLoginAt, &s.TokenHash, &s.UserID, &s.CSRFToken, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	exp, _ := time.Parse(time.RFC3339, expires)
	if time.Now().UTC().After(exp) {
		_, _ = m.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash=?`, hash)
		return nil, ErrUnauthenticated
	}
	s.ExpiresAt = exp
	_, _ = m.db.ExecContext(r.Context(), `UPDATE sessions SET last_seen_at=? WHERE token_hash=? AND last_seen_at < ?`,
		now, hash, db.Timestamp(time.Now().Add(-5*time.Minute)))
	return &AuthContext{User: &u, Session: &s, Via: "cookie", CSRFToken: s.CSRFToken, Scopes: []string{"read", "write", "trmnl"}}, nil
}

// Logout deletes the current session and clears the cookie.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		_, _ = m.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash=?`, hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// CleanupSessions removes expired session rows.
func (m *Manager) CleanupSessions(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, db.Now())
	return err
}

// RequireCSRF protects unsafe cookie-authenticated requests.
func (m *Manager) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}
		a := FromContext(r.Context())
		if a == nil || a.Session == nil {
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			base, _ := url.Parse(m.cfg.BaseURL)
			if err != nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		token := r.Header.Get(CSRFHeader)
		if token == "" {
			token = r.FormValue("csrf_token")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(a.CSRFToken)) != 1 {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthenticateBearer accepts personal tokens first, then OIDC JWTs.
func (m *Manager) AuthenticateBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "A bearer token is required.", nil)
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		a, err := m.TokenAuth(r.Context(), raw)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "The token is invalid or expired.", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithAuth(r.Context(), a)))
	})
}

func (m *Manager) TokenAuth(ctx context.Context, raw string) (*AuthContext, error) {
	if strings.HasPrefix(raw, "bg_") {
		return m.personalTokenAuth(ctx, raw)
	}
	return m.jwtAuth(ctx, raw)
}

func (m *Manager) personalTokenAuth(ctx context.Context, raw string) (*AuthContext, error) {
	var t Token
	var u User
	var scopes, lastUsed, revoked sql.NullString
	err := m.db.QueryRowContext(ctx, `SELECT t.id,t.user_id,t.name,t.prefix,t.scopes,t.created_at,t.last_used_at,t.revoked_at,
		u.id,u.email,u.name,u.avatar_url,u.role,coalesce(u.last_login_at,'')
		FROM api_tokens t JOIN users u ON u.id=t.user_id WHERE t.token_hash=?`, hashToken(raw)).
		Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &scopes, &t.CreatedAt, &lastUsed, &revoked,
			&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.Role, &u.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	t.Scopes = strings.Fields(scopes.String)
	if revoked.Valid && revoked.String != "" {
		return nil, ErrUnauthenticated
	}
	t.LastUsedAt = lastUsed.String
	if _, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at=? WHERE id=?`, db.Now(), t.ID); err != nil {
		slog.Warn("update token last used", "error", err)
	}
	return &AuthContext{User: &u, Token: &t, Scopes: t.Scopes, Via: "token"}, nil
}

func (m *Manager) jwtAuth(ctx context.Context, raw string) (*AuthContext, error) {
	if err := m.Discover(ctx); err != nil {
		return nil, ErrUnauthenticated
	}
	idToken, err := m.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, ErrUnauthenticated
	}
	var u User
	err = m.db.QueryRowContext(ctx, `SELECT id,email,name,avatar_url,role,coalesce(last_login_at,'') FROM users WHERE email=?`, strings.ToLower(claims.Email)).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.Role, &u.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	return &AuthContext{User: &u, Via: "jwt", Scopes: []string{"read", "write"}}, nil
}

// CreateToken returns the raw secret exactly once and stores its hash.
func (m *Manager) CreateToken(ctx context.Context, userID, name string, scopes []string) (string, *Token, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "API token"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	allowed := map[string]bool{"read": true, "write": true, "trmnl": true}
	seen := map[string]bool{}
	clean := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if !allowed[s] {
			return "", nil, fmt.Errorf("unknown scope %q", s)
		}
		if !seen[s] {
			seen[s] = true
			clean = append(clean, s)
		}
	}
	if len(clean) == 0 {
		clean = []string{"read"}
	}
	// A write token implies read access; keep the stored scopes explicit.
	secret, err := randomString(32)
	if err != nil {
		return "", nil, err
	}
	raw := "bg_" + secret
	t := &Token{ID: newID(), UserID: userID, Name: name, Prefix: raw[:11], Scopes: clean, CreatedAt: db.Now()}
	_, err = m.db.ExecContext(ctx, `INSERT INTO api_tokens(id,user_id,name,token_hash,prefix,scopes,created_at) VALUES(?,?,?,?,?,?,?)`,
		t.ID, userID, name, hashToken(raw), t.Prefix, strings.Join(clean, " "), t.CreatedAt)
	if err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

func (m *Manager) ListTokens(ctx context.Context, userID string) ([]Token, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id,user_id,name,prefix,scopes,created_at,coalesce(last_used_at,''),coalesce(revoked_at,'') FROM api_tokens WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		var scopes string
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &scopes, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		t.Scopes = strings.Fields(scopes)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (m *Manager) RevokeToken(ctx context.Context, userID, tokenID string) error {
	res, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=? WHERE id=? AND user_id=? AND revoked_at IS NULL`, db.Now(), tokenID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RequireScope guards API handlers.
func RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HasScope(r.Context(), scope) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", "This token lacks the required scope.", map[string]string{"scope": scope})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(status)
	payload := map[string]any{"error": map[string]any{"code": code, "message": message}}
	if len(fields) > 0 {
		payload["error"].(map[string]any)["fields"] = fields
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// UserSettings is the per-user configuration needed by the web layer.
type UserSettings struct {
	UserID              string
	BGGUsername         string
	BGGVerifiedAt       string
	SyncEnabled         bool
	SyncIntervalSeconds int
	Theme               string
	Timezone            string
}

func (m *Manager) Settings(ctx context.Context, userID string) (UserSettings, error) {
	var s UserSettings
	s.UserID = userID
	var enabled int
	err := m.db.QueryRowContext(ctx, `SELECT bgg_username,coalesce(bgg_username_verified_at,''),bgg_sync_enabled,bgg_sync_interval_seconds,theme,timezone FROM user_settings WHERE user_id=?`, userID).
		Scan(&s.BGGUsername, &s.BGGVerifiedAt, &enabled, &s.SyncIntervalSeconds, &s.Theme, &s.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return UserSettings{UserID: userID, Theme: "system", Timezone: m.cfg.Timezone, SyncEnabled: true, SyncIntervalSeconds: 21600}, nil
	}
	s.SyncEnabled = enabled == 1
	return s, err
}

func (m *Manager) EnsureSettings(ctx context.Context, userID string) error {
	_, err := m.db.ExecContext(ctx, `INSERT INTO user_settings(user_id, timezone, updated_at) VALUES(?,?,?) ON CONFLICT(user_id) DO NOTHING`, userID, m.cfg.Timezone, db.Now())
	return err
}
