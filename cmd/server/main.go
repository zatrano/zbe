package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/internal/app"
	"github.com/zatrano/zbe/migrations"
	"github.com/zatrano/zbe/pkg/logger"
	"github.com/zatrano/zbe/pkg/mail"
	"github.com/zatrano/zbe/seed"
)

func main() {
	// ── Load configuration ─────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: config load: %v\n", err)
		os.Exit(1)
	}

	// ── Initialise logger ──────────────────────────────────────────────────────
	logger.Init(cfg.App.Env)
	defer logger.Sync()

	logger.Info("starting ZBE server",
		zap.String("env",     cfg.App.Env),
		zap.String("address", cfg.ServerAddress()),
		zap.String("version", "1.0.0"),
	)

	// ── Connect to PostgreSQL ──────────────────────────────────────────────────
	db, err := app.NewDatabase(cfg)
	if err != nil {
		logger.Fatal("database connection failed", zap.Error(err))
	}
	defer app.CloseDatabase(db)

	// ── Run migrations ─────────────────────────────────────────────────────────
	if err := migrations.Run(db); err != nil {
		logger.Fatal("migrations failed", zap.Error(err))
	}

	// ── Run seeds ──────────────────────────────────────────────────────────────
	if err := seed.Run(db); err != nil {
		logger.Fatal("seeding failed", zap.Error(err))
	}

	// ── Start mail service ─────────────────────────────────────────────────────
	mailSvc := mail.New(cfg.SMTP, cfg.App)
	defer mailSvc.Close()

	// ── Build Fiber router ─────────────────────────────────────────────────────
	fiberApp := app.SetupRouter(cfg, db, mailSvc)

	// ── Background jobs ───────────────────────────────────────────────────────
	go runTokenCleanup(db)

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		addr := cfg.ServerAddress()
		logger.Info("HTTP server listening", zap.String("address", addr))
		if err := fiberApp.Listen(addr); err != nil {
			serverErr <- err
		}
	}()

	select {
	case sig := <-quit:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-serverErr:
		logger.Error("server error", zap.Error(err))
	}

	logger.Info("gracefully shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := fiberApp.ShutdownWithContext(ctx); err != nil {
		logger.Error("forced shutdown", zap.Error(err))
	}

	logger.Info("server stopped")
}

// runTokenCleanup periodically removes expired revoked-token rows.
func runTokenCleanup(db *gorm.DB) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		result := db.Exec("DELETE FROM revoked_tokens WHERE expires_at <= NOW()")
		if result.Error != nil {
			logger.Warnf("token cleanup error: %v", result.Error)
		} else {
			logger.Debugf("token cleanup: removed %d expired records", result.RowsAffected)
		}
	}
}
