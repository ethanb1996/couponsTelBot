package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/jackc/pgx/v5"
)

func TestRedemptionScanByTokenGetRendersSuccessHTMLAndNotifies(t *testing.T) {
	scannedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	recorder := &stubRedemptionRecorder{result: store.CouponRedemptionScanResult{
		FirstScan:       true,
		OrderNumber:     "PB-20260609180338",
		MerchantName:    "הרובע י״ב",
		OfferTitle:      "פיצה משפחתית + תוספת",
		BuyerDisplay:    "Buyer One",
		BuyerTelegramID: 1111,
		Redemption: store.CouponRedemption{
			ID:                1,
			OrderID:           501,
			PredefinedCodeID:  7001,
			RedemptionToken:   "token-1",
			Status:            "redeemed",
			MerchantReference: "branch-1",
			ScannerReference:  "scanner-1",
			ScannedAt:         &scannedAt,
		},
	}}
	notifier := &stubRedemptionNotifier{}
	handler := redemptionScanByTokenHandler(recorder, notifier)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/token-1?merchant_reference=branch-1&scanner_reference=scanner-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected html response, got %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "הקופון מומש בהצלחה") || !strings.Contains(body, "PB-20260609180338") {
		t.Fatalf("expected success html with order number, got %q", body)
	}
	if recorder.params.RedemptionToken != "token-1" || recorder.params.MerchantReference != "branch-1" || recorder.params.ScannerReference != "scanner-1" {
		t.Fatalf("unexpected recorder params: %+v", recorder.params)
	}
	if len(notifier.results) != 1 || !notifier.results[0].FirstScan {
		t.Fatalf("expected first-scan notification, got %+v", notifier.results)
	}
}

func TestRedemptionScanByTokenGetRendersRepeatHTMLAndNotifies(t *testing.T) {
	recorder := &stubRedemptionRecorder{result: store.CouponRedemptionScanResult{
		FirstScan:   false,
		OrderNumber: "PB-20260609180338",
		Redemption: store.CouponRedemption{
			RedemptionToken: "token-1",
			Status:          "redeemed",
		},
	}}
	notifier := &stubRedemptionNotifier{}
	handler := redemptionScanByTokenHandler(recorder, notifier)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "הקופון כבר מומש") {
		t.Fatalf("expected already redeemed html, got %q", rec.Body.String())
	}
	if len(notifier.results) != 1 || notifier.results[0].FirstScan {
		t.Fatalf("expected repeat-scan notification, got %+v", notifier.results)
	}
}

func TestRedemptionScanByTokenGetRendersInvalidHTML(t *testing.T) {
	recorder := &stubRedemptionRecorder{err: pgx.ErrNoRows}
	notifier := &stubRedemptionNotifier{}
	handler := redemptionScanByTokenHandler(recorder, notifier)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/bad-token", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "קוד לא תקין") {
		t.Fatalf("expected invalid html, got %q", rec.Body.String())
	}
	if len(notifier.results) != 0 {
		t.Fatalf("did not expect invalid-token notification, got %+v", notifier.results)
	}
}

func TestRedemptionScanPostKeepsJSONResponse(t *testing.T) {
	scannedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	recorder := &stubRedemptionRecorder{result: store.CouponRedemptionScanResult{
		FirstScan:   true,
		OrderNumber: "PB-1",
		Redemption: store.CouponRedemption{
			OrderID:           501,
			PredefinedCodeID:  7001,
			RedemptionToken:   "token-1",
			Status:            "redeemed",
			MerchantReference: "branch-1",
			ScannedAt:         &scannedAt,
		},
	}}
	handler := redemptionScanHandler(recorder, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/scan", strings.NewReader(`{"redemption_token":"token-1","merchant_reference":"branch-1"}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json response, got %q", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json response: %v", err)
	}
	if payload["status"] != "redeemed" || payload["first_scan"] != true || payload["order_number"] != "PB-1" {
		t.Fatalf("unexpected json payload: %+v", payload)
	}
}

type stubRedemptionRecorder struct {
	params store.RecordCouponRedemptionScanParams
	result store.CouponRedemptionScanResult
	err    error
}

func (s *stubRedemptionRecorder) RecordCouponRedemptionScan(ctx context.Context, params store.RecordCouponRedemptionScanParams) (store.CouponRedemptionScanResult, error) {
	s.params = params
	if s.err != nil {
		return store.CouponRedemptionScanResult{}, s.err
	}
	return s.result, nil
}

type stubRedemptionNotifier struct {
	results []store.CouponRedemptionScanResult
	err     error
}

func (s *stubRedemptionNotifier) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionScanResult) error {
	s.results = append(s.results, result)
	if s.err != nil {
		return s.err
	}
	return nil
}

func TestRedemptionNotifierErrorsDoNotFailScanResponse(t *testing.T) {
	recorder := &stubRedemptionRecorder{result: store.CouponRedemptionScanResult{
		FirstScan:  true,
		Redemption: store.CouponRedemption{RedemptionToken: "token-1", Status: "redeemed"},
	}}
	notifier := &stubRedemptionNotifier{err: errors.New("telegram down")}
	handler := redemptionScanByTokenHandler(recorder, notifier)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected notifier failure not to fail scan, got %d: %s", rec.Code, rec.Body.String())
	}
}
