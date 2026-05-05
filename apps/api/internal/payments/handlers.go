package payments

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const webhookProcessingTimeout = 30 * time.Second

type WebhookHandler struct {
	logger           *slog.Logger
	expectedProvider string
	service          WebhookProcessor
}

type WebhookProcessor interface {
	HandleWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error
}

func NewWebhookHandler(logger *slog.Logger, expectedProvider string, service WebhookProcessor) *WebhookHandler {
	return &WebhookHandler{
		logger:           logger,
		expectedProvider: expectedProvider,
		service:          service,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	provider := strings.TrimPrefix(r.URL.Path, "/webhooks/payments/")
	if provider == "" || provider == "/" || strings.Contains(provider, "/") {
		http.NotFound(w, r)
		return
	}

	if h.expectedProvider != "" && provider != h.expectedProvider {
		http.NotFound(w, r)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	processCtx, cancel := context.WithTimeout(context.Background(), webhookProcessingTimeout)
	defer cancel()

	if err := h.service.HandleWebhook(processCtx, provider, r.Header, body); err != nil {
		statusCode := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrWebhookUnauthorized):
			statusCode = http.StatusUnauthorized
		case errors.Is(err, ErrUnsupportedProvider):
			statusCode = http.StatusNotFound
		}

		h.logger.Error("payment webhook processing failed",
			"provider", provider,
			"error", err,
			"paypal_auth_algo_present", headerPresent(r.Header, "PAYPAL-AUTH-ALGO"),
			"paypal_cert_url_present", headerPresent(r.Header, "PAYPAL-CERT-URL"),
			"paypal_transmission_id_present", headerPresent(r.Header, "PAYPAL-TRANSMISSION-ID"),
			"paypal_transmission_sig_present", headerPresent(r.Header, "PAYPAL-TRANSMISSION-SIG"),
			"paypal_transmission_time_present", headerPresent(r.Header, "PAYPAL-TRANSMISSION-TIME"),
			"request_id", r.Header.Get("X-Request-ID"),
		)
		http.Error(w, http.StatusText(statusCode), statusCode)
		return
	}

	h.logger.Info("payment webhook processed",
		"provider", provider,
		"payload_bytes", len(body),
		"request_id", r.Header.Get("X-Request-ID"),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func headerPresent(headers http.Header, name string) bool {
	return strings.TrimSpace(headers.Get(name)) != ""
}
