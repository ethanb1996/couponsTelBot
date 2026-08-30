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
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	catalog, err := telegram.NewCatalog(cfg.OffersFile)
	if err != nil {
		logger.Error("failed to load offer catalog", "error", err)
		os.Exit(1)
	}
	bot, err := telegram.NewBot(logger, cfg.TelegramBotToken, cfg.TelegramChannelID, cfg.TelegramAdminUserIDs, catalog)
	if err != nil {
		logger.Error("failed to initialize Telegram bot", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/webhooks/telegram", telegram.NewWebhookHandler(logger, cfg.TelegramWebhookSecret, bot))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		logger.Info("coupon menu bot listening", "port", cfg.Port, "offers_file", cfg.OffersFile)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
