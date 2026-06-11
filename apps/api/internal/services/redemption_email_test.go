package services

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestNewSMTPRedemptionNotifierDisabledWithoutHost(t *testing.T) {
	notifier, err := NewSMTPRedemptionNotifier(SMTPRedemptionNotifierOptions{})
	if err != nil {
		t.Fatalf("expected disabled notifier without error, got %v", err)
	}
	if notifier != nil {
		t.Fatalf("expected nil notifier when SMTP host is empty, got %#v", notifier)
	}
}

func TestNewSMTPRedemptionNotifierRequiresValidFrom(t *testing.T) {
	_, err := NewSMTPRedemptionNotifier(SMTPRedemptionNotifierOptions{Host: "smtp.example"})
	if err == nil || !strings.Contains(err.Error(), "smtp from is required") {
		t.Fatalf("expected missing from error, got %v", err)
	}

	_, err = NewSMTPRedemptionNotifier(SMTPRedemptionNotifierOptions{Host: "smtp.example", From: "not-email"})
	if err == nil || !strings.Contains(err.Error(), "valid email") {
		t.Fatalf("expected invalid from error, got %v", err)
	}
}

func TestRedemptionMerchantEmail(t *testing.T) {
	email, ok := redemptionMerchantEmail(store.CouponRedemptionScanResult{MerchantContact: "Pizza Shop <merchant@example.com>"})
	if !ok || email != "merchant@example.com" {
		t.Fatalf("expected parsed merchant email, got %q ok=%v", email, ok)
	}

	if _, ok := redemptionMerchantEmail(store.CouponRedemptionScanResult{MerchantContact: "@merchant"}); ok {
		t.Fatal("did not expect non-email merchant contact to be accepted")
	}
}

func TestBuildRedemptionEmailMessage(t *testing.T) {
	scannedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	message, err := buildRedemptionEmailMessage("noreply@example.com", "merchant@example.com", store.CouponRedemptionScanResult{
		OrderNumber:     "PB-20260609180338",
		MerchantName:    "הרובע י״ב",
		OfferTitle:      "פיצה משפחתית + תוספת",
		BuyerDisplay:    "Buyer One",
		BuyerTelegramID: 1111,
		Redemption: store.CouponRedemption{
			Status:    "redeemed",
			ScannedAt: &scannedAt,
		},
	})
	if err != nil {
		t.Fatalf("expected email message build to succeed: %v", err)
	}
	text := string(message)
	for _, want := range []string{
		"Subject: KuponFast coupon redeemed - PB-20260609180338",
		"Order: PB-20260609180338",
		"Merchant: הרובע י״ב",
		"Coupon: פיצה משפחתית + תוספת",
		"Buyer Telegram ID: 1111",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected email message to contain %q, got %q", want, text)
		}
	}
}
