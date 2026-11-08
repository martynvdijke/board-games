package config

import (
	"os"
	"testing"
)

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

func TestLoadsOIDCClientSecretFromFile(t *testing.T) {
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com")
	t.Setenv("OIDC_CLIENT_ID", "board-games")
	path := t.TempDir() + "/oidc-client-secret"
	if err := os.WriteFile(path, []byte("  s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OIDC_CLIENT_SECRET_FILE", path)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDCClientSecret != "s3cret" {
		t.Fatalf("OIDCClientSecret = %q, want %q", cfg.OIDCClientSecret, "s3cret")
	}
}

func TestLoadsBGGAPITokenFromFileAndNormalisesBaseURL(t *testing.T) {
	t.Setenv("BGG_API_TOKEN", "")
	t.Setenv("BGG_BASE_URL", "https://www.boardgamegeek.com/xmlapi2/")
	path := t.TempDir() + "/bgg-token"
	if err := os.WriteFile(path, []byte("  bgg-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BGG_API_TOKEN_FILE", path)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BGGAPIToken != "bgg-secret" {
		t.Fatalf("BGGAPIToken = %q, want %q", cfg.BGGAPIToken, "bgg-secret")
	}
	if cfg.BGGBaseURL != "https://boardgamegeek.com/xmlapi2" {
		t.Fatalf("BGGBaseURL = %q", cfg.BGGBaseURL)
	}
	var logged string
	for _, attr := range cfg.LogValue().Group() {
		if attr.Key == "bgg_api_token" {
			logged = attr.Value.String()
		}
	}
	if logged != "[redacted]" {
		t.Fatalf("bgg_api_token log value = %q, want redacted", logged)
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

func TestTelegramConfigAndRedaction(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "  123456:secret-token  ")
	t.Setenv("TELEGRAM_CHAT_ID", "  -100987654  ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TelegramBotToken != "123456:secret-token" {
		t.Fatalf("TelegramBotToken = %q", cfg.TelegramBotToken)
	}
	if cfg.TelegramChatID != "-100987654" {
		t.Fatalf("TelegramChatID = %q", cfg.TelegramChatID)
	}
	logged := map[string]string{}
	for _, attr := range cfg.LogValue().Group() {
		logged[attr.Key] = attr.Value.String()
	}
	if logged["telegram_bot_token"] != "[redacted]" {
		t.Fatalf("telegram_bot_token log value = %q, want redacted", logged["telegram_bot_token"])
	}
	if logged["telegram_chat_id"] != "[redacted]" {
		t.Fatalf("telegram_chat_id log value = %q, want redacted", logged["telegram_chat_id"])
	}
}

func TestTelegramConfigOptional(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without Telegram settings: %v", err)
	}
	if cfg.TelegramBotToken != "" || cfg.TelegramChatID != "" {
		t.Fatalf("expected empty Telegram config, got %q / %q", cfg.TelegramBotToken, cfg.TelegramChatID)
	}
	logged := map[string]string{}
	for _, attr := range cfg.LogValue().Group() {
		logged[attr.Key] = attr.Value.String()
	}
	if logged["telegram_bot_token"] != "" || logged["telegram_chat_id"] != "" {
		t.Fatalf("empty values should log empty, got %q / %q", logged["telegram_bot_token"], logged["telegram_chat_id"])
	}
}
