// Command server runs The Shelf: a single-user-friendly, multi-user-capable
// board game collection, play log, API, and TRMNL feed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/api"
	"boardgames/internal/auth"
	"boardgames/internal/bgg"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/demo"
	"boardgames/internal/loans"
	"boardgames/internal/notify"
	"boardgames/internal/observability"
	"boardgames/internal/plays"
	"boardgames/internal/trmnl"
	"boardgames/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	providers, err := observability.Setup(ctx, cfg)
	if err != nil {
		return fmt.Errorf("setup observability: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = providers.Shutdown(shutdownCtx)
	}()

	database, err := db.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	authManager := auth.New(database, cfg)
	if promoted, err := authManager.EnsureAdmin(ctx); err != nil {
		return fmt.Errorf("ensure administrator: %w", err)
	} else if promoted != "" {
		slog.Info("promoted the oldest account to administrator", "user_id", promoted)
	}
	games := collection.NewRepo(database)
	playRepo := plays.NewRepo(database)
	var notifier notify.Notifier
	if telegram := notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID); telegram != nil {
		notifier = telegram
	}
	playRepo.SetNotifier(notifier)
	loanRepo := loans.New(database)
	loanRepo.SetIdleDays(cfg.LoanIdleReminderDays)
	bggClient := bgg.NewClient(cfg.BGGBaseURL, cfg.BGGUserAgent, cfg.BGGRequestsPerSecond)
	syncService := collection.NewSyncService(database, bggClient, games, playRepo, cfg)
	syncService.SetNotifier(notifier)
	// A game that leaves the collection must not keep an open loan: close it
	// with an explanatory note as soon as the sync has rewritten ownership.
	syncService.OnCollectionChanged = func(ctx context.Context, userID string) error {
		_, err := loanRepo.CloseForOwnershipChange(ctx, userID)
		return err
	}
	trmnlService := trmnl.NewService(database, games, playRepo, cfg)
	setupMediaDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "setup")
	accountService := account.NewWithMedia(database, setupMediaDir)
	imageDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "covers")
	images := trmnl.NewImageService(database, imageDir)

	demoService := demo.New(database, authManager, cfg)
	if err := demoService.Ensure(ctx); err != nil {
		return fmt.Errorf("ensure demo account: %w", err)
	}

	webHandler := &web.Handler{
		Config: cfg, Auth: authManager, Account: accountService, Games: games, Plays: playRepo, Loans: loanRepo, Sync: syncService,
		TRMNL: trmnlService, Images: images, SetupMediaDir: setupMediaDir, DB: database, MetricsHandler: providers.MetricsHandler(),
	}
	apiHandler := &api.Handler{
		Config: cfg, Auth: authManager, Account: accountService, Games: games, Plays: playRepo, Sync: syncService,
		TRMNL: trmnlService, DB: database,
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler.Routes()))
	mux.Handle("/", webHandler.Routes())

	trusted := observability.ParseTrustedProxies(cfg.TrustedProxies)
	handler := observability.Middleware(mux, providers.Metrics, trusted)

	go syncService.Scheduler(ctx)
	go cleanupSessions(ctx, authManager)
	if cfg.RemindersEnabled {
		go remindLoans(ctx, loanRepo, notifier, time.Duration(cfg.LoanReminderDays)*24*time.Hour)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("The Shelf is open", "addr", server.Addr, "base_url", cfg.BaseURL, "env", cfg.Env, "version", cfg.Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

func cleanupSessions(ctx context.Context, manager *auth.Manager) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.CleanupSessions(ctx); err != nil {
				slog.Warn("session cleanup", "error", err)
			}
		}
	}
}

// loanReminderInterval is how often the loan reminder pass runs. It shares the
// six-hour cadence of the other background jobs.
const loanReminderInterval = 6 * time.Hour

// remindLoans sends due loan reminders. A nil notifier makes every pass a
// no-op; interval is the minimum wait before reminding the same loan again.
func remindLoans(ctx context.Context, repo *loans.Repo, n notify.Notifier, interval time.Duration) {
	ticker := time.NewTicker(loanReminderInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := repo.Remind(ctx, n, interval); err != nil {
				slog.Warn("loan reminders", "error", err)
			}
		}
	}
}
