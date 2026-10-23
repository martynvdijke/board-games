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

	"boardgames/internal/api"
	"boardgames/internal/auth"
	"boardgames/internal/bgg"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
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
	games := collection.NewRepo(database)
	playRepo := plays.NewRepo(database)
	bggClient := bgg.NewClient(cfg.BGGBaseURL, cfg.BGGUserAgent, cfg.BGGRequestsPerSecond)
	syncService := collection.NewSyncService(database, bggClient, games, playRepo, cfg)
	trmnlService := trmnl.NewService(database, games, playRepo, cfg)
	imageDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "covers")
	images := trmnl.NewImageService(database, imageDir)

	webHandler := &web.Handler{
		Config: cfg, Auth: authManager, Games: games, Plays: playRepo, Sync: syncService,
		TRMNL: trmnlService, Images: images, DB: database, MetricsHandler: providers.MetricsHandler(),
	}
	apiHandler := &api.Handler{
		Config: cfg, Auth: authManager, Games: games, Plays: playRepo, Sync: syncService,
		TRMNL: trmnlService, DB: database,
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler.Routes()))
	mux.Handle("/", webHandler.Routes())

	trusted := observability.ParseTrustedProxies(cfg.TrustedProxies)
	handler := observability.Middleware(mux, providers.Metrics, trusted)

	go syncService.Scheduler(ctx)
	go cleanupSessions(ctx, authManager)

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
