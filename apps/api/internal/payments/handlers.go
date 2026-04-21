package payments

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type WebhookHandler struct {
	logger           *slog.Logger
	expectedProvider string
	secret           string
}

func NewWebhookHandler(logger *slog.Logger, expectedProvider string, secret string) *WebhookHandler {
	return &WebhookHandler{
		logger:           logger,
		expectedProvider: expectedProvider,
		secret:           secret,
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

	if h.secret != "" && r.Header.Get("X-Payment-Webhook-Secret") != h.secret {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	h.logger.Info("payment webhook received",
		"provider", provider,
		"payload_bytes", len(body),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"ok":true}`))
}
