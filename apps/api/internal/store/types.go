package store

import "time"

type User struct {
	ID             int64
	TelegramUserID int64
	TelegramUser   string
	DisplayName    string
	LanguageCode   string
	Status         string
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CouponSource struct {
	ID                int64
	SourceName        string
	SourceType        string
	ContactReference  string
	RightsStatus      string
	VerificationNotes string
	RiskRating        string
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Listing struct {
	ID                      int64
	MerchantName            string
	Title                   string
	Description             string
	CouponValueAmount       int64
	SalePriceAmount         int64
	CurrencyCode            string
	ExpirySummary           string
	TermsSummary            string
	RedemptionInstructions  string
	FinalSaleDisclosureText string
	Status                  string
	CreatedByAdminID        string
	PublishedAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Coupon struct {
	ID                    int64
	ListingID             int64
	SourceID              int64
	MerchantName          string
	CouponTitle           string
	CouponValueAmount     int64
	SalePriceAmount       int64
	CurrencyCode          string
	CouponMaskedDisplay   string
	ExpiryAt              time.Time
	TransferabilityStatus string
	InventoryStatus       string
	RightsVerifiedAt      *time.Time
	RightsVerificationNote string
	AcquiredCostAmount    int64
	AcquiredAt            time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Order struct {
	ID                        int64
	UserID                    int64
	ListingID                 int64
	CouponID                  *int64
	OrderNumber               string
	Status                    string
	CurrencyCode              string
	SalePriceAmount           int64
	ProviderCheckoutReference string
	FinalSaleAcknowledgedAt   *time.Time
	FailureReason             string
	PlacedAt                  *time.Time
	DeliveredAt               *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type Payment struct {
	ID                int64
	OrderID           int64
	ProviderName      string
	ProviderPaymentID string
	ProviderCheckoutID string
	Status            string
	Amount            int64
	CurrencyCode      string
	FailureCode       string
	FailureMessage    string
	CapturedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CouponDelivery struct {
	ID                 int64
	OrderID            int64
	CouponID           int64
	DeliveryChannel    string
	Status             string
	TelegramMessageID  *int64
	DeliveryPayloadHash string
	SentAt             *time.Time
	ConfirmedAt        *time.Time
	FailureReason      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SupportCase struct {
	ID              int64
	UserID          *int64
	OrderID         *int64
	CouponID        *int64
	CaseType        string
	Status          string
	Priority        string
	Summary         string
	ResolutionNote  string
	AssignedAdminID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AdminAction struct {
	ID              int64
	AdminActor      string
	EntityType      string
	EntityID        int64
	ActionType      string
	BeforeStateJSON string
	AfterStateJSON  string
	ReasonText      string
	CreatedAt       time.Time
}
