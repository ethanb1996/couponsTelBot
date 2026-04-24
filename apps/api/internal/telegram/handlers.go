package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const asyncUpdateTimeout = 30 * time.Second

type UpdateHandler interface {
	HandleUpdate(ctx context.Context, update tgbotapi.Update) error
}

type WebhookHandler struct {
	logger      *slog.Logger
	secret      string
	updateLogic UpdateHandler
}

func NewWebhookHandler(logger *slog.Logger, secret string, updateLogic UpdateHandler) *WebhookHandler {
	return &WebhookHandler{
		logger:      logger,
		secret:      secret,
		updateLogic: updateLogic,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if h.secret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != h.secret {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		h.logger.Error("failed to unmarshal telegram update", "error", err, "request_id", r.Header.Get("X-Request-ID"))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Process the update asynchronously
	requestID := r.Header.Get("X-Request-ID")
	go func(update tgbotapi.Update, requestID string) {
		ctx, cancel := context.WithTimeout(context.Background(), asyncUpdateTimeout)
		defer cancel()

		if err := h.updateLogic.HandleUpdate(ctx, update); err != nil {
			h.logger.Error("failed to handle update", "error", err, "update_id", update.UpdateID, "request_id", requestID)
		}
	}(update, requestID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func readUpdateID(payload map[string]any) int64 {
	raw, ok := payload["update_id"]
	if !ok {
		return 0
	}

	switch value := raw.(type) {
	case float64:
		return int64(value)
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return parsed
		}
	}

	return 0
}
