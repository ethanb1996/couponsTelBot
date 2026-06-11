package store

import "testing"

func TestIsFirstCouponRedemptionScan(t *testing.T) {
	t.Parallel()

	if !isFirstCouponRedemptionScan(CouponRedemption{Status: couponRedemptionStatusIssued}) {
		t.Fatal("expected issued redemption to be treated as first scan")
	}
	if isFirstCouponRedemptionScan(CouponRedemption{Status: couponRedemptionStatusRedeemed}) {
		t.Fatal("did not expect redeemed redemption to be treated as first scan")
	}
}
