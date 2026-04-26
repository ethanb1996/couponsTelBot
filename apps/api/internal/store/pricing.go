package store

// EffectiveListingPriceAmount returns the customer-facing listing price.
func EffectiveListingPriceAmount(listing Listing) int64 {
	if listing.ResellPriceAmount > 0 {
		return listing.ResellPriceAmount
	}
	return listing.SalePriceAmount
}
