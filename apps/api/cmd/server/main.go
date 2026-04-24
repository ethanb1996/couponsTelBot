package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	apphttp "github.com/ethanb1996/couponsTelBot/apps/api/internal/http"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/ops"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	}))

	for _, warning := range store.DatabaseConnectionWarnings(cfg.DatabaseURL, cfg.DatabaseQueryExecMode) {
		logger.Warn("database configuration warning", "warning", warning)
	}

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          cfg.DatabaseURL,
		ApplicationName:      cfg.DatabaseApplicationName,
		ConnectTimeout:       cfg.DatabaseConnectTimeout,
		MaxConns:             cfg.DatabaseMaxConns,
		MinConns:             cfg.DatabaseMinConns,
		MaxConnLifetime:      cfg.DatabaseMaxConnLifetime,
		MaxConnIdleTime:      cfg.DatabaseMaxConnIdleTime,
		HealthCheckPeriod:    cfg.DatabaseHealthCheckPeriod,
		DefaultQueryExecMode: cfg.DatabaseQueryExecMode,
	})
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()

	if err := db.Ping(pingCtx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	router, err := apphttp.NewRouter(apphttp.Dependencies{
		Logger: logger,
		Config: cfg,
		Store:  db,
	})
	if err != nil {
		logger.Error("failed to build router", "error", err)
		os.Exit(1)
	}

	ops.NewRunner(
		logger,
		db,
		router.PaymentService,
		cfg.OpsSweepInterval,
		cfg.OpsDeliveryAlertAfter,
		cfg.OpsReconcileAfter,
		cfg.OpsBatchSize,
	).Start(ctx)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrCh := make(chan error, 1)

	go func() {
		logger.Info("server starting", "addr", server.Addr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErrCh:
		logger.Error("server stopped unexpectedly", "error", err)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped cleanly")
}
