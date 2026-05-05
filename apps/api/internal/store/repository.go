package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	inventoryStatusAvailable = "available"
	inventoryStatusReserved  = "reserved"
	inventoryStatusAssigned  = "assigned"
	inventoryStatusDelivered = "delivered"

	listingStatusActive = "active"

	orderStatusDraft           = "draft"
	orderStatusPendingPayment  = "pending_payment"
	orderStatusPaid            = "paid"
	orderStatusDeliveryPending = "delivery_pending"
	orderStatusDelivered       = "delivered"
	orderStatusFailed          = "failed"
	orderStatusCancelled       = "cancelled"
	orderStatusDisputed        = "disputed"

	paymentStatusPending    = "pending"
	paymentStatusAuthorized = "authorized"
	paymentStatusCaptured   = "captured"
	paymentStatusFailed     = "failed"
	paymentStatusCancelled  = "cancelled"
	paymentStatusChargeback = "chargeback"
	paymentStatusDisputed   = "disputed"

	deliveryStatusPending   = "pending"
	deliveryStatusSent      = "sent"
	deliveryStatusConfirmed = "confirmed"
	deliveryStatusFailed    = "failed"

	fulfillmentJobStatusPending        = "pending"
	fulfillmentJobStatusProcessing     = "processing"
	fulfillmentJobStatusRetryScheduled = "retry_scheduled"
	fulfillmentJobStatusSucceeded      = "succeeded"
	fulfillmentJobStatusFailedTerminal = "failed_terminal"
)

var (
	ErrNotFound                = errors.New("store: record not found")
	ErrInvalidArgument         = errors.New("store: invalid argument")
	ErrListingInactive         = errors.New("store: listing is not active")
	ErrListingSoldOut          = errors.New("store: listing is sold out")
	ErrOrderNotReadyForPayment = errors.New("store: order is not ready for payment")
	ErrOrderNotReadyForCoupon  = errors.New("store: order is not ready for coupon assignment")
	ErrCouponAlreadyAssigned   = errors.New("store: coupon already assigned to order")
	ErrReservedCouponRequired  = errors.New("store: reserved coupon required before delivery")
	ErrReservedCouponInvalid   = errors.New("store: reserved coupon is not deliverable")
	ErrPaymentRequired         = errors.New("store: successful payment required before delivery")
	ErrCouponMismatch          = errors.New("store: coupon does not match order assignment")
)

func (p *Postgres) CreateCouponSource(ctx context.Context, params CreateCouponSourceParams) (CouponSource, error) {
	if err := p.ensurePool(); err != nil {
		return CouponSource{}, err
	}
	if strings.TrimSpace(params.SourceName) == "" || strings.TrimSpace(params.SourceType) == "" {
		return CouponSource{}, fmt.Errorf("%w: source name and source type are required", ErrInvalidArgument)
	}

	isActive := true
	if params.IsActive != nil {
		isActive = *params.IsActive
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO coupon_sources (
			source_name,
			source_type,
			contact_reference,
			rights_status,
			verification_notes,
			risk_rating,
			is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			source_name,
			source_type,
			contact_reference,
			rights_status,
			verification_notes,
			risk_rating,
			is_active,
			created_at,
			updated_at
	`,
		params.SourceName,
		params.SourceType,
		params.ContactReference,
		defaultString(params.RightsStatus, "unknown"),
		params.VerificationNotes,
		defaultString(params.RiskRating, "medium"),
		isActive,
	)

	source, err := scanCouponSource(row)
	if err != nil {
		return CouponSource{}, mapStoreErr(err)
	}

	return source, nil
}

func (p *Postgres) CreateListing(ctx context.Context, params CreateListingParams) (Listing, error) {
	if err := p.ensurePool(); err != nil {
		return Listing{}, err
	}
	if strings.TrimSpace(params.MerchantName) == "" || strings.TrimSpace(params.Title) == "" {
		return Listing{}, fmt.Errorf("%w: merchant name and title are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.FinalSaleDisclosureText) == "" {
		return Listing{}, fmt.Errorf("%w: final sale disclosure text is required", ErrInvalidArgument)
	}
	if params.ResellPriceAmount <= 0 {
		params.ResellPriceAmount = params.SalePriceAmount
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO listings (
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			resell_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			status,
			created_by_admin_id,
			published_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING
			id,
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			resell_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			photo_key,
			external_import_key,
			status,
			created_by_admin_id,
			published_at,
			created_at,
			updated_at
	`,
		params.MerchantName,
		params.Title,
		params.Description,
		params.CouponValueAmount,
		params.SalePriceAmount,
		params.ResellPriceAmount,
		defaultString(params.CurrencyCode, "ILS"),
		params.ExpirySummary,
		params.TermsSummary,
		params.RedemptionInstructions,
		params.FinalSaleDisclosureText,
		defaultString(params.Status, "draft"),
		params.CreatedByAdminID,
		params.PublishedAt,
	)

	listing, err := scanListing(row)
	if err != nil {
		return Listing{}, mapStoreErr(err)
	}

	return listing, nil
}

func (p *Postgres) IngestCoupons(ctx context.Context, params IngestCouponsParams) ([]Coupon, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}
	if params.ListingID == 0 {
		return nil, fmt.Errorf("%w: listing id is required", ErrInvalidArgument)
	}
	if len(params.Coupons) == 0 {
		return []Coupon{}, nil
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	inserted := make([]Coupon, 0, len(params.Coupons))
	for _, coupon := range params.Coupons {
		if coupon.SourceID == 0 {
			return nil, fmt.Errorf("%w: source id is required for coupon ingest", ErrInvalidArgument)
		}
		if coupon.ExpiryAt.IsZero() {
			return nil, fmt.Errorf("%w: coupon expiry is required", ErrInvalidArgument)
		}

		acquiredAt := time.Now().UTC()
		if coupon.AcquiredAt != nil {
			acquiredAt = coupon.AcquiredAt.UTC()
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO coupons (
				listing_id,
				source_id,
				merchant_name,
				coupon_title,
				coupon_value_amount,
				sale_price_amount,
				currency_code,
				coupon_code_ciphertext,
				coupon_code_nonce,
				coupon_masked_display,
				expiry_at,
				transferability_status,
				rights_verified_at,
				rights_verification_note,
				acquired_cost_amount,
				acquired_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			RETURNING
				id,
				listing_id,
				source_id,
				merchant_name,
				coupon_title,
				coupon_value_amount,
				sale_price_amount,
				currency_code,
				coupon_code_ciphertext,
				coupon_code_nonce,
				coupon_masked_display,
				expiry_at,
				transferability_status,
				inventory_status,
				rights_verified_at,
				rights_verification_note,
				acquired_cost_amount,
				acquired_at,
				created_at,
				updated_at
		`,
			params.ListingID,
			coupon.SourceID,
			coupon.MerchantName,
			coupon.CouponTitle,
			coupon.CouponValueAmount,
			coupon.SalePriceAmount,
			defaultString(coupon.CurrencyCode, "ILS"),
			coupon.CouponCodeCiphertext,
			coupon.CouponCodeNonce,
			coupon.CouponMaskedDisplay,
			coupon.ExpiryAt.UTC(),
			defaultString(coupon.TransferabilityStatus, "unknown"),
			coupon.RightsVerifiedAt,
			coupon.RightsVerificationNote,
			coupon.AcquiredCostAmount,
			acquiredAt,
		)

		insertedCoupon, err := scanCoupon(row)
		if err != nil {
			return nil, mapStoreErr(err)
		}
		inserted = append(inserted, insertedCoupon)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return inserted, nil
}

func (p *Postgres) ListActiveListings(ctx context.Context) ([]Listing, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	rows, err := p.Pool.Query(ctx, `
		SELECT
			l.id,
			l.merchant_name,
			l.title,
			l.description,
			l.coupon_value_amount,
			l.sale_price_amount,
			l.resell_price_amount,
			l.currency_code,
			l.expiry_summary,
			l.terms_summary,
			l.redemption_instructions,
			l.final_sale_disclosure_text,
			l.photo_key,
			l.external_import_key,
			l.status,
			l.created_by_admin_id,
			l.published_at,
			COUNT(c.id) FILTER (
				WHERE c.inventory_status = 'available'
					AND c.expiry_at > NOW()
			) AS available_inventory_count,
			MIN(c.expiry_at) FILTER (
				WHERE c.inventory_status = 'available'
					AND c.expiry_at > NOW()
			) AS next_coupon_expiry_at,
			l.created_at,
			l.updated_at
		FROM listings l
		LEFT JOIN coupons c ON c.listing_id = l.id
		WHERE l.status = 'active'
		GROUP BY l.id
		HAVING COUNT(c.id) FILTER (
			WHERE c.inventory_status = 'available'
				AND c.expiry_at > NOW()
		) > 0
		ORDER BY l.published_at DESC NULLS LAST, l.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]Listing, 0)
	for rows.Next() {
		listing, err := scanListingWithInventory(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return listings, nil
}

func (p *Postgres) CreateDraftOrder(ctx context.Context, params CreateDraftOrderParams) (Order, error) {
	if err := p.ensurePool(); err != nil {
		return Order{}, err
	}
	if params.UserID == 0 || params.ListingID == 0 {
		return Order{}, fmt.Errorf("%w: user id and listing id are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.OrderNumber) == "" {
		return Order{}, fmt.Errorf("%w: order number is required", ErrInvalidArgument)
	}
	if params.FinalSaleAcknowledgedAt.IsZero() {
		return Order{}, fmt.Errorf("%w: final sale acknowledgement timestamp is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Order{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	listing, err := loadListingForCheckout(ctx, tx, params.ListingID)
	if err != nil {
		return Order{}, err
	}
	if listing.Status != listingStatusActive {
		return Order{}, ErrListingInactive
	}

	coupon, err := reserveAvailableCoupon(ctx, tx, listing.ID)
	if err != nil {
		return Order{}, err
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO orders (
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			final_sale_acknowledged_at
		) VALUES ($1, $2, $3, $4, 'draft', $5, $6, $7)
		RETURNING
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
	`,
		params.UserID,
		params.ListingID,
		coupon.ID,
		params.OrderNumber,
		listing.CurrencyCode,
		listingEffectivePriceAmount(listing),
		params.FinalSaleAcknowledgedAt.UTC(),
	)

	order, err := scanOrder(row)
	if err != nil {
		return Order{}, mapStoreErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}

	return order, nil
}

func (p *Postgres) MarkOrderPendingPayment(ctx context.Context, params MarkOrderPendingPaymentParams) (Order, error) {
	if err := p.ensurePool(); err != nil {
		return Order{}, err
	}
	if params.OrderID == 0 {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE orders
		SET
			status = 'pending_payment',
			provider_checkout_reference = $2,
			placed_at = COALESCE(placed_at, NOW()),
			updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
		RETURNING
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
	`,
		params.OrderID,
		params.ProviderCheckoutReference,
	)

	order, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrOrderNotReadyForPayment
		}
		return Order{}, err
	}

	return order, nil
}

func (p *Postgres) ReleaseCheckoutReservation(ctx context.Context, params ReleaseCheckoutReservationParams) (Order, error) {
	if err := p.ensurePool(); err != nil {
		return Order{}, err
	}
	if params.OrderID == 0 {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.OrderStatus) == "" {
		return Order{}, fmt.Errorf("%w: order status is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Order{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order, err := loadOrderForUpdate(ctx, tx, params.OrderID)
	if err != nil {
		return Order{}, err
	}

	if order.CouponID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE coupons
			SET
				inventory_status = 'available',
				updated_at = NOW()
			WHERE id = $1
				AND inventory_status = 'reserved'
		`, *order.CouponID)
		if err != nil {
			return Order{}, err
		}
	}

	row := tx.QueryRow(ctx, `
		UPDATE orders
		SET
			coupon_id = NULL,
			status = $2,
			failure_reason = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
	`, params.OrderID, params.OrderStatus, params.FailureReason)

	releasedOrder, err := scanOrder(row)
	if err != nil {
		return Order{}, mapStoreErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}

	return releasedOrder, nil
}

func (p *Postgres) GetOrder(ctx context.Context, orderID int64) (Order, error) {
	if err := p.ensurePool(); err != nil {
		return Order{}, err
	}
	if orderID == 0 {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
		FROM orders
		WHERE id = $1
	`, orderID)

	order, err := scanOrder(row)
	if err != nil {
		return Order{}, mapStoreErr(err)
	}

	return order, nil
}

func (p *Postgres) GetOrderByProviderCheckoutReference(ctx context.Context, providerCheckoutReference string) (Order, error) {
	if err := p.ensurePool(); err != nil {
		return Order{}, err
	}
	if strings.TrimSpace(providerCheckoutReference) == "" {
		return Order{}, fmt.Errorf("%w: provider checkout reference is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
		FROM orders
		WHERE provider_checkout_reference = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, providerCheckoutReference)

	order, err := scanOrder(row)
	if err != nil {
		return Order{}, mapStoreErr(err)
	}

	return order, nil
}

func (p *Postgres) RecordPaymentEvent(ctx context.Context, params RecordPaymentEventParams) (Payment, error) {
	if err := p.ensurePool(); err != nil {
		return Payment{}, err
	}
	if params.OrderID == 0 {
		return Payment{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.ProviderName) == "" || strings.TrimSpace(params.Status) == "" {
		return Payment{}, fmt.Errorf("%w: provider name and status are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.ProviderPaymentID) == "" && strings.TrimSpace(params.ProviderCheckoutID) == "" {
		return Payment{}, fmt.Errorf("%w: provider payment id or checkout id is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Payment{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	orderStatus, err := orderStatusForPayment(params.Status)
	if err != nil {
		return Payment{}, err
	}

	payment, err := upsertPayment(ctx, tx, params)
	if err != nil {
		return Payment{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = $2,
			failure_reason = CASE
				WHEN $2 = 'failed' THEN $3
				WHEN $2 = 'pending_payment' THEN ''
				ELSE failure_reason
			END,
			updated_at = NOW()
		WHERE id = $1
	`,
		params.OrderID,
		orderStatus,
		params.FailureMessage,
	)
	if err != nil {
		return Payment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Payment{}, err
	}

	return payment, nil
}

func (p *Postgres) GetCoupon(ctx context.Context, couponID int64) (Coupon, error) {
	if err := p.ensurePool(); err != nil {
		return Coupon{}, err
	}
	if couponID == 0 {
		return Coupon{}, fmt.Errorf("%w: coupon id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			listing_id,
			source_id,
			merchant_name,
			coupon_title,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			coupon_code_ciphertext,
			coupon_code_nonce,
			coupon_masked_display,
			expiry_at,
			transferability_status,
			inventory_status,
			rights_verified_at,
			rights_verification_note,
			acquired_cost_amount,
			acquired_at,
			created_at,
			updated_at
		FROM coupons
		WHERE id = $1
	`, couponID)

	coupon, err := scanCoupon(row)
	if err != nil {
		return Coupon{}, mapStoreErr(err)
	}

	return coupon, nil
}

func (p *Postgres) PrepareReservedCouponForDelivery(ctx context.Context, orderID int64) (Coupon, error) {
	preparation, err := p.PrepareFulfillment(ctx, orderID)
	if err != nil {
		return Coupon{}, err
	}
	if preparation.Coupon == nil {
		return Coupon{}, ErrReservedCouponRequired
	}

	return *preparation.Coupon, nil
}

func (p *Postgres) PrepareFulfillment(ctx context.Context, orderID int64) (FulfillmentPreparation, error) {
	if err := p.ensurePool(); err != nil {
		return FulfillmentPreparation{}, err
	}
	if orderID == 0 {
		return FulfillmentPreparation{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return FulfillmentPreparation{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	preparation := FulfillmentPreparation{}

	order, err := loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return FulfillmentPreparation{}, err
	}
	preparation.Order = order

	delivery, err := loadCouponDeliveryForOrder(ctx, tx, order.ID)
	if err != nil {
		return FulfillmentPreparation{}, err
	}
	preparation.Delivery = delivery
	if delivery != nil && (delivery.Status == deliveryStatusSent || delivery.Status == deliveryStatusConfirmed) {
		if err := tx.Commit(ctx); err != nil {
			return FulfillmentPreparation{}, err
		}
		return preparation, nil
	}
	if order.Status == orderStatusDelivered {
		if err := tx.Commit(ctx); err != nil {
			return FulfillmentPreparation{}, err
		}
		return preparation, nil
	}

	if order.CouponID == nil {
		return preparation, ErrReservedCouponRequired
	}

	hasSuccessfulPayment, err := orderHasSuccessfulPayment(ctx, tx, order.ID)
	if err != nil {
		return FulfillmentPreparation{}, err
	}
	if err := fulfillmentStateError(order.Status, hasSuccessfulPayment); err != nil {
		return preparation, err
	}

	row := tx.QueryRow(ctx, `
		SELECT
			id,
			listing_id,
			source_id,
			merchant_name,
			coupon_title,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			coupon_code_ciphertext,
			coupon_code_nonce,
			coupon_masked_display,
			expiry_at,
			transferability_status,
			inventory_status,
			rights_verified_at,
			rights_verification_note,
			acquired_cost_amount,
			acquired_at,
			created_at,
			updated_at
		FROM coupons
		WHERE id = $1
		FOR UPDATE
	`, *order.CouponID)

	coupon, err := scanCoupon(row)
	if err != nil {
		return FulfillmentPreparation{}, mapStoreErr(err)
	}
	preparation.Coupon = &coupon

	if !coupon.ExpiryAt.After(time.Now().UTC()) {
		return preparation, ErrReservedCouponInvalid
	}

	switch coupon.InventoryStatus {
	case inventoryStatusReserved:
		row = tx.QueryRow(ctx, `
			UPDATE coupons
			SET
				inventory_status = 'assigned',
				updated_at = NOW()
			WHERE id = $1
			RETURNING
				id,
				listing_id,
				source_id,
				merchant_name,
				coupon_title,
				coupon_value_amount,
				sale_price_amount,
				currency_code,
				coupon_code_ciphertext,
				coupon_code_nonce,
				coupon_masked_display,
				expiry_at,
				transferability_status,
				inventory_status,
				rights_verified_at,
				rights_verification_note,
				acquired_cost_amount,
				acquired_at,
				created_at,
			updated_at
		`, coupon.ID)
		coupon, err = scanCoupon(row)
		if err != nil {
			return FulfillmentPreparation{}, mapStoreErr(err)
		}
		preparation.Coupon = &coupon
	case inventoryStatusAssigned, inventoryStatusDelivered:
		// Keep the already-linked coupon for idempotent delivery handling.
	default:
		return preparation, ErrReservedCouponInvalid
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = CASE
				WHEN status = 'paid' THEN 'delivery_pending'
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $1
	`, order.ID)
	if err != nil {
		return FulfillmentPreparation{}, err
	}
	if order.Status == orderStatusPaid {
		order.Status = orderStatusDeliveryPending
	}
	preparation.Order = order

	userRow := tx.QueryRow(ctx, `
		SELECT
			id,
			telegram_user_id,
			telegram_username,
			display_name,
			language_code,
			status,
			first_seen_at,
			last_seen_at,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`, order.UserID)
	user, err := scanUser(userRow)
	if err != nil {
		return FulfillmentPreparation{}, mapStoreErr(err)
	}
	preparation.User = &user

	if err := tx.Commit(ctx); err != nil {
		return FulfillmentPreparation{}, err
	}

	return preparation, nil
}

func fulfillmentStateError(orderStatus string, hasSuccessfulPayment bool) error {
	if !hasSuccessfulPayment {
		switch orderStatus {
		case orderStatusPendingPayment, orderStatusPaid, orderStatusDeliveryPending, orderStatusDelivered:
			return ErrPaymentRequired
		default:
			return ErrOrderNotReadyForCoupon
		}
	}

	switch orderStatus {
	case orderStatusPaid, orderStatusDeliveryPending, orderStatusDelivered:
		return nil
	default:
		return ErrOrderNotReadyForCoupon
	}
}

func (p *Postgres) AssignAvailableCoupon(ctx context.Context, orderID int64) (Coupon, error) {
	if err := p.ensurePool(); err != nil {
		return Coupon{}, err
	}
	if orderID == 0 {
		return Coupon{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Coupon{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order, err := loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return Coupon{}, err
	}
	if order.CouponID != nil {
		return Coupon{}, ErrCouponAlreadyAssigned
	}
	if order.Status != orderStatusPaid {
		return Coupon{}, ErrOrderNotReadyForCoupon
	}

	hasSuccessfulPayment, err := orderHasSuccessfulPayment(ctx, tx, order.ID)
	if err != nil {
		return Coupon{}, err
	}
	if !hasSuccessfulPayment {
		return Coupon{}, ErrPaymentRequired
	}

	var couponID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM coupons
		WHERE listing_id = $1
			AND inventory_status = 'available'
			AND expiry_at > NOW()
		ORDER BY expiry_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, order.ListingID).Scan(&couponID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Coupon{}, ErrListingSoldOut
		}
		return Coupon{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE coupons
		SET
			inventory_status = 'assigned',
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			listing_id,
			source_id,
			merchant_name,
			coupon_title,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			coupon_code_ciphertext,
			coupon_code_nonce,
			coupon_masked_display,
			expiry_at,
			transferability_status,
			inventory_status,
			rights_verified_at,
			rights_verification_note,
			acquired_cost_amount,
			acquired_at,
			created_at,
			updated_at
	`, couponID)

	coupon, err := scanCoupon(row)
	if err != nil {
		return Coupon{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			coupon_id = $2,
			status = 'delivery_pending',
			updated_at = NOW()
		WHERE id = $1
	`, order.ID, coupon.ID)
	if err != nil {
		return Coupon{}, mapStoreErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Coupon{}, err
	}

	return coupon, nil
}

func (p *Postgres) RecordDeliveryEvent(ctx context.Context, params RecordDeliveryEventParams) (CouponDelivery, error) {
	if err := p.ensurePool(); err != nil {
		return CouponDelivery{}, err
	}
	if params.OrderID == 0 || params.CouponID == 0 {
		return CouponDelivery{}, fmt.Errorf("%w: order id and coupon id are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.Status) == "" {
		return CouponDelivery{}, fmt.Errorf("%w: delivery status is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CouponDelivery{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order, err := loadOrderForUpdate(ctx, tx, params.OrderID)
	if err != nil {
		return CouponDelivery{}, err
	}
	if order.CouponID == nil || *order.CouponID != params.CouponID {
		return CouponDelivery{}, ErrCouponMismatch
	}

	hasSuccessfulPayment, err := orderHasSuccessfulPayment(ctx, tx, order.ID)
	if err != nil {
		return CouponDelivery{}, err
	}
	if !hasSuccessfulPayment {
		return CouponDelivery{}, ErrPaymentRequired
	}

	deliveryStatus := params.Status
	deliveryChannel := defaultString(params.DeliveryChannel, "telegram_bot")

	row := tx.QueryRow(ctx, `
		INSERT INTO coupon_deliveries (
			order_id,
			coupon_id,
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
			coupon_id = EXCLUDED.coupon_id,
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
			coupon_id,
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
		params.CouponID,
		deliveryChannel,
		deliveryStatus,
		params.TelegramMessageID,
		params.DeliveryPayloadHash,
		params.SentAt,
		params.ConfirmedAt,
		params.FailureReason,
	)

	delivery, err := scanCouponDelivery(row)
	if err != nil {
		return CouponDelivery{}, mapStoreErr(err)
	}

	couponInventoryStatus, nextOrderStatus, deliveredAt := deliveryOutcome(delivery)

	_, err = tx.Exec(ctx, `
		UPDATE coupons
		SET
			inventory_status = $2,
			updated_at = NOW()
		WHERE id = $1
	`, params.CouponID, couponInventoryStatus)
	if err != nil {
		return CouponDelivery{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = $2,
			delivered_at = $3,
			updated_at = NOW()
		WHERE id = $1
	`, order.ID, nextOrderStatus, deliveredAt)
	if err != nil {
		return CouponDelivery{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CouponDelivery{}, err
	}

	return delivery, nil
}

func (p *Postgres) CreateSupportCase(ctx context.Context, params CreateSupportCaseParams) (SupportCase, error) {
	if err := p.ensurePool(); err != nil {
		return SupportCase{}, err
	}
	if strings.TrimSpace(params.CaseType) == "" || strings.TrimSpace(params.Summary) == "" {
		return SupportCase{}, fmt.Errorf("%w: case type and summary are required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO support_cases (
			user_id,
			order_id,
			coupon_id,
			case_type,
			status,
			priority,
			summary,
			resolution_note,
			assigned_admin_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING
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
	`,
		params.UserID,
		params.OrderID,
		params.CouponID,
		params.CaseType,
		defaultString(params.Status, "open"),
		defaultString(params.Priority, "medium"),
		params.Summary,
		params.ResolutionNote,
		params.AssignedAdminID,
	)

	supportCase, err := scanSupportCase(row)
	if err != nil {
		return SupportCase{}, mapStoreErr(err)
	}

	return supportCase, nil
}

func (p *Postgres) ensurePool() error {
	if p == nil || p.Pool == nil {
		return errors.New("postgres pool is not initialized")
	}
	return nil
}

func loadListingForCheckout(ctx context.Context, tx pgx.Tx, listingID int64) (Listing, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			resell_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			photo_key,
			external_import_key,
			status,
			created_by_admin_id,
			published_at,
			created_at,
			updated_at
		FROM listings
		WHERE id = $1
	`, listingID)

	var listing Listing
	err := row.Scan(
		&listing.ID,
		&listing.MerchantName,
		&listing.Title,
		&listing.Description,
		&listing.CouponValueAmount,
		&listing.SalePriceAmount,
		&listing.ResellPriceAmount,
		&listing.CurrencyCode,
		&listing.ExpirySummary,
		&listing.TermsSummary,
		&listing.RedemptionInstructions,
		&listing.FinalSaleDisclosureText,
		&listing.PhotoKey,
		&listing.ExternalImportKey,
		&listing.Status,
		&listing.CreatedByAdminID,
		&listing.PublishedAt,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}

	return listing, nil
}

func reserveAvailableCoupon(ctx context.Context, tx pgx.Tx, listingID int64) (Coupon, error) {
	var couponID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM coupons
		WHERE listing_id = $1
			AND inventory_status = 'available'
			AND expiry_at > NOW()
		ORDER BY expiry_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, listingID).Scan(&couponID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Coupon{}, ErrListingSoldOut
		}
		return Coupon{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE coupons
		SET
			inventory_status = 'reserved',
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			listing_id,
			source_id,
			merchant_name,
			coupon_title,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			coupon_code_ciphertext,
			coupon_code_nonce,
			coupon_masked_display,
			expiry_at,
			transferability_status,
			inventory_status,
			rights_verified_at,
			rights_verification_note,
			acquired_cost_amount,
			acquired_at,
			created_at,
			updated_at
	`, couponID)

	coupon, err := scanCoupon(row)
	if err != nil {
		return Coupon{}, mapStoreErr(err)
	}

	return coupon, nil
}

func loadOrderForUpdate(ctx context.Context, tx pgx.Tx, orderID int64) (Order, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			listing_id,
			coupon_id,
			order_number,
			status,
			currency_code,
			sale_price_amount,
			provider_checkout_reference,
			final_sale_acknowledged_at,
			failure_reason,
			placed_at,
			delivered_at,
			created_at,
			updated_at
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID)

	order, err := scanOrder(row)
	if err != nil {
		return Order{}, mapStoreErr(err)
	}

	return order, nil
}

func orderHasSuccessfulPayment(ctx context.Context, tx pgx.Tx, orderID int64) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM payments
			WHERE order_id = $1
				AND status IN ('authorized', 'captured')
		)
	`, orderID).Scan(&ok)
	return ok, err
}

func loadCouponDeliveryForOrder(ctx context.Context, tx pgx.Tx, orderID int64) (*CouponDelivery, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			order_id,
			coupon_id,
			delivery_channel,
			status,
			telegram_message_id,
			delivery_payload_hash,
			sent_at,
			confirmed_at,
			failure_reason,
			created_at,
			updated_at
		FROM coupon_deliveries
		WHERE order_id = $1
	`, orderID)

	delivery, err := scanCouponDelivery(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &delivery, nil
}

func upsertPayment(ctx context.Context, tx pgx.Tx, params RecordPaymentEventParams) (Payment, error) {
	if strings.TrimSpace(params.ProviderPaymentID) != "" && strings.TrimSpace(params.ProviderCheckoutID) != "" {
		row := tx.QueryRow(ctx, `
			UPDATE payments
			SET
				order_id = $3,
				provider_payment_id = $4,
				provider_checkout_id = $5,
				status = $6,
				amount = $7,
				currency_code = $8,
				failure_code = $9,
				failure_message = $10,
				captured_at = $11,
				updated_at = NOW()
			WHERE provider_name = $1
				AND provider_checkout_id = $2
			RETURNING
				id,
				order_id,
				provider_name,
				provider_payment_id,
				provider_checkout_id,
				status,
				amount,
				currency_code,
				failure_code,
				failure_message,
				captured_at,
				created_at,
				updated_at
		`,
			params.ProviderName,
			params.ProviderCheckoutID,
			params.OrderID,
			params.ProviderPaymentID,
			params.ProviderCheckoutID,
			params.Status,
			params.Amount,
			defaultString(params.CurrencyCode, "ILS"),
			params.FailureCode,
			params.FailureMessage,
			params.CapturedAt,
		)

		payment, err := scanPayment(row)
		if err == nil {
			return payment, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, err
		}
	}

	conflictTarget := "(provider_name, provider_checkout_id) WHERE provider_checkout_id <> ''"
	conflictValue := params.ProviderCheckoutID
	if strings.TrimSpace(params.ProviderPaymentID) != "" {
		conflictTarget = "(provider_name, provider_payment_id) WHERE provider_payment_id <> ''"
		conflictValue = params.ProviderPaymentID
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO payments (
			order_id,
			provider_name,
			provider_payment_id,
			provider_checkout_id,
			status,
			amount,
			currency_code,
			failure_code,
			failure_message,
			captured_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT %s DO UPDATE
		SET
			order_id = EXCLUDED.order_id,
			provider_payment_id = EXCLUDED.provider_payment_id,
			provider_checkout_id = EXCLUDED.provider_checkout_id,
			status = EXCLUDED.status,
			amount = EXCLUDED.amount,
			currency_code = EXCLUDED.currency_code,
			failure_code = EXCLUDED.failure_code,
			failure_message = EXCLUDED.failure_message,
			captured_at = EXCLUDED.captured_at,
			updated_at = NOW()
		RETURNING
			id,
			order_id,
			provider_name,
			provider_payment_id,
			provider_checkout_id,
			status,
			amount,
			currency_code,
			failure_code,
			failure_message,
			captured_at,
			created_at,
			updated_at
	`, conflictTarget),
		params.OrderID,
		params.ProviderName,
		defaultString(params.ProviderPaymentID, ""),
		defaultString(params.ProviderCheckoutID, ""),
		params.Status,
		params.Amount,
		defaultString(params.CurrencyCode, "ILS"),
		params.FailureCode,
		params.FailureMessage,
		params.CapturedAt,
	)

	payment, err := scanPayment(row)
	if err != nil {
		return Payment{}, fmt.Errorf("upsert payment using conflict key %q: %w", conflictValue, err)
	}

	return payment, nil
}

func deliveryOutcome(delivery CouponDelivery) (string, string, *time.Time) {
	switch delivery.Status {
	case deliveryStatusSent, deliveryStatusConfirmed:
		if delivery.ConfirmedAt != nil {
			return inventoryStatusDelivered, orderStatusDelivered, delivery.ConfirmedAt
		}
		if delivery.SentAt != nil {
			return inventoryStatusDelivered, orderStatusDelivered, delivery.SentAt
		}
		now := time.Now().UTC()
		return inventoryStatusDelivered, orderStatusDelivered, &now
	default:
		return inventoryStatusAssigned, orderStatusDeliveryPending, nil
	}
}

func orderStatusForPayment(paymentStatus string) (string, error) {
	switch paymentStatus {
	case paymentStatusPending:
		return orderStatusPendingPayment, nil
	case paymentStatusAuthorized, paymentStatusCaptured:
		return orderStatusPaid, nil
	case paymentStatusFailed, paymentStatusCancelled:
		return orderStatusFailed, nil
	case paymentStatusChargeback, paymentStatusDisputed:
		return orderStatusDisputed, nil
	default:
		return "", fmt.Errorf("%w: unsupported payment status %q", ErrInvalidArgument, paymentStatus)
	}
}

func scanCouponSource(row pgx.Row) (CouponSource, error) {
	var source CouponSource
	err := row.Scan(
		&source.ID,
		&source.SourceName,
		&source.SourceType,
		&source.ContactReference,
		&source.RightsStatus,
		&source.VerificationNotes,
		&source.RiskRating,
		&source.IsActive,
		&source.CreatedAt,
		&source.UpdatedAt,
	)
	return source, err
}

func scanListing(row pgx.Row) (Listing, error) {
	var listing Listing
	err := row.Scan(
		&listing.ID,
		&listing.MerchantName,
		&listing.Title,
		&listing.Description,
		&listing.CouponValueAmount,
		&listing.SalePriceAmount,
		&listing.ResellPriceAmount,
		&listing.CurrencyCode,
		&listing.ExpirySummary,
		&listing.TermsSummary,
		&listing.RedemptionInstructions,
		&listing.FinalSaleDisclosureText,
		&listing.PhotoKey,
		&listing.ExternalImportKey,
		&listing.Status,
		&listing.CreatedByAdminID,
		&listing.PublishedAt,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	return listing, err
}

func scanListingWithInventory(row pgx.Row) (Listing, error) {
	var listing Listing
	err := row.Scan(
		&listing.ID,
		&listing.MerchantName,
		&listing.Title,
		&listing.Description,
		&listing.CouponValueAmount,
		&listing.SalePriceAmount,
		&listing.ResellPriceAmount,
		&listing.CurrencyCode,
		&listing.ExpirySummary,
		&listing.TermsSummary,
		&listing.RedemptionInstructions,
		&listing.FinalSaleDisclosureText,
		&listing.PhotoKey,
		&listing.ExternalImportKey,
		&listing.Status,
		&listing.CreatedByAdminID,
		&listing.PublishedAt,
		&listing.AvailableInventoryCount,
		&listing.NextCouponExpiryAt,
		&listing.CreatedAt,
		&listing.UpdatedAt,
	)
	return listing, err
}

func scanCoupon(row pgx.Row) (Coupon, error) {
	var coupon Coupon
	err := row.Scan(
		&coupon.ID,
		&coupon.ListingID,
		&coupon.SourceID,
		&coupon.MerchantName,
		&coupon.CouponTitle,
		&coupon.CouponValueAmount,
		&coupon.SalePriceAmount,
		&coupon.CurrencyCode,
		&coupon.CouponCodeCiphertext,
		&coupon.CouponCodeNonce,
		&coupon.CouponMaskedDisplay,
		&coupon.ExpiryAt,
		&coupon.TransferabilityStatus,
		&coupon.InventoryStatus,
		&coupon.RightsVerifiedAt,
		&coupon.RightsVerificationNote,
		&coupon.AcquiredCostAmount,
		&coupon.AcquiredAt,
		&coupon.CreatedAt,
		&coupon.UpdatedAt,
	)
	return coupon, err
}

func scanOrder(row pgx.Row) (Order, error) {
	var order Order
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.ListingID,
		&order.CouponID,
		&order.OrderNumber,
		&order.Status,
		&order.CurrencyCode,
		&order.SalePriceAmount,
		&order.ProviderCheckoutReference,
		&order.FinalSaleAcknowledgedAt,
		&order.FailureReason,
		&order.PlacedAt,
		&order.DeliveredAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	return order, err
}

func scanPayment(row pgx.Row) (Payment, error) {
	var payment Payment
	err := row.Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.ProviderName,
		&payment.ProviderPaymentID,
		&payment.ProviderCheckoutID,
		&payment.Status,
		&payment.Amount,
		&payment.CurrencyCode,
		&payment.FailureCode,
		&payment.FailureMessage,
		&payment.CapturedAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	return payment, err
}

func scanCouponDelivery(row pgx.Row) (CouponDelivery, error) {
	var delivery CouponDelivery
	err := row.Scan(
		&delivery.ID,
		&delivery.OrderID,
		&delivery.CouponID,
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

func scanSupportCase(row pgx.Row) (SupportCase, error) {
	var supportCase SupportCase
	err := row.Scan(
		&supportCase.ID,
		&supportCase.UserID,
		&supportCase.OrderID,
		&supportCase.CouponID,
		&supportCase.CaseType,
		&supportCase.Status,
		&supportCase.Priority,
		&supportCase.Summary,
		&supportCase.ResolutionNote,
		&supportCase.AssignedAdminID,
		&supportCase.CreatedAt,
		&supportCase.UpdatedAt,
	)
	return supportCase, err
}

func (p *Postgres) GetOrCreateUser(ctx context.Context, telegramUserID int64, displayName, userName, languageCode string) (User, error) {
	if err := p.ensurePool(); err != nil {
		return User{}, err
	}
	if telegramUserID == 0 {
		return User{}, fmt.Errorf("%w: telegram user id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO users (
			telegram_user_id,
			telegram_username,
			display_name,
			language_code,
			status,
			first_seen_at,
			last_seen_at
		) VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE
		SET
			telegram_username = EXCLUDED.telegram_username,
			display_name = EXCLUDED.display_name,
			language_code = EXCLUDED.language_code,
			last_seen_at = NOW(),
			updated_at = NOW()
		RETURNING
			id,
			telegram_user_id,
			telegram_username,
			display_name,
			language_code,
			status,
			first_seen_at,
			last_seen_at,
			created_at,
			updated_at
	`,
		telegramUserID,
		userName,
		displayName,
		languageCode,
	)

	return scanUser(row)
}

func (p *Postgres) GetUserByID(ctx context.Context, userID int64) (User, error) {
	if err := p.ensurePool(); err != nil {
		return User{}, err
	}
	if userID == 0 {
		return User{}, fmt.Errorf("%w: user id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			telegram_user_id,
			telegram_username,
			display_name,
			language_code,
			status,
			first_seen_at,
			last_seen_at,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`, userID)

	user, err := scanUser(row)
	if err != nil {
		return User{}, mapStoreErr(err)
	}

	return user, nil
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func listingEffectivePriceAmount(listing Listing) int64 {
	return EffectiveListingPriceAmount(listing)
}

func mapStoreErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func isNotFoundErr(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrNotFound)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.TelegramUsername,
		&user.DisplayName,
		&user.LanguageCode,
		&user.Status,
		&user.FirstSeenAt,
		&user.LastSeenAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return User{}, mapStoreErr(err)
	}
	return user, nil
}
