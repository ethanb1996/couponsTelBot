package payments

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubWebhookProcessor struct {
	err error
}

func (s stubWebhookProcessor) HandleWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	return s.err
}

func TestWebhookHandlerAcceptsExpectedProvider(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "paypal", stubWebhookProcessor{})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/paypal", http.NoBody)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestWebhookHandlerRejectsNestedProviderPath(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "paypal", stubWebhookProcessor{})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/paypal/extra", http.NoBody)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestWebhookHandlerRejectsUnauthorizedWebhook(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "paypal", stubWebhookProcessor{
		err: ErrWebhookUnauthorized,
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/paypal", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestWebhookHandlerRejectsUnsupportedProvider(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "paypal", stubWebhookProcessor{
		err: ErrUnsupportedProvider,
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/paypal", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestWebhookHandlerRejectsBadWebhookPayloads(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "paypal", stubWebhookProcessor{
		err: errors.New("bad payload"),
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/paypal", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
