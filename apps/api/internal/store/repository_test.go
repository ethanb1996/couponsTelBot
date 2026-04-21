package store

import (
	"errors"
	"testing"
	"time"
)

func TestOrderStatusForPayment(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		paymentStatus string
		want          string
		wantErr       error
	}{
		{name: "pending keeps order pending payment", paymentStatus: paymentStatusPending, want: orderStatusPendingPayment},
		{name: "authorized marks order paid", paymentStatus: paymentStatusAuthorized, want: orderStatusPaid},
		{name: "captured marks order paid", paymentStatus: paymentStatusCaptured, want: orderStatusPaid},
		{name: "failed marks order failed", paymentStatus: paymentStatusFailed, want: orderStatusFailed},
		{name: "cancelled marks order failed", paymentStatus: paymentStatusCancelled, want: orderStatusFailed},
		{name: "chargeback marks order disputed", paymentStatus: paymentStatusChargeback, want: orderStatusDisputed},
		{name: "unsupported payment status errors", paymentStatus: "mystery", wantErr: ErrInvalidArgument},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := orderStatusForPayment(tc.paymentStatus)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDeliveryOutcomeDeliveredStates(t *testing.T) {
	t.Parallel()

	sentAt := time.Date(2026, time.April, 21, 12, 0, 0, 0, time.UTC)
	confirmedAt := sentAt.Add(5 * time.Minute)

	testCases := []struct {
		name              string
		delivery          CouponDelivery
		wantInventory     string
		wantOrderStatus   string
		wantDeliveredTime time.Time
	}{
		{
			name: "sent delivery marks coupon and order delivered",
			delivery: CouponDelivery{
				Status: deliveryStatusSent,
				SentAt: &sentAt,
			},
			wantInventory:     inventoryStatusDelivered,
			wantOrderStatus:   orderStatusDelivered,
			wantDeliveredTime: sentAt,
		},
		{
			name: "confirmed delivery prefers confirmation timestamp",
			delivery: CouponDelivery{
				Status:      deliveryStatusConfirmed,
				SentAt:      &sentAt,
				ConfirmedAt: &confirmedAt,
			},
			wantInventory:     inventoryStatusDelivered,
			wantOrderStatus:   orderStatusDelivered,
			wantDeliveredTime: confirmedAt,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotInventory, gotOrderStatus, gotDeliveredAt := deliveryOutcome(tc.delivery)

			if gotInventory != tc.wantInventory {
				t.Fatalf("expected inventory status %q, got %q", tc.wantInventory, gotInventory)
			}
			if gotOrderStatus != tc.wantOrderStatus {
				t.Fatalf("expected order status %q, got %q", tc.wantOrderStatus, gotOrderStatus)
			}
			if gotDeliveredAt == nil || !gotDeliveredAt.Equal(tc.wantDeliveredTime) {
				t.Fatalf("expected delivered timestamp %v, got %v", tc.wantDeliveredTime, gotDeliveredAt)
			}
		})
	}
}

func TestDeliveryOutcomePendingStatesKeepAssignment(t *testing.T) {
	t.Parallel()

	testCases := []string{
		deliveryStatusPending,
		deliveryStatusFailed,
	}

	for _, status := range testCases {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			gotInventory, gotOrderStatus, gotDeliveredAt := deliveryOutcome(CouponDelivery{Status: status})

			if gotInventory != inventoryStatusAssigned {
				t.Fatalf("expected inventory status %q, got %q", inventoryStatusAssigned, gotInventory)
			}
			if gotOrderStatus != orderStatusDeliveryPending {
				t.Fatalf("expected order status %q, got %q", orderStatusDeliveryPending, gotOrderStatus)
			}
			if gotDeliveredAt != nil {
				t.Fatalf("expected nil delivered timestamp, got %v", gotDeliveredAt)
			}
		})
	}
}
