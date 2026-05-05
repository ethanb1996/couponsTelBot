package store

import "testing"

func TestFulfillmentStateError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		orderStatus          string
		hasSuccessfulPayment bool
		wantErr              error
	}{
		{
			name:                 "pending payment without successful payment requests capture",
			orderStatus:          orderStatusPendingPayment,
			hasSuccessfulPayment: false,
			wantErr:              ErrPaymentRequired,
		},
		{
			name:                 "paid without successful payment still requests capture",
			orderStatus:          orderStatusPaid,
			hasSuccessfulPayment: false,
			wantErr:              ErrPaymentRequired,
		},
		{
			name:                 "failed order is not deliverable",
			orderStatus:          orderStatusFailed,
			hasSuccessfulPayment: false,
			wantErr:              ErrOrderNotReadyForCoupon,
		},
		{
			name:                 "paid with successful payment can continue",
			orderStatus:          orderStatusPaid,
			hasSuccessfulPayment: true,
			wantErr:              nil,
		},
		{
			name:                 "pending payment with successful payment is still not ready",
			orderStatus:          orderStatusPendingPayment,
			hasSuccessfulPayment: true,
			wantErr:              ErrOrderNotReadyForCoupon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := fulfillmentStateError(tt.orderStatus, tt.hasSuccessfulPayment)
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
