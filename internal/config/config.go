package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the complete runtime configuration, loaded from the environment.
type Config struct {
	AppName              string
	Env                  string
	Port                 string
	BaseURL              string
	DatabasePath         string
	SessionSecret        string
	SessionTTL           time.Duration
	OIDCIssuer           string
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string
	AllowedEmails        []string
	AllowedDomains       []string
	BGGBaseURL           string
	BGGAPIToken          string
	TelegramBotToken     string
	TelegramChatID       string
	BGGUserAgent         string
	BGGRequestsPerSecond float64
	BGGRefreshDays       int
	SyncInterval         time.Duration
	TrustedProxies       []string
	MetricsEnabled       bool
	OTLPEndpoint         string
	OTELServiceName      string
	UmamiURL             string
	UmamiWebsiteID       string
	LegalControllerName  string
	LegalContactEmail    string
	LegalContactAddress  string
	LegalHostingRegion   string
	DevFakeAuth          bool
	DevFakeEmail         string
	DevFakeName          string
	DemoUserEnabled      bool
	DemoUserEmail        string
	DemoUserName         string
	DemoUserToken        string
	LogLevel             slog.Level
	Timezone             string
	Version              string
}

// Load reads and validates configuration. Development mode can generate a
// session secret so `go run ./cmd/server` works without ceremony.
func Load() (*Config, error) {
	c := &Config{
		AppName:              env("APP_NAME", "The Shelf"),
		Env:                  env("ENV", "development"),
		Port:                 env("PORT", "6270"),
		BaseURL:              strings.TrimRight(env("BASE_URL", "http://localhost:6270"), "/"),
		DatabasePath:         env("DATABASE_PATH", "./data/boardgames.db"),
		SessionSecret:        os.Getenv("SESSION_SECRET"),
		SessionTTL:           duration("SESSION_TTL", 720*time.Hour),
		OIDCIssuer:           os.Getenv("OIDC_ISSUER"),
		OIDCClientID:         os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:     os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:      os.Getenv("OIDC_REDIRECT_URL"),
		AllowedEmails:        list("ALLOWED_EMAILS"),
		AllowedDomains:       list("ALLOWED_DOMAINS"),
		BGGBaseURL:           strings.TrimRight(env("BGG_BASE_URL", "https://boardgamegeek.com/xmlapi2"), "/"),
		BGGAPIToken:          strings.TrimSpace(os.Getenv("BGG_API_TOKEN")),
		TelegramBotToken:     strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:       strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		BGGUserAgent:         env("BGG_USER_AGENT", "TheShelf/1.0 (self-hosted board game shelf)"),
		BGGRequestsPerSecond: float("BGG_REQUESTS_PER_SECOND", 1),
		BGGRefreshDays:       integer("BGG_REFRESH_DAYS", 30),
		SyncInterval:         duration("BGG_SYNC_INTERVAL", 6*time.Hour),
		TrustedProxies:       list("TRUSTED_PROXIES"),
		MetricsEnabled:       boolean("METRICS_ENABLED", true),
		OTLPEndpoint:         os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTELServiceName:      env("OTEL_SERVICE_NAME", "boardgames"),
		UmamiURL:             strings.TrimRight(os.Getenv("UMAMI_URL"), "/"),
		UmamiWebsiteID:       os.Getenv("UMAMI_WEBSITE_ID"),
		LegalControllerName:  strings.TrimSpace(os.Getenv("LEGAL_CONTROLLER_NAME")),
		LegalContactEmail:    strings.TrimSpace(os.Getenv("LEGAL_CONTACT_EMAIL")),
		LegalContactAddress:  strings.TrimSpace(os.Getenv("LEGAL_CONTACT_ADDRESS")),
		LegalHostingRegion:   env("LEGAL_HOSTING_REGION", "the European Union"),
		DevFakeAuth:          boolean("DEV_FAKE_AUTH", false),
		DevFakeEmail:         env("DEV_FAKE_EMAIL", "owner@example.com"),
		DevFakeName:          env("DEV_FAKE_NAME", "Shelf Owner"),
		DemoUserEnabled:      boolean("DEMO_USER_ENABLED", isDemoHost(env("BASE_URL", "http://localhost:6270"))),
		DemoUserEmail:        strings.ToLower(strings.TrimSpace(env("DEMO_USER_EMAIL", "demo@theshelf.local"))),
		DemoUserName:         env("DEMO_USER_NAME", "Shelf Demo"),
		DemoUserToken:        strings.TrimSpace(os.Getenv("DEMO_USER_TOKEN")),
		LogLevel:             level(env("LOG_LEVEL", "info")),
		Timezone:             env("TZ", "UTC"),
		Version:              env("VERSION", "dev"),
	}
	if c.OIDCRedirectURL == "" && c.BaseURL != "" {
		c.OIDCRedirectURL = c.BaseURL + "/auth/callback"
	}
	if c.OIDCClientSecret == "" {
		if path := os.Getenv("OIDC_CLIENT_SECRET_FILE"); path != "" {
			secret, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read OIDC_CLIENT_SECRET_FILE: %w", err)
			}
			c.OIDCClientSecret = strings.TrimSpace(string(secret))
		}
	}
	// Go strips the Authorization header on cross-domain redirects, and BGG
	// redirects www.boardgamegeek.com to the apex host, so normalise to it.
	c.BGGBaseURL = strings.Replace(c.BGGBaseURL, "://www.boardgamegeek.com", "://boardgamegeek.com", 1)
	if c.BGGAPIToken == "" {
		if path := os.Getenv("BGG_API_TOKEN_FILE"); path != "" {
			token, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read BGG_API_TOKEN_FILE: %w", err)
			}
			c.BGGAPIToken = strings.TrimSpace(string(token))
		}
	}
	if c.SessionSecret == "" {
		if c.Env == "production" {
			return nil, errors.New("SESSION_SECRET is required in production")
		}
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		c.SessionSecret = hex.EncodeToString(b)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Validate() error {
	if c.Port == "" {
		return errors.New("PORT must not be empty")
	}
	if c.BaseURL == "" {
		return errors.New("BASE_URL must not be empty")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("BASE_URL must be an absolute URL, got %q", c.BaseURL)
	}
	if c.DemoUserEnabled && c.Env == "production" && c.DemoUserToken == "" {
		return errors.New("DEMO_USER_TOKEN is required when the demo account is enabled in production")
	}
	if c.DemoUserToken != "" && !strings.HasPrefix(c.DemoUserToken, "bg_") {
		return errors.New("DEMO_USER_TOKEN must begin with bg_")
	}
	if c.DemoUserEnabled && c.DemoUserEmail == "" {
		return errors.New("DEMO_USER_EMAIL must not be empty when the demo account is enabled")
	}
	if c.Env == "production" {
		if c.DevFakeAuth {
			return errors.New("DEV_FAKE_AUTH must not be enabled in production")
		}
		if c.OIDCIssuer == "" {
			return errors.New("OIDC_ISSUER is required in production")
		}
		if c.OIDCClientID == "" {
			return errors.New("OIDC_CLIENT_ID is required in production")
		}
		if !strings.Contains(c.BGGUserAgent, "(") {
			return errors.New("BGG_USER_AGENT should include a contact address in parentheses, e.g. 'TheShelf/1.0 (you@example.com)'")
		}
	}
	for _, cidr := range c.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if net.ParseIP(cidr) == nil {
				return fmt.Errorf("TRUSTED_PROXIES entry %q is not an IP or CIDR", cidr)
			}
		}
	}
	if c.BGGRequestsPerSecond <= 0 {
		return errors.New("BGG_REQUESTS_PER_SECOND must be positive")
	}
	if c.SessionTTL <= 0 {
		return errors.New("SESSION_TTL must be positive")
	}
	return nil
}

// isDemoHost reports whether the public host is the demonstration deployment.
func isDemoHost(baseURL string) bool {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), "board-games.vandijke.xyz")
}

// SecureCookies reports whether public URLs use HTTPS.
func (c *Config) SecureCookies() bool { return strings.HasPrefix(c.BaseURL, "https://") }

// RedirectURI is the effective OIDC callback URL.
func (c *Config) RedirectURI() string {
	if c.OIDCRedirectURL != "" {
		return c.OIDCRedirectURL
	}
	return strings.TrimRight(c.BaseURL, "/") + "/auth/callback"
}

// LogValue redacts secrets for structured startup logging.
func (c *Config) LogValue() slog.Value {
	redact := func(s string) string {
		if s == "" {
			return ""
		}
		return "[redacted]"
	}
	return slog.GroupValue(
		slog.String("app_name", c.AppName),
		slog.String("env", c.Env),
		slog.String("port", c.Port),
		slog.String("base_url", c.BaseURL),
		slog.String("database", c.DatabasePath),
		slog.String("session_secret", redact(c.SessionSecret)),
		slog.String("oidc_issuer", c.OIDCIssuer),
		slog.String("oidc_client_id", c.OIDCClientID),
		slog.String("oidc_client_secret", redact(c.OIDCClientSecret)),
		slog.String("oidc_redirect_uri", c.RedirectURI()),
		slog.String("bgg_base_url", c.BGGBaseURL),
		slog.String("bgg_api_token", redact(c.BGGAPIToken)),
		slog.String("telegram_bot_token", redact(c.TelegramBotToken)),
		slog.String("telegram_chat_id", redact(c.TelegramChatID)),
		slog.Int("bgg_refresh_days", c.BGGRefreshDays),
		slog.Duration("sync_interval", c.SyncInterval),
		slog.Bool("metrics_enabled", c.MetricsEnabled),
		slog.String("legal_controller", c.LegalControllerName),
		slog.String("legal_contact_email", c.LegalContactEmail),
		slog.Bool("dev_fake_auth", c.DevFakeAuth),
		slog.Bool("demo_user_enabled", c.DemoUserEnabled),
		slog.String("demo_user_token", redact(c.DemoUserToken)),
		slog.String("timezone", c.Timezone),
		slog.String("version", c.Version),
	)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func list(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func integer(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func float(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}

func boolean(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func level(v string) slog.Level {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
