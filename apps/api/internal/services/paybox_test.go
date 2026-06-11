package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestPayBoxServiceStartPaymentCreatesAwaitingPaymentOrder(t *testing.T) {
	t.Parallel()

	payboxStore := &stubPayBoxStore{
		offer: store.Offer{
			ID:                     77,
			MerchantName:           "Cafe Local",
			Title:                  "Breakfast deal",
			PaymentLink:            "https://paybox.example/cafe",
			MerchantDisclosureText: "Sold with Cafe Local approval.",
			RedemptionTerms:        "Show the code at checkout.",
			SupportContact:         "@cafe_support",
		},
		order: store.MVPOrder{
			ID:                501,
			UserID:            11,
			OfferID:           77,
			OrderNumber:       "PB-501",
			OfferTitle:        "Breakfast deal",
			MerchantName:      "Cafe Local",
			PriceAmount:       4900,
			CurrencyCode:      "ILS",
			PayBoxPaymentLink: "https://paybox.example/cafe",
			RedemptionTerms:   "Show the code at checkout.",
			SupportContact:    "@cafe_support",
		},
	}

	service := newTestPayBoxService(t, payboxStore, &stubPayBoxMessenger{}, nil, nil, fixedOrderNumberer{value: "PB-501"})
	start, err := service.StartPayment(context.Background(), 11, 77)
	if err != nil {
		t.Fatalf("expected start payment to succeed, got %v", err)
	}

	if payboxStore.createdOrder.OrderNumber != "PB-501" || payboxStore.createdOrder.UserID != 11 || payboxStore.createdOrder.OfferID != 77 {
		t.Fatalf("unexpected created order params: %+v", payboxStore.createdOrder)
	}
	if start.PaymentLink != "https://paybox.example/cafe" {
		t.Fatalf("expected PayBox link in response, got %q", start.PaymentLink)
	}
	if !strings.Contains(start.NextStepMessage, "payment screenshot") {
		t.Fatalf("expected claim instructions, got %q", start.NextStepMessage)
	}
	if start.MerchantDisclosureText != "Sold with Cafe Local approval." {
		t.Fatalf("expected disclosure snapshot, got %q", start.MerchantDisclosureText)
	}
}

func TestPayBoxServiceSubmitPaymentClaimNotifiesAdmin(t *testing.T) {
	t.Parallel()

	payboxStore := &stubPayBoxStore{
		claim: store.ManualPaymentClaim{
			ID:                      901,
			OrderID:                 501,
			PayerUsername:           "telegram:1111",
			ClaimedAmount:           4900,
			PaymentScreenshotFileID: "photo-file-id",
		},
	}
	admin := &stubPayBoxAdminNotifier{}
	service := newTestPayBoxService(t, payboxStore, &stubPayBoxMessenger{}, admin, nil, fixedOrderNumberer{})

	messageID := int64(44)
	claim, err := service.SubmitPaymentClaim(context.Background(), 501, PayBoxPaymentEvidence{
		PayerReference:              "telegram:1111",
		ScreenshotFileID:            "photo-file-id",
		ScreenshotUniqueID:          "photo-unique-id",
		ScreenshotTelegramMessageID: &messageID,
		ScreenshotCaption:           "paid",
	}, 0)
	if err != nil {
		t.Fatalf("expected claim submission to succeed, got %v", err)
	}

	if claim.ID != 901 {
		t.Fatalf("expected claim 901, got %+v", claim)
	}
	if payboxStore.submittedClaim.PayerUsername != "telegram:1111" || payboxStore.submittedClaim.PaymentScreenshotFileID != "photo-file-id" {
		t.Fatalf("unexpected submitted claim params: %+v", payboxStore.submittedClaim)
	}
	if len(admin.submittedClaims) != 1 || admin.submittedClaims[0].ID != 901 {
		t.Fatalf("expected one admin notification, got %+v", admin.submittedClaims)
	}
}

func TestPayBoxServiceApproveClaimDeliversCodeAndNotifiesAdmin(t *testing.T) {
	t.Parallel()

	codeID := int64(7001)
	payboxStore := &stubPayBoxStore{
		user: store.User{ID: 11, TelegramUserID: 1111},
		approval: store.ApproveManualPaymentClaimResult{
			Order: store.MVPOrder{
				ID:               501,
				UserID:           11,
				OrderNumber:      "PB-501",
				PredefinedCodeID: &codeID,
				RedemptionTerms:  "Show the code at checkout.",
				SupportContact:   "@support",
			},
			Claim: store.ManualPaymentClaim{ID: 901, OrderID: 501},
			Code:  &store.PredefinedCode{ID: codeID, CodeEncrypted: []byte("cipher"), CodeMaskedDisplay: "***1234"},
		},
		delivery:   store.MVPDelivery{ID: 3001, OrderID: 501, PredefinedCodeID: codeID, Status: "confirmed"},
		redemption: store.CouponRedemption{ID: 4001, OrderID: 501, PredefinedCodeID: codeID, RedemptionToken: "token-1", Status: "issued"},
	}
	messenger := &stubPayBoxMessenger{}
	admin := &stubPayBoxAdminNotifier{}
	renderer := stubPayBoxCodeRenderer{code: "CODE-1234"}
	service := newTestPayBoxService(t, payboxStore, messenger, admin, renderer, fixedOrderNumberer{})

	result, err := service.ApproveClaim(context.Background(), 901, "admin-1", "matched")
	if err != nil {
		t.Fatalf("expected approval to succeed, got %v", err)
	}

	if result.Delivery == nil || result.Delivery.ID != 3001 {
		t.Fatalf("expected delivery result, got %+v", result.Delivery)
	}
	if result.Redemption == nil || result.Redemption.RedemptionToken != "token-1" {
		t.Fatalf("expected redemption result, got %+v", result.Redemption)
	}
	if len(messenger.photos) != 1 || !strings.Contains(messenger.photos[0].caption, "Show this QR code") {
		t.Fatalf("expected qr delivery message, got %+v", messenger.photos)
	}
	if messenger.photos[0].telegramUserID != 1111 {
		t.Fatalf("expected qr delivery to buyer telegram id 1111, got %+v", messenger.photos[0])
	}
	if payboxStore.recordedDelivery.Status != "confirmed" || payboxStore.recordedDelivery.PredefinedCodeID != codeID {
		t.Fatalf("unexpected recorded delivery params: %+v", payboxStore.recordedDelivery)
	}
	if len(admin.approvedResults) != 1 || admin.approvedResults[0].Order.ID != 501 {
		t.Fatalf("expected admin approval notification, got %+v", admin.approvedResults)
	}
	if len(payboxStore.recordedActions) != 1 || payboxStore.recordedActions[0].ActionType != "approve_payment_claim" {
		t.Fatalf("expected approval audit action, got %+v", payboxStore.recordedActions)
	}
}

func TestPayBoxServiceApproveAlreadySentClaimDoesNotRedeliver(t *testing.T) {
	t.Parallel()

	codeID := int64(7001)
	payboxStore := &stubPayBoxStore{
		user: store.User{ID: 11, TelegramUserID: 1111},
		approval: store.ApproveManualPaymentClaimResult{
			Order: store.MVPOrder{
				ID:               501,
				UserID:           11,
				OrderNumber:      "PB-501",
				Status:           "coupon_sent",
				PredefinedCodeID: &codeID,
			},
			Claim: store.ManualPaymentClaim{ID: 901, OrderID: 501},
			Code:  &store.PredefinedCode{ID: codeID, CodeEncrypted: []byte("cipher"), CodeMaskedDisplay: "***1234"},
		},
	}
	messenger := &stubPayBoxMessenger{}
	admin := &stubPayBoxAdminNotifier{}
	service := newTestPayBoxService(t, payboxStore, messenger, admin, stubPayBoxCodeRenderer{code: "CODE-1234"}, fixedOrderNumberer{})

	result, err := service.ApproveClaim(context.Background(), 901, "admin-1", "matched")
	if err != nil {
		t.Fatalf("expected already-sent approval to succeed, got %v", err)
	}

	if !result.AlreadyDelivered {
		t.Fatal("expected already delivered result")
	}
	if len(messenger.photos) != 0 {
		t.Fatalf("expected no repeated qr delivery, got %+v", messenger.photos)
	}
	if payboxStore.createdRedemption.OrderID != 0 || payboxStore.recordedDelivery.OrderID != 0 {
		t.Fatalf("expected no redemption or delivery record, got redemption=%+v delivery=%+v", payboxStore.createdRedemption, payboxStore.recordedDelivery)
	}
	if len(admin.approvedResults) != 1 || !admin.approvedResults[0].AlreadyDelivered {
		t.Fatalf("expected admin already-delivered notification, got %+v", admin.approvedResults)
	}
}

func TestPayBoxServiceApproveClaimWithoutCodeNotifiesSupportState(t *testing.T) {
	t.Parallel()

	payboxStore := &stubPayBoxStore{
		user: store.User{ID: 11, TelegramUserID: 1111},
		approval: store.ApproveManualPaymentClaimResult{
			Order: store.MVPOrder{ID: 501, UserID: 11, OrderNumber: "PB-501"},
			Claim: store.ManualPaymentClaim{ID: 901, OrderID: 501},
		},
	}
	messenger := &stubPayBoxMessenger{}
	admin := &stubPayBoxAdminNotifier{}
	service := newTestPayBoxService(t, payboxStore, messenger, admin, stubPayBoxCodeRenderer{}, fixedOrderNumberer{})

	result, err := service.ApproveClaim(context.Background(), 901, "admin-1", "matched")
	if err != nil {
		t.Fatalf("expected approval without code to succeed, got %v", err)
	}

	if !result.SupportState {
		t.Fatal("expected support state")
	}
	if len(messenger.messages) != 1 || !strings.Contains(messenger.messages[0], "manual support") {
		t.Fatalf("expected support message, got %+v", messenger.messages)
	}
	if len(admin.supportResults) != 1 {
		t.Fatalf("expected support notification, got %+v", admin.supportResults)
	}
}

func TestPayBoxServiceRejectClaimNotifiesBuyerAndAdmin(t *testing.T) {
	t.Parallel()

	payboxStore := &stubPayBoxStore{
		user:          store.User{ID: 11, TelegramUserID: 1111},
		rejectedClaim: store.ManualPaymentClaim{ID: 901, OrderID: 501},
		rejectedOrder: store.MVPOrder{ID: 501, UserID: 11, OrderNumber: "PB-501"},
	}
	messenger := &stubPayBoxMessenger{}
	admin := &stubPayBoxAdminNotifier{}
	service := newTestPayBoxService(t, payboxStore, messenger, admin, nil, fixedOrderNumberer{})

	claim, order, err := service.RejectClaim(context.Background(), 901, "admin-1", "not matched")
	if err != nil {
		t.Fatalf("expected rejection to succeed, got %v", err)
	}

	if claim.ID != 901 || order.ID != 501 {
		t.Fatalf("unexpected rejection result: %+v %+v", claim, order)
	}
	if len(messenger.messages) != 1 || !strings.Contains(messenger.messages[0], "could not match") {
		t.Fatalf("expected buyer rejection message, got %+v", messenger.messages)
	}
	if len(admin.rejectedClaims) != 1 {
		t.Fatalf("expected admin rejection notification, got %+v", admin.rejectedClaims)
	}
	if len(payboxStore.recordedActions) != 1 || payboxStore.recordedActions[0].ActionType != "reject_payment_claim" {
		t.Fatalf("expected rejection audit action, got %+v", payboxStore.recordedActions)
	}
}

func TestPayBoxServiceRecordsFailedDelivery(t *testing.T) {
	t.Parallel()

	codeID := int64(7001)
	payboxStore := &stubPayBoxStore{
		user: store.User{ID: 11, TelegramUserID: 1111},
	}
	messenger := &stubPayBoxMessenger{err: errors.New("telegram down")}
	renderer := stubPayBoxCodeRenderer{code: "CODE-1234"}
	payboxStore.redemption = store.CouponRedemption{ID: 4001, OrderID: 501, PredefinedCodeID: codeID, RedemptionToken: "token-1"}
	service := newTestPayBoxService(t, payboxStore, messenger, nil, renderer, fixedOrderNumberer{})

	_, err := service.DeliverApprovedCode(context.Background(), store.MVPOrder{
		ID:               501,
		UserID:           11,
		PredefinedCodeID: &codeID,
	}, store.PredefinedCode{ID: codeID})
	if err == nil {
		t.Fatal("expected delivery to fail")
	}

	if payboxStore.recordedDelivery.Status != "failed" || payboxStore.recordedDelivery.FailureReason != "telegram down" {
		t.Fatalf("expected failed delivery record, got %+v", payboxStore.recordedDelivery)
	}
}

func TestBuildPayBoxQRMessageEscapesCodeAndTerms(t *testing.T) {
	t.Parallel()

	got := buildPayBoxQRMessage(`A<&>"'`, "Use <today>", "@support", "")
	if !strings.Contains(got, "<code>A&lt;&amp;&gt;&quot;&#39;</code>") {
		t.Fatalf("expected escaped code, got %q", got)
	}
	if !strings.Contains(got, "Use &lt;today&gt;") {
		t.Fatalf("expected escaped terms, got %q", got)
	}
}

func newTestPayBoxService(t *testing.T, store PayBoxStore, messenger PayBoxMessenger, admin PayBoxAdminNotifier, renderer PayBoxCodeRenderer, orderNumberer OrderNumberGenerator) *PayBoxService {
	t.Helper()

	service, err := NewPayBoxService(PayBoxServiceOptions{
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:             store,
		Messenger:         messenger,
		AdminNotifier:     admin,
		CodeRenderer:      renderer,
		QRRenderer:        stubPayBoxQRRenderer{},
		OrderNumberer:     orderNumberer,
		SupportContact:    "@support",
		RedemptionBaseURL: "https://api.example",
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service
}

type stubPayBoxStore struct {
	offer             store.Offer
	order             store.MVPOrder
	claim             store.ManualPaymentClaim
	approval          store.ApproveManualPaymentClaimResult
	rejectedClaim     store.ManualPaymentClaim
	rejectedOrder     store.MVPOrder
	redemption        store.CouponRedemption
	delivery          store.MVPDelivery
	user              store.User
	createdOrder      store.CreateAwaitingPaymentOrderParams
	submittedClaim    store.SubmitManualPaymentClaimParams
	reviewedClaim     store.ReviewManualPaymentClaimParams
	recordedDelivery  store.RecordPredefinedCodeDeliveryEventParams
	createdRedemption store.CreateCouponRedemptionParams
	recordedActions   []store.RecordAdminActionParams
	pendingClaims     []store.ManualPaymentClaim
}

func (s *stubPayBoxStore) ListActiveOffers(ctx context.Context, limit int) ([]store.Offer, error) {
	return []store.Offer{s.offer}, nil
}

func (s *stubPayBoxStore) GetOffer(ctx context.Context, offerID int64) (store.Offer, error) {
	return s.offer, nil
}

func (s *stubPayBoxStore) CreateAwaitingPaymentOrder(ctx context.Context, params store.CreateAwaitingPaymentOrderParams) (store.MVPOrder, error) {
	s.createdOrder = params
	return s.order, nil
}

func (s *stubPayBoxStore) SubmitManualPaymentClaim(ctx context.Context, params store.SubmitManualPaymentClaimParams) (store.ManualPaymentClaim, error) {
	s.submittedClaim = params
	return s.claim, nil
}

func (s *stubPayBoxStore) ListPendingManualPaymentClaims(ctx context.Context, limit int) ([]store.ManualPaymentClaim, error) {
	return s.pendingClaims, nil
}

func (s *stubPayBoxStore) ApproveManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ApproveManualPaymentClaimResult, error) {
	s.reviewedClaim = params
	return s.approval, nil
}

func (s *stubPayBoxStore) RejectManualPaymentClaim(ctx context.Context, params store.ReviewManualPaymentClaimParams) (store.ManualPaymentClaim, store.MVPOrder, error) {
	s.reviewedClaim = params
	return s.rejectedClaim, s.rejectedOrder, nil
}

func (s *stubPayBoxStore) CreateCouponRedemption(ctx context.Context, params store.CreateCouponRedemptionParams) (store.CouponRedemption, error) {
	s.createdRedemption = params
	if s.redemption.ID != 0 {
		return s.redemption, nil
	}
	return store.CouponRedemption{ID: 1, OrderID: params.OrderID, PredefinedCodeID: params.PredefinedCodeID, RedemptionToken: params.RedemptionToken, Status: "issued"}, nil
}

func (s *stubPayBoxStore) RecordPredefinedCodeDeliveryEvent(ctx context.Context, params store.RecordPredefinedCodeDeliveryEventParams) (store.MVPDelivery, error) {
	s.recordedDelivery = params
	if s.delivery.ID != 0 {
		return s.delivery, nil
	}
	return store.MVPDelivery{ID: 1, OrderID: params.OrderID, PredefinedCodeID: params.PredefinedCodeID, Status: params.Status}, nil
}

func (s *stubPayBoxStore) RecordAdminAction(ctx context.Context, params store.RecordAdminActionParams) (store.AdminAction, error) {
	s.recordedActions = append(s.recordedActions, params)
	return store.AdminAction{ID: int64(len(s.recordedActions)), EntityType: params.EntityType, EntityID: params.EntityID, ActionType: params.ActionType}, nil
}

func (s *stubPayBoxStore) GetUserByID(ctx context.Context, userID int64) (store.User, error) {
	if s.user.ID == 0 {
		return store.User{ID: userID, TelegramUserID: userID}, nil
	}
	return s.user, nil
}

type stubPayBoxMessenger struct {
	messages []string
	photos   []stubPayBoxPhoto
	err      error
}

type stubPayBoxPhoto struct {
	telegramUserID int64
	filename       string
	caption        string
}

func (s *stubPayBoxMessenger) SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error) {
	s.messages = append(s.messages, text)
	if s.err != nil {
		return 0, s.err
	}
	return int64(9000 + len(s.messages)), nil
}

func (s *stubPayBoxMessenger) SendPhotoMessage(ctx context.Context, telegramUserID int64, photo []byte, filename string, caption string) (int64, error) {
	s.photos = append(s.photos, stubPayBoxPhoto{telegramUserID: telegramUserID, filename: filename, caption: caption})
	if s.err != nil {
		return 0, s.err
	}
	return int64(9500 + len(s.photos)), nil
}

type stubPayBoxAdminNotifier struct {
	submittedClaims []store.ManualPaymentClaim
	approvedResults []PayBoxApprovalResult
	rejectedClaims  []store.ManualPaymentClaim
	supportResults  []PayBoxApprovalResult
}

func (s *stubPayBoxAdminNotifier) NotifyPaymentClaimSubmitted(ctx context.Context, claim store.ManualPaymentClaim) error {
	s.submittedClaims = append(s.submittedClaims, claim)
	return nil
}

func (s *stubPayBoxAdminNotifier) NotifyPaymentClaimApproved(ctx context.Context, result PayBoxApprovalResult) error {
	s.approvedResults = append(s.approvedResults, result)
	return nil
}

func (s *stubPayBoxAdminNotifier) NotifyPaymentClaimRejected(ctx context.Context, claim store.ManualPaymentClaim, order store.MVPOrder) error {
	s.rejectedClaims = append(s.rejectedClaims, claim)
	return nil
}

func (s *stubPayBoxAdminNotifier) NotifyPaymentClaimNeedsSupport(ctx context.Context, result PayBoxApprovalResult) error {
	s.supportResults = append(s.supportResults, result)
	return nil
}

type stubPayBoxCodeRenderer struct {
	code string
	err  error
}

func (s stubPayBoxCodeRenderer) RenderPredefinedCode(ctx context.Context, code store.PredefinedCode) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.code, nil
}

type fixedOrderNumberer struct {
	value string
}

func (f fixedOrderNumberer) NewOrderNumber() string {
	if f.value == "" {
		return "PB-fixed"
	}
	return f.value
}

type stubPayBoxQRRenderer struct{}

func (stubPayBoxQRRenderer) RenderPNG(payload string) ([]byte, error) {
	if !strings.Contains(payload, "/api/redemptions/scan/") {
		return nil, errors.New("unexpected qr payload")
	}
	return []byte("png"), nil
}
