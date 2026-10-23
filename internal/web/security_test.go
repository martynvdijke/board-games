package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"boardgames/internal/config"
)

func TestSecurityHeadersAllowUmamiScriptAndConnect(t *testing.T) {
	h := &Handler{Config: &config.Config{UmamiURL: "https://umami.example.com", UmamiWebsiteID: "site-1"}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	h.securityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://umami.example.com") {
		t.Fatalf("script-src missing Umami: %s", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' https://umami.example.com") {
		t.Fatalf("connect-src missing Umami: %s", csp)
	}
}

func TestSecurityHeadersWithoutUmami(t *testing.T) {
	h := &Handler{Config: &config.Config{}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	h.securityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "umami") {
		t.Fatalf("unexpected Umami origin in CSP: %s", csp)
	}
	if !strings.Contains(csp, "connect-src 'self';") {
		t.Fatalf("connect-src should stay self-only: %s", csp)
	}
}

func TestLoginRendersUmamiScriptWhenConfigured(t *testing.T) {
	h := &Handler{Config: &config.Config{AppName: "The Shelf", UmamiURL: "https://umami.example.com", UmamiWebsiteID: "site-1"}}
	rec := httptest.NewRecorder()
	h.render(rec, httptest.NewRequest(http.MethodGet, "/login", nil), "login.html", Page{
		Title: "Sign in", AppName: "The Shelf", Theme: "system", Data: map[string]string{},
		UmamiURL: "https://umami.example.com", UmamiWebsiteID: "site-1",
	})
	body := rec.Body.String()
	if !strings.Contains(body, `src="https://umami.example.com/script.js"`) {
		t.Fatal("Umami script was not rendered")
	}
	if !strings.Contains(body, `data-website-id="site-1"`) {
		t.Fatal("Umami website id was not rendered")
	}
}

func TestLoginOmitsUmamiScriptWhenUnconfigured(t *testing.T) {
	h := &Handler{Config: &config.Config{AppName: "The Shelf"}}
	rec := httptest.NewRecorder()
	h.render(rec, httptest.NewRequest(http.MethodGet, "/login", nil), "login.html", Page{
		Title: "Sign in", AppName: "The Shelf", Theme: "system", Data: map[string]string{},
	})
	if strings.Contains(rec.Body.String(), "umami") {
		t.Fatal("Umami script should not render without configuration")
	}
}
