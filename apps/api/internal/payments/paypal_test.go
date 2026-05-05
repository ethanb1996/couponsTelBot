package payments

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestApprovalURLFromPayPalLinksPrefersApprove(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/self", Rel: "self", Method: "GET"},
		{Href: "https://example.com/approve", Rel: "approve", Method: "GET"},
		{Href: "https://example.com/payer-action", Rel: "payer-action", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/approve" {
		t.Fatalf("expected approve URL, got %q", got)
	}
}

func TestApprovalURLFromPayPalLinksFallsBackToPayerAction(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/self", Rel: "self", Method: "GET"},
		{Href: "https://example.com/payer-action", Rel: "payer-action", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/payer-action" {
		t.Fatalf("expected payer-action URL, got %q", got)
	}
}

func TestApprovalURLFromPayPalLinksFallsBackToGetLink(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/redirect", Rel: "redirect", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/redirect" {
		t.Fatalf("expected GET fallback URL, got %q", got)
	}
}

func TestVerifyAndParseWebhookRejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	client := newPayPalClient(payPalClientOptions{
		BaseURL:   "https://api-m.sandbox.paypal.com",
		ClientID:  "client-id",
		Secret:    "secret",
		WebhookID: "webhook-id",
	})

	_, err := client.VerifyAndParseWebhook(context.Background(), http.Header{}, []byte(`{}`))
	if err == nil {
		t.Fatal("expected webhook verification to fail")
	}
	if !errors.Is(err, ErrWebhookUnauthorized) {
		t.Fatalf("expected unauthorized webhook error, got %v", err)
	}
	if !strings.Contains(err.Error(), "PAYPAL-TRANSMISSION-ID") {
		t.Fatalf("expected missing header details, got %v", err)
	}
}

func TestVerifyAndParseWebhookRejectsUnsuccessfulVerificationStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access-token"}`))
		case "/v1/notifications/verify-webhook-signature":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"verification_status":"FAILURE"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newPayPalClient(payPalClientOptions{
		BaseURL:    server.URL,
		ClientID:   "client-id",
		Secret:     "secret",
		WebhookID:  "webhook-id",
		HTTPClient: server.Client(),
	})

	headers := http.Header{}
	headers.Set("PAYPAL-AUTH-ALGO", "SHA256withRSA")
	headers.Set("PAYPAL-CERT-URL", "https://api-m.sandbox.paypal.com/certs/example")
	headers.Set("PAYPAL-TRANSMISSION-ID", "abc123")
	headers.Set("PAYPAL-TRANSMISSION-SIG", "signature")
	headers.Set("PAYPAL-TRANSMISSION-TIME", "2026-04-27T20:23:26Z")

	_, err := client.VerifyAndParseWebhook(context.Background(), headers, []byte(`{"id":"WH-1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`))
	if err == nil {
		t.Fatal("expected webhook verification to fail")
	}
	if !errors.Is(err, ErrWebhookUnauthorized) {
		t.Fatalf("expected unauthorized webhook error, got %v", err)
	}
	if !strings.Contains(err.Error(), "verification_status=FAILURE") {
		t.Fatalf("expected verification status in error, got %v", err)
	}
}

func TestPayPalClientCachesAccessTokensUntilExpiry(t *testing.T) {
	t.Parallel()

	var tokenCalls int32
	var orderCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			atomic.AddInt32(&tokenCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"cached-token","expires_in":3600}`))
		case "/v2/checkout/orders":
			atomic.AddInt32(&orderCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ORDER-1","links":[{"href":"https://paypal.example/approve","rel":"approve","method":"GET"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newPayPalClient(payPalClientOptions{
		BaseURL:    server.URL,
		ClientID:   "client-id",
		Secret:     "secret",
		WebhookID:  "webhook-id",
		HTTPClient: server.Client(),
	})

	for i := 0; i < 2; i++ {
		_, err := client.CreateCheckout(context.Background(), payPalCreateCheckoutInput{
			OrderID:      int64(i + 1),
			OrderNumber:  "ORD-1",
			Amount:       1000,
			CurrencyCode: "ILS",
			ReturnURL:    "https://example.com/return",
			CancelURL:    "https://example.com/cancel",
		})
		if err != nil {
			t.Fatalf("expected checkout creation to succeed, got %v", err)
		}
	}

	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("expected one token fetch, got %d", got)
	}
	if got := atomic.LoadInt32(&orderCalls); got != 2 {
		t.Fatalf("expected two order calls, got %d", got)
	}
}

func TestPayPalClientRefreshesTokenAfterUnauthorized(t *testing.T) {
	t.Parallel()

	var tokenCalls int32
	var orderCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			call := atomic.AddInt32(&tokenCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_, _ = w.Write([]byte(`{"access_token":"stale-token","expires_in":3600}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"fresh-token","expires_in":3600}`))
		case "/v2/checkout/orders/ORDER-2":
			call := atomic.AddInt32(&orderCalls, 1)
			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"name":"AUTHORIZATION_ERROR","message":"expired token"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ORDER-2","status":"COMPLETED","purchase_units":[{"amount":{"currency_code":"ILS","value":"10.00"},"payments":{"captures":[{"id":"CAP-2","status":"COMPLETED","amount":{"currency_code":"ILS","value":"10.00"}}]}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newPayPalClient(payPalClientOptions{
		BaseURL:    server.URL,
		ClientID:   "client-id",
		Secret:     "secret",
		WebhookID:  "webhook-id",
		HTTPClient: server.Client(),
	})

	snapshot, err := client.GetOrder(context.Background(), "ORDER-2")
	if err != nil {
		t.Fatalf("expected order lookup to recover after unauthorized, got %v", err)
	}
	if snapshot.OrderID != "ORDER-2" || snapshot.CaptureID != "CAP-2" {
		t.Fatalf("unexpected order snapshot: %+v", snapshot)
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 2 {
		t.Fatalf("expected token refresh after unauthorized, got %d token fetches", got)
	}
	if got := atomic.LoadInt32(&orderCalls); got != 2 {
		t.Fatalf("expected one retry after unauthorized, got %d order calls", got)
	}
}

func TestPayPalClientRefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	var tokenCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			call := atomic.AddInt32(&tokenCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_, _ = w.Write([]byte(`{"access_token":"short-lived","expires_in":1}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"refreshed","expires_in":3600}`))
		case "/v2/checkout/orders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ORDER-3","links":[{"href":"https://paypal.example/approve","rel":"approve","method":"GET"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newPayPalClient(payPalClientOptions{
		BaseURL:    server.URL,
		ClientID:   "client-id",
		Secret:     "secret",
		WebhookID:  "webhook-id",
		HTTPClient: server.Client(),
	})

	_, err := client.CreateCheckout(context.Background(), payPalCreateCheckoutInput{
		OrderID:      1,
		OrderNumber:  "ORD-3",
		Amount:       1000,
		CurrencyCode: "ILS",
		ReturnURL:    "https://example.com/return",
		CancelURL:    "https://example.com/cancel",
	})
	if err != nil {
		t.Fatalf("expected first checkout creation to succeed, got %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	_, err = client.CreateCheckout(context.Background(), payPalCreateCheckoutInput{
		OrderID:      2,
		OrderNumber:  "ORD-4",
		Amount:       1000,
		CurrencyCode: "ILS",
		ReturnURL:    "https://example.com/return",
		CancelURL:    "https://example.com/cancel",
	})
	if err != nil {
		t.Fatalf("expected second checkout creation to succeed, got %v", err)
	}

	if got := atomic.LoadInt32(&tokenCalls); got != 2 {
		t.Fatalf("expected token refresh after expiry, got %d token fetches", got)
	}
}
