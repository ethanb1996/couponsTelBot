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
	"strconv"
	"strings"
	"time"

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

type Service struct {
	logger              *slog.Logger
	store               *store.Postgres
	deliverer           CouponDeliverer
	providerName        string
	appBaseURL          string
	couponEncryptionKey string
	paypal              *paypalClient
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
		ReturnURL:    s.appBaseURL + "/payments/paypal/return?order_number=" + order.OrderNumber,
		CancelURL:    s.appBaseURL + "/payments/paypal/cancel?order_number=" + order.OrderNumber,
	})
	if err != nil {
		return CheckoutLink{}, err
	}

	if _, err := s.store.MarkOrderPendingPayment(ctx, store.MarkOrderPendingPaymentParams{
		OrderID:                   order.ID,
		ProviderCheckoutReference: checkout.OrderID,
	}); err != nil {
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

	coupon, err := s.store.AssignAvailableCoupon(ctx, orderID)
	if err != nil {
		if !errors.Is(err, store.ErrCouponAlreadyAssigned) {
			return err
		}
		order, err = s.store.GetOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if order.CouponID == nil {
			return store.ErrCouponAlreadyAssigned
		}
		coupon, err = s.store.GetCoupon(ctx, *order.CouponID)
		if err != nil {
			return err
		}
	} else {
		order.CouponID = &coupon.ID
	}

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
	return err
}

func buildCheckoutDescription(listing store.Listing) string {
	description := strings.TrimSpace(listing.MerchantName + " " + listing.Title)
	return truncateText(description, 127)
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
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
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
