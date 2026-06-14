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

	couponRedemptionEventViewed       = "viewed"
	couponRedemptionEventRedeemed     = "redeemed"
	couponRedemptionEventRepeatRedeem = "repeat_redeem"
	couponRedemptionEventInvalidToken = "invalid_token"
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
			first_viewed_at,
			redeemed_at,
			redeemed_by_reference,
			redeem_metadata_json,
			restaurant_notification_chat_id,
			restaurant_notification_message_id,
			restaurant_notified_at,
			restaurant_notification_error,
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

func (p *Postgres) GetCouponRedemptionPreview(ctx context.Context, token string) (CouponRedemptionPreview, error) {
	if err := p.ensurePool(); err != nil {
		return CouponRedemptionPreview{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return CouponRedemptionPreview{}, fmt.Errorf("%w: redemption token is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CouponRedemptionPreview{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	current, err := loadCouponRedemptionForUpdate(ctx, tx, token)
	if err != nil {
		_ = recordCouponRedemptionEvent(ctx, tx, nil, couponRedemptionEventInvalidToken, "", "{}")
		_ = tx.Commit(ctx)
		return CouponRedemptionPreview{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE coupon_redemptions
		SET
			first_viewed_at = COALESCE(first_viewed_at, NOW()),
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
			first_viewed_at,
			redeemed_at,
			redeemed_by_reference,
			redeem_metadata_json,
			restaurant_notification_chat_id,
			restaurant_notification_message_id,
			restaurant_notified_at,
			restaurant_notification_error,
			created_at,
			updated_at
	`, current.ID)
	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemptionPreview{}, mapStoreErr(err)
	}

	if err := recordCouponRedemptionEvent(ctx, tx, &redemption.ID, couponRedemptionEventViewed, "", "{}"); err != nil {
		return CouponRedemptionPreview{}, err
	}

	preview, err := loadCouponRedemptionPreview(ctx, tx, redemption.ID)
	if err != nil {
		return CouponRedemptionPreview{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CouponRedemptionPreview{}, err
	}
	return preview, nil
}

func (p *Postgres) ConfirmCouponRedemption(ctx context.Context, params ConfirmCouponRedemptionParams) (CouponRedemptionConfirmResult, error) {
	if err := p.ensurePool(); err != nil {
		return CouponRedemptionConfirmResult{}, err
	}
	token := strings.TrimSpace(params.RedemptionToken)
	if token == "" {
		return CouponRedemptionConfirmResult{}, fmt.Errorf("%w: redemption token is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CouponRedemptionConfirmResult{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	current, err := loadCouponRedemptionForUpdate(ctx, tx, token)
	if err != nil {
		_ = recordCouponRedemptionEvent(ctx, tx, nil, couponRedemptionEventInvalidToken, strings.TrimSpace(params.ScannerReference), strings.TrimSpace(params.RedeemMetadataJSON))
		_ = tx.Commit(ctx)
		return CouponRedemptionConfirmResult{}, err
	}

	firstRedeem := current.Status == couponRedemptionStatusIssued
	row := tx.QueryRow(ctx, `
		UPDATE coupon_redemptions
		SET
			status = CASE WHEN status = 'issued' THEN 'redeemed' ELSE status END,
			merchant_reference = CASE WHEN status = 'issued' THEN COALESCE(NULLIF($2, ''), merchant_reference) ELSE merchant_reference END,
			scanner_reference = CASE WHEN status = 'issued' THEN COALESCE(NULLIF($3, ''), scanner_reference) ELSE scanner_reference END,
			scan_metadata_json = CASE
				WHEN status = 'issued' AND NULLIF($4, '') IS NOT NULL THEN $4::jsonb
				ELSE scan_metadata_json
			END,
			first_viewed_at = COALESCE(first_viewed_at, NOW()),
			scanned_at = COALESCE(scanned_at, NOW()),
			redeemed_at = CASE WHEN status = 'issued' THEN NOW() ELSE redeemed_at END,
			redeemed_by_reference = CASE WHEN status = 'issued' THEN COALESCE(NULLIF($3, ''), redeemed_by_reference) ELSE redeemed_by_reference END,
			redeem_metadata_json = CASE
				WHEN status = 'issued' AND NULLIF($4, '') IS NOT NULL THEN $4::jsonb
				ELSE redeem_metadata_json
			END,
			restaurant_notification_chat_id = CASE
				WHEN status = 'issued' THEN (
					SELECT COALESCE(mp.redemption_notification_chat_id, NULLIF($5, 0)::BIGINT)
					FROM orders o
					INNER JOIN offers offer ON offer.id = o.offer_id
					INNER JOIN merchant_partners mp ON mp.id = offer.merchant_partner_id
					WHERE o.id = coupon_redemptions.order_id
				)
				ELSE restaurant_notification_chat_id
			END,
			updated_at = NOW()
		WHERE coupon_redemptions.id = $1
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
			first_viewed_at,
			redeemed_at,
			redeemed_by_reference,
			redeem_metadata_json,
			restaurant_notification_chat_id,
			restaurant_notification_message_id,
			restaurant_notified_at,
			restaurant_notification_error,
			created_at,
			updated_at
	`,
		current.ID,
		strings.TrimSpace(params.MerchantReference),
		strings.TrimSpace(params.ScannerReference),
		strings.TrimSpace(params.RedeemMetadataJSON),
		params.RestaurantNotificationFallbackChatID,
	)
	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemptionConfirmResult{}, mapStoreErr(err)
	}

	eventType := couponRedemptionEventRepeatRedeem
	if firstRedeem {
		eventType = couponRedemptionEventRedeemed
		if _, err = tx.Exec(ctx, `
			UPDATE orders
			SET
				status = 'coupon_redeemed',
				updated_at = NOW()
			WHERE id = $1
				AND status = 'coupon_sent'
		`, redemption.OrderID); err != nil {
			return CouponRedemptionConfirmResult{}, err
		}

		if _, err = tx.Exec(ctx, `
			UPDATE predefined_codes
			SET
				status = 'redeemed',
				updated_at = NOW()
			WHERE id = $1
				AND status = 'sent'
		`, redemption.PredefinedCodeID); err != nil {
			return CouponRedemptionConfirmResult{}, err
		}
	}
	if err := recordCouponRedemptionEvent(ctx, tx, &redemption.ID, eventType, strings.TrimSpace(params.ScannerReference), strings.TrimSpace(params.RedeemMetadataJSON)); err != nil {
		return CouponRedemptionConfirmResult{}, err
	}

	result, err := loadCouponRedemptionConfirmResult(ctx, tx, redemption.ID, firstRedeem, params.RestaurantNotificationFallbackChatID)
	if err != nil {
		return CouponRedemptionConfirmResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CouponRedemptionConfirmResult{}, err
	}
	return result, nil
}

func (p *Postgres) RecordRestaurantRedemptionNotification(ctx context.Context, params RecordRestaurantRedemptionNotificationParams) error {
	if err := p.ensurePool(); err != nil {
		return err
	}
	if params.CouponRedemptionID == 0 {
		return fmt.Errorf("%w: coupon redemption id is required", ErrInvalidArgument)
	}

	var messageID *int64
	if params.MessageID != 0 {
		messageID = &params.MessageID
	}
	_, err := p.Pool.Exec(ctx, `
		UPDATE coupon_redemptions
		SET
			restaurant_notification_chat_id = COALESCE(NULLIF($2, 0)::BIGINT, restaurant_notification_chat_id),
			restaurant_notification_message_id = COALESCE($3::BIGINT, restaurant_notification_message_id),
			restaurant_notified_at = CASE WHEN $4 = '' THEN NOW() ELSE restaurant_notified_at END,
			restaurant_notification_error = $4,
			updated_at = NOW()
		WHERE id = $1
	`, params.CouponRedemptionID, params.ChatID, messageID, strings.TrimSpace(params.Error))
	return err
}

func loadCouponRedemptionForUpdate(ctx context.Context, tx pgx.Tx, token string) (CouponRedemption, error) {
	row := tx.QueryRow(ctx, couponRedemptionSelectSQL(`
		FROM coupon_redemptions
		WHERE redemption_token = $1
		FOR UPDATE
	`), token)

	redemption, err := scanCouponRedemption(row)
	if err != nil {
		return CouponRedemption{}, mapStoreErr(err)
	}
	return redemption, nil
}

func loadCouponRedemptionPreview(ctx context.Context, tx pgx.Tx, redemptionID int64) (CouponRedemptionPreview, error) {
	row := tx.QueryRow(ctx, couponRedemptionPreviewSelectSQL(`
		WHERE cr.id = $1
	`), redemptionID)

	preview, err := scanCouponRedemptionPreview(row)
	if err != nil {
		return CouponRedemptionPreview{}, mapStoreErr(err)
	}
	return preview, nil
}

func loadCouponRedemptionConfirmResult(ctx context.Context, tx pgx.Tx, redemptionID int64, firstRedeem bool, fallbackChatID int64) (CouponRedemptionConfirmResult, error) {
	row := tx.QueryRow(ctx, couponRedemptionPreviewSelectSQL(`
		WHERE cr.id = $1
	`), redemptionID)

	preview, err := scanCouponRedemptionPreview(row)
	if err != nil {
		return CouponRedemptionConfirmResult{}, mapStoreErr(err)
	}

	result := CouponRedemptionConfirmResult{
		Preview:                      preview,
		FirstRedeem:                  firstRedeem,
		RestaurantNotificationChatID: preview.Redemption.RestaurantNotificationChatID,
	}
	if result.RestaurantNotificationChatID == nil && fallbackChatID != 0 {
		result.RestaurantNotificationChatID = &fallbackChatID
	}
	if preview.Redemption.RestaurantNotificationMessageID != nil {
		result.RestaurantNotificationMessage = preview.Redemption.RestaurantNotificationMessageID
	}
	return result, nil
}

func recordCouponRedemptionEvent(ctx context.Context, tx pgx.Tx, redemptionID *int64, eventType, actorReference, metadataJSON string) error {
	if strings.TrimSpace(metadataJSON) == "" {
		metadataJSON = "{}"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO coupon_redemption_events (
			coupon_redemption_id,
			event_type,
			actor_reference,
			metadata_json
		) VALUES ($1, $2, $3, $4::jsonb)
	`, redemptionID, eventType, strings.TrimSpace(actorReference), metadataJSON)
	return err
}

func couponRedemptionSelectSQL(suffix string) string {
	return `
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
			first_viewed_at,
			redeemed_at,
			redeemed_by_reference,
			redeem_metadata_json,
			restaurant_notification_chat_id,
			restaurant_notification_message_id,
			restaurant_notified_at,
			restaurant_notification_error,
			created_at,
			updated_at
	` + suffix
}

func couponRedemptionPreviewSelectSQL(suffix string) string {
	return `
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
			cr.first_viewed_at,
			cr.redeemed_at,
			cr.redeemed_by_reference,
			cr.redeem_metadata_json,
			cr.restaurant_notification_chat_id,
			cr.restaurant_notification_message_id,
			cr.restaurant_notified_at,
			cr.restaurant_notification_error,
			cr.created_at,
			cr.updated_at,
			o.order_number,
			offers.merchant_name,
			mp.contact_reference,
			offers.title,
			o.sale_price_amount,
			o.currency_code,
			'אושר במערכת KuponFast' AS payment_status_summary,
			o.verified_at,
			COALESCE(u.display_name, '') AS buyer_display,
			u.telegram_user_id,
			offers.redemption_terms,
			pc.expiry_at
		FROM coupon_redemptions cr
		INNER JOIN orders o ON o.id = cr.order_id
		INNER JOIN offers ON offers.id = o.offer_id
		INNER JOIN merchant_partners mp ON mp.id = offers.merchant_partner_id
		INNER JOIN users u ON u.id = o.user_id
		INNER JOIN predefined_codes pc ON pc.id = cr.predefined_code_id
	` + suffix
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
		&redemption.FirstViewedAt,
		&redemption.RedeemedAt,
		&redemption.RedeemedByReference,
		&redemption.RedeemMetadataJSON,
		&redemption.RestaurantNotificationChatID,
		&redemption.RestaurantNotificationMessageID,
		&redemption.RestaurantNotifiedAt,
		&redemption.RestaurantNotificationError,
		&redemption.CreatedAt,
		&redemption.UpdatedAt,
	)
	return redemption, err
}

func scanCouponRedemptionPreview(row interface {
	Scan(dest ...any) error
}) (CouponRedemptionPreview, error) {
	var preview CouponRedemptionPreview
	err := row.Scan(
		&preview.Redemption.ID,
		&preview.Redemption.OrderID,
		&preview.Redemption.PredefinedCodeID,
		&preview.Redemption.RedemptionToken,
		&preview.Redemption.Status,
		&preview.Redemption.MerchantReference,
		&preview.Redemption.ScannerReference,
		&preview.Redemption.ScanMetadataJSON,
		&preview.Redemption.ScannedAt,
		&preview.Redemption.FirstViewedAt,
		&preview.Redemption.RedeemedAt,
		&preview.Redemption.RedeemedByReference,
		&preview.Redemption.RedeemMetadataJSON,
		&preview.Redemption.RestaurantNotificationChatID,
		&preview.Redemption.RestaurantNotificationMessageID,
		&preview.Redemption.RestaurantNotifiedAt,
		&preview.Redemption.RestaurantNotificationError,
		&preview.Redemption.CreatedAt,
		&preview.Redemption.UpdatedAt,
		&preview.OrderNumber,
		&preview.MerchantName,
		&preview.MerchantContact,
		&preview.OfferTitle,
		&preview.AmountPaid,
		&preview.CurrencyCode,
		&preview.PaymentStatusSummary,
		&preview.ApprovalTime,
		&preview.BuyerDisplay,
		&preview.BuyerTelegramID,
		&preview.RedemptionTerms,
		&preview.PredefinedCodeExpiryAt,
	)
	return preview, err
}
