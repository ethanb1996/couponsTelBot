package telegram

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeHandler struct{ called bool }

func (f *fakeHandler) HandleUpdate(_ context.Context, _ tgbotapi.Update) error {
	f.called = true
	return nil
}

func TestWebhookRequiresSecretAndProcessesUpdate(t *testing.T) {
	fake := &fakeHandler{}
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "secret", fake)

	unauthorized := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"update_id":1}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, unauthorized)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", w.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"update_id":2}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !fake.called {
		t.Fatalf("expected processed update, code=%d called=%v", w.Code, fake.called)
	}
}
