package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/resend/resend-go/v3"
)

func TestNewResendRedemptionNotifierDisabledWithoutConfig(t *testing.T) {
	notifier, err := NewResendRedemptionNotifier(ResendRedemptionNotifierOptions{})
	if err != nil {
		t.Fatalf("expected disabled notifier without error, got %v", err)
	}
	if notifier != nil {
		t.Fatalf("expected nil notifier when Resend config is empty, got %#v", notifier)
	}
}

func TestNewResendRedemptionNotifierRequiresAPIKeyAndValidFrom(t *testing.T) {
	_, err := NewResendRedemptionNotifier(ResendRedemptionNotifierOptions{From: "noreply@example.com"})
	if err == nil || !strings.Contains(err.Error(), "api key is required") {
		t.Fatalf("expected missing api key error, got %v", err)
	}

	_, err = NewResendRedemptionNotifier(ResendRedemptionNotifierOptions{APIKey: resendSampleAPIKey, From: "noreply@example.com"})
	if err == nil || !strings.Contains(err.Error(), "replace re_xxxxxxxxx") {
		t.Fatalf("expected sample api key error, got %v", err)
	}

	_, err = NewResendRedemptionNotifier(ResendRedemptionNotifierOptions{APIKey: "re_real"})
	if err == nil || !strings.Contains(err.Error(), "from is required") {
		t.Fatalf("expected missing from error, got %v", err)
	}

	_, err = NewResendRedemptionNotifier(ResendRedemptionNotifierOptions{APIKey: "re_real", From: "not-email"})
	if err == nil || !strings.Contains(err.Error(), "valid email") {
		t.Fatalf("expected invalid from error, got %v", err)
	}
}

func TestRedemptionMerchantEmail(t *testing.T) {
	email, ok := redemptionMerchantEmail(store.CouponRedemptionConfirmResult{
		Preview: store.CouponRedemptionPreview{MerchantContact: "Pizza Shop <merchant@example.com>"},
	})
	if !ok || email != "merchant@example.com" {
		t.Fatalf("expected parsed merchant email, got %q ok=%v", email, ok)
	}

	if _, ok := redemptionMerchantEmail(store.CouponRedemptionConfirmResult{
		Preview: store.CouponRedemptionPreview{MerchantContact: "@merchant"},
	}); ok {
		t.Fatal("did not expect non-email merchant contact to be accepted")
	}
}

func TestBuildRedemptionEmailRequest(t *testing.T) {
	redeemedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	request, err := buildRedemptionEmailRequest("noreply@example.com", "merchant@example.com", store.CouponRedemptionConfirmResult{
		FirstRedeem: true,
		Preview: store.CouponRedemptionPreview{
			OrderNumber:     "PB-20260609180338",
			MerchantName:    "׳”׳¨׳•׳‘׳¢ ׳™\"׳‘",
			OfferTitle:      "׳₪׳™׳¦׳” ׳׳©׳₪׳—׳×׳™׳× + ׳×׳•׳¡׳₪׳×",
			BuyerDisplay:    "Buyer One",
			BuyerTelegramID: 1111,
			Redemption: store.CouponRedemption{
				Status:     "redeemed",
				RedeemedAt: &redeemedAt,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected email request build to succeed: %v", err)
	}
	if request.From != "noreply@example.com" || len(request.To) != 1 || request.To[0] != "merchant@example.com" {
		t.Fatalf("unexpected request destinations: %#v", request)
	}
	if request.Subject != "KuponFast coupon redeemed - PB-20260609180338" {
		t.Fatalf("unexpected subject: %q", request.Subject)
	}

	text := request.Text + "\n" + request.Html
	for _, want := range []string{
		"Order: PB-20260609180338",
		"Merchant: ׳”׳¨׳•׳‘׳¢ ׳™\"׳‘",
		"Coupon: ׳₪׳™׳¦׳” ׳׳©׳₪׳—׳×׳™׳× + ׳×׳•׳¡׳₪׳×",
		"Buyer Telegram ID: 1111",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected email request to contain %q, got %q", want, text)
		}
	}
}

func TestResendRedemptionNotifierSendsFirstRedeemOnly(t *testing.T) {
	sender := &stubResendSender{}
	notifier := &ResendRedemptionNotifier{from: "noreply@example.com", sender: sender}
	result := store.CouponRedemptionConfirmResult{
		FirstRedeem: true,
		Preview: store.CouponRedemptionPreview{
			MerchantContact: "Pizza Shop <merchant@example.com>",
			OrderNumber:     "PB-1",
		},
	}

	if err := notifier.NotifyCouponRedeemed(context.Background(), result); err != nil {
		t.Fatalf("expected notify to succeed: %v", err)
	}
	if sender.calls != 1 || sender.lastRequest == nil {
		t.Fatalf("expected one send, got calls=%d request=%#v", sender.calls, sender.lastRequest)
	}

	result.FirstRedeem = false
	if err := notifier.NotifyCouponRedeemed(context.Background(), result); err != nil {
		t.Fatalf("expected repeat redeem to be ignored without error: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("expected repeat redeem not to send, got %d calls", sender.calls)
	}
}

func TestResendRedemptionNotifierReturnsSenderError(t *testing.T) {
	wantErr := errors.New("resend down")
	sender := &stubResendSender{err: wantErr}
	notifier := &ResendRedemptionNotifier{from: "noreply@example.com", sender: sender}

	err := notifier.NotifyCouponRedeemed(context.Background(), store.CouponRedemptionConfirmResult{
		FirstRedeem: true,
		Preview: store.CouponRedemptionPreview{
			MerchantContact: "merchant@example.com",
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected sender error, got %v", err)
	}
}

type stubResendSender struct {
	calls       int
	lastRequest *resend.SendEmailRequest
	err         error
}

func (s *stubResendSender) SendWithContext(ctx context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	s.calls++
	s.lastRequest = params
	if s.err != nil {
		return nil, s.err
	}
	return &resend.SendEmailResponse{Id: "email_123"}, nil
}
