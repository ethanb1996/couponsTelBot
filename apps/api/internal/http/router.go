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
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/payments"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/security"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/telegram"
)

type Dependencies struct {
	Logger *slog.Logger
	Config config.Config
	Store  *store.Postgres
}

type Router struct {
	Handler        http.Handler
	PaymentService *payments.Service
}

func NewRouter(deps Dependencies) (Router, error) {
	if deps.Logger == nil {
		return Router{}, errors.New("logger is required")
	}

	if deps.Store == nil {
		return Router{}, errors.New("store is required")
	}

	adminHandler, err := admin.NewHandler(deps.Logger, deps.Store, deps.Config.CouponEncryptionKey)
	if err != nil {
		return Router{}, err
	}

	// Create bot service
	botService, err := telegram.NewBotService(deps.Logger, deps.Store, deps.Config.TelegramBotToken, &deps.Config)
	if err != nil {
		return Router{}, err
	}

	paymentService, err := payments.NewService(deps.Logger, deps.Store, deps.Config, botService)
	if err != nil {
		return Router{}, err
	}
	botService.SetCheckoutStarter(paymentService)

	telegramHandler := telegram.NewWebhookHandler(deps.Logger, deps.Config.TelegramWebhookSecret, botService)
	paymentHandler := payments.NewWebhookHandler(
		deps.Logger,
		deps.Config.PaymentProviderName,
		paymentService,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(deps.Store))
	mux.Handle("/admin/", security.BasicAuth(deps.Config.AdminBasicAuthUser, deps.Config.AdminBasicAuthPass)(http.HandlerFunc(adminHandler.Route)))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	})
	mux.HandleFunc("/webhooks/telegram", telegramHandler.ServeHTTP)
	mux.HandleFunc("/webhooks/payments/", paymentHandler.ServeHTTP)
	mux.HandleFunc("/payments/paypal/return", payments.ReturnPage)
	mux.HandleFunc("/payments/paypal/cancel", payments.CancelPage)

	return Router{
		Handler: Chain(
			mux,
			WithRecovery(deps.Logger),
			WithRequestID(),
			WithSecurityHeaders(),
			WithAccessLog(deps.Logger),
		),
		PaymentService: paymentService,
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
