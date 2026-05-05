package payments

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestHandleWebhookApprovedOrderEnqueuesFulfillmentJob(t *testing.T) {
	t.Parallel()

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:                        101,
			OrderNumber:               "ORD-101",
			ProviderCheckoutReference: "checkout-101",
		},
	}
	notifier := &stubFulfillmentNotifier{}

	var event payPalWebhookEvent
	event.ID = "WH-101"
	event.EventType = "CHECKOUT.ORDER.APPROVED"
	event.Resource.ID = "checkout-101"

	service := &Service{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:               mockStore,
		providerName:        "paypal",
		paypal:              &stubPayPalGateway{verifyWebhookEvent: event},
		fulfillmentNotifier: notifier,
	}

	if err := service.HandleWebhook(context.Background(), "paypal", http.Header{}, []byte(`{}`)); err != nil {
		t.Fatalf("expected webhook handling to succeed, got %v", err)
	}

	if len(mockStore.enqueuedJobs) != 1 {
		t.Fatalf("expected one fulfillment job enqueue, got %d", len(mockStore.enqueuedJobs))
	}
	if mockStore.enqueuedJobs[0].OrderID != 101 {
		t.Fatalf("expected enqueue for order 101, got %+v", mockStore.enqueuedJobs[0])
	}
	if notifier.notifiedOrderIDs[0] != 101 {
		t.Fatalf("expected notifier to receive order 101, got %v", notifier.notifiedOrderIDs)
	}
	if mockStore.recordPaymentCalls != 0 {
		t.Fatalf("expected no payment record on approved webhook, got %d calls", mockStore.recordPaymentCalls)
	}
}

func TestHandleWebhookCaptureCompletedRecordsPaymentAndEnqueuesJob(t *testing.T) {
	t.Parallel()

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:                        102,
			OrderNumber:               "ORD-102",
			ProviderCheckoutReference: "checkout-102",
			CurrencyCode:              "ILS",
			SalePriceAmount:           2499,
		},
	}

	var event payPalWebhookEvent
	event.ID = "WH-102"
	event.EventType = "PAYMENT.CAPTURE.COMPLETED"
	event.Resource.ID = "capture-102"
	event.Resource.Amount.Value = "24.99"
	event.Resource.Amount.CurrencyCode = "ILS"
	event.Resource.SupplementaryData.RelatedIDs.OrderID = "checkout-102"

	service := &Service{
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:        mockStore,
		providerName: "paypal",
		paypal:       &stubPayPalGateway{verifyWebhookEvent: event},
	}

	if err := service.HandleWebhook(context.Background(), "paypal", http.Header{}, []byte(`{}`)); err != nil {
		t.Fatalf("expected webhook handling to succeed, got %v", err)
	}

	if mockStore.recordPaymentCalls != 1 {
		t.Fatalf("expected one payment record call, got %d", mockStore.recordPaymentCalls)
	}
	if got := mockStore.recordedPayments[0]; got.Status != "captured" || got.ProviderPaymentID != "capture-102" {
		t.Fatalf("unexpected payment record params: %+v", got)
	}
	if len(mockStore.enqueuedJobs) != 1 {
		t.Fatalf("expected one fulfillment job enqueue, got %d", len(mockStore.enqueuedJobs))
	}
}

func TestProcessFulfillmentJobCapturesAndDeliversCoupon(t *testing.T) {
	t.Parallel()

	testKey := "1234567890abcdef1234567890abcdef"
	ciphertext, nonce := encryptTestCouponCode(t, testKey, "CODE-9999")

	order := store.Order{
		ID:                        103,
		UserID:                    11,
		ListingID:                 66,
		OrderNumber:               "ORD-103",
		Status:                    "pending_payment",
		CouponID:                  int64Ptr(701),
		CurrencyCode:              "ILS",
		SalePriceAmount:           1999,
		ProviderCheckoutReference: "checkout-103",
	}
	coupon := store.Coupon{
		ID:                   701,
		ListingID:            66,
		CouponCodeCiphertext: ciphertext,
		CouponCodeNonce:      nonce,
		CouponMaskedDisplay:  "***9999",
		ExpiryAt:             time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC),
	}
	user := store.User{ID: 11, TelegramUserID: 111}

	mockStore := &mockPaymentStore{
		order: order,
		preparations: []mockPreparationResult{
			{preparation: store.FulfillmentPreparation{Order: order}, err: store.ErrPaymentRequired},
			{
				preparation: store.FulfillmentPreparation{
					Order:  store.Order{ID: order.ID, UserID: order.UserID, OrderNumber: order.OrderNumber, CouponID: order.CouponID, ProviderCheckoutReference: order.ProviderCheckoutReference, Status: "delivery_pending"},
					Coupon: &coupon,
					User:   &user,
				},
			},
		},
	}
	deliverer := &stubDeliverer{}
	gateway := &stubPayPalGateway{
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

	err := service.ProcessFulfillmentJob(context.Background(), store.FulfillmentJob{
		ID:                        1,
		OrderID:                   103,
		ProviderCheckoutReference: "checkout-103",
		Status:                    "processing",
		AttemptCount:              1,
	})
	if err != nil {
		t.Fatalf("expected fulfillment processing to succeed, got %v", err)
	}

	if gateway.captureCalls != 1 {
		t.Fatalf("expected one capture call, got %d", gateway.captureCalls)
	}
	if deliverer.sendCount != 1 {
		t.Fatalf("expected one coupon delivery, got %d", deliverer.sendCount)
	}
	if len(mockStore.recordDeliveryEvents) != 1 || mockStore.recordDeliveryEvents[0].Status != "confirmed" {
		t.Fatalf("expected one confirmed delivery event, got %+v", mockStore.recordDeliveryEvents)
	}
	if len(mockStore.succeededJobs) != 1 {
		t.Fatalf("expected job success marker, got %d", len(mockStore.succeededJobs))
	}
}

func TestProcessFulfillmentJobSkipsConfirmedDelivery(t *testing.T) {
	t.Parallel()

	order := store.Order{
		ID:                        104,
		UserID:                    9,
		OrderNumber:               "ORD-104",
		Status:                    "delivered",
		ProviderCheckoutReference: "checkout-104",
	}

	mockStore := &mockPaymentStore{
		order: order,
		preparations: []mockPreparationResult{
			{
				preparation: store.FulfillmentPreparation{
					Order: order,
					Delivery: &store.CouponDelivery{
						OrderID:  104,
						CouponID: 600,
						Status:   "confirmed",
					},
				},
			},
		},
	}
	deliverer := &stubDeliverer{}

	service := &Service{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:     mockStore,
		deliverer: deliverer,
	}

	if err := service.ProcessFulfillmentJob(context.Background(), store.FulfillmentJob{ID: 2, OrderID: 104, AttemptCount: 1}); err != nil {
		t.Fatalf("expected no-op fulfillment to succeed, got %v", err)
	}
	if deliverer.sendCount != 0 {
		t.Fatalf("expected no Telegram send, got %d", deliverer.sendCount)
	}
	if len(mockStore.succeededJobs) != 1 {
		t.Fatalf("expected job success marker, got %d", len(mockStore.succeededJobs))
	}
}

func TestProcessFulfillmentJobRetriesTelegramTimeout(t *testing.T) {
	t.Parallel()

	testKey := "1234567890abcdef1234567890abcdef"
	ciphertext, nonce := encryptTestCouponCode(t, testKey, "CODE-1111")

	order := store.Order{
		ID:                        105,
		UserID:                    15,
		OrderNumber:               "ORD-105",
		Status:                    "delivery_pending",
		CouponID:                  int64Ptr(801),
		ProviderCheckoutReference: "checkout-105",
	}
	coupon := store.Coupon{
		ID:                   801,
		CouponCodeCiphertext: ciphertext,
		CouponCodeNonce:      nonce,
		ExpiryAt:             time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC),
	}
	user := store.User{ID: 15, TelegramUserID: 515}

	mockStore := &mockPaymentStore{
		order: order,
		preparations: []mockPreparationResult{
			{
				preparation: store.FulfillmentPreparation{
					Order:  order,
					Coupon: &coupon,
					User:   &user,
				},
			},
		},
	}
	deliverer := &stubDeliverer{err: timeoutErr{}}

	service := &Service{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:               mockStore,
		deliverer:           deliverer,
		couponEncryptionKey: testKey,
	}

	if err := service.ProcessFulfillmentJob(context.Background(), store.FulfillmentJob{ID: 3, OrderID: 105, AttemptCount: 1}); err != nil {
		t.Fatalf("expected retry scheduling to succeed, got %v", err)
	}

	if len(mockStore.rescheduledJobs) != 1 {
		t.Fatalf("expected one reschedule, got %d", len(mockStore.rescheduledJobs))
	}
	if len(mockStore.recordDeliveryEvents) != 1 || mockStore.recordDeliveryEvents[0].Status != "failed" {
		t.Fatalf("expected one failed delivery record, got %+v", mockStore.recordDeliveryEvents)
	}
	if mockStore.supportCaseCalls != 0 {
		t.Fatalf("expected no support case before final attempt, got %d", mockStore.supportCaseCalls)
	}
}

func TestProcessFulfillmentJobEscalatesAfterFifthDeliveryFailure(t *testing.T) {
	t.Parallel()

	testKey := "1234567890abcdef1234567890abcdef"
	ciphertext, nonce := encryptTestCouponCode(t, testKey, "CODE-2222")

	order := store.Order{
		ID:                        106,
		UserID:                    16,
		OrderNumber:               "ORD-106",
		Status:                    "delivery_pending",
		CouponID:                  int64Ptr(802),
		ProviderCheckoutReference: "checkout-106",
	}
	coupon := store.Coupon{
		ID:                   802,
		CouponCodeCiphertext: ciphertext,
		CouponCodeNonce:      nonce,
		ExpiryAt:             time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC),
	}
	user := store.User{ID: 16, TelegramUserID: 616}

	mockStore := &mockPaymentStore{
		order: order,
		preparations: []mockPreparationResult{
			{
				preparation: store.FulfillmentPreparation{
					Order:  order,
					Coupon: &coupon,
					User:   &user,
				},
			},
		},
	}
	deliverer := &stubDeliverer{err: timeoutErr{}}

	service := &Service{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:               mockStore,
		deliverer:           deliverer,
		couponEncryptionKey: testKey,
	}

	if err := service.ProcessFulfillmentJob(context.Background(), store.FulfillmentJob{ID: 4, OrderID: 106, AttemptCount: 5}); err != nil {
		t.Fatalf("expected terminal escalation handling to succeed, got %v", err)
	}

	if len(mockStore.rescheduledJobs) != 0 {
		t.Fatalf("expected no retry scheduling on fifth attempt, got %d", len(mockStore.rescheduledJobs))
	}
	if mockStore.supportCaseCalls != 1 {
		t.Fatalf("expected one support case after fifth failure, got %d", mockStore.supportCaseCalls)
	}
	if len(mockStore.failedJobs) != 1 {
		t.Fatalf("expected one terminal job failure mark, got %d", len(mockStore.failedJobs))
	}
}

func TestStartCheckoutReleasesReservationWhenProviderCreationFails(t *testing.T) {
	t.Parallel()

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:              107,
			UserID:          31,
			ListingID:       90,
			OrderNumber:     "ORD-107",
			Status:          "draft",
			CouponID:        int64Ptr(901),
			CurrencyCode:    "ILS",
			SalePriceAmount: 1299,
		},
	}
	gateway := &stubPayPalGateway{createCheckoutErr: errors.New("paypal down")}

	service := &Service{
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:        mockStore,
		providerName: "paypal",
		paypal:       gateway,
	}

	_, err := service.StartCheckout(context.Background(), mockStore.order, store.Listing{
		ID:           90,
		MerchantName: "Cafe",
		Title:        "ILS 20 coupon",
	})
	if err == nil {
		t.Fatal("expected checkout creation to fail")
	}

	if len(mockStore.releaseCalls) != 1 {
		t.Fatalf("expected one checkout release call, got %d", len(mockStore.releaseCalls))
	}
	if mockStore.releaseCalls[0].FailureReason != "checkout_provider_create_failed" {
		t.Fatalf("unexpected failure reason: %+v", mockStore.releaseCalls[0])
	}
}

func TestStartCheckoutSendsMerchantAndCouponMetadataToPayPal(t *testing.T) {
	t.Parallel()

	mockStore := &mockPaymentStore{
		order: store.Order{
			ID:              108,
			UserID:          32,
			ListingID:       91,
			OrderNumber:     "ORD-108",
			Status:          "draft",
			CouponID:        int64Ptr(902),
			CurrencyCode:    "ILS",
			SalePriceAmount: 2800,
		},
	}
	gateway := &stubPayPalGateway{
		createCheckout: payPalCheckout{
			OrderID:     "pp-order-108",
			ApprovalURL: "https://paypal.example/approve",
		},
	}

	service := &Service{
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:        mockStore,
		providerName: "paypal",
		appBaseURL:   "https://coupons.example.com",
		paypal:       gateway,
	}

	_, err := service.StartCheckout(context.Background(), mockStore.order, store.Listing{
		ID:           91,
		MerchantName: "Test Store",
		Title:        "ג‚×50 coupon",
	})
	if err != nil {
		t.Fatalf("expected checkout creation to succeed, got %v", err)
	}

	if gateway.lastCreateCheckoutInput.BrandName != "Test Store" {
		t.Fatalf("expected brand name to include merchant, got %q", gateway.lastCreateCheckoutInput.BrandName)
	}
	if gateway.lastCreateCheckoutInput.ItemName != "Test Store - ג‚×50 coupon" {
		t.Fatalf("expected item name to include merchant and coupon title, got %q", gateway.lastCreateCheckoutInput.ItemName)
	}
	if gateway.lastCreateCheckoutInput.Description != "Test Store - ג‚×50 coupon" {
		t.Fatalf("expected description to include merchant and coupon title, got %q", gateway.lastCreateCheckoutInput.Description)
	}
}

func TestBuildCheckoutItemImageURLUsesPublicAppBaseURL(t *testing.T) {
	t.Parallel()

	got := buildCheckoutItemImageURL("https://coupons.example.com", store.Listing{
		PhotoKey: "deal.jpg",
	})

	if got != "https://coupons.example.com/assets/coupons/deal.jpg" {
		t.Fatalf("expected public image url, got %q", got)
	}
}

func TestBuildCheckoutItemImageURLSkipsLocalhost(t *testing.T) {
	t.Parallel()

	got := buildCheckoutItemImageURL("http://localhost:8080", store.Listing{
		PhotoKey: "deal.jpg",
	})

	if got != "" {
		t.Fatalf("expected empty image url for localhost, got %q", got)
	}
}

func TestBuildCheckoutItemSummaryFallsBackToRedemptionInstructions(t *testing.T) {
	t.Parallel()

	got := buildCheckoutItemSummary(store.Listing{
		RedemptionInstructions: "Show the QR code at checkout.",
	})

	if got != "Show the QR code at checkout." {
		t.Fatalf("expected redemption instructions fallback, got %q", got)
	}
}

func TestBuildCheckoutItemNameAvoidsDuplicateMerchantLabel(t *testing.T) {
	t.Parallel()

	got := buildCheckoutItemName(store.Listing{
		MerchantName: "Test Store",
		Title:        "Test Store ג‚×50 coupon",
	})

	if got != "Test Store ג‚×50 coupon" {
		t.Fatalf("expected duplicate merchant name to be avoided, got %q", got)
	}
}

func TestBuildCouponDeliveryMessageReturnsOnlyCouponCode(t *testing.T) {
	t.Parallel()

	got := buildCouponDeliveryMessage(`ABC<&>"'123`)
	want := "<code>ABC&lt;&amp;&gt;&quot;&#39;123</code>"

	if got != want {
		t.Fatalf("expected minimal coupon-only delivery message, got %q", got)
	}
}

type mockPreparationResult struct {
	preparation store.FulfillmentPreparation
	err         error
}

type mockPaymentStore struct {
	order                store.Order
	preparations         []mockPreparationResult
	recordedPayments     []store.RecordPaymentEventParams
	recordPaymentCalls   int
	recordDeliveryEvents []store.RecordDeliveryEventParams
	releaseCalls         []store.ReleaseCheckoutReservationParams
	enqueuedJobs         []store.EnqueueFulfillmentJobParams
	succeededJobs        []store.SucceedFulfillmentJobParams
	rescheduledJobs      []store.RescheduleFulfillmentJobParams
	failedJobs           []store.FailFulfillmentJobParams
	supportCaseCalls     int
}

func (m *mockPaymentStore) MarkOrderPendingPayment(ctx context.Context, params store.MarkOrderPendingPaymentParams) (store.Order, error) {
	m.order.Status = "pending_payment"
	m.order.ProviderCheckoutReference = params.ProviderCheckoutReference
	return m.order, nil
}

func (m *mockPaymentStore) ReleaseCheckoutReservation(ctx context.Context, params store.ReleaseCheckoutReservationParams) (store.Order, error) {
	m.releaseCalls = append(m.releaseCalls, params)
	m.order.CouponID = nil
	m.order.Status = params.OrderStatus
	m.order.FailureReason = params.FailureReason
	return m.order, nil
}

func (m *mockPaymentStore) RecordPaymentEvent(ctx context.Context, params store.RecordPaymentEventParams) (store.Payment, error) {
	m.recordPaymentCalls++
	m.recordedPayments = append(m.recordedPayments, params)
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

func (m *mockPaymentStore) GetOrder(ctx context.Context, orderID int64) (store.Order, error) {
	return m.order, nil
}

func (m *mockPaymentStore) PrepareFulfillment(ctx context.Context, orderID int64) (store.FulfillmentPreparation, error) {
	if len(m.preparations) == 0 {
		return store.FulfillmentPreparation{Order: m.order}, nil
	}
	result := m.preparations[0]
	m.preparations = m.preparations[1:]
	return result.preparation, result.err
}

func (m *mockPaymentStore) RecordDeliveryEvent(ctx context.Context, params store.RecordDeliveryEventParams) (store.CouponDelivery, error) {
	m.recordDeliveryEvents = append(m.recordDeliveryEvents, params)
	return store.CouponDelivery{
		OrderID:             params.OrderID,
		CouponID:            params.CouponID,
		Status:              params.Status,
		TelegramMessageID:   params.TelegramMessageID,
		DeliveryPayloadHash: params.DeliveryPayloadHash,
		SentAt:              params.SentAt,
		ConfirmedAt:         params.ConfirmedAt,
		FailureReason:       params.FailureReason,
	}, nil
}

func (m *mockPaymentStore) EnsureSupportCase(ctx context.Context, params store.EnsureSupportCaseParams) (store.SupportCase, bool, error) {
	m.supportCaseCalls++
	return store.SupportCase{ID: int64(m.supportCaseCalls)}, true, nil
}

func (m *mockPaymentStore) EnqueueFulfillmentJob(ctx context.Context, params store.EnqueueFulfillmentJobParams) (store.FulfillmentJob, error) {
	m.enqueuedJobs = append(m.enqueuedJobs, params)
	return store.FulfillmentJob{ID: int64(len(m.enqueuedJobs)), OrderID: params.OrderID, ProviderCheckoutReference: params.ProviderCheckoutReference}, nil
}

func (m *mockPaymentStore) MarkFulfillmentJobSucceeded(ctx context.Context, params store.SucceedFulfillmentJobParams) (store.FulfillmentJob, error) {
	m.succeededJobs = append(m.succeededJobs, params)
	return store.FulfillmentJob{ID: params.JobID, Status: "succeeded"}, nil
}

func (m *mockPaymentStore) RescheduleFulfillmentJob(ctx context.Context, params store.RescheduleFulfillmentJobParams) (store.FulfillmentJob, error) {
	m.rescheduledJobs = append(m.rescheduledJobs, params)
	return store.FulfillmentJob{ID: params.JobID, Status: "retry_scheduled", NextAttemptAt: params.NextAttemptAt}, nil
}

func (m *mockPaymentStore) FailFulfillmentJobTerminal(ctx context.Context, params store.FailFulfillmentJobParams) (store.FulfillmentJob, error) {
	m.failedJobs = append(m.failedJobs, params)
	return store.FulfillmentJob{ID: params.JobID, Status: "failed_terminal"}, nil
}

type stubDeliverer struct {
	sendCount int
	err       error
}

func (s *stubDeliverer) SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error) {
	s.sendCount++
	if s.err != nil {
		return 0, s.err
	}
	return int64(9000 + s.sendCount), nil
}

type stubFulfillmentNotifier struct {
	notifiedOrderIDs []int64
}

func (s *stubFulfillmentNotifier) NotifyFulfillment(orderID int64) {
	s.notifiedOrderIDs = append(s.notifiedOrderIDs, orderID)
}

type stubPayPalGateway struct {
	verifyWebhookEvent      payPalWebhookEvent
	verifyWebhookErr        error
	orderSnapshot           payPalOrderSnapshot
	fallbackOrderSnapshot   payPalOrderSnapshot
	capture                 payPalCapture
	createCheckout          payPalCheckout
	createCheckoutErr       error
	captureErr              error
	getOrderErr             error
	captureCalls            int
	getOrderCalls           int
	lastCreateCheckoutInput payPalCreateCheckoutInput
}

func (s *stubPayPalGateway) CreateCheckout(ctx context.Context, input payPalCreateCheckoutInput) (payPalCheckout, error) {
	s.lastCreateCheckoutInput = input
	return s.createCheckout, s.createCheckoutErr
}

func (s *stubPayPalGateway) VerifyAndParseWebhook(ctx context.Context, headers http.Header, body []byte) (payPalWebhookEvent, error) {
	return s.verifyWebhookEvent, s.verifyWebhookErr
}

func (s *stubPayPalGateway) CaptureOrder(ctx context.Context, orderID string) (payPalCapture, error) {
	s.captureCalls++
	if s.captureErr != nil {
		return payPalCapture{}, s.captureErr
	}
	return s.capture, nil
}

func (s *stubPayPalGateway) GetOrder(ctx context.Context, orderID string) (payPalOrderSnapshot, error) {
	s.getOrderCalls++
	if s.getOrderErr != nil {
		return payPalOrderSnapshot{}, s.getOrderErr
	}
	if s.getOrderCalls > 1 && strings.TrimSpace(s.fallbackOrderSnapshot.OrderID) != "" {
		return s.fallbackOrderSnapshot, nil
	}
	return s.orderSnapshot, nil
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

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

func int64Ptr(value int64) *int64 {
	return &value
}
