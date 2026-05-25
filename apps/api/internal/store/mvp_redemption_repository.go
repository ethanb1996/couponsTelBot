package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	couponRedemptionStatusIssued   = "issued"
	couponRedemptionStatusRedeemed = "redeemed"
)

func (p *Postgres) CreateCouponRedemption(ctx context.Context, params CreateCouponRedemptionParams) (CouponRedemption, error) {
	if err := p.ensurePool(); err != nil {
		return CouponRedemption{}, err
	}
	if params.OrderID == 0 || params.PredefinedCodeID == 0 {
		return CouponRedemption{}, fmt.Errorf("%w: order id and predefined code id are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.RedemptionToken) == "" {
		return CouponRedemption{}, fmt.Errorf("%w: redemption token is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO coupon_redemptions (
			order_id,
			predefined_code_id,
			redemption_token,
			status
		) VALUES ($1, $2, $3, 'issued')
		ON CONFLICT (order_id) DO UPDATE
		SET updated_at = coupon_redemptions.updated_at
		RETURNING
			id,
			order_id,
			predefined_code_id,
			redemption_token,
			status,
			merchant_reference,
			scanner_reference,
			scan_metadata_json,
			scanned_at,
			created_at,
			updated_at
	`,
		params.OrderID,
		params.PredefinedCodeID,
		strings.TrimSpace(params.RedemptionToken),
	)

	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemption{}, mapStoreErr(err)
	}
	return redemption, nil
}

func (p *Postgres) RecordCouponRedemptionScan(ctx context.Context, params RecordCouponRedemptionScanParams) (CouponRedemption, error) {
	if err := p.ensurePool(); err != nil {
		return CouponRedemption{}, err
	}
	if strings.TrimSpace(params.RedemptionToken) == "" {
		return CouponRedemption{}, fmt.Errorf("%w: redemption token is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CouponRedemption{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
		UPDATE coupon_redemptions
		SET
			status = CASE
				WHEN status = 'issued' THEN 'redeemed'
				ELSE status
			END,
			merchant_reference = COALESCE(NULLIF($2, ''), merchant_reference),
			scanner_reference = COALESCE(NULLIF($3, ''), scanner_reference),
			scan_metadata_json = CASE
				WHEN NULLIF($4, '') IS NULL THEN scan_metadata_json
				ELSE $4::jsonb
			END,
			scanned_at = COALESCE(scanned_at, NOW()),
			updated_at = NOW()
		WHERE redemption_token = $1
		RETURNING
			id,
			order_id,
			predefined_code_id,
			redemption_token,
			status,
			merchant_reference,
			scanner_reference,
			scan_metadata_json,
			scanned_at,
			created_at,
			updated_at
	`,
		strings.TrimSpace(params.RedemptionToken),
		strings.TrimSpace(params.MerchantReference),
		strings.TrimSpace(params.ScannerReference),
		strings.TrimSpace(params.ScanMetadataJSON),
	)

	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemption{}, mapStoreErr(err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = 'coupon_redeemed',
			updated_at = NOW()
		WHERE id = $1
			AND status = 'coupon_sent'
	`, redemption.OrderID)
	if err != nil {
		return CouponRedemption{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE predefined_codes
		SET
			status = 'redeemed',
			updated_at = NOW()
		WHERE id = $1
			AND status = 'sent'
	`, redemption.PredefinedCodeID)
	if err != nil {
		return CouponRedemption{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CouponRedemption{}, err
	}

	return redemption, nil
}

func scanCouponRedemption(row interface {
	Scan(dest ...any) error
}) (CouponRedemption, error) {
	var redemption CouponRedemption
	err := row.Scan(
		&redemption.ID,
		&redemption.OrderID,
		&redemption.PredefinedCodeID,
		&redemption.RedemptionToken,
		&redemption.Status,
		&redemption.MerchantReference,
		&redemption.ScannerReference,
		&redemption.ScanMetadataJSON,
		&redemption.ScannedAt,
		&redemption.CreatedAt,
		&redemption.UpdatedAt,
	)
	return redemption, err
}
