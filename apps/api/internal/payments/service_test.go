package payments

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestProcessCaptureDeliversOnlyOnceAcrossDuplicateWebhooks(t *testing.T) {
	t.Parallel()

	testKey := "1234567890abcdef1234567890abcdef"
	ciphertext, nonce := encryptTestCouponCode(t, testKey, "CODE-1234")

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:                        101,
			UserID:                    7,
			ListingID:                 88,
			OrderNumber:               "ORD-101",
			Status:                    "pending_payment",
			CurrencyCode:              "ILS",
			SalePriceAmount:           2599,
			ProviderCheckoutReference: "checkout-101",
			FinalSaleAcknowledgedAt:   timePtr(time.Date(2026, time.April, 23, 9, 0, 0, 0, time.UTC)),
		},
		user: store.User{ID: 7, TelegramUserID: 777, DisplayName: "Buyer"},
		listing: store.Listing{
			ID:                     88,
			MerchantName:           "Coffee Shop",
			Title:                  "ILS 50 coupon",
			RedemptionInstructions: "Show the code at checkout.",
		},
		coupon: store.Coupon{
			ID:                   501,
			ListingID:            88,
			CouponCodeCiphertext: ciphertext,
			CouponCodeNonce:      nonce,
			CouponMaskedDisplay:  "***1234",
			ExpiryAt:             time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
	}
	deliverer := &stubDeliverer{}

	service := &Service{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:               mockStore,
		deliverer:           deliverer,
		providerName:        "paypal",
		couponEncryptionKey: testKey,
	}

	ctx := context.Background()
	if err := service.processCapture(ctx, "checkout-101", "capture-1", "captured", 2599, "ILS"); err != nil {
		t.Fatalf("first capture failed: %v", err)
	}
	if err := service.processCapture(ctx, "checkout-101", "capture-1", "captured", 2599, "ILS"); err != nil {
		t.Fatalf("duplicate capture failed: %v", err)
	}

	if deliverer.sendCount != 1 {
		t.Fatalf("expected one delivery send, got %d", deliverer.sendCount)
	}
	if mockStore.assignCalls != 1 {
		t.Fatalf("expected one coupon assignment, got %d", mockStore.assignCalls)
	}
	if len(mockStore.recordDeliveryEvents) != 1 {
		t.Fatalf("expected one delivery record, got %d", len(mockStore.recordDeliveryEvents))
	}
}

func TestFulfillPaidOrderSkipsConfirmedDelivery(t *testing.T) {
	t.Parallel()

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:                        102,
			UserID:                    9,
			ListingID:                 77,
			OrderNumber:               "ORD-102",
			Status:                    "delivered",
			ProviderCheckoutReference: "checkout-102",
		},
		delivery: &store.CouponDelivery{
			OrderID:  102,
			CouponID: 600,
			Status:   "confirmed",
		},
	}
	deliverer := &stubDeliverer{}

	service := &Service{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:     mockStore,
		deliverer: deliverer,
	}

	if err := service.fulfillPaidOrder(context.Background(), 102); err != nil {
		t.Fatalf("expected fulfillPaidOrder to no-op, got %v", err)
	}
	if deliverer.sendCount != 0 {
		t.Fatalf("expected no message send, got %d", deliverer.sendCount)
	}
	if mockStore.assignCalls != 0 {
		t.Fatalf("expected no assignment, got %d", mockStore.assignCalls)
	}
}

func TestReconcilePendingOrderCapturesApprovedOrder(t *testing.T) {
	t.Parallel()

	testKey := "1234567890abcdef1234567890abcdef"
	ciphertext, nonce := encryptTestCouponCode(t, testKey, "CODE-9999")

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:                        103,
			UserID:                    11,
			ListingID:                 66,
			OrderNumber:               "ORD-103",
			Status:                    "pending_payment",
			CurrencyCode:              "ILS",
			SalePriceAmount:           1999,
			ProviderCheckoutReference: "checkout-103",
		},
		user: store.User{ID: 11, TelegramUserID: 111},
		listing: store.Listing{
			ID:                     66,
			MerchantName:           "Bakery",
			Title:                  "ILS 30 coupon",
			RedemptionInstructions: "Use in store.",
		},
		coupon: store.Coupon{
			ID:                   701,
			ListingID:            66,
			CouponCodeCiphertext: ciphertext,
			CouponCodeNonce:      nonce,
			CouponMaskedDisplay:  "***9999",
			ExpiryAt:             time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC),
		},
	}
	deliverer := &stubDeliverer{}
	gateway := &stubPayPalGateway{
		orderSnapshot: payPalOrderSnapshot{
			OrderID:      "checkout-103",
			Status:       "approved",
			Amount:       1999,
			CurrencyCode: "ILS",
		},
		capture: payPalCapture{
			OrderID:      "checkout-103",
			CaptureID:    "capture-103",
			Status:       "captured",
			Amount:       1999,
			CurrencyCode: "ILS",
		},
	}

	service := &Service{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:               mockStore,
		deliverer:           deliverer,
		providerName:        "paypal",
		couponEncryptionKey: testKey,
		paypal:              gateway,
	}

	err := service.ReconcilePendingOrder(context.Background(), store.PendingPaymentReconciliationCandidate{
		OrderID:                   103,
		OrderNumber:               "ORD-103",
		Status:                    "pending_payment",
		ProviderCheckoutReference: "checkout-103",
	})
	if err != nil {
		t.Fatalf("expected reconciliation to succeed, got %v", err)
	}

	if gateway.captureCalls != 1 {
		t.Fatalf("expected one capture call, got %d", gateway.captureCalls)
	}
	if deliverer.sendCount != 1 {
		t.Fatalf("expected one coupon delivery, got %d", deliverer.sendCount)
	}
}

type mockPaymentStore struct {
	order                store.Order
	user                 store.User
	listing              store.Listing
	coupon               store.Coupon
	delivery             *store.CouponDelivery
	assignCalls          int
	recordDeliveryEvents []store.RecordDeliveryEventParams
}

func (m *mockPaymentStore) MarkOrderPendingPayment(ctx context.Context, params store.MarkOrderPendingPaymentParams) (store.Order, error) {
	m.order.Status = "pending_payment"
	m.order.ProviderCheckoutReference = params.ProviderCheckoutReference
	return m.order, nil
}

func (m *mockPaymentStore) RecordPaymentEvent(ctx context.Context, params store.RecordPaymentEventParams) (store.Payment, error) {
	switch params.Status {
	case "captured", "authorized":
		m.order.Status = "paid"
	default:
		m.order.Status = params.Status
	}
	return store.Payment{
		OrderID:            params.OrderID,
		ProviderName:       params.ProviderName,
		ProviderPaymentID:  params.ProviderPaymentID,
		ProviderCheckoutID: params.ProviderCheckoutID,
		Status:             params.Status,
		Amount:             params.Amount,
		CurrencyCode:       params.CurrencyCode,
		CapturedAt:         params.CapturedAt,
	}, nil
}

func (m *mockPaymentStore) GetOrderByProviderCheckoutReference(ctx context.Context, providerCheckoutReference string) (store.Order, error) {
	return m.order, nil
}

func (m *mockPaymentStore) GetCouponDeliveryForOrder(ctx context.Context, orderID int64) (*store.CouponDelivery, error) {
	return m.delivery, nil
}

func (m *mockPaymentStore) GetOrder(ctx context.Context, orderID int64) (store.Order, error) {
	return m.order, nil
}

func (m *mockPaymentStore) AssignAvailableCoupon(ctx context.Context, orderID int64) (store.Coupon, error) {
	m.assignCalls++
	couponID := m.coupon.ID
	m.order.CouponID = &couponID
	m.order.Status = "delivery_pending"
	return m.coupon, nil
}

func (m *mockPaymentStore) GetCoupon(ctx context.Context, couponID int64) (store.Coupon, error) {
	return m.coupon, nil
}

func (m *mockPaymentStore) GetUserByID(ctx context.Context, userID int64) (store.User, error) {
	return m.user, nil
}

func (m *mockPaymentStore) GetListing(ctx context.Context, listingID int64) (store.Listing, error) {
	return m.listing, nil
}

func (m *mockPaymentStore) RecordDeliveryEvent(ctx context.Context, params store.RecordDeliveryEventParams) (store.CouponDelivery, error) {
	m.recordDeliveryEvents = append(m.recordDeliveryEvents, params)
	m.delivery = &store.CouponDelivery{
		OrderID:             params.OrderID,
		CouponID:            params.CouponID,
		Status:              params.Status,
		TelegramMessageID:   params.TelegramMessageID,
		DeliveryPayloadHash: params.DeliveryPayloadHash,
		SentAt:              params.SentAt,
		ConfirmedAt:         params.ConfirmedAt,
		FailureReason:       params.FailureReason,
	}
	m.order.Status = "delivered"
	return *m.delivery, nil
}

func (m *mockPaymentStore) EnsureSupportCase(ctx context.Context, params store.EnsureSupportCaseParams) (store.SupportCase, bool, error) {
	return store.SupportCase{ID: 1}, true, nil
}

type stubDeliverer struct {
	sendCount int
}

func (s *stubDeliverer) SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error) {
	s.sendCount++
	return int64(9000 + s.sendCount), nil
}

type stubPayPalGateway struct {
	orderSnapshot payPalOrderSnapshot
	capture       payPalCapture
	captureCalls  int
}

func (s *stubPayPalGateway) CreateCheckout(ctx context.Context, input payPalCreateCheckoutInput) (payPalCheckout, error) {
	return payPalCheckout{}, nil
}

func (s *stubPayPalGateway) VerifyAndParseWebhook(ctx context.Context, headers http.Header, body []byte) (payPalWebhookEvent, error) {
	return payPalWebhookEvent{}, nil
}

func (s *stubPayPalGateway) CaptureOrder(ctx context.Context, orderID string) (payPalCapture, error) {
	s.captureCalls++
	return s.capture, nil
}

func (s *stubPayPalGateway) GetOrder(ctx context.Context, orderID string) (payPalOrderSnapshot, error) {
	return s.orderSnapshot, nil
}

func encryptTestCouponCode(t *testing.T, key string, couponCode string) ([]byte, []byte) {
	t.Helper()

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new gcm: %v", err)
	}

	nonce := []byte("nonce-123456")
	ciphertext := gcm.Seal(nil, nonce, []byte(couponCode), nil)
	return ciphertext, nonce
}

func timePtr(value time.Time) *time.Time {
	return &value
}
