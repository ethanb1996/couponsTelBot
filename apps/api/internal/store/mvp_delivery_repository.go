package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	predefinedCodeStatusSent = "sent"
	mvpOrderStatusCouponSent = "coupon_sent"
)

type MVPDelivery struct {
	ID                  int64
	OrderID             int64
	PredefinedCodeID    int64
	DeliveryChannel     string
	Status              string
	TelegramMessageID   *int64
	DeliveryPayloadHash string
	SentAt              *time.Time
	ConfirmedAt         *time.Time
	FailureReason       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type RecordPredefinedCodeDeliveryEventParams struct {
	OrderID             int64
	PredefinedCodeID    int64
	DeliveryChannel     string
	Status              string
	TelegramMessageID   *int64
	DeliveryPayloadHash string
	SentAt              *time.Time
	ConfirmedAt         *time.Time
	FailureReason       string
}

func (p *Postgres) RecordPredefinedCodeDeliveryEvent(ctx context.Context, params RecordPredefinedCodeDeliveryEventParams) (MVPDelivery, error) {
	if err := p.ensurePool(); err != nil {
		return MVPDelivery{}, err
	}
	if params.OrderID == 0 || params.PredefinedCodeID == 0 {
		return MVPDelivery{}, fmt.Errorf("%w: order id and predefined code id are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.Status) == "" {
		return MVPDelivery{}, fmt.Errorf("%w: delivery status is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MVPDelivery{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order, err := loadMVPOrderForUpdate(ctx, tx, params.OrderID)
	if err != nil {
		return MVPDelivery{}, err
	}
	if order.PredefinedCodeID == nil || *order.PredefinedCodeID != params.PredefinedCodeID {
		return MVPDelivery{}, ErrCouponMismatch
	}
	if order.Status != mvpOrderStatusPaymentVerified && order.Status != mvpOrderStatusCouponSent {
		return MVPDelivery{}, ErrOrderNotReadyForCoupon
	}

	deliveryChannel := defaultString(params.DeliveryChannel, "telegram_bot")
	row := tx.QueryRow(ctx, `
		INSERT INTO coupon_deliveries (
			order_id,
			predefined_code_id,
			delivery_channel,
			status,
			telegram_message_id,
			delivery_payload_hash,
			sent_at,
			confirmed_at,
			failure_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (order_id) DO UPDATE
		SET
			predefined_code_id = EXCLUDED.predefined_code_id,
			delivery_channel = EXCLUDED.delivery_channel,
			status = EXCLUDED.status,
			telegram_message_id = EXCLUDED.telegram_message_id,
			delivery_payload_hash = EXCLUDED.delivery_payload_hash,
			sent_at = EXCLUDED.sent_at,
			confirmed_at = EXCLUDED.confirmed_at,
			failure_reason = EXCLUDED.failure_reason,
			updated_at = NOW()
		RETURNING
			id,
			order_id,
			predefined_code_id,
			delivery_channel,
			status,
			telegram_message_id,
			delivery_payload_hash,
			sent_at,
			confirmed_at,
			failure_reason,
			created_at,
			updated_at
	`,
		order.ID,
		params.PredefinedCodeID,
		deliveryChannel,
		params.Status,
		params.TelegramMessageID,
		params.DeliveryPayloadHash,
		params.SentAt,
		params.ConfirmedAt,
		params.FailureReason,
	)

	delivery, err := scanMVPDelivery(row)
	if err != nil {
		return MVPDelivery{}, mapStoreErr(err)
	}

	codeStatus, orderStatus, deliveredAt := predefinedCodeDeliveryOutcome(delivery)
	_, err = tx.Exec(ctx, `
		UPDATE predefined_codes
		SET
			status = $2,
			updated_at = NOW()
		WHERE id = $1
	`, params.PredefinedCodeID, codeStatus)
	if err != nil {
		return MVPDelivery{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = $2,
			delivered_at = $3,
			updated_at = NOW()
		WHERE id = $1
	`, order.ID, orderStatus, deliveredAt)
	if err != nil {
		return MVPDelivery{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return MVPDelivery{}, err
	}

	return delivery, nil
}

func predefinedCodeDeliveryOutcome(delivery MVPDelivery) (string, string, *time.Time) {
	switch delivery.Status {
	case deliveryStatusSent, deliveryStatusConfirmed:
		if delivery.ConfirmedAt != nil {
			return predefinedCodeStatusSent, mvpOrderStatusCouponSent, delivery.ConfirmedAt
		}
		if delivery.SentAt != nil {
			return predefinedCodeStatusSent, mvpOrderStatusCouponSent, delivery.SentAt
		}
		now := time.Now().UTC()
		return predefinedCodeStatusSent, mvpOrderStatusCouponSent, &now
	default:
		return predefinedCodeStatusAssigned, mvpOrderStatusPaymentVerified, nil
	}
}

func scanMVPDelivery(row interface {
	Scan(dest ...any) error
}) (MVPDelivery, error) {
	var delivery MVPDelivery
	err := row.Scan(
		&delivery.ID,
		&delivery.OrderID,
		&delivery.PredefinedCodeID,
		&delivery.DeliveryChannel,
		&delivery.Status,
		&delivery.TelegramMessageID,
		&delivery.DeliveryPayloadHash,
		&delivery.SentAt,
		&delivery.ConfirmedAt,
		&delivery.FailureReason,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	return delivery, err
}
