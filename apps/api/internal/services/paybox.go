package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

var (
	ErrPayBoxStoreRequired        = errors.New("paybox: store is required")
	ErrPayBoxMessengerRequired    = errors.New("paybox: messenger is required")
	ErrPayBoxCodeRendererRequired = errors.New("paybox: code renderer is required")
)

type PayBoxStore interface {
	ListActiveOffers(ctx context.Context, limit int) ([]store.Offer, error)
	GetOffer(ctx context.Context, offerID int64) (store.Offer, error)
	CreateAwaitingPaymentOrder(ctx context.Context, params store.CreateAwaitingPaymentOrderParams) (store.MVPOrder, error)
	SubmitManualPaymentClaim(ctx context.Context, params store.SubmitManualPaymentClaimParams) (store.ManualPaymentClaim, error)
	ListPendingManualPaymentClaims(ctx context.Context, limit int) ([]store.ManualPaymentClaim, error)
	ApproveManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ApproveManualPaymentClaimResult, error)
	RejectManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ManualPaymentClaim, store.MVPOrder, error)
	RecordPredefinedCodeDeliveryEvent(ctx context.Context, params store.RecordPredefinedCodeDeliveryEventParams) (store.MVPDelivery, error)
	GetUserByID(ctx context.Context, userID int64) (store.User, error)
}

type PayBoxMessenger interface {
	SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error)
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

type OrderNumberGenerator interface {
	NewOrderNumber() string
}

type PayBoxService struct {
	logger          *slog.Logger
	store           PayBoxStore
	messenger       PayBoxMessenger
	adminNotifier   PayBoxAdminNotifier
	codeRenderer    PayBoxCodeRenderer
	orderNumberer   OrderNumberGenerator
	supportContact  string
	approvalMessage string
}

type PayBoxServiceOptions struct {
	Logger          *slog.Logger
	Store           PayBoxStore
	Messenger       PayBoxMessenger
	AdminNotifier   PayBoxAdminNotifier
	CodeRenderer    PayBoxCodeRenderer
	OrderNumberer   OrderNumberGenerator
	SupportContact  string
	ApprovalMessage string
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
	Delivery     *store.MVPDelivery
	SupportState bool
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
		logger:          logger,
		store:           options.Store,
		messenger:       options.Messenger,
		adminNotifier:   options.AdminNotifier,
		codeRenderer:    options.CodeRenderer,
		orderNumberer:   orderNumberer,
		supportContact:  strings.TrimSpace(options.SupportContact),
		approvalMessage: strings.TrimSpace(options.ApprovalMessage),
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

func (s *PayBoxService) SubmitPaymentClaim(ctx context.Context, orderID int64, payerUsername string, claimedAmount int64) (store.ManualPaymentClaim, error) {
	claim, err := s.store.SubmitManualPaymentClaim(ctx, store.SubmitManualPaymentClaimParams{
		OrderID:       orderID,
		PayerUsername: payerUsername,
		ClaimedAmount: claimedAmount,
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
		if err := s.notifyAdminNeedsSupport(ctx, result); err != nil {
			return PayBoxApprovalResult{}, err
		}
		return result, nil
	}

	delivery, err := s.deliverApprovedCode(ctx, approved.Order, *approved.Code)
	if err != nil {
		return PayBoxApprovalResult{}, err
	}
	result.Delivery = &delivery

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
	if err := s.notifyAdminRejected(ctx, claim, order); err != nil {
		return store.ManualPaymentClaim{}, store.MVPOrder{}, err
	}

	return claim, order, nil
}

func (s *PayBoxService) DeliverApprovedCode(ctx context.Context, order store.MVPOrder, code store.PredefinedCode) (store.MVPDelivery, error) {
	return s.deliverApprovedCode(ctx, order, code)
}

func (s *PayBoxService) deliverApprovedCode(ctx context.Context, order store.MVPOrder, code store.PredefinedCode) (store.MVPDelivery, error) {
	if s.codeRenderer == nil {
		return store.MVPDelivery{}, ErrPayBoxCodeRendererRequired
	}

	codeText, err := s.codeRenderer.RenderPredefinedCode(ctx, code)
	if err != nil {
		return store.MVPDelivery{}, err
	}
	message := buildPayBoxCodeMessage(codeText, order.RedemptionTerms, firstNonEmpty(order.SupportContact, s.supportContact), s.approvalMessage)
	payloadHash := hashPayBoxPayload(message)

	user, err := s.store.GetUserByID(ctx, order.UserID)
	if err != nil {
		return store.MVPDelivery{}, err
	}

	messageID, err := s.messenger.SendHTMLMessage(ctx, user.TelegramUserID, message)
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
		return store.MVPDelivery{}, err
	}

	now := time.Now().UTC()
	return s.store.RecordPredefinedCodeDeliveryEvent(ctx, store.RecordPredefinedCodeDeliveryEventParams{
		OrderID:             order.ID,
		PredefinedCodeID:    code.ID,
		DeliveryChannel:     "telegram_bot",
		Status:              "confirmed",
		TelegramMessageID:   &messageID,
		DeliveryPayloadHash: payloadHash,
		SentAt:              &now,
		ConfirmedAt:         &now,
	})
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

type timeOrderNumberer struct{}

func (timeOrderNumberer) NewOrderNumber() string {
	return "PB-" + time.Now().UTC().Format("20060102150405")
}

func buildPayBoxNextStepMessage(supportContact string) string {
	support := ""
	if strings.TrimSpace(supportContact) != "" {
		support = "\nSupport: " + strings.TrimSpace(supportContact)
	}
	return "Pay through the PayBox link, return here, tap I paid, and send the exact PayBox username used for the payment." + support
}

func buildPayBoxCodeMessage(code string, redemptionTerms string, supportContact string, approvalMessage string) string {
	parts := []string{"Payment approved."}
	if strings.TrimSpace(approvalMessage) != "" {
		parts[0] = strings.TrimSpace(approvalMessage)
	}
	parts = append(parts, "<code>"+htmlEscapePayBox(code)+"</code>")
	if strings.TrimSpace(redemptionTerms) != "" {
		parts = append(parts, "Redemption: "+htmlEscapePayBox(redemptionTerms))
	}
	if strings.TrimSpace(supportContact) != "" {
		parts = append(parts, "Support: "+htmlEscapePayBox(supportContact))
	}
	return strings.Join(parts, "\n")
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
