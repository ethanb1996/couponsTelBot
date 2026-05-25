package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

var (
	ErrPayBoxStoreRequired        = errors.New("paybox: store is required")
	ErrPayBoxMessengerRequired    = errors.New("paybox: messenger is required")
	ErrPayBoxCodeRendererRequired = errors.New("paybox: code renderer is required")
	ErrPayBoxQRRendererRequired   = errors.New("paybox: qr renderer is required")
)

type PayBoxStore interface {
	ListActiveOffers(ctx context.Context, limit int) ([]store.Offer, error)
	GetOffer(ctx context.Context, offerID int64) (store.Offer, error)
	CreateAwaitingPaymentOrder(ctx context.Context, params store.CreateAwaitingPaymentOrderParams) (store.MVPOrder, error)
	SubmitManualPaymentClaim(ctx context.Context, params store.SubmitManualPaymentClaimParams) (store.ManualPaymentClaim, error)
	ListPendingManualPaymentClaims(ctx context.Context, limit int) ([]store.ManualPaymentClaim, error)
	ApproveManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ApproveManualPaymentClaimResult, error)
	RejectManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ManualPaymentClaim, store.MVPOrder, error)
	CreateCouponRedemption(ctx context.Context, params store.CreateCouponRedemptionParams) (store.CouponRedemption, error)
	RecordPredefinedCodeDeliveryEvent(ctx context.Context, params store.RecordPredefinedCodeDeliveryEventParams) (store.MVPDelivery, error)
	RecordAdminAction(ctx context.Context, params store.RecordAdminActionParams) (store.AdminAction, error)
	GetUserByID(ctx context.Context, userID int64) (store.User, error)
}

type PayBoxMessenger interface {
	SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error)
	SendPhotoMessage(ctx context.Context, telegramUserID int64, photo []byte, filename string, caption string) (int64, error)
}

type PayBoxAdminNotifier interface {
	NotifyPaymentClaimSubmitted(ctx context.Context, claim store.ManualPaymentClaim) error
	NotifyPaymentClaimApproved(ctx context.Context, result PayBoxApprovalResult) error
	NotifyPaymentClaimRejected(ctx context.Context, claim store.ManualPaymentClaim, order store.MVPOrder) error
	NotifyPaymentClaimNeedsSupport(ctx context.Context, result PayBoxApprovalResult) error
}

type PayBoxCodeRenderer interface {
	RenderPredefinedCode(ctx context.Context, code store.PredefinedCode) (string, error)
}

type PayBoxQRRenderer interface {
	RenderPNG(payload string) ([]byte, error)
}

type OrderNumberGenerator interface {
	NewOrderNumber() string
}

type PayBoxService struct {
	logger            *slog.Logger
	store             PayBoxStore
	messenger         PayBoxMessenger
	adminNotifier     PayBoxAdminNotifier
	codeRenderer      PayBoxCodeRenderer
	qrRenderer        PayBoxQRRenderer
	orderNumberer     OrderNumberGenerator
	supportContact    string
	approvalMessage   string
	redemptionBaseURL string
}

type PayBoxServiceOptions struct {
	Logger            *slog.Logger
	Store             PayBoxStore
	Messenger         PayBoxMessenger
	AdminNotifier     PayBoxAdminNotifier
	CodeRenderer      PayBoxCodeRenderer
	QRRenderer        PayBoxQRRenderer
	OrderNumberer     OrderNumberGenerator
	SupportContact    string
	ApprovalMessage   string
	RedemptionBaseURL string
}

type PayBoxPaymentStart struct {
	OrderID                int64
	OrderNumber            string
	OfferID                int64
	OfferTitle             string
	MerchantName           string
	PriceAmount            int64
	CurrencyCode           string
	PaymentLink            string
	MerchantDisclosureText string
	RedemptionTerms        string
	SupportContact         string
	NextStepMessage        string
}

type PayBoxApprovalResult struct {
	Order        store.MVPOrder
	Claim        store.ManualPaymentClaim
	Code         *store.PredefinedCode
	Redemption   *store.CouponRedemption
	Delivery     *store.MVPDelivery
	SupportState bool
}

type PayBoxPaymentEvidence struct {
	PayerReference              string
	ScreenshotFileID            string
	ScreenshotUniqueID          string
	ScreenshotTelegramMessageID *int64
	ScreenshotCaption           string
}

func NewPayBoxService(options PayBoxServiceOptions) (*PayBoxService, error) {
	if options.Store == nil {
		return nil, ErrPayBoxStoreRequired
	}
	if options.Messenger == nil {
		return nil, ErrPayBoxMessengerRequired
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	orderNumberer := options.OrderNumberer
	if orderNumberer == nil {
		orderNumberer = timeOrderNumberer{}
	}

	return &PayBoxService{
		logger:            logger,
		store:             options.Store,
		messenger:         options.Messenger,
		adminNotifier:     options.AdminNotifier,
		codeRenderer:      options.CodeRenderer,
		qrRenderer:        options.QRRenderer,
		orderNumberer:     orderNumberer,
		supportContact:    strings.TrimSpace(options.SupportContact),
		approvalMessage:   strings.TrimSpace(options.ApprovalMessage),
		redemptionBaseURL: strings.TrimRight(strings.TrimSpace(options.RedemptionBaseURL), "/"),
	}, nil
}

func (s *PayBoxService) ListActiveOffers(ctx context.Context, limit int) ([]store.Offer, error) {
	return s.store.ListActiveOffers(ctx, limit)
}

func (s *PayBoxService) GetOffer(ctx context.Context, offerID int64) (store.Offer, error) {
	return s.store.GetOffer(ctx, offerID)
}

func (s *PayBoxService) StartPayment(ctx context.Context, userID int64, offerID int64) (PayBoxPaymentStart, error) {
	order := store.MVPOrder{}
	created, err := s.store.CreateAwaitingPaymentOrder(ctx, store.CreateAwaitingPaymentOrderParams{
		UserID:      userID,
		OfferID:     offerID,
		OrderNumber: s.orderNumberer.NewOrderNumber(),
	})
	if err != nil {
		return PayBoxPaymentStart{}, err
	}
	order = created

	offer, err := s.store.GetOffer(ctx, offerID)
	if err != nil {
		return PayBoxPaymentStart{}, err
	}

	supportContact := firstNonEmpty(offer.SupportContact, order.SupportContact, s.supportContact)
	return PayBoxPaymentStart{
		OrderID:                order.ID,
		OrderNumber:            order.OrderNumber,
		OfferID:                order.OfferID,
		OfferTitle:             firstNonEmpty(order.OfferTitle, offer.Title),
		MerchantName:           firstNonEmpty(order.MerchantName, offer.MerchantName),
		PriceAmount:            order.PriceAmount,
		CurrencyCode:           order.CurrencyCode,
		PaymentLink:            firstNonEmpty(order.PayBoxPaymentLink, offer.PaymentLink),
		MerchantDisclosureText: offer.MerchantDisclosureText,
		RedemptionTerms:        firstNonEmpty(order.RedemptionTerms, offer.RedemptionTerms),
		SupportContact:         supportContact,
		NextStepMessage:        buildPayBoxNextStepMessage(supportContact),
	}, nil
}

func (s *PayBoxService) SubmitPaymentClaim(ctx context.Context, orderID int64, evidence PayBoxPaymentEvidence, claimedAmount int64) (store.ManualPaymentClaim, error) {
	claim, err := s.store.SubmitManualPaymentClaim(ctx, store.SubmitManualPaymentClaimParams{
		OrderID:                    orderID,
		PayerUsername:              evidence.PayerReference,
		ClaimedAmount:              claimedAmount,
		PaymentScreenshotFileID:    evidence.ScreenshotFileID,
		PaymentScreenshotUniqueID:  evidence.ScreenshotUniqueID,
		PaymentScreenshotMessageID: evidence.ScreenshotTelegramMessageID,
		PaymentScreenshotCaption:   evidence.ScreenshotCaption,
	})
	if err != nil {
		return store.ManualPaymentClaim{}, err
	}

	if err := s.notifyAdminClaimSubmitted(ctx, claim); err != nil {
		return store.ManualPaymentClaim{}, err
	}

	return claim, nil
}

func (s *PayBoxService) ListPendingClaims(ctx context.Context, limit int) ([]store.ManualPaymentClaim, error) {
	return s.store.ListPendingManualPaymentClaims(ctx, limit)
}

func (s *PayBoxService) ApproveClaim(ctx context.Context, claimID int64, reviewedBy string, reviewNote string) (PayBoxApprovalResult, error) {
	approved, err := s.store.ApproveManualPaymentClaim(ctx, store.ReviewManualPaymentClaimParams{
		ClaimID:    claimID,
		ReviewedBy: reviewedBy,
		ReviewNote: reviewNote,
	})
	if err != nil {
		return PayBoxApprovalResult{}, err
	}

	result := PayBoxApprovalResult{
		Order: approved.Order,
		Claim: approved.Claim,
		Code:  approved.Code,
	}
	if approved.Code == nil {
		result.SupportState = true
		if err := s.notifyUser(ctx, approved.Order.UserID, buildPayBoxSupportRequiredMessage(s.supportContact)); err != nil {
			return PayBoxApprovalResult{}, err
		}
		s.recordPaymentClaimAudit(ctx, approved.Claim.ID, reviewedBy, "approve_payment_claim", reviewNote, result)
		if err := s.notifyAdminNeedsSupport(ctx, result); err != nil {
			return PayBoxApprovalResult{}, err
		}
		return result, nil
	}

	redemption, delivery, err := s.deliverApprovedCode(ctx, approved.Order, *approved.Code)
	if err != nil {
		return PayBoxApprovalResult{}, err
	}
	result.Redemption = &redemption
	result.Delivery = &delivery
	s.recordPaymentClaimAudit(ctx, approved.Claim.ID, reviewedBy, "approve_payment_claim", reviewNote, result)

	if err := s.notifyAdminApproved(ctx, result); err != nil {
		return PayBoxApprovalResult{}, err
	}

	return result, nil
}

func (s *PayBoxService) RejectClaim(ctx context.Context, claimID int64, reviewedBy string, reviewNote string) (store.ManualPaymentClaim, store.MVPOrder, error) {
	claim, order, err := s.store.RejectManualPaymentClaim(ctx, store.ReviewManualPaymentClaimParams{
		ClaimID:    claimID,
		ReviewedBy: reviewedBy,
		ReviewNote: reviewNote,
	})
	if err != nil {
		return store.ManualPaymentClaim{}, store.MVPOrder{}, err
	}

	if err := s.notifyUser(ctx, order.UserID, buildPayBoxRejectedMessage(s.supportContact)); err != nil {
		return store.ManualPaymentClaim{}, store.MVPOrder{}, err
	}
	s.recordPaymentClaimAudit(ctx, claim.ID, reviewedBy, "reject_payment_claim", reviewNote, map[string]any{"claim": claim, "order": order})
	if err := s.notifyAdminRejected(ctx, claim, order); err != nil {
		return store.ManualPaymentClaim{}, store.MVPOrder{}, err
	}

	return claim, order, nil
}

func (s *PayBoxService) DeliverApprovedCode(ctx context.Context, order store.MVPOrder, code store.PredefinedCode) (store.MVPDelivery, error) {
	_, delivery, err := s.deliverApprovedCode(ctx, order, code)
	return delivery, err
}

func (s *PayBoxService) deliverApprovedCode(ctx context.Context, order store.MVPOrder, code store.PredefinedCode) (store.CouponRedemption, store.MVPDelivery, error) {
	if s.codeRenderer == nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, ErrPayBoxCodeRendererRequired
	}
	if s.qrRenderer == nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, ErrPayBoxQRRendererRequired
	}

	codeText, err := s.codeRenderer.RenderPredefinedCode(ctx, code)
	if err != nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}

	redemption, err := s.store.CreateCouponRedemption(ctx, store.CreateCouponRedemptionParams{
		OrderID:          order.ID,
		PredefinedCodeID: code.ID,
		RedemptionToken:  newPayBoxRedemptionToken(),
	})
	if err != nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}

	qrPayload := buildPayBoxRedemptionURL(s.redemptionBaseURL, redemption.RedemptionToken)
	qrPNG, err := s.qrRenderer.RenderPNG(qrPayload)
	if err != nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}
	message := buildPayBoxQRMessage(codeText, order.RedemptionTerms, firstNonEmpty(order.SupportContact, s.supportContact), s.approvalMessage)
	payloadHash := hashPayBoxPayload(message + "\n" + qrPayload)

	user, err := s.store.GetUserByID(ctx, order.UserID)
	if err != nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}

	messageID, err := s.messenger.SendPhotoMessage(ctx, user.TelegramUserID, qrPNG, "paybox-coupon-qr.png", message)
	if err != nil {
		if _, recordErr := s.store.RecordPredefinedCodeDeliveryEvent(ctx, store.RecordPredefinedCodeDeliveryEventParams{
			OrderID:             order.ID,
			PredefinedCodeID:    code.ID,
			DeliveryChannel:     "telegram_bot",
			Status:              "failed",
			DeliveryPayloadHash: payloadHash,
			FailureReason:       truncatePayBoxText(err.Error(), 255),
		}); recordErr != nil {
			s.logger.Warn("failed to record paybox delivery failure",
				"order_id", order.ID,
				"predefined_code_id", code.ID,
				"error", recordErr,
			)
		}
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}

	now := time.Now().UTC()
	delivery, err := s.store.RecordPredefinedCodeDeliveryEvent(ctx, store.RecordPredefinedCodeDeliveryEventParams{
		OrderID:             order.ID,
		PredefinedCodeID:    code.ID,
		DeliveryChannel:     "telegram_bot",
		Status:              "confirmed",
		TelegramMessageID:   &messageID,
		DeliveryPayloadHash: payloadHash,
		SentAt:              &now,
		ConfirmedAt:         &now,
	})
	if err != nil {
		return store.CouponRedemption{}, store.MVPDelivery{}, err
	}
	return redemption, delivery, nil
}

func (s *PayBoxService) notifyUser(ctx context.Context, userID int64, message string) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = s.messenger.SendHTMLMessage(ctx, user.TelegramUserID, message)
	return err
}

func (s *PayBoxService) notifyAdminClaimSubmitted(ctx context.Context, claim store.ManualPaymentClaim) error {
	if s.adminNotifier == nil {
		return nil
	}
	return s.adminNotifier.NotifyPaymentClaimSubmitted(ctx, claim)
}

func (s *PayBoxService) notifyAdminApproved(ctx context.Context, result PayBoxApprovalResult) error {
	if s.adminNotifier == nil {
		return nil
	}
	return s.adminNotifier.NotifyPaymentClaimApproved(ctx, result)
}

func (s *PayBoxService) notifyAdminRejected(ctx context.Context, claim store.ManualPaymentClaim, order store.MVPOrder) error {
	if s.adminNotifier == nil {
		return nil
	}
	return s.adminNotifier.NotifyPaymentClaimRejected(ctx, claim, order)
}

func (s *PayBoxService) notifyAdminNeedsSupport(ctx context.Context, result PayBoxApprovalResult) error {
	if s.adminNotifier == nil {
		return nil
	}
	return s.adminNotifier.NotifyPaymentClaimNeedsSupport(ctx, result)
}

func (s *PayBoxService) recordPaymentClaimAudit(ctx context.Context, claimID int64, reviewedBy string, actionType string, reason string, after any) {
	afterJSON, err := json.Marshal(after)
	if err != nil {
		s.logger.Warn("failed to marshal paybox audit state",
			"claim_id", claimID,
			"action_type", actionType,
			"error", err,
		)
		afterJSON = []byte("{}")
	}

	if _, err := s.store.RecordAdminAction(ctx, store.RecordAdminActionParams{
		AdminActor:      firstNonEmpty(reviewedBy, "paybox-service"),
		EntityType:      "manual_payment_claim",
		EntityID:        claimID,
		ActionType:      actionType,
		BeforeStateJSON: "{}",
		AfterStateJSON:  string(afterJSON),
		ReasonText:      strings.TrimSpace(reason),
	}); err != nil {
		s.logger.Warn("failed to record paybox audit action",
			"claim_id", claimID,
			"action_type", actionType,
			"error", err,
		)
	}
}

type timeOrderNumberer struct{}

func (timeOrderNumberer) NewOrderNumber() string {
	return "PB-" + time.Now().UTC().Format("20060102150405")
}

func buildPayBoxNextStepMessage(supportContact string) string {
	support := ""
	if strings.TrimSpace(supportContact) != "" {
		support = "\nSupport: " + strings.TrimSpace(supportContact)
	}
	return "Pay through the PayBox link, then upload the payment screenshot here for admin approval." + support
}

func buildPayBoxQRMessage(code string, redemptionTerms string, supportContact string, approvalMessage string) string {
	parts := []string{"Payment approved."}
	if strings.TrimSpace(approvalMessage) != "" {
		parts[0] = strings.TrimSpace(approvalMessage)
	}
	parts = append(parts, "Show this QR code to the merchant.")
	if strings.TrimSpace(code) != "" {
		parts = append(parts, "Coupon: <code>"+htmlEscapePayBox(code)+"</code>")
	}
	if strings.TrimSpace(redemptionTerms) != "" {
		parts = append(parts, "Redemption: "+htmlEscapePayBox(redemptionTerms))
	}
	if strings.TrimSpace(supportContact) != "" {
		parts = append(parts, "Support: "+htmlEscapePayBox(supportContact))
	}
	return strings.Join(parts, "\n")
}

func buildPayBoxRedemptionURL(baseURL string, token string) string {
	cleanToken := url.PathEscape(strings.TrimSpace(token))
	if strings.TrimSpace(baseURL) == "" {
		return "paybox-redemption:" + cleanToken
	}
	return strings.TrimRight(baseURL, "/") + "/api/redemptions/scan/" + cleanToken
}

func buildPayBoxRejectedMessage(supportContact string) string {
	message := "We could not match your PayBox payment claim yet. Please check the username and contact support if you think this is a mistake."
	if strings.TrimSpace(supportContact) != "" {
		message += "\nSupport: " + htmlEscapePayBox(supportContact)
	}
	return message
}

func buildPayBoxSupportRequiredMessage(supportContact string) string {
	message := "Your PayBox payment was approved, but the code needs manual support before delivery. We are checking it now."
	if strings.TrimSpace(supportContact) != "" {
		message += "\nSupport: " + htmlEscapePayBox(supportContact)
	}
	return message
}

func hashPayBoxPayload(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func newPayBoxRedemptionToken() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "rt-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw[:])
}

func truncatePayBoxText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "")
	}
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func htmlEscapePayBox(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
