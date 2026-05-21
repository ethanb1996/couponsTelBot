package store

import (
	"testing"
	"time"
)

func TestPredefinedCodeDeliveryOutcomeSentStates(t *testing.T) {
	t.Parallel()

	sentAt := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)
	confirmedAt := sentAt.Add(3 * time.Minute)

	tests := []struct {
		name          string
		delivery      MVPDelivery
		wantCode      string
		wantOrder     string
		wantDelivered time.Time
	}{
		{
			name: "sent marks code sent and order coupon sent",
			delivery: MVPDelivery{
				Status: deliveryStatusSent,
				SentAt: &sentAt,
			},
			wantCode:      predefinedCodeStatusSent,
			wantOrder:     mvpOrderStatusCouponSent,
			wantDelivered: sentAt,
		},
		{
			name: "confirmed prefers confirmation timestamp",
			delivery: MVPDelivery{
				Status:      deliveryStatusConfirmed,
				SentAt:      &sentAt,
				ConfirmedAt: &confirmedAt,
			},
			wantCode:      predefinedCodeStatusSent,
			wantOrder:     mvpOrderStatusCouponSent,
			wantDelivered: confirmedAt,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotCode, gotOrder, gotDelivered := predefinedCodeDeliveryOutcome(tt.delivery)
			if gotCode != tt.wantCode {
				t.Fatalf("expected code status %q, got %q", tt.wantCode, gotCode)
			}
			if gotOrder != tt.wantOrder {
				t.Fatalf("expected order status %q, got %q", tt.wantOrder, gotOrder)
			}
			if gotDelivered == nil || !gotDelivered.Equal(tt.wantDelivered) {
				t.Fatalf("expected delivered at %v, got %v", tt.wantDelivered, gotDelivered)
			}
		})
	}
}

func TestPredefinedCodeDeliveryOutcomePendingStatesKeepAssignment(t *testing.T) {
	t.Parallel()

	for _, status := range []string{deliveryStatusPending, deliveryStatusFailed} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			gotCode, gotOrder, gotDelivered := predefinedCodeDeliveryOutcome(MVPDelivery{Status: status})
			if gotCode != predefinedCodeStatusAssigned {
				t.Fatalf("expected code status %q, got %q", predefinedCodeStatusAssigned, gotCode)
			}
			if gotOrder != mvpOrderStatusPaymentVerified {
				t.Fatalf("expected order status %q, got %q", mvpOrderStatusPaymentVerified, gotOrder)
			}
			if gotDelivered != nil {
				t.Fatalf("expected no delivered timestamp, got %v", gotDelivered)
			}
		})
	}
}
