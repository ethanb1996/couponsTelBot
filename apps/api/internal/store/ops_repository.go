package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (p *Postgres) RecordAdminAction(ctx context.Context, params RecordAdminActionParams) (AdminAction, error) {
	if err := p.ensurePool(); err != nil {
		return AdminAction{}, err
	}
	if strings.TrimSpace(params.AdminActor) == "" || strings.TrimSpace(params.EntityType) == "" || strings.TrimSpace(params.ActionType) == "" {
		return AdminAction{}, fmt.Errorf("%w: admin actor, entity type, and action type are required", ErrInvalidArgument)
	}
	if params.EntityID == 0 {
		return AdminAction{}, fmt.Errorf("%w: entity id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO admin_actions (
			admin_actor,
			entity_type,
			entity_id,
			action_type,
			before_state_json,
			after_state_json,
			reason_text
		) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)
		RETURNING
			id,
			admin_actor,
			entity_type,
			entity_id,
			action_type,
			before_state_json::TEXT,
			after_state_json::TEXT,
			reason_text,
			created_at
	`,
		params.AdminActor,
		params.EntityType,
		params.EntityID,
		params.ActionType,
		defaultJSON(params.BeforeStateJSON),
		defaultJSON(params.AfterStateJSON),
		params.ReasonText,
	)

	action, err := scanAdminAction(row)
	if err != nil {
		return AdminAction{}, mapStoreErr(err)
	}

	return action, nil
}

func (p *Postgres) ListAdminActions(ctx context.Context, filter AdminActionFilter) ([]AdminAction, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := p.Pool.Query(ctx, `
		SELECT
			id,
			admin_actor,
			entity_type,
			entity_id,
			action_type,
			before_state_json::TEXT,
			after_state_json::TEXT,
			reason_text,
			created_at
		FROM admin_actions
		WHERE ($1::TEXT = '' OR entity_type = $1)
			AND ($2::BIGINT IS NULL OR entity_id = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`, filter.EntityType, filter.EntityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := make([]AdminAction, 0, limit)
	for rows.Next() {
		action, err := scanAdminAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return actions, nil
}

func (p *Postgres) SweepExpiredInventory(ctx context.Context) (InventorySweepResult, error) {
	if err := p.ensurePool(); err != nil {
		return InventorySweepResult{}, err
	}

	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return InventorySweepResult{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result := InventorySweepResult{}

	expiredCoupons, err := tx.Exec(ctx, `
		UPDATE coupons
		SET
			inventory_status = 'expired',
			updated_at = NOW()
		WHERE expiry_at <= NOW()
			AND inventory_status IN ('available', 'assigned')
	`)
	if err != nil {
		return InventorySweepResult{}, err
	}
	result.ExpiredCoupons = expiredCoupons.RowsAffected()

	expiredListings, err := tx.Exec(ctx, `
		UPDATE listings l
		SET
			status = 'expired',
			updated_at = NOW()
		WHERE l.status IN ('active', 'paused', 'sold_out')
			AND EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
			)
			AND NOT EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
					AND c.inventory_status = 'available'
					AND c.expiry_at > NOW()
			)
			AND EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
					AND c.inventory_status = 'expired'
			)
	`)
	if err != nil {
		return InventorySweepResult{}, err
	}
	result.ExpiredListings = expiredListings.RowsAffected()

	soldOutListings, err := tx.Exec(ctx, `
		UPDATE listings l
		SET
			status = 'sold_out',
			updated_at = NOW()
		WHERE l.status IN ('active', 'paused')
			AND EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
			)
			AND NOT EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
					AND c.inventory_status = 'available'
					AND c.expiry_at > NOW()
			)
			AND NOT EXISTS (
				SELECT 1
				FROM coupons c
				WHERE c.listing_id = l.id
					AND c.inventory_status = 'expired'
			)
	`)
	if err != nil {
		return InventorySweepResult{}, err
	}
	result.SoldOutListings = soldOutListings.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return InventorySweepResult{}, err
	}

	return result, nil
}

func (p *Postgres) ListPaidUndeliveredOrders(ctx context.Context, olderThan time.Duration, limit int) ([]PaidUndeliveredOrderAlert, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 25
	}

	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := p.Pool.Query(ctx, `
		SELECT
			o.id,
			o.order_number,
			o.user_id,
			COALESCE(u.display_name, '') AS user_display_name,
			o.listing_id,
			COALESCE(l.title, '') AS listing_title,
			o.coupon_id,
			o.status,
			o.provider_checkout_reference,
			lp.status,
			lp.captured_at,
			COALESCE(cd.status, '') AS delivery_status,
			COALESCE(cd.failure_reason, '') AS delivery_failure_reason,
			COALESCE(lp.captured_at, lp.created_at) AS last_paid_at
		FROM orders o
		INNER JOIN users u ON u.id = o.user_id
		INNER JOIN listings l ON l.id = o.listing_id
		INNER JOIN LATERAL (
			SELECT
				status,
				captured_at,
				created_at
			FROM payments
			WHERE order_id = o.id
				AND status IN ('authorized', 'captured')
			ORDER BY COALESCE(captured_at, created_at) DESC, id DESC
			LIMIT 1
		) lp ON TRUE
		LEFT JOIN coupon_deliveries cd ON cd.order_id = o.id
		WHERE o.status IN ('paid', 'delivery_pending')
			AND COALESCE(cd.status, '') NOT IN ('sent', 'confirmed')
			AND COALESCE(lp.captured_at, lp.created_at) <= $1
		ORDER BY COALESCE(lp.captured_at, lp.created_at) ASC, o.id ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]PaidUndeliveredOrderAlert, 0, limit)
	for rows.Next() {
		var alert PaidUndeliveredOrderAlert
		if err := rows.Scan(
			&alert.OrderID,
			&alert.OrderNumber,
			&alert.UserID,
			&alert.UserDisplayName,
			&alert.ListingID,
			&alert.ListingTitle,
			&alert.CouponID,
			&alert.Status,
			&alert.ProviderCheckoutReference,
			&alert.PaymentStatus,
			&alert.PaymentCapturedAt,
			&alert.DeliveryStatus,
			&alert.DeliveryFailureReason,
			&alert.LastPaidAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return alerts, nil
}

func (p *Postgres) ListPendingPaymentReconciliationCandidates(ctx context.Context, olderThan time.Duration, limit int) ([]PendingPaymentReconciliationCandidate, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 25
	}

	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := p.Pool.Query(ctx, `
		SELECT
			o.id,
			o.order_number,
			o.status,
			o.provider_checkout_reference,
			o.placed_at,
			o.updated_at
		FROM orders o
		LEFT JOIN LATERAL (
			SELECT status
			FROM payments
			WHERE order_id = o.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) lp ON TRUE
		WHERE o.status = 'pending_payment'
			AND o.provider_checkout_reference <> ''
			AND COALESCE(o.placed_at, o.updated_at, o.created_at) <= $1
			AND COALESCE(lp.status, 'pending') IN ('pending', 'failed', 'cancelled')
			AND NOT EXISTS (
				SELECT 1
				FROM payments p
				WHERE p.order_id = o.id
					AND p.status IN ('authorized', 'captured')
			)
		ORDER BY COALESCE(o.placed_at, o.updated_at, o.created_at) ASC, o.id ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]PendingPaymentReconciliationCandidate, 0, limit)
	for rows.Next() {
		var candidate PendingPaymentReconciliationCandidate
		if err := rows.Scan(
			&candidate.OrderID,
			&candidate.OrderNumber,
			&candidate.Status,
			&candidate.ProviderCheckoutReference,
			&candidate.PlacedAt,
			&candidate.UpdatedAt,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return candidates, nil
}

func (p *Postgres) EnsureSupportCase(ctx context.Context, params EnsureSupportCaseParams) (SupportCase, bool, error) {
	if err := p.ensurePool(); err != nil {
		return SupportCase{}, false, err
	}
	if strings.TrimSpace(params.CaseType) == "" || strings.TrimSpace(params.Summary) == "" {
		return SupportCase{}, false, fmt.Errorf("%w: case type and summary are required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			order_id,
			coupon_id,
			case_type,
			status,
			priority,
			summary,
			resolution_note,
			assigned_admin_id,
			created_at,
			updated_at
		FROM support_cases
		WHERE case_type = $1
			AND order_id IS NOT DISTINCT FROM $2
			AND coupon_id IS NOT DISTINCT FROM $3
			AND status IN ('open', 'in_progress', 'waiting_on_user')
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, params.CaseType, params.OrderID, params.CouponID)

	supportCase, err := scanSupportCase(row)
	if err == nil {
		return supportCase, false, nil
	}
	if mapStoreErr(err) != ErrNotFound {
		return SupportCase{}, false, err
	}

	created, err := p.CreateSupportCase(ctx, CreateSupportCaseParams{
		UserID:          params.UserID,
		OrderID:         params.OrderID,
		CouponID:        params.CouponID,
		CaseType:        params.CaseType,
		Status:          "open",
		Priority:        defaultString(params.Priority, "high"),
		Summary:         params.Summary,
		AssignedAdminID: params.AssignedAdminID,
	})
	if err != nil {
		return SupportCase{}, false, err
	}

	return created, true, nil
}

func scanAdminAction(row interface {
	Scan(dest ...any) error
}) (AdminAction, error) {
	var action AdminAction
	if err := row.Scan(
		&action.ID,
		&action.AdminActor,
		&action.EntityType,
		&action.EntityID,
		&action.ActionType,
		&action.BeforeStateJSON,
		&action.AfterStateJSON,
		&action.ReasonText,
		&action.CreatedAt,
	); err != nil {
		return AdminAction{}, err
	}

	return action, nil
}

func defaultJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}
