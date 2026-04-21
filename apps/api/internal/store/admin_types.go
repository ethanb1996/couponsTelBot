package store

import "time"

type ListingInventorySummary struct {
	Listing
	TotalCoupons     int64
	AvailableCoupons int64
	AssignedCoupons  int64
	DeliveredCoupons int64
	VoidedCoupons    int64
	DisputedCoupons  int64
}

type CouponListFilter struct {
	ListingID *int64
	Status    string
	Limit     int
}

type CouponAdminSummary struct {
	Coupon
	ListingTitle               string
	SourceName                 string
	AssignedOrderID            *int64
	AssignedOrderStatus        string
	AssignedUserDisplayName    string
	AssignedUserTelegramUserID *int64
}

type OrderAdminSummary struct {
	Order
	TelegramUserID              int64
	TelegramUsername            string
	UserDisplayName             string
	PaymentStatus               string
	PaymentProviderName         string
	PaymentCapturedAt           *time.Time
	DeliveryStatus              string
	DeliveryChannel             string
	DeliverySentAt              *time.Time
	AssignedCouponMaskedDisplay string
	AssignedCouponExpiryAt      *time.Time
}

type SupportCaseSummary struct {
	SupportCase
	OrderStatus     string
	CouponStatus    string
	ListingTitle    string
	UserDisplayName string
}
