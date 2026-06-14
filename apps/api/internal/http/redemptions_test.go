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

func TestRedemptionScanByTokenGetRendersPreviewHTMLWithoutRedeeming(t *testing.T) {
	repo := &stubRedemptionStore{preview: validPreview("issued")}
	handler := redemptionScanByTokenHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"בדיקת קופון", "PB-20260609180338", "ממש קופון"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected preview html to contain %q, got %q", want, body)
		}
	}
	if repo.previewToken != "token-1" {
		t.Fatalf("unexpected preview token %q", repo.previewToken)
	}
	if repo.confirmCalled {
		t.Fatal("GET preview must not confirm redemption")
	}
}

func TestRedemptionScanByTokenGetRendersAlreadyRedeemedHTML(t *testing.T) {
	redeemedAt := time.Date(2026, 6, 11, 18, 15, 0, 0, time.UTC)
	preview := validPreview("redeemed")
	preview.Redemption.RedeemedAt = &redeemedAt
	repo := &stubRedemptionStore{preview: preview}
	handler := redemptionScanByTokenHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "הקופון כבר מומש") || strings.Contains(body, "ממש קופון") {
		t.Fatalf("expected already-redeemed html without redeem button, got %q", body)
	}
}

func TestRedemptionScanByTokenGetRendersInvalidHTML(t *testing.T) {
	repo := &stubRedemptionStore{previewErr: pgx.ErrNoRows}
	handler := redemptionScanByTokenHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/redemptions/scan/bad-token", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "קוד לא תקין") {
		t.Fatalf("expected invalid html, got %q", rec.Body.String())
	}
}

func TestRedemptionRedeemByTokenPostRendersSuccessAndNotifies(t *testing.T) {
	result := validConfirmResult(true)
	repo := &stubRedemptionStore{confirm: result}
	notifier := &stubRedemptionNotifier{}
	handler := redemptionRedeemByTokenHandler(repo, notifier, -100123)

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/redeem/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "קופון מומש בהצלחה") || !strings.Contains(body, "PB-20260609180338") {
		t.Fatalf("expected success html, got %q", body)
	}
	if repo.confirmParams.RedemptionToken != "token-1" || repo.confirmParams.RestaurantNotificationFallbackChatID != -100123 {
		t.Fatalf("unexpected confirm params: %+v", repo.confirmParams)
	}
	if len(notifier.results) != 1 || !notifier.results[0].FirstRedeem {
		t.Fatalf("expected first redeem notification, got %+v", notifier.results)
	}
}

func TestRedemptionRedeemByTokenPostRendersRepeatHTML(t *testing.T) {
	repo := &stubRedemptionStore{confirm: validConfirmResult(false)}
	handler := redemptionRedeemByTokenHandler(repo, nil, 0)

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/redeem/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "הקופון כבר מומש") {
		t.Fatalf("expected already redeemed html, got %q", rec.Body.String())
	}
}

func TestRedemptionRedeemJSONReturnsConfirmPayload(t *testing.T) {
	repo := &stubRedemptionStore{confirm: validConfirmResult(true)}
	handler := redemptionRedeemHandler(repo, nil, -100123)

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/redeem", strings.NewReader(`{"redemption_token":"token-1","merchant_reference":"branch-1"}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json response: %v", err)
	}
	if payload["status"] != "redeemed" || payload["first_redeem"] != true || payload["order_number"] != "PB-20260609180338" {
		t.Fatalf("unexpected json payload: %+v", payload)
	}
}

func TestRedemptionScanPostNoLongerRedeems(t *testing.T) {
	handler := redemptionScanHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/scan", strings.NewReader(`{"redemption_token":"token-1"}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRedemptionNotifierErrorsDoNotFailRedeemResponse(t *testing.T) {
	repo := &stubRedemptionStore{confirm: validConfirmResult(true)}
	notifier := &stubRedemptionNotifier{err: errors.New("telegram down")}
	handler := redemptionRedeemByTokenHandler(repo, notifier, 0)

	req := httptest.NewRequest(http.MethodPost, "/api/redemptions/redeem/token-1", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected notifier failure not to fail redeem, got %d: %s", rec.Code, rec.Body.String())
	}
}

type stubRedemptionStore struct {
	preview       store.CouponRedemptionPreview
	previewToken  string
	previewErr    error
	confirm       store.CouponRedemptionConfirmResult
	confirmParams store.ConfirmCouponRedemptionParams
	confirmCalled bool
	confirmErr    error
}

func (s *stubRedemptionStore) GetCouponRedemptionPreview(ctx context.Context, token string) (store.CouponRedemptionPreview, error) {
	s.previewToken = token
	if s.previewErr != nil {
		return store.CouponRedemptionPreview{}, s.previewErr
	}
	return s.preview, nil
}

func (s *stubRedemptionStore) ConfirmCouponRedemption(ctx context.Context, params store.ConfirmCouponRedemptionParams) (store.CouponRedemptionConfirmResult, error) {
	s.confirmCalled = true
	s.confirmParams = params
	if s.confirmErr != nil {
		return store.CouponRedemptionConfirmResult{}, s.confirmErr
	}
	return s.confirm, nil
}

type stubRedemptionNotifier struct {
	results []store.CouponRedemptionConfirmResult
	err     error
}

func (s *stubRedemptionNotifier) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionConfirmResult) error {
	s.results = append(s.results, result)
	if s.err != nil {
		return s.err
	}
	return nil
}

func validConfirmResult(first bool) store.CouponRedemptionConfirmResult {
	chatID := int64(-1001234567890)
	preview := validPreview("redeemed")
	redeemedAt := time.Date(2026, 6, 11, 18, 15, 0, 0, time.UTC)
	preview.Redemption.RedeemedAt = &redeemedAt
	return store.CouponRedemptionConfirmResult{
		Preview:                      preview,
		FirstRedeem:                  first,
		RestaurantNotificationChatID: &chatID,
	}
}

func validPreview(status string) store.CouponRedemptionPreview {
	approvedAt := time.Date(2026, 6, 11, 17, 30, 0, 0, time.UTC)
	return store.CouponRedemptionPreview{
		Redemption: store.CouponRedemption{
			ID:               1,
			OrderID:          501,
			PredefinedCodeID: 7001,
			RedemptionToken:  "token-1",
			Status:           status,
		},
		OrderNumber:          "PB-20260609180338",
		MerchantName:         "הרובע י\"ב",
		OfferTitle:           "פיצה משפחתית + תוספת",
		AmountPaid:           5900,
		CurrencyCode:         "ILS",
		PaymentStatusSummary: "אושר במערכת KuponFast",
		ApprovalTime:         &approvedAt,
		BuyerDisplay:         "ישראל ישראלי",
		RedemptionTerms:      "להציג את הקופון בקופה.",
	}
}
