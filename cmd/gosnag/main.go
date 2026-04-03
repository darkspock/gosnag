package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database"
	dbpkg "github.com/darkspock/gosnag/internal/database/db"
	"github.com/darkspock/gosnag/internal/issue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queries := dbpkg.New(db)
	go issue.CooldownChecker(ctx, queries, 1*time.Minute)
	go sessionCleanup(ctx, queries, 1*time.Hour)
	if cfg.EventRetentionDays > 0 {
		go eventRetention(ctx, queries, cfg.EventRetentionDays, 6*time.Hour)
	}

	router := setupRouter(db, cfg)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("GoSnag started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	cancel() // stop cooldown checker

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}
}

func eventRetention(ctx context.Context, queries *dbpkg.Queries, retentionDays int, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().AddDate(0, 0, -retentionDays)
			result, err := queries.DeleteEventsOlderThan(ctx, cutoff)
			if err != nil {
				slog.Error("event retention cleanup failed", "error", err)
			} else if n, _ := result.RowsAffected(); n > 0 {
				slog.Info("event retention cleanup", "deleted", n, "older_than_days", retentionDays)
			}
		}
	}
}

func sessionCleanup(ctx context.Context, queries *dbpkg.Queries, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := queries.DeleteExpiredSessions(ctx); err != nil {
				slog.Error("session cleanup failed", "error", err)
			}
		}
	}
}
