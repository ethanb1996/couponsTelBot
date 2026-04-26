package payments

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

var (
	ErrUnsupportedProvider = errors.New("payments: unsupported provider")
	ErrWebhookUnauthorized = errors.New("payments: webhook verification failed")
)

type CheckoutLink struct {
	ProviderName       string
	ProviderCheckoutID string
	ApprovalURL        string
}

type CouponDeliverer interface {
	SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error)
}

type paymentStore interface {
	MarkOrderPendingPayment(ctx context.Context, params store.MarkOrderPendingPaymentParams) (store.Order, error)
	ReleaseCheckoutReservation(ctx context.Context, params store.ReleaseCheckoutReservationParams) (store.Order, error)
	RecordPaymentEvent(ctx context.Context, params store.RecordPaymentEventParams) (store.Payment, error)
	GetOrderByProviderCheckoutReference(ctx context.Context, providerCheckoutReference string) (store.Order, error)
	GetCouponDeliveryForOrder(ctx context.Context, orderID int64) (*store.CouponDelivery, error)
	GetOrder(ctx context.Context, orderID int64) (store.Order, error)
	PrepareReservedCouponForDelivery(ctx context.Context, orderID int64) (store.Coupon, error)
	GetCoupon(ctx context.Context, couponID int64) (store.Coupon, error)
	GetUserByID(ctx context.Context, userID int64) (store.User, error)
	GetListing(ctx context.Context, listingID int64) (store.Listing, error)
	RecordDeliveryEvent(ctx context.Context, params store.RecordDeliveryEventParams) (store.CouponDelivery, error)
	EnsureSupportCase(ctx context.Context, params store.EnsureSupportCaseParams) (store.SupportCase, bool, error)
}

type payPalGateway interface {
	CreateCheckout(ctx context.Context, input payPalCreateCheckoutInput) (payPalCheckout, error)
	VerifyAndParseWebhook(ctx context.Context, headers http.Header, body []byte) (payPalWebhookEvent, error)
	CaptureOrder(ctx context.Context, orderID string) (payPalCapture, error)
	GetOrder(ctx context.Context, orderID string) (payPalOrderSnapshot, error)
}

type Service struct {
	logger              *slog.Logger
	store               paymentStore
	deliverer           CouponDeliverer
	providerName        string
	appBaseURL          string
	couponEncryptionKey string
	paypal              payPalGateway
}

func NewService(logger *slog.Logger, repo *store.Postgres, cfg config.Config, deliverer CouponDeliverer) (*Service, error) {
	if logger == nil {
		return nil, errors.New("payments: logger is required")
	}
	if repo == nil {
		return nil, errors.New("payments: store is required")
	}
	if cfg.PaymentProviderName != "paypal" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, cfg.PaymentProviderName)
	}

	return &Service{
		logger:              logger,
		store:               repo,
		deliverer:           deliverer,
		providerName:        cfg.PaymentProviderName,
		appBaseURL:          strings.TrimRight(cfg.AppBaseURL, "/"),
		couponEncryptionKey: cfg.CouponEncryptionKey,
		paypal: newPayPalClient(payPalClientOptions{
			BaseURL:   cfg.PaymentProviderBaseURL,
			ClientID:  cfg.PaymentProviderClientID,
			Secret:    cfg.PaymentProviderSecret,
			WebhookID: cfg.PaymentProviderWebhookID,
		}),
	}, nil
}

func (s *Service) StartCheckout(ctx context.Context, order store.Order, listing store.Listing) (CheckoutLink, error) {
	checkout, err := s.paypal.CreateCheckout(ctx, payPalCreateCheckoutInput{
		OrderID:      order.ID,
		OrderNumber:  order.OrderNumber,
		Amount:       order.SalePriceAmount,
		CurrencyCode: order.CurrencyCode,
		Description:  buildCheckoutDescription(listing),
		ItemName:     buildCheckoutItemName(listing),
		ItemSummary:  buildCheckoutItemSummary(listing),
		ItemImageURL: buildCheckoutItemImageURL(s.appBaseURL, listing),
		ReturnURL:    s.appBaseURL + "/payments/paypal/return?order_number=" + order.OrderNumber,
		CancelURL:    s.appBaseURL + "/payments/paypal/cancel?order_number=" + order.OrderNumber,
	})
	if err != nil {
		s.releaseFailedCheckout(ctx, order.ID, "checkout_provider_create_failed")
		return CheckoutLink{}, err
	}

	if _, err := s.store.MarkOrderPendingPayment(ctx, store.MarkOrderPendingPaymentParams{
		OrderID:                   order.ID,
		ProviderCheckoutReference: checkout.OrderID,
	}); err != nil {
		s.releaseFailedCheckout(ctx, order.ID, "checkout_mark_pending_failed")
		return CheckoutLink{}, err
	}

	if _, err := s.store.RecordPaymentEvent(ctx, store.RecordPaymentEventParams{
		OrderID:            order.ID,
		ProviderName:       s.providerName,
		ProviderCheckoutID: checkout.OrderID,
		Status:             "pending",
		Amount:             order.SalePriceAmount,
		CurrencyCode:       order.CurrencyCode,
	}); err != nil {
		s.releaseFailedCheckout(ctx, order.ID, "checkout_payment_record_failed")
		return CheckoutLink{}, err
	}

	return CheckoutLink{
		ProviderName:       s.providerName,
		ProviderCheckoutID: checkout.OrderID,
		ApprovalURL:        checkout.ApprovalURL,
	}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	if provider != s.providerName {
		return fmt.Errorf("%w: %s", ErrUnsupportedProvider, provider)
	}

	event, err := s.paypal.VerifyAndParseWebhook(ctx, headers, body)
	if err != nil {
		return err
	}

	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		capture, err := s.paypal.CaptureOrder(ctx, event.OrderID())
		if err != nil {
			return err
		}
		return s.processCapture(ctx, capture.OrderID, capture.CaptureID, capture.Status, capture.Amount, capture.CurrencyCode)
	case "PAYMENT.CAPTURE.COMPLETED":
		return s.processCapture(ctx, event.RelatedOrderID(), event.CaptureID(), "captured", event.AmountMinorUnits(), event.CurrencyCode())
	case "PAYMENT.CAPTURE.PENDING":
		return s.processCapture(ctx, event.RelatedOrderID(), event.CaptureID(), "pending", event.AmountMinorUnits(), event.CurrencyCode())
	case "PAYMENT.CAPTURE.DENIED", "PAYMENT.CAPTURE.DECLINED":
		return s.processCapture(ctx, event.RelatedOrderID(), event.CaptureID(), "failed", event.AmountMinorUnits(), event.CurrencyCode())
	default:
		s.logger.Info("ignoring unsupported payment webhook event", "provider", provider, "event_type", event.EventType, "event_id", event.ID)
		return nil
	}
}

func (s *Service) ReconcilePendingOrder(ctx context.Context, candidate store.PendingPaymentReconciliationCandidate) error {
	if strings.TrimSpace(candidate.ProviderCheckoutReference) == "" {
		return nil
	}

	orderSnapshot, err := s.paypal.GetOrder(ctx, candidate.ProviderCheckoutReference)
	if err != nil {
		return err
	}

	switch orderSnapshot.Status {
	case "approved":
		capture, err := s.paypal.CaptureOrder(ctx, candidate.ProviderCheckoutReference)
		if err != nil {
			return err
		}
		return s.processCapture(ctx, capture.OrderID, capture.CaptureID, capture.Status, capture.Amount, capture.CurrencyCode)
	case "completed":
		if strings.TrimSpace(orderSnapshot.CaptureID) == "" {
			return nil
		}
		return s.processCapture(ctx, orderSnapshot.OrderID, orderSnapshot.CaptureID, "captured", orderSnapshot.Amount, orderSnapshot.CurrencyCode)
	case "voided":
		order, err := s.store.GetOrder(ctx, candidate.OrderID)
		if err != nil {
			return err
		}
		_, err = s.store.RecordPaymentEvent(ctx, store.RecordPaymentEventParams{
			OrderID:            order.ID,
			ProviderName:       s.providerName,
			ProviderCheckoutID: candidate.ProviderCheckoutReference,
			Status:             "cancelled",
			Amount:             order.SalePriceAmount,
			CurrencyCode:       order.CurrencyCode,
			FailureMessage:     "payment order was voided during reconciliation",
		})
		return err
	default:
		return nil
	}
}

func (s *Service) processCapture(ctx context.Context, providerCheckoutID string, providerPaymentID string, paymentStatus string, amount int64, currencyCode string) error {
	if strings.TrimSpace(providerCheckoutID) == "" {
		return store.ErrInvalidArgument
	}

	order, err := s.store.GetOrderByProviderCheckoutReference(ctx, providerCheckoutID)
	if err != nil {
		return err
	}

	params := store.RecordPaymentEventParams{
		OrderID:            order.ID,
		ProviderName:       s.providerName,
		ProviderPaymentID:  providerPaymentID,
		ProviderCheckoutID: providerCheckoutID,
		Status:             paymentStatus,
		Amount:             amount,
		CurrencyCode:       defaultCurrency(currencyCode, order.CurrencyCode),
	}
	if paymentStatus == "captured" {
		now := time.Now().UTC()
		params.CapturedAt = &now
	}

	if _, err := s.store.RecordPaymentEvent(ctx, params); err != nil {
		return err
	}

	if paymentStatus != "captured" && paymentStatus != "authorized" {
		return nil
	}

	return s.fulfillPaidOrder(ctx, order.ID)
}

func (s *Service) fulfillPaidOrder(ctx context.Context, orderID int64) error {
	delivery, err := s.store.GetCouponDeliveryForOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if delivery != nil && (delivery.Status == "sent" || delivery.Status == "confirmed") {
		return nil
	}

	order, err := s.store.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}

	if order.CouponID == nil {
		return s.escalatePaidOrderIssue(ctx, order, nil, "payment confirmed after checkout hold was released; manual refund review required")
	}

	coupon, err := s.store.PrepareReservedCouponForDelivery(ctx, orderID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrReservedCouponRequired):
			return s.escalatePaidOrderIssue(ctx, order, nil, "payment confirmed without a reserved coupon; manual refund review required")
		case errors.Is(err, store.ErrReservedCouponInvalid):
			return s.escalatePaidOrderIssue(ctx, order, order.CouponID, "reserved coupon is no longer deliverable after payment confirmation")
		default:
			return err
		}
	}
	order.CouponID = &coupon.ID

	user, err := s.store.GetUserByID(ctx, order.UserID)
	if err != nil {
		return err
	}
	listing, err := s.store.GetListing(ctx, order.ListingID)
	if err != nil {
		return err
	}

	couponCode, err := decryptCouponCode(s.couponEncryptionKey, coupon.CouponCodeCiphertext, coupon.CouponCodeNonce)
	if err != nil {
		return err
	}

	text := buildCouponDeliveryMessage(order, listing, coupon, couponCode)
	payloadHash := hashDeliveryPayload(text)
	telegramMessageID, err := s.deliverer.SendHTMLMessage(ctx, user.TelegramUserID, text)
	if err != nil {
		if recordErr := s.recordFailedDelivery(ctx, order.ID, coupon.ID, payloadHash, err); recordErr != nil {
			s.logger.Error("failed to record delivery failure", "order_id", order.ID, "coupon_id", coupon.ID, "error", recordErr)
		}
		return err
	}

	now := time.Now().UTC()
	if _, err := s.store.RecordDeliveryEvent(ctx, store.RecordDeliveryEventParams{
		OrderID:             order.ID,
		CouponID:            coupon.ID,
		DeliveryChannel:     "telegram_bot",
		Status:              "confirmed",
		TelegramMessageID:   &telegramMessageID,
		DeliveryPayloadHash: payloadHash,
		SentAt:              &now,
		ConfirmedAt:         &now,
	}); err != nil {
		return err
	}

	return nil
}

func (s *Service) recordFailedDelivery(ctx context.Context, orderID int64, couponID int64, payloadHash string, deliveryErr error) error {
	returnErr := deliveryErr.Error()
	_, err := s.store.RecordDeliveryEvent(ctx, store.RecordDeliveryEventParams{
		OrderID:             orderID,
		CouponID:            couponID,
		DeliveryChannel:     "telegram_bot",
		Status:              "failed",
		DeliveryPayloadHash: payloadHash,
		FailureReason:       truncateText(returnErr, 255),
	})
	if err != nil {
		return err
	}

	orderIDCopy := orderID
	couponIDCopy := couponID
	_, _, err = s.store.EnsureSupportCase(ctx, store.EnsureSupportCaseParams{
		OrderID:         &orderIDCopy,
		CouponID:        &couponIDCopy,
		CaseType:        "delivery_issue",
		Priority:        "high",
		Summary:         truncateText(fmt.Sprintf("Automatic delivery escalation for order %d: %s", orderID, returnErr), 255),
		AssignedAdminID: "ops-job",
	})
	return err
}

func (s *Service) releaseFailedCheckout(ctx context.Context, orderID int64, failureReason string) {
	if _, err := s.store.ReleaseCheckoutReservation(ctx, store.ReleaseCheckoutReservationParams{
		OrderID:       orderID,
		OrderStatus:   "failed",
		FailureReason: failureReason,
	}); err != nil {
		s.logger.Error("failed to release checkout reservation",
			"order_id", orderID,
			"failure_reason", failureReason,
			"error", err,
		)
	}
}

func (s *Service) escalatePaidOrderIssue(ctx context.Context, order store.Order, couponID *int64, summary string) error {
	_, _, err := s.store.EnsureSupportCase(ctx, store.EnsureSupportCaseParams{
		UserID:          &order.UserID,
		OrderID:         &order.ID,
		CouponID:        couponID,
		CaseType:        "payment_issue",
		Priority:        "high",
		Summary:         truncateText(fmt.Sprintf("Order %s requires manual payment review: %s", order.OrderNumber, summary), 255),
		AssignedAdminID: "ops-job",
	})
	return err
}

func buildCheckoutDescription(listing store.Listing) string {
	description := strings.TrimSpace(listing.MerchantName + " " + listing.Title)
	return truncateText(description, 127)
}

func buildCheckoutItemName(listing store.Listing) string {
	name := strings.TrimSpace(listing.Title)
	if name == "" {
		name = strings.TrimSpace(listing.MerchantName)
	}
	return truncateText(name, 127)
}

func buildCheckoutItemSummary(listing store.Listing) string {
	summary := strings.TrimSpace(listing.Description)
	if summary == "" {
		summary = strings.TrimSpace(listing.RedemptionInstructions)
	}
	return truncateText(summary, 2048)
}

func buildCheckoutItemImageURL(appBaseURL string, listing store.Listing) string {
	if strings.TrimSpace(listing.PhotoKey) == "" {
		return ""
	}

	parsed, err := url.Parse(strings.TrimSpace(appBaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" || strings.HasSuffix(host, ".local") {
		return ""
	}

	return strings.TrimRight(parsed.String(), "/") + "/assets/coupons/" + url.PathEscape(strings.TrimSpace(listing.PhotoKey))
}

func buildCouponDeliveryMessage(order store.Order, listing store.Listing, coupon store.Coupon, couponCode string) string {
	return fmt.Sprintf(`<b>Payment confirmed for order %s</b>

<b>%s</b>
%s

<b>Your coupon code:</b>
<code>%s</code>

<b>Masked reference:</b> %s
<b>Expires:</b> %s

<b>How to redeem:</b>
%s

If anything looks wrong, reply here with your order number and we will investigate manually.`,
		html.EscapeString(order.OrderNumber),
		html.EscapeString(listing.MerchantName),
		html.EscapeString(listing.Title),
		html.EscapeString(couponCode),
		html.EscapeString(coupon.CouponMaskedDisplay),
		html.EscapeString(coupon.ExpiryAt.UTC().Format("2 January 2006")),
		html.EscapeString(listing.RedemptionInstructions),
	)
}

func decryptCouponCode(key string, ciphertext []byte, nonce []byte) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func hashDeliveryPayload(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func truncateText(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "")
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func defaultCurrency(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatMinorUnits(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	return sign + strconv.FormatInt(amount/100, 10) + "." + fmt.Sprintf("%02d", amount%100)
}
