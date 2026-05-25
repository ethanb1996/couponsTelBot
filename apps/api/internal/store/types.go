package store

import "time"

type User struct {
	ID               int64
	TelegramUserID   int64
	TelegramUsername string
	DisplayName      string
	LanguageCode     string
	Status           string
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
	ResellPriceAmount       int64
	CurrencyCode            string
	ExpirySummary           string
	TermsSummary            string
	RedemptionInstructions  string
	FinalSaleDisclosureText string
	PhotoKey                string
	ExternalImportKey       string
	Status                  string
	CreatedByAdminID        string
	PublishedAt             *time.Time
	AvailableInventoryCount int64
	NextCouponExpiryAt      *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Coupon struct {
	ID                     int64
	ListingID              int64
	SourceID               int64
	MerchantName           string
	CouponTitle            string
	CouponValueAmount      int64
	SalePriceAmount        int64
	CurrencyCode           string
	CouponCodeCiphertext   []byte
	CouponCodeNonce        []byte
	CouponMaskedDisplay    string
	ExpiryAt               time.Time
	TransferabilityStatus  string
	InventoryStatus        string
	RightsVerifiedAt       *time.Time
	RightsVerificationNote string
	AcquiredCostAmount     int64
	AcquiredAt             time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
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
	ID                 int64
	OrderID            int64
	ProviderName       string
	ProviderPaymentID  string
	ProviderCheckoutID string
	Status             string
	Amount             int64
	CurrencyCode       string
	FailureCode        string
	FailureMessage     string
	CapturedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CouponDelivery struct {
	ID                  int64
	OrderID             int64
	CouponID            int64
	DeliveryChannel     string
	Status              string
	TelegramMessageID   *int64
	DeliveryPayloadHash string
	SentAt              *time.Time
	ConfirmedAt         *time.Time
	FailureReason       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FulfillmentJob struct {
	ID                        int64
	OrderID                   int64
	ProviderCheckoutReference string
	Status                    string
	AttemptCount              int
	NextAttemptAt             time.Time
	LastStep                  string
	LastError                 string
	LockedAt                  *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type MerchantPartner struct {
	ID                     int64
	BusinessName           string
	ContactReference       string
	Status                 string
	ApprovalNotes          string
	MerchantDisclosureText string
	SupportContact         string
	DefaultPaymentLink     string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Offer struct {
	ID                     int64
	MerchantPartnerID      int64
	MerchantName           string
	Title                  string
	Description            string
	PriceAmount            int64
	CurrencyCode           string
	PaymentLink            string
	MerchantDisclosureText string
	RedemptionTerms        string
	SupportContact         string
	Status                 string
	PublishedAt            *time.Time
	AvailableCodeCount     int64
	NextCodeExpiryAt       *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type PredefinedCode struct {
	ID                   int64
	OfferID              int64
	MerchantPartnerID    int64
	CodeEncrypted        []byte
	CodeMaskedDisplay    string
	Status               string
	ExpiryAt             *time.Time
	IssuedBatchReference string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type MVPOrder struct {
	ID                 int64
	UserID             int64
	OfferID            int64
	PredefinedCodeID   *int64
	OrderNumber        string
	Status             string
	CurrencyCode       string
	PriceAmount        int64
	PayBoxPaymentLink  string
	PlacedAt           *time.Time
	VerifiedAt         *time.Time
	DeliveredAt        *time.Time
	FailureReason      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	OfferTitle         string
	MerchantName       string
	RedemptionTerms    string
	SupportContact     string
	AvailableCodeCount int64
}

type ManualPaymentClaim struct {
	ID                         int64
	OrderID                    int64
	PayerUsername              string
	ClaimedAmount              int64
	PaymentScreenshotFileID    string
	PaymentScreenshotUniqueID  string
	PaymentScreenshotMessageID *int64
	PaymentScreenshotCaption   string
	SubmittedAt                time.Time
	ReviewStatus               string
	ReviewedBy                 string
	ReviewedAt                 *time.Time
	ReviewNote                 string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	OrderNumber                string
	BuyerUserID                int64
	OfferTitle                 string
	MerchantName               string
	BuyerDisplay               string
	BuyerTelegramID            int64
}

type CouponRedemption struct {
	ID                int64
	OrderID           int64
	PredefinedCodeID  int64
	RedemptionToken   string
	Status            string
	MerchantReference string
	ScannerReference  string
	ScanMetadataJSON  string
	ScannedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
type FulfillmentPreparation struct {
	Order    Order
	Coupon   *Coupon
	User     *User
	Delivery *CouponDelivery
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

type RecordAdminActionParams struct {
	AdminActor      string
	EntityType      string
	EntityID        int64
	ActionType      string
	BeforeStateJSON string
	AfterStateJSON  string
	ReasonText      string
}

type AdminActionFilter struct {
	EntityType string
	EntityID   *int64
	Limit      int
}

type InventorySweepResult struct {
	ExpiredCoupons  int64
	ExpiredListings int64
	SoldOutListings int64
}

type PaidUndeliveredOrderAlert struct {
	OrderID                   int64
	OrderNumber               string
	UserID                    int64
	UserDisplayName           string
	ListingID                 int64
	ListingTitle              string
	CouponID                  *int64
	Status                    string
	ProviderCheckoutReference string
	PaymentStatus             string
	PaymentCapturedAt         *time.Time
	DeliveryStatus            string
	DeliveryFailureReason     string
	LastPaidAt                time.Time
}

type PendingPaymentReconciliationCandidate struct {
	OrderID                   int64
	OrderNumber               string
	Status                    string
	ProviderCheckoutReference string
	PlacedAt                  *time.Time
	UpdatedAt                 time.Time
}

type ReleasedCheckoutHold struct {
	OrderID        int64
	OrderNumber    string
	UserID         int64
	TelegramUserID int64
	ListingID      int64
}

type CreateCouponSourceParams struct {
	SourceName        string
	SourceType        string
	ContactReference  string
	RightsStatus      string
	VerificationNotes string
	RiskRating        string
	IsActive          *bool
}

type CreateMerchantPartnerParams struct {
	BusinessName           string
	ContactReference       string
	Status                 string
	ApprovalNotes          string
	MerchantDisclosureText string
	SupportContact         string
	DefaultPaymentLink     string
}

type CreateOfferParams struct {
	MerchantPartnerID      int64
	MerchantName           string
	Title                  string
	Description            string
	PriceAmount            int64
	CurrencyCode           string
	PaymentLink            string
	MerchantDisclosureText string
	RedemptionTerms        string
	SupportContact         string
	Status                 string
	PublishedAt            *time.Time
}

type PredefinedCodeInput struct {
	CodeEncrypted        []byte
	CodeMaskedDisplay    string
	ExpiryAt             *time.Time
	IssuedBatchReference string
}

type IngestPredefinedCodesParams struct {
	OfferID int64
	Codes   []PredefinedCodeInput
}

type CreateAwaitingPaymentOrderParams struct {
	UserID      int64
	OfferID     int64
	OrderNumber string
}

type SubmitManualPaymentClaimParams struct {
	OrderID                    int64
	PayerUsername              string
	ClaimedAmount              int64
	PaymentScreenshotFileID    string
	PaymentScreenshotUniqueID  string
	PaymentScreenshotMessageID *int64
	PaymentScreenshotCaption   string
}

type ReviewManualPaymentClaimParams struct {
	ClaimID    int64
	ReviewedBy string
	ReviewNote string
}

type ApproveManualPaymentClaimResult struct {
	Order MVPOrder
	Claim ManualPaymentClaim
	Code  *PredefinedCode
}

type CreateCouponRedemptionParams struct {
	OrderID          int64
	PredefinedCodeID int64
	RedemptionToken  string
}

type RecordCouponRedemptionScanParams struct {
	RedemptionToken   string
	MerchantReference string
	ScannerReference  string
	ScanMetadataJSON  string
}
type CreateListingParams struct {
	MerchantName            string
	Title                   string
	Description             string
	CouponValueAmount       int64
	SalePriceAmount         int64
	ResellPriceAmount       int64
	CurrencyCode            string
	ExpirySummary           string
	TermsSummary            string
	RedemptionInstructions  string
	FinalSaleDisclosureText string
	Status                  string
	CreatedByAdminID        string
	PublishedAt             *time.Time
}

type CouponInventoryInput struct {
	SourceID               int64
	MerchantName           string
	CouponTitle            string
	CouponValueAmount      int64
	SalePriceAmount        int64
	CurrencyCode           string
	CouponCodeCiphertext   []byte
	CouponCodeNonce        []byte
	CouponMaskedDisplay    string
	ExpiryAt               time.Time
	TransferabilityStatus  string
	RightsVerifiedAt       *time.Time
	RightsVerificationNote string
	AcquiredCostAmount     int64
	AcquiredAt             *time.Time
}

type IngestCouponsParams struct {
	ListingID int64
	Coupons   []CouponInventoryInput
}

type CreateDraftOrderParams struct {
	UserID                  int64
	ListingID               int64
	OrderNumber             string
	FinalSaleAcknowledgedAt time.Time
}

type MarkOrderPendingPaymentParams struct {
	OrderID                   int64
	ProviderCheckoutReference string
}

type ReleaseCheckoutReservationParams struct {
	OrderID       int64
	OrderStatus   string
	FailureReason string
}

type RecordPaymentEventParams struct {
	OrderID            int64
	ProviderName       string
	ProviderPaymentID  string
	ProviderCheckoutID string
	Status             string
	Amount             int64
	CurrencyCode       string
	FailureCode        string
	FailureMessage     string
	CapturedAt         *time.Time
}

type RecordDeliveryEventParams struct {
	OrderID             int64
	CouponID            int64
	DeliveryChannel     string
	Status              string
	TelegramMessageID   *int64
	DeliveryPayloadHash string
	SentAt              *time.Time
	ConfirmedAt         *time.Time
	FailureReason       string
}

type CreateSupportCaseParams struct {
	UserID          *int64
	OrderID         *int64
	CouponID        *int64
	CaseType        string
	Status          string
	Priority        string
	Summary         string
	ResolutionNote  string
	AssignedAdminID string
}

type EnsureSupportCaseParams struct {
	UserID          *int64
	OrderID         *int64
	CouponID        *int64
	CaseType        string
	Priority        string
	Summary         string
	AssignedAdminID string
}

type EnqueueFulfillmentJobParams struct {
	OrderID                   int64
	ProviderCheckoutReference string
	LastStep                  string
}

type RescheduleFulfillmentJobParams struct {
	JobID         int64
	NextAttemptAt time.Time
	LastStep      string
	LastError     string
}

type FailFulfillmentJobParams struct {
	JobID     int64
	LastStep  string
	LastError string
}

type SucceedFulfillmentJobParams struct {
	JobID    int64
	LastStep string
}
