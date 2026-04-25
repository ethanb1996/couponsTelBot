package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeUpdateHandler struct {
	ctxCh    chan context.Context
	updateCh chan tgbotapi.Update
	err      error
}

func (f *fakeUpdateHandler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if f.ctxCh != nil {
		f.ctxCh <- ctx
	}
	if f.updateCh != nil {
		f.updateCh <- update
	}
	return f.err
}

func TestWebhookHandlerRejectsWrongSecret(t *testing.T) {
	handler := NewWebhookHandler(testLogger(), "expected-secret", &fakeUpdateHandler{})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"update_id":1}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestWebhookHandlerProcessesUpdateWithDetachedContext(t *testing.T) {
	ctxCh := make(chan context.Context, 1)
	updateCh := make(chan tgbotapi.Update, 1)
	handler := NewWebhookHandler(testLogger(), "expected-secret", &fakeUpdateHandler{
		ctxCh:    ctxCh,
		updateCh: updateCh,
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"update_id":42}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "expected-secret")
	reqCtx, cancelReq := context.WithCancel(req.Context())
	req = req.WithContext(reqCtx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cancelReq()

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, rec.Code)
	}

	select {
	case update := <-updateCh:
		if update.UpdateID != 42 {
			t.Fatalf("expected update id 42, got %d", update.UpdateID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async update handling")
	}

	select {
	case ctx := <-ctxCh:
		if ctx == reqCtx {
			t.Fatal("expected detached async context, got request context")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected async context to have timeout deadline")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async context")
	}
}

func TestWebhookHandlerRejectsBadJSON(t *testing.T) {
	handler := NewWebhookHandler(testLogger(), "", &fakeUpdateHandler{})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{`))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestWebhookHandlerStillAcceptsWhenUpdateHandlerFails(t *testing.T) {
	handler := NewWebhookHandler(testLogger(), "", &fakeUpdateHandler{err: errors.New("boom")})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{"update_id":7}`))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
