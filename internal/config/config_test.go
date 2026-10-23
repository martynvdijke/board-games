package config

import "testing"

func TestProductionRejectsFakeAuth(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("BASE_URL", "https://shelf.example.com")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DEV_FAKE_AUTH", "true")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com")
	t.Setenv("OIDC_CLIENT_ID", "the-shelf")
	t.Setenv("BGG_USER_AGENT", "TheShelf/1.0 (owner@example.com)")
	if _, err := Load(); err == nil {
		t.Fatal("expected production fake auth to be rejected")
	}
}

func TestProductionRequiresOIDC(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("BASE_URL", "https://shelf.example.com")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DEV_FAKE_AUTH", "false")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("BGG_USER_AGENT", "TheShelf/1.0 (owner@example.com)")
	if _, err := Load(); err == nil {
		t.Fatal("expected OIDC to be required")
	}
}
