package ops

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type PaymentReconciler interface {
	ReconcilePendingOrder(ctx context.Context, candidate store.PendingPaymentReconciliationCandidate) error
	ProcessFulfillmentJob(ctx context.Context, job store.FulfillmentJob) error
}

type ExpiredCheckoutHoldNotifier interface {
	NotifyExpiredCheckoutHold(ctx context.Context, hold store.ReleasedCheckoutHold) error
}

type Runner struct {
	logger             *slog.Logger
	store              *store.Postgres
	reconciler         PaymentReconciler
	sweepInterval      time.Duration
	deliveryAlertAfter time.Duration
	reconcileAfter     time.Duration
	checkoutHoldAfter  time.Duration
	batchSize          int
	fulfillmentWakeCh  chan int64
	holdNotifier       ExpiredCheckoutHoldNotifier
}

func NewRunner(logger *slog.Logger, repo *store.Postgres, reconciler PaymentReconciler, holdNotifier ExpiredCheckoutHoldNotifier, sweepInterval time.Duration, deliveryAlertAfter time.Duration, reconcileAfter time.Duration, checkoutHoldAfter time.Duration, batchSize int) *Runner {
	return &Runner{
		logger:             logger,
		store:              repo,
		reconciler:         reconciler,
		holdNotifier:       holdNotifier,
		sweepInterval:      sweepInterval,
		deliveryAlertAfter: deliveryAlertAfter,
		reconcileAfter:     reconcileAfter,
		checkoutHoldAfter:  checkoutHoldAfter,
		batchSize:          batchSize,
		fulfillmentWakeCh:  make(chan int64, max(64, batchSize*2)),
	}
}

func (r *Runner) Start(ctx context.Context) {
	if r == nil || r.logger == nil || r.store == nil || r.reconciler == nil {
		return
	}

	go r.loop(ctx)
	go r.fulfillmentLoop(ctx)
}

func (r *Runner) NotifyFulfillment(orderID int64) {
	if r == nil || orderID == 0 {
		return
	}

	select {
	case r.fulfillmentWakeCh <- orderID:
	default:
	}
}

func (r *Runner) loop(ctx context.Context) {
	r.runOnce(ctx)

	ticker := time.NewTicker(r.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) fulfillmentLoop(ctx context.Context) {
	r.runFulfillmentJobs(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runFulfillmentJobs(ctx)
		case <-r.fulfillmentWakeCh:
			r.runFulfillmentJobs(ctx)
		}
	}
}

func (r *Runner) runOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	r.runSweep(ctx)
	r.runCheckoutHoldRelease(ctx)
	r.runDeliveryDetection(ctx)
	r.runPaymentReconciliation(ctx)
}

func (r *Runner) runSweep(ctx context.Context) {
	result, err := r.store.SweepExpiredInventory(ctx)
	if err != nil {
		r.logger.Error("ops sweep failed", "error", err)
		return
	}

	if result.ExpiredCoupons == 0 && result.ExpiredListings == 0 && result.SoldOutListings == 0 {
		return
	}

	r.logger.Info("ops sweep updated inventory",
		"expired_coupons", result.ExpiredCoupons,
		"expired_listings", result.ExpiredListings,
		"sold_out_listings", result.SoldOutListings,
	)
}

func (r *Runner) runCheckoutHoldRelease(ctx context.Context) {
	released, err := r.store.ReleaseExpiredCheckoutHolds(ctx, r.checkoutHoldAfter)
	if err != nil {
		r.logger.Error("failed to release expired checkout holds", "error", err)
		return
	}

	if len(released) == 0 {
		return
	}

	r.logger.Warn("released expired checkout holds",
		"released_orders", len(released),
		"hold_duration", r.checkoutHoldAfter.String(),
	)

	if r.holdNotifier == nil {
		return
	}

	for _, hold := range released {
		if err := r.holdNotifier.NotifyExpiredCheckoutHold(ctx, hold); err != nil {
			r.logger.Error("failed to notify expired checkout hold",
				"order_id", hold.OrderID,
				"order_number", hold.OrderNumber,
				"telegram_user_id", hold.TelegramUserID,
				"error", err,
			)
		}
	}
}

func (r *Runner) runDeliveryDetection(ctx context.Context) {
	alerts, err := r.store.ListPaidUndeliveredOrders(ctx, r.deliveryAlertAfter, r.batchSize)
	if err != nil {
		r.logger.Error("failed to detect paid but undelivered orders", "error", err)
		return
	}

	for _, alert := range alerts {
		summary := fmt.Sprintf("Paid order %s has not been delivered since %s", alert.OrderNumber, alert.LastPaidAt.UTC().Format(time.RFC3339))
		createdCase, created, ensureErr := r.store.EnsureSupportCase(ctx, store.EnsureSupportCaseParams{
			UserID:          int64Ptr(alert.UserID),
			OrderID:         int64Ptr(alert.OrderID),
			CouponID:        alert.CouponID,
			CaseType:        "delivery_issue",
			Priority:        "high",
			Summary:         summary,
			AssignedAdminID: "ops-job",
		})
		if ensureErr != nil {
			r.logger.Error("failed to escalate delivery issue",
				"order_id", alert.OrderID,
				"order_number", alert.OrderNumber,
				"error", ensureErr,
			)
			continue
		}

		r.logger.Warn("paid but undelivered order detected",
			"order_id", alert.OrderID,
			"order_number", alert.OrderNumber,
			"support_case_id", createdCase.ID,
			"support_case_created", created,
			"listing_id", alert.ListingID,
			"delivery_status", alert.DeliveryStatus,
			"payment_status", alert.PaymentStatus,
			"paid_at", alert.LastPaidAt,
		)
	}
}

func (r *Runner) runPaymentReconciliation(ctx context.Context) {
	candidates, err := r.store.ListPendingPaymentReconciliationCandidates(ctx, r.reconcileAfter, r.batchSize)
	if err != nil {
		r.logger.Error("failed to list pending payment reconciliations", "error", err)
		return
	}

	for _, candidate := range candidates {
		if err := r.reconciler.ReconcilePendingOrder(ctx, candidate); err != nil {
			r.logger.Error("payment reconciliation failed",
				"order_id", candidate.OrderID,
				"order_number", candidate.OrderNumber,
				"provider_checkout_reference", candidate.ProviderCheckoutReference,
				"error", err,
			)
		}
	}
}

func (r *Runner) runFulfillmentJobs(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	jobs, err := r.store.ClaimDueFulfillmentJobs(ctx, r.batchSize)
	if err != nil {
		r.logger.Error("failed to claim fulfillment jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := r.reconciler.ProcessFulfillmentJob(ctx, job); err != nil {
			r.logger.Error("fulfillment job processing failed",
				"job_id", job.ID,
				"order_id", job.OrderID,
				"provider_checkout_reference", job.ProviderCheckoutReference,
				"attempt_count", job.AttemptCount,
				"error", err,
			)
		}
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
