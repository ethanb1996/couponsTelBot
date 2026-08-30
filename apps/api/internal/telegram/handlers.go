package telegram

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler interface {
	HandleUpdate(context.Context, tgbotapi.Update) error
}

type WebhookHandler struct {
	logger  *slog.Logger
	secret  string
	handler UpdateHandler
}

func NewWebhookHandler(logger *slog.Logger, secret string, handler UpdateHandler) *WebhookHandler {
	return &WebhookHandler{logger: logger, secret: secret, handler: handler}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != h.secret {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var update tgbotapi.Update
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&update); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := h.handler.HandleUpdate(ctx, update); err != nil {
		h.logger.Error("telegram update failed", "error", err, "update_id", update.UpdateID)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
