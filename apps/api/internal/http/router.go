package apphttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/admin"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/security"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/services"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/telegram"
)

type Dependencies struct {
	Logger *slog.Logger
	Config config.Config
	Store  *store.Postgres
}

type Router struct {
	Handler    http.Handler
	BotService *telegram.BotService
}

func NewRouter(deps Dependencies) (Router, error) {
	if deps.Logger == nil {
		return Router{}, errors.New("logger is required")
	}

	if deps.Store == nil {
		return Router{}, errors.New("store is required")
	}

	// Create bot service
	botService, err := telegram.NewBotService(deps.Logger, deps.Store, deps.Config.TelegramBotToken, &deps.Config)
	if err != nil {
		return Router{}, err
	}

	payBoxService, err := services.NewPayBoxService(services.PayBoxServiceOptions{
		Logger:            deps.Logger,
		Store:             deps.Store,
		Messenger:         botService,
		CodeRenderer:      services.MaskedPredefinedCodeRenderer{},
		QRRenderer:        services.NewQRCodeRenderer(320),
		RedemptionBaseURL: deps.Config.AppBaseURL,
	})
	if err != nil {
		return Router{}, err
	}
	botService.SetPayBoxFlow(payBoxService)

	adminHandler, err := admin.NewHandler(deps.Logger, deps.Store, deps.Config.CouponEncryptionKey, payBoxService)
	if err != nil {
		return Router{}, err
	}

	telegramHandler := telegram.NewWebhookHandler(deps.Logger, deps.Config.TelegramWebhookSecret, botService)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(deps.Store))
	mux.Handle("/admin/", security.BasicAuth(deps.Config.AdminBasicAuthUser, deps.Config.AdminBasicAuthPass)(http.HandlerFunc(adminHandler.Route)))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	})
	mux.HandleFunc("/webhooks/telegram", telegramHandler.ServeHTTP)
	mux.HandleFunc("/api/redemptions/scan", redemptionScanHandler(deps.Store))
	mux.HandleFunc("/api/redemptions/scan/", redemptionScanByTokenHandler(deps.Store))

	return Router{
		Handler: Chain(
			mux,
			WithRecovery(deps.Logger),
			WithRequestID(),
			WithSecurityHeaders(),
			WithAccessLog(deps.Logger),
		),
		BotService: botService,
	}, nil
}

func healthHandler(db *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		databaseStatus := "ok"
		if err := db.Ping(ctx); err != nil {
			databaseStatus = "unavailable"
		}

		WriteJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"database": databaseStatus,
			"service":  "coupon-sales-api",
			"path":     strings.TrimSpace(r.URL.Path),
		})
	}
}
