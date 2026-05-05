package payments

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	ErrUnsupportedProvider = errors.New("payments: unsupported provider")
	ErrWebhookUnauthorized = errors.New("payments: webhook verification failed")

	fulfillmentRetryBackoff = []time.Duration{
		15 * time.Second,
		30 * time.Second,
		60 * time.Second,
		120 * time.Second,
		300 * time.Second,
	}
)

type CheckoutLink struct {
	ProviderName       string
	ProviderCheckoutID string
	ApprovalURL        string
}

type CouponDeliverer interface {
	SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error)
}

type FulfillmentNotifier interface {
	NotifyFulfillment(orderID int64)
}

type paymentStore interface {
	MarkOrderPendingPayment(ctx context.Context, params store.MarkOrderPendingPaymentParams) (store.Order, error)
	ReleaseCheckoutReservation(ctx context.Context, params store.ReleaseCheckoutReservationParams) (store.Order, error)
	RecordPaymentEvent(ctx context.Context, params store.RecordPaymentEventParams) (store.Payment, error)
	GetOrderByProviderCheckoutReference(ctx context.Context, providerCheckoutReference string) (store.Order, error)
	GetOrder(ctx context.Context, orderID int64) (store.Order, error)
	PrepareFulfillment(ctx context.Context, orderID int64) (store.FulfillmentPreparation, error)
	RecordDeliveryEvent(ctx context.Context, params store.RecordDeliveryEventParams) (store.CouponDelivery, error)
	EnsureSupportCase(ctx context.Context, params store.EnsureSupportCaseParams) (store.SupportCase, bool, error)
	EnqueueFulfillmentJob(ctx context.Context, params store.EnqueueFulfillmentJobParams) (store.FulfillmentJob, error)
	MarkFulfillmentJobSucceeded(ctx context.Context, params store.SucceedFulfillmentJobParams) (store.FulfillmentJob, error)
	RescheduleFulfillmentJob(ctx context.Context, params store.RescheduleFulfillmentJobParams) (store.FulfillmentJob, error)
	FailFulfillmentJobTerminal(ctx context.Context, params store.FailFulfillmentJobParams) (store.FulfillmentJob, error)
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
	fulfillmentNotifier FulfillmentNotifier
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

func (s *Service) SetFulfillmentNotifier(notifier FulfillmentNotifier) {
	s.fulfillmentNotifier = notifier
}

func (s *Service) StartCheckout(ctx context.Context, order store.Order, listing store.Listing) (CheckoutLink, error) {
	checkout, err := s.paypal.CreateCheckout(ctx, payPalCreateCheckoutInput{
		OrderID:      order.ID,
		OrderNumber:  order.OrderNumber,
		Amount:       order.SalePriceAmount,
		CurrencyCode: order.CurrencyCode,
		BrandName:    buildCheckoutBrandName(listing),
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

	verifyStart := time.Now()
	event, err := s.paypal.VerifyAndParseWebhook(ctx, headers, body)
	if err != nil {
		return err
	}
	s.logStepLatency("paypal_webhook_verify", verifyStart, "event_type", event.EventType, "event_id", event.ID)

	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		return s.enqueueFulfillmentByCheckoutReference(ctx, event.OrderID(), "webhook_checkout_order_approved")
	case "PAYMENT.CAPTURE.COMPLETED":
		order, err := s.recordPaymentEventByCheckoutReference(ctx, event.RelatedOrderID(), event.CaptureID(), "captured", event.AmountMinorUnits(), event.CurrencyCode(), "")
		if err != nil {
			return err
		}
		return s.enqueueFulfillmentJob(ctx, order, "webhook_capture_completed")
	case "PAYMENT.CAPTURE.PENDING":
		_, err := s.recordPaymentEventByCheckoutReference(ctx, event.RelatedOrderID(), event.CaptureID(), "pending", event.AmountMinorUnits(), event.CurrencyCode(), "")
		return err
	case "PAYMENT.CAPTURE.DENIED", "PAYMENT.CAPTURE.DECLINED":
		_, err := s.recordPaymentEventByCheckoutReference(ctx, event.RelatedOrderID(), event.CaptureID(), "failed", event.AmountMinorUnits(), event.CurrencyCode(), "paypal capture was denied or declined")
		return err
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
		return s.enqueueFulfillmentByCheckoutReference(ctx, candidate.ProviderCheckoutReference, "reconcile_order_approved")
	case "completed":
		if strings.TrimSpace(orderSnapshot.CaptureID) == "" {
			return nil
		}
		order, err := s.recordPaymentEventByCheckoutReference(ctx, orderSnapshot.OrderID, orderSnapshot.CaptureID, "captured", orderSnapshot.Amount, orderSnapshot.CurrencyCode, "")
		if err != nil {
			return err
		}
		return s.enqueueFulfillmentJob(ctx, order, "reconcile_capture_completed")
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

func (s *Service) ProcessFulfillmentJob(ctx context.Context, job store.FulfillmentJob) error {
	preparation, err := s.prepareFulfillment(ctx, job.OrderID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPaymentRequired):
			return s.captureAndContinueFulfillment(ctx, job, preparation.Order)
		case errors.Is(err, store.ErrReservedCouponRequired):
			return s.failFulfillmentAsPaymentIssue(ctx, job, preparation.Order, nil, "payment confirmed after checkout hold was released; manual refund review required", "prepare_fulfillment", err)
		case errors.Is(err, store.ErrReservedCouponInvalid):
			return s.failFulfillmentAsPaymentIssue(ctx, job, preparation.Order, preparation.Order.CouponID, "reserved coupon is no longer deliverable after payment confirmation", "prepare_fulfillment", err)
		case errors.Is(err, store.ErrOrderNotReadyForCoupon):
			return s.failFulfillmentAsPaymentIssue(ctx, job, preparation.Order, preparation.Order.CouponID, "order is not ready for coupon delivery after payment confirmation", "prepare_fulfillment", err)
		default:
			return s.retryOrFailFulfillment(ctx, job, "prepare_fulfillment", err, preparation.Order, preparation.Order.CouponID, "", false)
		}
	}

	if preparation.Delivery != nil && (preparation.Delivery.Status == "sent" || preparation.Delivery.Status == "confirmed") {
		_, err := s.store.MarkFulfillmentJobSucceeded(ctx, store.SucceedFulfillmentJobParams{
			JobID:    job.ID,
			LastStep: "already_delivered",
		})
		return err
	}
	if preparation.Order.Status == "delivered" {
		_, err := s.store.MarkFulfillmentJobSucceeded(ctx, store.SucceedFulfillmentJobParams{
			JobID:    job.ID,
			LastStep: "already_delivered",
		})
		return err
	}
	if preparation.Coupon == nil || preparation.User == nil {
		return s.failFulfillmentAsPaymentIssue(ctx, job, preparation.Order, preparation.Order.CouponID, "fulfillment preparation returned incomplete delivery context", "prepare_fulfillment", store.ErrReservedCouponRequired)
	}

	couponCode, err := decryptCouponCode(s.couponEncryptionKey, preparation.Coupon.CouponCodeCiphertext, preparation.Coupon.CouponCodeNonce)
	if err != nil {
		return s.failFulfillmentAsPaymentIssue(ctx, job, preparation.Order, preparation.Order.CouponID, "coupon code could not be decrypted for delivery", "decrypt_coupon", err)
	}

	text := buildCouponDeliveryMessage(couponCode)
	payloadHash := hashDeliveryPayload(text)
	sendStart := time.Now()
	telegramMessageID, err := s.deliverer.SendHTMLMessage(ctx, preparation.User.TelegramUserID, text)
	s.logStepLatency("telegram_send", sendStart, "order_id", preparation.Order.ID, "job_id", job.ID)
	if err != nil {
		if recordErr := s.recordFailedDeliveryState(ctx, preparation.Order.ID, preparation.Coupon.ID, payloadHash, err); recordErr != nil {
			s.logger.Error("failed to record transient delivery failure", "order_id", preparation.Order.ID, "coupon_id", preparation.Coupon.ID, "job_id", job.ID, "error", recordErr)
		}
		return s.retryOrFailFulfillment(ctx, job, "telegram_send", err, preparation.Order, &preparation.Coupon.ID, payloadHash, true)
	}

	now := time.Now().UTC()
	recordStart := time.Now()
	if _, err := s.store.RecordDeliveryEvent(ctx, store.RecordDeliveryEventParams{
		OrderID:             preparation.Order.ID,
		CouponID:            preparation.Coupon.ID,
		DeliveryChannel:     "telegram_bot",
		Status:              "confirmed",
		TelegramMessageID:   &telegramMessageID,
		DeliveryPayloadHash: payloadHash,
		SentAt:              &now,
		ConfirmedAt:         &now,
	}); err != nil {
		s.logStepLatency("record_delivery", recordStart, "order_id", preparation.Order.ID, "job_id", job.ID)
		return s.retryOrFailFulfillment(ctx, job, "record_delivery", err, preparation.Order, &preparation.Coupon.ID, payloadHash, true)
	}
	s.logStepLatency("record_delivery", recordStart, "order_id", preparation.Order.ID, "job_id", job.ID)

	_, err = s.store.MarkFulfillmentJobSucceeded(ctx, store.SucceedFulfillmentJobParams{
		JobID:    job.ID,
		LastStep: "delivery_recorded",
	})
	return err
}

func (s *Service) recordFailedDeliveryState(ctx context.Context, orderID int64, couponID int64, payloadHash string, deliveryErr error) error {
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

func (s *Service) captureApprovedOrder(ctx context.Context, providerCheckoutID string) (payPalCapture, error) {
	capture, err := s.paypal.CaptureOrder(ctx, providerCheckoutID)
	if err == nil {
		return capture, nil
	}
	if !isPayPalOrderAlreadyCapturedError(err) {
		return payPalCapture{}, err
	}

	orderSnapshot, lookupErr := s.paypal.GetOrder(ctx, providerCheckoutID)
	if lookupErr != nil {
		return payPalCapture{}, lookupErr
	}
	if orderSnapshot.Status != "completed" || strings.TrimSpace(orderSnapshot.CaptureID) == "" {
		return payPalCapture{}, err
	}

	return payPalCapture{
		OrderID:      orderSnapshot.OrderID,
		CaptureID:    orderSnapshot.CaptureID,
		Status:       "captured",
		Amount:       orderSnapshot.Amount,
		CurrencyCode: orderSnapshot.CurrencyCode,
	}, nil
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

func (s *Service) enqueueFulfillmentByCheckoutReference(ctx context.Context, providerCheckoutReference string, lastStep string) error {
	order, err := s.store.GetOrderByProviderCheckoutReference(ctx, providerCheckoutReference)
	if err != nil {
		return err
	}

	return s.enqueueFulfillmentJob(ctx, order, lastStep)
}

func (s *Service) enqueueFulfillmentJob(ctx context.Context, order store.Order, lastStep string) error {
	if _, err := s.store.EnqueueFulfillmentJob(ctx, store.EnqueueFulfillmentJobParams{
		OrderID:                   order.ID,
		ProviderCheckoutReference: order.ProviderCheckoutReference,
		LastStep:                  lastStep,
	}); err != nil {
		return err
	}
	if s.fulfillmentNotifier != nil {
		s.fulfillmentNotifier.NotifyFulfillment(order.ID)
	}
	return nil
}

func (s *Service) recordPaymentEventByCheckoutReference(ctx context.Context, providerCheckoutID string, providerPaymentID string, paymentStatus string, amount int64, currencyCode string, failureMessage string) (store.Order, error) {
	if strings.TrimSpace(providerCheckoutID) == "" {
		return store.Order{}, store.ErrInvalidArgument
	}

	order, err := s.store.GetOrderByProviderCheckoutReference(ctx, providerCheckoutID)
	if err != nil {
		return store.Order{}, err
	}

	params := store.RecordPaymentEventParams{
		OrderID:            order.ID,
		ProviderName:       s.providerName,
		ProviderPaymentID:  providerPaymentID,
		ProviderCheckoutID: providerCheckoutID,
		Status:             paymentStatus,
		Amount:             amount,
		CurrencyCode:       defaultCurrency(currencyCode, order.CurrencyCode),
		FailureMessage:     failureMessage,
	}
	if paymentStatus == "captured" {
		now := time.Now().UTC()
		params.CapturedAt = &now
	}

	if _, err := s.store.RecordPaymentEvent(ctx, params); err != nil {
		return store.Order{}, err
	}

	return order, nil
}

func (s *Service) prepareFulfillment(ctx context.Context, orderID int64) (store.FulfillmentPreparation, error) {
	start := time.Now()
	preparation, err := s.store.PrepareFulfillment(ctx, orderID)
	s.logStepLatency("prepare_fulfillment", start, "order_id", orderID)
	return preparation, err
}

func (s *Service) captureAndContinueFulfillment(ctx context.Context, job store.FulfillmentJob, order store.Order) error {
	providerCheckoutReference := strings.TrimSpace(job.ProviderCheckoutReference)
	if providerCheckoutReference == "" {
		providerCheckoutReference = strings.TrimSpace(order.ProviderCheckoutReference)
	}
	if providerCheckoutReference == "" {
		return s.failFulfillmentAsPaymentIssue(ctx, job, order, order.CouponID, "payment confirmation is missing a provider checkout reference", "capture_order", store.ErrPaymentRequired)
	}

	captureStart := time.Now()
	capture, err := s.captureApprovedOrder(ctx, providerCheckoutReference)
	s.logStepLatency("paypal_capture", captureStart, "order_id", order.ID, "job_id", job.ID)
	if err != nil {
		if isRetryablePayPalError(err) {
			return s.retryOrFailFulfillment(ctx, job, "capture_order", err, order, order.CouponID, "", false)
		}
		return s.failFulfillmentAsPaymentIssue(ctx, job, order, order.CouponID, "PayPal capture could not be completed automatically", "capture_order", err)
	}

	if _, err := s.recordPaymentEventByCheckoutReference(ctx, capture.OrderID, capture.CaptureID, capture.Status, capture.Amount, capture.CurrencyCode, ""); err != nil {
		return s.retryOrFailFulfillment(ctx, job, "record_captured_payment", err, order, order.CouponID, "", false)
	}

	switch capture.Status {
	case "captured", "authorized":
		return s.ProcessFulfillmentJob(ctx, job)
	case "pending":
		return s.retryOrFailFulfillment(ctx, job, "capture_pending", fmt.Errorf("paypal capture is still pending"), order, order.CouponID, "", false)
	default:
		return s.failFulfillmentAsPaymentIssue(ctx, job, order, order.CouponID, fmt.Sprintf("PayPal capture ended in %s state", capture.Status), "capture_order", nil)
	}
}

func (s *Service) retryOrFailFulfillment(ctx context.Context, job store.FulfillmentJob, step string, err error, order store.Order, couponID *int64, payloadHash string, deliveryFailure bool) error {
	if shouldRetryFulfillment(job, err) {
		delay := fulfillmentRetryBackoff[job.AttemptCount-1]
		_, rescheduleErr := s.store.RescheduleFulfillmentJob(ctx, store.RescheduleFulfillmentJobParams{
			JobID:         job.ID,
			NextAttemptAt: time.Now().UTC().Add(delay),
			LastStep:      step,
			LastError:     err.Error(),
		})
		return rescheduleErr
	}

	if deliveryFailure && couponID != nil {
		if escalateErr := s.escalateDeliveryFailure(ctx, order.ID, *couponID, err); escalateErr != nil {
			return escalateErr
		}
		if payloadHash != "" {
			s.logger.Warn("fulfillment delivery failed permanently",
				"order_id", order.ID,
				"coupon_id", *couponID,
				"job_id", job.ID,
				"step", step,
				"payload_hash", payloadHash,
				"error", err,
			)
		}
	} else {
		if err := s.escalatePaidOrderIssue(ctx, order, couponID, truncateText(err.Error(), 255)); err != nil {
			return err
		}
	}

	_, failErr := s.store.FailFulfillmentJobTerminal(ctx, store.FailFulfillmentJobParams{
		JobID:     job.ID,
		LastStep:  step,
		LastError: err.Error(),
	})
	return failErr
}

func (s *Service) failFulfillmentAsPaymentIssue(ctx context.Context, job store.FulfillmentJob, order store.Order, couponID *int64, summary string, step string, cause error) error {
	if err := s.escalatePaidOrderIssue(ctx, order, couponID, summary); err != nil {
		return err
	}

	lastError := summary
	if cause != nil {
		lastError = cause.Error()
	}
	_, err := s.store.FailFulfillmentJobTerminal(ctx, store.FailFulfillmentJobParams{
		JobID:     job.ID,
		LastStep:  step,
		LastError: lastError,
	})
	return err
}

func (s *Service) escalateDeliveryFailure(ctx context.Context, orderID int64, couponID int64, deliveryErr error) error {
	orderIDCopy := orderID
	couponIDCopy := couponID
	_, _, err := s.store.EnsureSupportCase(ctx, store.EnsureSupportCaseParams{
		OrderID:         &orderIDCopy,
		CouponID:        &couponIDCopy,
		CaseType:        "delivery_issue",
		Priority:        "high",
		Summary:         truncateText(fmt.Sprintf("Automatic delivery escalation for order %d: %s", orderID, deliveryErr.Error()), 255),
		AssignedAdminID: "ops-job",
	})
	return err
}

func shouldRetryFulfillment(job store.FulfillmentJob, err error) bool {
	if err == nil {
		return false
	}
	if job.AttemptCount <= 0 || job.AttemptCount >= len(fulfillmentRetryBackoff) {
		return false
	}
	return isRetryablePayPalError(err) || isRetryableTelegramError(err) || isRetryableStoreError(err)
}

func isRetryableStoreError(err error) bool {
	switch {
	case errors.Is(err, store.ErrInvalidArgument),
		errors.Is(err, store.ErrListingInactive),
		errors.Is(err, store.ErrListingSoldOut),
		errors.Is(err, store.ErrOrderNotReadyForPayment),
		errors.Is(err, store.ErrOrderNotReadyForCoupon),
		errors.Is(err, store.ErrCouponAlreadyAssigned),
		errors.Is(err, store.ErrReservedCouponRequired),
		errors.Is(err, store.ErrReservedCouponInvalid),
		errors.Is(err, store.ErrPaymentRequired),
		errors.Is(err, store.ErrCouponMismatch),
		errors.Is(err, store.ErrNotFound):
		return false
	default:
		return err != nil
	}
}

func isRetryablePayPalError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *payPalAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= http.StatusInternalServerError || apiErr.StatusCode == http.StatusTooManyRequests
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func isRetryableTelegramError(err error) bool {
	if err == nil {
		return false
	}

	var tgErr tgbotapi.Error
	if errors.As(err, &tgErr) {
		return tgErr.Code >= http.StatusInternalServerError || tgErr.Code == http.StatusTooManyRequests
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func (s *Service) logStepLatency(step string, startedAt time.Time, attrs ...any) {
	if s == nil || s.logger == nil {
		return
	}

	fields := make([]any, 0, len(attrs)+2)
	fields = append(fields, "step", step, "duration_ms", time.Since(startedAt).Milliseconds())
	fields = append(fields, attrs...)
	s.logger.Info("payment step completed", fields...)
}

func buildCheckoutDescription(listing store.Listing) string {
	return truncateText(combineCheckoutLabels(listing.MerchantName, listing.Title), 127)
}

func buildCheckoutBrandName(listing store.Listing) string {
	return truncateText(strings.TrimSpace(listing.MerchantName), 127)
}

func buildCheckoutItemName(listing store.Listing) string {
	return truncateText(combineCheckoutLabels(listing.MerchantName, listing.Title), 127)
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

func combineCheckoutLabels(merchantName string, title string) string {
	merchantName = strings.TrimSpace(merchantName)
	title = strings.TrimSpace(title)

	switch {
	case merchantName == "":
		return title
	case title == "":
		return merchantName
	}

	merchantLower := strings.ToLower(merchantName)
	titleLower := strings.ToLower(title)
	switch {
	case strings.Contains(titleLower, merchantLower):
		return title
	case strings.Contains(merchantLower, titleLower):
		return merchantName
	default:
		return merchantName + " - " + title
	}
}

func buildCouponDeliveryMessage(couponCode string) string {
	return "<code>" + htmlEscape(couponCode) + "</code>"
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

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func formatMinorUnits(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	return sign + strconv.FormatInt(amount/100, 10) + "." + fmt.Sprintf("%02d", amount%100)
}
