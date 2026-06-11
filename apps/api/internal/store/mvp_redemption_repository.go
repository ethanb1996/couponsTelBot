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

func (p *Postgres) RecordCouponRedemptionScan(ctx context.Context, params RecordCouponRedemptionScanParams) (CouponRedemptionScanResult, error) {
	if err := p.ensurePool(); err != nil {
		return CouponRedemptionScanResult{}, err
	}
	if strings.TrimSpace(params.RedemptionToken) == "" {
		return CouponRedemptionScanResult{}, fmt.Errorf("%w: redemption token is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CouponRedemptionScanResult{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	current, err := loadCouponRedemptionForUpdate(ctx, tx, strings.TrimSpace(params.RedemptionToken))
	if err != nil {
		return CouponRedemptionScanResult{}, err
	}
	firstScan := isFirstCouponRedemptionScan(current)

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
		WHERE id = $1
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
		current.ID,
		strings.TrimSpace(params.MerchantReference),
		strings.TrimSpace(params.ScannerReference),
		strings.TrimSpace(params.ScanMetadataJSON),
	)

	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemptionScanResult{}, mapStoreErr(err)
	}

	if firstScan {
		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET
				status = 'coupon_redeemed',
				updated_at = NOW()
			WHERE id = $1
				AND status = 'coupon_sent'
		`, redemption.OrderID)
		if err != nil {
			return CouponRedemptionScanResult{}, err
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
			return CouponRedemptionScanResult{}, err
		}
	}

	result, err := loadCouponRedemptionScanResult(ctx, tx, redemption.ID, firstScan)
	if err != nil {
		return CouponRedemptionScanResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CouponRedemptionScanResult{}, err
	}

	return result, nil
}

func isFirstCouponRedemptionScan(redemption CouponRedemption) bool {
	return redemption.Status == couponRedemptionStatusIssued
}

func loadCouponRedemptionForUpdate(ctx context.Context, tx pgx.Tx, token string) (CouponRedemption, error) {
	row := tx.QueryRow(ctx, `
		SELECT
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
		FROM coupon_redemptions
		WHERE redemption_token = $1
		FOR UPDATE
	`, token)

	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemption{}, mapStoreErr(err)
	}
	return redemption, nil
}

func loadCouponRedemptionScanResult(ctx context.Context, tx pgx.Tx, redemptionID int64, firstScan bool) (CouponRedemptionScanResult, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			cr.id,
			cr.order_id,
			cr.predefined_code_id,
			cr.redemption_token,
			cr.status,
			cr.merchant_reference,
			cr.scanner_reference,
			cr.scan_metadata_json,
			cr.scanned_at,
			cr.created_at,
			cr.updated_at,
			o.order_number,
			offers.merchant_name,
			mp.contact_reference,
			offers.title,
			COALESCE(u.display_name, '') AS buyer_display,
			u.telegram_user_id
		FROM coupon_redemptions cr
		INNER JOIN orders o ON o.id = cr.order_id
		INNER JOIN offers ON offers.id = o.offer_id
		INNER JOIN merchant_partners mp ON mp.id = offers.merchant_partner_id
		INNER JOIN users u ON u.id = o.user_id
		WHERE cr.id = $1
	`, redemptionID)

	var result CouponRedemptionScanResult
	err := row.Scan(
		&result.Redemption.ID,
		&result.Redemption.OrderID,
		&result.Redemption.PredefinedCodeID,
		&result.Redemption.RedemptionToken,
		&result.Redemption.Status,
		&result.Redemption.MerchantReference,
		&result.Redemption.ScannerReference,
		&result.Redemption.ScanMetadataJSON,
		&result.Redemption.ScannedAt,
		&result.Redemption.CreatedAt,
		&result.Redemption.UpdatedAt,
		&result.OrderNumber,
		&result.MerchantName,
		&result.MerchantContact,
		&result.OfferTitle,
		&result.BuyerDisplay,
		&result.BuyerTelegramID,
	)
	if err != nil {
		return CouponRedemptionScanResult{}, mapStoreErr(err)
	}
	result.FirstScan = firstScan
	return result, nil
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
