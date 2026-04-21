package store

import (
	"context"
	"fmt"
	"strings"
)

func (p *Postgres) ListCouponSources(ctx context.Context) ([]CouponSource, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	rows, err := p.Pool.Query(ctx, `
		SELECT
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
		FROM coupon_sources
		ORDER BY is_active DESC, source_name ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]CouponSource, 0)
	for rows.Next() {
		source, err := scanCouponSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sources, nil
}

func (p *Postgres) GetCouponSource(ctx context.Context, sourceID int64) (CouponSource, error) {
	if err := p.ensurePool(); err != nil {
		return CouponSource{}, err
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
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
		FROM coupon_sources
		WHERE id = $1
	`, sourceID)

	source, err := scanCouponSource(row)
	if err != nil {
		return CouponSource{}, mapStoreErr(err)
	}

	return source, nil
}

func (p *Postgres) UpdateCouponSource(ctx context.Context, sourceID int64, params CreateCouponSourceParams) (CouponSource, error) {
	if err := p.ensurePool(); err != nil {
		return CouponSource{}, err
	}
	if sourceID == 0 {
		return CouponSource{}, fmt.Errorf("%w: source id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.SourceName) == "" || strings.TrimSpace(params.SourceType) == "" {
		return CouponSource{}, fmt.Errorf("%w: source name and source type are required", ErrInvalidArgument)
	}

	isActive := true
	if params.IsActive != nil {
		isActive = *params.IsActive
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE coupon_sources
		SET
			source_name = $2,
			source_type = $3,
			contact_reference = $4,
			rights_status = $5,
			verification_notes = $6,
			risk_rating = $7,
			is_active = $8,
			updated_at = NOW()
		WHERE id = $1
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
		sourceID,
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

func (p *Postgres) ListListingsForAdmin(ctx context.Context) ([]ListingInventorySummary, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	rows, err := p.Pool.Query(ctx, listingInventorySummarySQL("", `
		ORDER BY l.published_at DESC NULLS LAST, l.created_at DESC, l.id DESC
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]ListingInventorySummary, 0)
	for rows.Next() {
		listing, err := scanListingInventorySummary(rows)
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

func (p *Postgres) GetListingForAdmin(ctx context.Context, listingID int64) (ListingInventorySummary, error) {
	if err := p.ensurePool(); err != nil {
		return ListingInventorySummary{}, err
	}

	row := p.Pool.QueryRow(ctx, listingInventorySummarySQL(`
		WHERE l.id = $1
	`, ""), listingID)

	listing, err := scanListingInventorySummary(row)
	if err != nil {
		return ListingInventorySummary{}, mapStoreErr(err)
	}

	return listing, nil
}

func (p *Postgres) GetListing(ctx context.Context, listingID int64) (Listing, error) {
	if err := p.ensurePool(); err != nil {
		return Listing{}, err
	}

	row := p.Pool.QueryRow(ctx, `
		SELECT
			id,
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			status,
			created_by_admin_id,
			published_at,
			created_at,
			updated_at
		FROM listings
		WHERE id = $1
	`, listingID)

	listing, err := scanListing(row)
	if err != nil {
		return Listing{}, mapStoreErr(err)
	}

	return listing, nil
}

func (p *Postgres) UpdateListing(ctx context.Context, listingID int64, params CreateListingParams) (Listing, error) {
	if err := p.ensurePool(); err != nil {
		return Listing{}, err
	}
	if listingID == 0 {
		return Listing{}, fmt.Errorf("%w: listing id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.MerchantName) == "" || strings.TrimSpace(params.Title) == "" {
		return Listing{}, fmt.Errorf("%w: merchant name and title are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.FinalSaleDisclosureText) == "" {
		return Listing{}, fmt.Errorf("%w: final sale disclosure text is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE listings
		SET
			merchant_name = $2,
			title = $3,
			description = $4,
			coupon_value_amount = $5,
			sale_price_amount = $6,
			currency_code = $7,
			expiry_summary = $8,
			terms_summary = $9,
			redemption_instructions = $10,
			final_sale_disclosure_text = $11,
			created_by_admin_id = $12,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			status,
			created_by_admin_id,
			published_at,
			created_at,
			updated_at
	`,
		listingID,
		params.MerchantName,
		params.Title,
		params.Description,
		params.CouponValueAmount,
		params.SalePriceAmount,
		defaultString(params.CurrencyCode, "ILS"),
		params.ExpirySummary,
		params.TermsSummary,
		params.RedemptionInstructions,
		params.FinalSaleDisclosureText,
		params.CreatedByAdminID,
	)

	listing, err := scanListing(row)
	if err != nil {
		return Listing{}, mapStoreErr(err)
	}

	return listing, nil
}

func (p *Postgres) UpdateListingStatus(ctx context.Context, listingID int64, status string) (Listing, error) {
	if err := p.ensurePool(); err != nil {
		return Listing{}, err
	}
	if listingID == 0 {
		return Listing{}, fmt.Errorf("%w: listing id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(status) == "" {
		return Listing{}, fmt.Errorf("%w: listing status is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE listings
		SET
			status = $2,
			published_at = CASE
				WHEN $2 = 'active' AND published_at IS NULL THEN NOW()
				ELSE published_at
			END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			merchant_name,
			title,
			description,
			coupon_value_amount,
			sale_price_amount,
			currency_code,
			expiry_summary,
			terms_summary,
			redemption_instructions,
			final_sale_disclosure_text,
			status,
			created_by_admin_id,
			published_at,
			created_at,
			updated_at
	`,
		listingID,
		status,
	)

	listing, err := scanListing(row)
	if err != nil {
		return Listing{}, mapStoreErr(err)
	}

	return listing, nil
}

func (p *Postgres) ListCouponsForAdmin(ctx context.Context, filter CouponListFilter) ([]CouponAdminSummary, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}

	rows, err := p.Pool.Query(ctx, `
		SELECT
			c.id,
			c.listing_id,
			c.source_id,
			c.merchant_name,
			c.coupon_title,
			c.coupon_value_amount,
			c.sale_price_amount,
			c.currency_code,
			c.coupon_code_ciphertext,
			c.coupon_code_nonce,
			c.coupon_masked_display,
			c.expiry_at,
			c.transferability_status,
			c.inventory_status,
			c.rights_verified_at,
			c.rights_verification_note,
			c.acquired_cost_amount,
			c.acquired_at,
			c.created_at,
			c.updated_at,
			l.title AS listing_title,
			s.source_name,
			o.id AS assigned_order_id,
			COALESCE(o.status, '') AS assigned_order_status,
			COALESCE(u.display_name, '') AS assigned_user_display_name,
			u.telegram_user_id
		FROM coupons c
		INNER JOIN listings l ON l.id = c.listing_id
		INNER JOIN coupon_sources s ON s.id = c.source_id
		LEFT JOIN orders o ON o.coupon_id = c.id
		LEFT JOIN users u ON u.id = o.user_id
		WHERE ($1::BIGINT IS NULL OR c.listing_id = $1)
			AND ($2::TEXT = '' OR c.inventory_status = $2)
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $3
	`, filter.ListingID, filter.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coupons := make([]CouponAdminSummary, 0)
	for rows.Next() {
		coupon, err := scanCouponAdminSummary(rows)
		if err != nil {
			return nil, err
		}
		coupons = append(coupons, coupon)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return coupons, nil
}

func (p *Postgres) UpdateCouponInventoryStatus(ctx context.Context, couponID int64, status string) (Coupon, error) {
	if err := p.ensurePool(); err != nil {
		return Coupon{}, err
	}
	if couponID == 0 {
		return Coupon{}, fmt.Errorf("%w: coupon id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(status) == "" {
		return Coupon{}, fmt.Errorf("%w: coupon status is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE coupons
		SET
			inventory_status = $2,
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
	`,
		couponID,
		status,
	)

	coupon, err := scanCoupon(row)
	if err != nil {
		return Coupon{}, mapStoreErr(err)
	}

	return coupon, nil
}

func (p *Postgres) ListOrdersForAdmin(ctx context.Context, status string, limit int) ([]OrderAdminSummary, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := p.Pool.Query(ctx, orderAdminSummarySQL(`
		WHERE ($1::TEXT = '' OR o.status = $1)
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $2
	`), status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]OrderAdminSummary, 0)
	for rows.Next() {
		order, err := scanOrderAdminSummary(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (p *Postgres) GetOrderForAdmin(ctx context.Context, orderID int64) (OrderAdminSummary, error) {
	if err := p.ensurePool(); err != nil {
		return OrderAdminSummary{}, err
	}

	row := p.Pool.QueryRow(ctx, orderAdminSummarySQL(`
		WHERE o.id = $1
	`), orderID)

	order, err := scanOrderAdminSummary(row)
	if err != nil {
		return OrderAdminSummary{}, mapStoreErr(err)
	}

	return order, nil
}

func (p *Postgres) ListPaymentsForOrder(ctx context.Context, orderID int64) ([]Payment, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	rows, err := p.Pool.Query(ctx, `
		SELECT
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
		FROM payments
		WHERE order_id = $1
		ORDER BY created_at DESC, id DESC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := make([]Payment, 0)
	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}

func (p *Postgres) GetCouponDeliveryForOrder(ctx context.Context, orderID int64) (*CouponDelivery, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	row := p.Pool.QueryRow(ctx, `
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
		if mapStoreErr(err) == ErrNotFound {
			return nil, nil
		}
		return nil, mapStoreErr(err)
	}

	return &delivery, nil
}

func (p *Postgres) ListSupportCases(ctx context.Context, status string) ([]SupportCaseSummary, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}

	rows, err := p.Pool.Query(ctx, supportCaseSummarySQL(`
		WHERE ($1::TEXT = '' OR sc.status = $1)
		ORDER BY sc.created_at DESC, sc.id DESC
	`), status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	supportCases := make([]SupportCaseSummary, 0)
	for rows.Next() {
		supportCase, err := scanSupportCaseSummary(rows)
		if err != nil {
			return nil, err
		}
		supportCases = append(supportCases, supportCase)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return supportCases, nil
}

func (p *Postgres) GetSupportCase(ctx context.Context, supportCaseID int64) (SupportCaseSummary, error) {
	if err := p.ensurePool(); err != nil {
		return SupportCaseSummary{}, err
	}

	row := p.Pool.QueryRow(ctx, supportCaseSummarySQL(`
		WHERE sc.id = $1
	`), supportCaseID)

	supportCase, err := scanSupportCaseSummary(row)
	if err != nil {
		return SupportCaseSummary{}, mapStoreErr(err)
	}

	return supportCase, nil
}

func (p *Postgres) ResolveSupportCase(ctx context.Context, supportCaseID int64, resolutionNote string, assignedAdminID string) (SupportCase, error) {
	if err := p.ensurePool(); err != nil {
		return SupportCase{}, err
	}
	if supportCaseID == 0 {
		return SupportCase{}, fmt.Errorf("%w: support case id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(resolutionNote) == "" {
		return SupportCase{}, fmt.Errorf("%w: resolution note is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		UPDATE support_cases
		SET
			status = 'resolved',
			resolution_note = $2,
			assigned_admin_id = $3,
			updated_at = NOW()
		WHERE id = $1
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
		supportCaseID,
		resolutionNote,
		assignedAdminID,
	)

	supportCase, err := scanSupportCase(row)
	if err != nil {
		return SupportCase{}, mapStoreErr(err)
	}

	return supportCase, nil
}

func listingInventorySummarySQL(whereClause string, suffix string) string {
	return `
		SELECT
			l.id,
			l.merchant_name,
			l.title,
			l.description,
			l.coupon_value_amount,
			l.sale_price_amount,
			l.currency_code,
			l.expiry_summary,
			l.terms_summary,
			l.redemption_instructions,
			l.final_sale_disclosure_text,
			l.status,
			l.created_by_admin_id,
			l.published_at,
			COUNT(c.id) AS total_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'available' AND c.expiry_at > NOW()) AS available_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'assigned') AS assigned_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'delivered') AS delivered_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'voided') AS voided_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'disputed') AS disputed_coupons,
			MIN(c.expiry_at) FILTER (WHERE c.inventory_status = 'available' AND c.expiry_at > NOW()) AS next_coupon_expiry_at,
			l.created_at,
			l.updated_at
		FROM listings l
		LEFT JOIN coupons c ON c.listing_id = l.id
	` + whereClause + `
		GROUP BY l.id
	` + suffix
}

func boolPtr(value bool) *bool {
	return &value
}

func orderAdminSummarySQL(suffix string) string {
	return `
		SELECT
			o.id,
			o.user_id,
			o.listing_id,
			o.coupon_id,
			o.order_number,
			o.status,
			o.currency_code,
			o.sale_price_amount,
			o.provider_checkout_reference,
			o.final_sale_acknowledged_at,
			o.failure_reason,
			o.placed_at,
			o.delivered_at,
			o.created_at,
			o.updated_at,
			u.telegram_user_id,
			u.telegram_username,
			u.display_name,
			COALESCE(lp.status, '') AS payment_status,
			COALESCE(lp.provider_name, '') AS payment_provider_name,
			lp.captured_at,
			COALESCE(cd.status, '') AS delivery_status,
			COALESCE(cd.delivery_channel, '') AS delivery_channel,
			cd.sent_at,
			COALESCE(c.coupon_masked_display, '') AS assigned_coupon_masked_display,
			c.expiry_at
		FROM orders o
		INNER JOIN users u ON u.id = o.user_id
		LEFT JOIN LATERAL (
			SELECT
				status,
				provider_name,
				captured_at
			FROM payments
			WHERE order_id = o.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) lp ON TRUE
		LEFT JOIN coupon_deliveries cd ON cd.order_id = o.id
		LEFT JOIN coupons c ON c.id = o.coupon_id
	` + suffix
}

func supportCaseSummarySQL(suffix string) string {
	return `
		SELECT
			sc.id,
			sc.user_id,
			sc.order_id,
			sc.coupon_id,
			sc.case_type,
			sc.status,
			sc.priority,
			sc.summary,
			sc.resolution_note,
			sc.assigned_admin_id,
			sc.created_at,
			sc.updated_at,
			COALESCE(o.status, '') AS order_status,
			COALESCE(c.inventory_status, '') AS coupon_status,
			COALESCE(l.title, '') AS listing_title,
			COALESCE(u.display_name, '') AS user_display_name
		FROM support_cases sc
		LEFT JOIN orders o ON o.id = sc.order_id
		LEFT JOIN coupons c ON c.id = sc.coupon_id
		LEFT JOIN listings l ON l.id = o.listing_id
		LEFT JOIN users u ON u.id = sc.user_id
	` + suffix
}

func scanListingInventorySummary(row interface {
	Scan(dest ...any) error
}) (ListingInventorySummary, error) {
	var summary ListingInventorySummary
	err := row.Scan(
		&summary.ID,
		&summary.MerchantName,
		&summary.Title,
		&summary.Description,
		&summary.CouponValueAmount,
		&summary.SalePriceAmount,
		&summary.CurrencyCode,
		&summary.ExpirySummary,
		&summary.TermsSummary,
		&summary.RedemptionInstructions,
		&summary.FinalSaleDisclosureText,
		&summary.Status,
		&summary.CreatedByAdminID,
		&summary.PublishedAt,
		&summary.TotalCoupons,
		&summary.AvailableCoupons,
		&summary.AssignedCoupons,
		&summary.DeliveredCoupons,
		&summary.VoidedCoupons,
		&summary.DisputedCoupons,
		&summary.NextCouponExpiryAt,
		&summary.CreatedAt,
		&summary.UpdatedAt,
	)
	return summary, err
}

func scanCouponAdminSummary(row interface {
	Scan(dest ...any) error
}) (CouponAdminSummary, error) {
	var summary CouponAdminSummary
	err := row.Scan(
		&summary.ID,
		&summary.ListingID,
		&summary.SourceID,
		&summary.MerchantName,
		&summary.CouponTitle,
		&summary.CouponValueAmount,
		&summary.SalePriceAmount,
		&summary.CurrencyCode,
		&summary.CouponCodeCiphertext,
		&summary.CouponCodeNonce,
		&summary.CouponMaskedDisplay,
		&summary.ExpiryAt,
		&summary.TransferabilityStatus,
		&summary.InventoryStatus,
		&summary.RightsVerifiedAt,
		&summary.RightsVerificationNote,
		&summary.AcquiredCostAmount,
		&summary.AcquiredAt,
		&summary.CreatedAt,
		&summary.UpdatedAt,
		&summary.ListingTitle,
		&summary.SourceName,
		&summary.AssignedOrderID,
		&summary.AssignedOrderStatus,
		&summary.AssignedUserDisplayName,
		&summary.AssignedUserTelegramUserID,
	)
	return summary, err
}

func scanOrderAdminSummary(row interface {
	Scan(dest ...any) error
}) (OrderAdminSummary, error) {
	var summary OrderAdminSummary
	err := row.Scan(
		&summary.ID,
		&summary.UserID,
		&summary.ListingID,
		&summary.CouponID,
		&summary.OrderNumber,
		&summary.Status,
		&summary.CurrencyCode,
		&summary.SalePriceAmount,
		&summary.ProviderCheckoutReference,
		&summary.FinalSaleAcknowledgedAt,
		&summary.FailureReason,
		&summary.PlacedAt,
		&summary.DeliveredAt,
		&summary.CreatedAt,
		&summary.UpdatedAt,
		&summary.TelegramUserID,
		&summary.TelegramUsername,
		&summary.UserDisplayName,
		&summary.PaymentStatus,
		&summary.PaymentProviderName,
		&summary.PaymentCapturedAt,
		&summary.DeliveryStatus,
		&summary.DeliveryChannel,
		&summary.DeliverySentAt,
		&summary.AssignedCouponMaskedDisplay,
		&summary.AssignedCouponExpiryAt,
	)
	return summary, err
}

func scanSupportCaseSummary(row interface {
	Scan(dest ...any) error
}) (SupportCaseSummary, error) {
	var summary SupportCaseSummary
	err := row.Scan(
		&summary.ID,
		&summary.UserID,
		&summary.OrderID,
		&summary.CouponID,
		&summary.CaseType,
		&summary.Status,
		&summary.Priority,
		&summary.Summary,
		&summary.ResolutionNote,
		&summary.AssignedAdminID,
		&summary.CreatedAt,
		&summary.UpdatedAt,
		&summary.OrderStatus,
		&summary.CouponStatus,
		&summary.ListingTitle,
		&summary.UserDisplayName,
	)
	return summary, err
}
