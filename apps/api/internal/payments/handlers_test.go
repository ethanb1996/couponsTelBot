package payments

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookHandlerAcceptsExpectedProvider(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "mockpay", "secret")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/mockpay", http.NoBody)
	req.Header.Set("X-Payment-Webhook-Secret", "secret")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestWebhookHandlerRejectsNestedProviderPath(t *testing.T) {
	handler := NewWebhookHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), "mockpay", "secret")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments/mockpay/extra", http.NoBody)
	req.Header.Set("X-Payment-Webhook-Secret", "secret")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
