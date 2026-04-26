package store

import "testing"

func TestEffectiveListingPriceAmountPrefersResellPrice(t *testing.T) {
	t.Parallel()

	listing := Listing{
		SalePriceAmount:   7400,
		ResellPriceAmount: 10204,
	}

	if got := EffectiveListingPriceAmount(listing); got != 10204 {
		t.Fatalf("expected resale price 10204, got %d", got)
	}
}

func TestEffectiveListingPriceAmountFallsBackToSalePrice(t *testing.T) {
	t.Parallel()

	listing := Listing{
		SalePriceAmount:   7400,
		ResellPriceAmount: 0,
	}

	if got := EffectiveListingPriceAmount(listing); got != 7400 {
		t.Fatalf("expected sale price 7400, got %d", got)
	}
}
