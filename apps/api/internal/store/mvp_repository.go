package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	offerStatusActive = "active"

	predefinedCodeStatusAvailable = "available"
	predefinedCodeStatusAssigned  = "assigned"

	mvpOrderStatusAwaitingPayment       = "awaiting_payment"
	mvpOrderStatusPaymentClaimSubmitted = "payment_claim_submitted"
	mvpOrderStatusPaymentVerified       = "payment_verified"
	mvpOrderStatusPaymentRejected       = "payment_rejected"
	mvpOrderStatusSupportRequired       = "support_required"

	manualClaimStatusPending  = "pending_review"
	manualClaimStatusVerified = "verified"
	manualClaimStatusRejected = "rejected"
)

var (
	ErrOfferInactive             = errors.New("store: offer is not active")
	ErrOfferSoldOut              = errors.New("store: offer is sold out")
	ErrOfferPaymentLinkRequired  = errors.New("store: offer payment link is required")
	ErrPaymentClaimNotReviewable = errors.New("store: payment claim is not reviewable")
)

func (p *Postgres) CreateMerchantPartner(ctx context.Context, params CreateMerchantPartnerParams) (MerchantPartner, error) {
	if err := p.ensurePool(); err != nil {
		return MerchantPartner{}, err
	}
	if strings.TrimSpace(params.BusinessName) == "" {
		return MerchantPartner{}, fmt.Errorf("%w: business name is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		INSERT INTO merchant_partners (
			business_name,
			contact_reference,
			status,
			approval_notes,
			merchant_disclosure_text,
			support_contact,
			default_payment_link
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			business_name,
			contact_reference,
			status,
			approval_notes,
			merchant_disclosure_text,
			support_contact,
			default_payment_link,
			created_at,
			updated_at
	`,
		params.BusinessName,
		params.ContactReference,
		defaultString(params.Status, "lead"),
		params.ApprovalNotes,
		params.MerchantDisclosureText,
		params.SupportContact,
		params.DefaultPaymentLink,
	)

	partner, err := scanMerchantPartner(row)
	if err != nil {
		return MerchantPartner{}, mapStoreErr(err)
	}

	return partner, nil
}

func (p *Postgres) CreateOffer(ctx context.Context, params CreateOfferParams) (Offer, error) {
	if err := p.ensurePool(); err != nil {
		return Offer{}, err
	}
	if params.MerchantPartnerID == 0 {
		return Offer{}, fmt.Errorf("%w: merchant partner id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.Title) == "" {
		return Offer{}, fmt.Errorf("%w: offer title is required", ErrInvalidArgument)
	}
	if params.PriceAmount <= 0 {
		return Offer{}, fmt.Errorf("%w: offer price must be positive", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, `
		WITH partner AS (
			SELECT
				id,
				business_name,
				merchant_disclosure_text,
				support_contact,
				default_payment_link
			FROM merchant_partners
			WHERE id = $1
		), inserted AS (
			INSERT INTO offers (
				merchant_partner_id,
				merchant_name,
				title,
				description,
				price_amount,
				currency_code,
				payment_link,
				merchant_disclosure_text,
				redemption_terms,
				support_contact,
				status,
				published_at
			)
			SELECT
				partner.id,
				COALESCE(NULLIF($2, ''), partner.business_name),
				$3,
				$4,
				$5,
				$6,
				COALESCE(NULLIF($7, ''), partner.default_payment_link),
				COALESCE(NULLIF($8, ''), partner.merchant_disclosure_text),
				$9,
				COALESCE(NULLIF($10, ''), partner.support_contact),
				$11,
				CASE
					WHEN $11 = 'active' THEN COALESCE($12, NOW())
					ELSE $12
				END
			FROM partner
			RETURNING
				id,
				merchant_partner_id,
				merchant_name,
				title,
				description,
				price_amount,
				currency_code,
				payment_link,
				merchant_disclosure_text,
				redemption_terms,
				support_contact,
				status,
				published_at,
				created_at,
				updated_at
		)
		SELECT
			id,
			merchant_partner_id,
			merchant_name,
			title,
			description,
			price_amount,
			currency_code,
			payment_link,
			merchant_disclosure_text,
			redemption_terms,
			support_contact,
			status,
			published_at,
			0::BIGINT AS available_code_count,
			NULL::TIMESTAMPTZ AS next_code_expiry_at,
			created_at,
			updated_at
		FROM inserted
	`,
		params.MerchantPartnerID,
		params.MerchantName,
		params.Title,
		params.Description,
		params.PriceAmount,
		defaultString(params.CurrencyCode, "ILS"),
		params.PaymentLink,
		params.MerchantDisclosureText,
		params.RedemptionTerms,
		params.SupportContact,
		defaultString(params.Status, "draft"),
		params.PublishedAt,
	)

	offer, err := scanOffer(row)
	if err != nil {
		return Offer{}, mapStoreErr(err)
	}

	return offer, nil
}

func (p *Postgres) ListActiveOffers(ctx context.Context, limit int) ([]Offer, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 3
	}

	rows, err := p.Pool.Query(ctx, offerSelectSQL(`
		WHERE o.status = 'active'
			AND COALESCE(inventory.available_code_count, 0) > 0
		ORDER BY o.published_at DESC NULLS LAST, o.id DESC
		LIMIT $1
	`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	offers := make([]Offer, 0, limit)
	for rows.Next() {
		offer, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return offers, nil
}

func (p *Postgres) GetOffer(ctx context.Context, offerID int64) (Offer, error) {
	if err := p.ensurePool(); err != nil {
		return Offer{}, err
	}
	if offerID == 0 {
		return Offer{}, fmt.Errorf("%w: offer id is required", ErrInvalidArgument)
	}

	row := p.Pool.QueryRow(ctx, offerSelectSQL(`
		WHERE o.id = $1
	`), offerID)

	offer, err := scanOffer(row)
	if err != nil {
		return Offer{}, mapStoreErr(err)
	}

	return offer, nil
}

func (p *Postgres) IngestPredefinedCodes(ctx context.Context, params IngestPredefinedCodesParams) ([]PredefinedCode, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}
	if params.OfferID == 0 {
		return nil, fmt.Errorf("%w: offer id is required", ErrInvalidArgument)
	}
	if len(params.Codes) == 0 {
		return []PredefinedCode{}, nil
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var merchantPartnerID int64
	err = tx.QueryRow(ctx, `
		SELECT merchant_partner_id
		FROM offers
		WHERE id = $1
	`, params.OfferID).Scan(&merchantPartnerID)
	if err != nil {
		return nil, mapStoreErr(err)
	}

	inserted := make([]PredefinedCode, 0, len(params.Codes))
	for _, code := range params.Codes {
		if len(code.CodeEncrypted) == 0 || strings.TrimSpace(code.CodeMaskedDisplay) == "" {
			return nil, fmt.Errorf("%w: encrypted code and masked display are required", ErrInvalidArgument)
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO predefined_codes (
				offer_id,
				merchant_partner_id,
				code_encrypted,
				code_masked_display,
				expiry_at,
				issued_batch_reference
			) VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING
				id,
				offer_id,
				merchant_partner_id,
				code_encrypted,
				code_masked_display,
				status,
				expiry_at,
				issued_batch_reference,
				created_at,
				updated_at
		`,
			params.OfferID,
			merchantPartnerID,
			code.CodeEncrypted,
			code.CodeMaskedDisplay,
			code.ExpiryAt,
			code.IssuedBatchReference,
		)

		created, err := scanPredefinedCode(row)
		if err != nil {
			return nil, mapStoreErr(err)
		}
		inserted = append(inserted, created)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return inserted, nil
}

func (p *Postgres) CreateAwaitingPaymentOrder(ctx context.Context, params CreateAwaitingPaymentOrderParams) (MVPOrder, error) {
	if err := p.ensurePool(); err != nil {
		return MVPOrder{}, err
	}
	if params.UserID == 0 || params.OfferID == 0 {
		return MVPOrder{}, fmt.Errorf("%w: user id and offer id are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.OrderNumber) == "" {
		return MVPOrder{}, fmt.Errorf("%w: order number is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MVPOrder{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	offer, err := loadOfferForUpdate(ctx, tx, params.OfferID)
	if err != nil {
		return MVPOrder{}, err
	}
	if offer.Status != offerStatusActive {
		return MVPOrder{}, ErrOfferInactive
	}
	if strings.TrimSpace(offer.PaymentLink) == "" {
		return MVPOrder{}, ErrOfferPaymentLinkRequired
	}

	var availableCodes int64
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM predefined_codes
		WHERE offer_id = $1
			AND status = 'available'
			AND (expiry_at IS NULL OR expiry_at > NOW())
	`, offer.ID).Scan(&availableCodes)
	if err != nil {
		return MVPOrder{}, err
	}
	if availableCodes == 0 {
		return MVPOrder{}, ErrOfferSoldOut
	}

	row := tx.QueryRow(ctx, mvpOrderSelectSQL(`
		FROM (
			INSERT INTO orders (
				user_id,
				offer_id,
				order_number,
				status,
				currency_code,
				sale_price_amount,
				paybox_payment_link,
				placed_at
			) VALUES ($1, $2, $3, 'awaiting_payment', $4, $5, $6, NOW())
			RETURNING *
		) ord
		INNER JOIN offers o ON o.id = ord.offer_id
	`),
		params.UserID,
		offer.ID,
		params.OrderNumber,
		offer.CurrencyCode,
		offer.PriceAmount,
		offer.PaymentLink,
	)

	order, err := scanMVPOrder(row)
	if err != nil {
		return MVPOrder{}, mapStoreErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return MVPOrder{}, err
	}

	return order, nil
}

func (p *Postgres) SubmitManualPaymentClaim(ctx context.Context, params SubmitManualPaymentClaimParams) (ManualPaymentClaim, error) {
	if err := p.ensurePool(); err != nil {
		return ManualPaymentClaim{}, err
	}
	if params.OrderID == 0 {
		return ManualPaymentClaim{}, fmt.Errorf("%w: order id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.PayerUsername) == "" {
		return ManualPaymentClaim{}, fmt.Errorf("%w: payer username is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ManualPaymentClaim{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order, err := loadMVPOrderForUpdate(ctx, tx, params.OrderID)
	if err != nil {
		return ManualPaymentClaim{}, err
	}

	existing, err := loadPendingManualPaymentClaim(ctx, tx, params.OrderID)
	if err != nil {
		return ManualPaymentClaim{}, err
	}
	if existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return ManualPaymentClaim{}, err
		}
		return *existing, nil
	}

	if order.Status != mvpOrderStatusAwaitingPayment {
		return ManualPaymentClaim{}, ErrOrderNotReadyForPayment
	}

	claimedAmount := params.ClaimedAmount
	if claimedAmount <= 0 {
		claimedAmount = order.PriceAmount
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO manual_payment_claims (
			order_id,
			payer_username,
			claimed_amount,
			submitted_at
		) VALUES ($1, $2, $3, NOW())
		RETURNING
			id,
			order_id,
			payer_username,
			claimed_amount,
			submitted_at,
			review_status,
			reviewed_by,
			reviewed_at,
			review_note,
			created_at,
			updated_at
	`, params.OrderID, strings.TrimSpace(params.PayerUsername), claimedAmount)

	claim, err := scanManualPaymentClaim(row)
	if err != nil {
		return ManualPaymentClaim{}, mapStoreErr(err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET
			status = 'payment_claim_submitted',
			updated_at = NOW()
		WHERE id = $1
	`, params.OrderID)
	if err != nil {
		return ManualPaymentClaim{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ManualPaymentClaim{}, err
	}

	return claim, nil
}

func (p *Postgres) ListPendingManualPaymentClaims(ctx context.Context, limit int) ([]ManualPaymentClaim, error) {
	if err := p.ensurePool(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := p.Pool.Query(ctx, manualPaymentClaimSummarySQL(`
		WHERE mpc.review_status = 'pending_review'
		ORDER BY mpc.submitted_at ASC, mpc.id ASC
		LIMIT $1
	`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claims := make([]ManualPaymentClaim, 0, limit)
	for rows.Next() {
		claim, err := scanManualPaymentClaimSummary(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return claims, nil
}

func (p *Postgres) ApproveManualPaymentClaim(ctx context.Context, params ReviewManualPaymentClaimParams) (ApproveManualPaymentClaimResult, error) {
	if err := p.ensurePool(); err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}
	if params.ClaimID == 0 {
		return ApproveManualPaymentClaimResult{}, fmt.Errorf("%w: claim id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.ReviewedBy) == "" {
		return ApproveManualPaymentClaimResult{}, fmt.Errorf("%w: reviewed by is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	claim, err := loadManualPaymentClaimForUpdate(ctx, tx, params.ClaimID)
	if err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}
	order, err := loadMVPOrderForUpdate(ctx, tx, claim.OrderID)
	if err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}

	if claim.ReviewStatus == manualClaimStatusVerified {
		code, err := loadPredefinedCodeMaybe(ctx, tx, order.PredefinedCodeID)
		if err != nil {
			return ApproveManualPaymentClaimResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ApproveManualPaymentClaimResult{}, err
		}
		return ApproveManualPaymentClaimResult{Order: order, Claim: claim, Code: code}, nil
	}
	if claim.ReviewStatus == manualClaimStatusRejected {
		return ApproveManualPaymentClaimResult{}, ErrPaymentClaimNotReviewable
	}

	code, err := assignAvailablePredefinedCode(ctx, tx, order.OfferID)
	if err != nil {
		if errors.Is(err, ErrOfferSoldOut) {
			claim, order, err = markManualClaimApprovedWithoutCode(ctx, tx, claim.ID, order.ID, params)
			if err != nil {
				return ApproveManualPaymentClaimResult{}, err
			}
			if err := createNoCodeSupportCase(ctx, tx, order, claim); err != nil {
				return ApproveManualPaymentClaimResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return ApproveManualPaymentClaimResult{}, err
			}
			return ApproveManualPaymentClaimResult{Order: order, Claim: claim}, nil
		}
		return ApproveManualPaymentClaimResult{}, err
	}

	claim, order, err = markManualClaimApprovedWithCode(ctx, tx, claim.ID, order.ID, code.ID, params)
	if err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ApproveManualPaymentClaimResult{}, err
	}

	return ApproveManualPaymentClaimResult{Order: order, Claim: claim, Code: &code}, nil
}

func (p *Postgres) RejectManualPaymentClaim(ctx context.Context, params ReviewManualPaymentClaimParams) (ManualPaymentClaim, MVPOrder, error) {
	if err := p.ensurePool(); err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}
	if params.ClaimID == 0 {
		return ManualPaymentClaim{}, MVPOrder{}, fmt.Errorf("%w: claim id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(params.ReviewedBy) == "" {
		return ManualPaymentClaim{}, MVPOrder{}, fmt.Errorf("%w: reviewed by is required", ErrInvalidArgument)
	}

	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	claim, err := loadManualPaymentClaimForUpdate(ctx, tx, params.ClaimID)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}
	order, err := loadMVPOrderForUpdate(ctx, tx, claim.OrderID)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}

	if claim.ReviewStatus == manualClaimStatusRejected {
		if err := tx.Commit(ctx); err != nil {
			return ManualPaymentClaim{}, MVPOrder{}, err
		}
		return claim, order, nil
	}
	if claim.ReviewStatus == manualClaimStatusVerified {
		return ManualPaymentClaim{}, MVPOrder{}, ErrPaymentClaimNotReviewable
	}

	row := tx.QueryRow(ctx, `
		UPDATE manual_payment_claims
		SET
			review_status = 'rejected',
			reviewed_by = $2,
			reviewed_at = NOW(),
			review_note = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			order_id,
			payer_username,
			claimed_amount,
			submitted_at,
			review_status,
			reviewed_by,
			reviewed_at,
			review_note,
			created_at,
			updated_at
	`, claim.ID, strings.TrimSpace(params.ReviewedBy), params.ReviewNote)
	claim, err = scanManualPaymentClaim(row)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, mapStoreErr(err)
	}

	row = tx.QueryRow(ctx, mvpOrderSelectSQL(`
		FROM (
			UPDATE orders
			SET
				status = 'payment_rejected',
				failure_reason = $2,
				updated_at = NOW()
			WHERE id = $1
			RETURNING *
		) ord
		INNER JOIN offers o ON o.id = ord.offer_id
	`), order.ID, defaultString(params.ReviewNote, "payment_not_matched"))
	order, err = scanMVPOrder(row)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, mapStoreErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}

	return claim, order, nil
}

func loadOfferForUpdate(ctx context.Context, tx pgx.Tx, offerID int64) (Offer, error) {
	row := tx.QueryRow(ctx, offerSelectSQL(`
		WHERE o.id = $1
		FOR UPDATE OF o
	`), offerID)

	offer, err := scanOffer(row)
	if err != nil {
		return Offer{}, mapStoreErr(err)
	}
	return offer, nil
}

func loadMVPOrderForUpdate(ctx context.Context, tx pgx.Tx, orderID int64) (MVPOrder, error) {
	row := tx.QueryRow(ctx, mvpOrderSelectSQL(`
		FROM orders ord
		INNER JOIN offers o ON o.id = ord.offer_id
		WHERE ord.id = $1
		FOR UPDATE OF ord
	`), orderID)

	order, err := scanMVPOrder(row)
	if err != nil {
		return MVPOrder{}, mapStoreErr(err)
	}
	return order, nil
}

func loadPendingManualPaymentClaim(ctx context.Context, tx pgx.Tx, orderID int64) (*ManualPaymentClaim, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			order_id,
			payer_username,
			claimed_amount,
			submitted_at,
			review_status,
			reviewed_by,
			reviewed_at,
			review_note,
			created_at,
			updated_at
		FROM manual_payment_claims
		WHERE order_id = $1
			AND review_status = 'pending_review'
		ORDER BY submitted_at DESC, id DESC
		LIMIT 1
	`, orderID)

	claim, err := scanManualPaymentClaim(row)
	if err != nil {
		if isNotFoundErr(err) || errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &claim, nil
}

func loadManualPaymentClaimForUpdate(ctx context.Context, tx pgx.Tx, claimID int64) (ManualPaymentClaim, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			order_id,
			payer_username,
			claimed_amount,
			submitted_at,
			review_status,
			reviewed_by,
			reviewed_at,
			review_note,
			created_at,
			updated_at
		FROM manual_payment_claims
		WHERE id = $1
		FOR UPDATE
	`, claimID)

	claim, err := scanManualPaymentClaim(row)
	if err != nil {
		return ManualPaymentClaim{}, mapStoreErr(err)
	}
	return claim, nil
}

func assignAvailablePredefinedCode(ctx context.Context, tx pgx.Tx, offerID int64) (PredefinedCode, error) {
	var codeID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM predefined_codes
		WHERE offer_id = $1
			AND status = 'available'
			AND (expiry_at IS NULL OR expiry_at > NOW())
		ORDER BY expiry_at ASC NULLS LAST, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, offerID).Scan(&codeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PredefinedCode{}, ErrOfferSoldOut
		}
		return PredefinedCode{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE predefined_codes
		SET
			status = 'assigned',
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			offer_id,
			merchant_partner_id,
			code_encrypted,
			code_masked_display,
			status,
			expiry_at,
			issued_batch_reference,
			created_at,
			updated_at
	`, codeID)

	code, err := scanPredefinedCode(row)
	if err != nil {
		return PredefinedCode{}, mapStoreErr(err)
	}
	return code, nil
}

func markManualClaimApprovedWithCode(ctx context.Context, tx pgx.Tx, claimID, orderID, codeID int64, params ReviewManualPaymentClaimParams) (ManualPaymentClaim, MVPOrder, error) {
	claim, err := updateManualClaimReview(ctx, tx, claimID, manualClaimStatusVerified, params)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}

	row := tx.QueryRow(ctx, mvpOrderSelectSQL(`
		FROM (
			UPDATE orders
			SET
				status = 'payment_verified',
				predefined_code_id = $2,
				verified_at = NOW(),
				failure_reason = '',
				updated_at = NOW()
			WHERE id = $1
			RETURNING *
		) ord
		INNER JOIN offers o ON o.id = ord.offer_id
	`), orderID, codeID)
	order, err := scanMVPOrder(row)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, mapStoreErr(err)
	}

	return claim, order, nil
}

func markManualClaimApprovedWithoutCode(ctx context.Context, tx pgx.Tx, claimID, orderID int64, params ReviewManualPaymentClaimParams) (ManualPaymentClaim, MVPOrder, error) {
	claim, err := updateManualClaimReview(ctx, tx, claimID, manualClaimStatusVerified, params)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, err
	}

	row := tx.QueryRow(ctx, mvpOrderSelectSQL(`
		FROM (
			UPDATE orders
			SET
				status = 'support_required',
				verified_at = NOW(),
				failure_reason = 'no_available_predefined_code',
				updated_at = NOW()
			WHERE id = $1
			RETURNING *
		) ord
		INNER JOIN offers o ON o.id = ord.offer_id
	`), orderID)
	order, err := scanMVPOrder(row)
	if err != nil {
		return ManualPaymentClaim{}, MVPOrder{}, mapStoreErr(err)
	}

	return claim, order, nil
}

func updateManualClaimReview(ctx context.Context, tx pgx.Tx, claimID int64, status string, params ReviewManualPaymentClaimParams) (ManualPaymentClaim, error) {
	row := tx.QueryRow(ctx, `
		UPDATE manual_payment_claims
		SET
			review_status = $2,
			reviewed_by = $3,
			reviewed_at = NOW(),
			review_note = $4,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			order_id,
			payer_username,
			claimed_amount,
			submitted_at,
			review_status,
			reviewed_by,
			reviewed_at,
			review_note,
			created_at,
			updated_at
	`, claimID, status, strings.TrimSpace(params.ReviewedBy), params.ReviewNote)

	claim, err := scanManualPaymentClaim(row)
	if err != nil {
		return ManualPaymentClaim{}, mapStoreErr(err)
	}
	return claim, nil
}

func loadPredefinedCodeMaybe(ctx context.Context, tx pgx.Tx, codeID *int64) (*PredefinedCode, error) {
	if codeID == nil {
		return nil, nil
	}

	row := tx.QueryRow(ctx, `
		SELECT
			id,
			offer_id,
			merchant_partner_id,
			code_encrypted,
			code_masked_display,
			status,
			expiry_at,
			issued_batch_reference,
			created_at,
			updated_at
		FROM predefined_codes
		WHERE id = $1
	`, *codeID)

	code, err := scanPredefinedCode(row)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return &code, nil
}

func createNoCodeSupportCase(ctx context.Context, tx pgx.Tx, order MVPOrder, claim ManualPaymentClaim) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO support_cases (
			user_id,
			order_id,
			payment_claim_id,
			case_type,
			status,
			priority,
			summary
		) VALUES ($1, $2, $3, 'delivery_issue', 'open', 'high', $4)
	`, order.UserID, order.ID, claim.ID, "Payment approved but no predefined code was available.")
	return err
}

func offerSelectSQL(suffix string) string {
	return `
		SELECT
			o.id,
			o.merchant_partner_id,
			o.merchant_name,
			o.title,
			o.description,
			o.price_amount,
			o.currency_code,
			o.payment_link,
			o.merchant_disclosure_text,
			o.redemption_terms,
			o.support_contact,
			o.status,
			o.published_at,
			COALESCE(inventory.available_code_count, 0) AS available_code_count,
			inventory.next_code_expiry_at,
			o.created_at,
			o.updated_at
		FROM offers o
		LEFT JOIN (
			SELECT
				offer_id,
				COUNT(*) FILTER (
					WHERE status = 'available'
						AND (expiry_at IS NULL OR expiry_at > NOW())
				) AS available_code_count,
				MIN(expiry_at) FILTER (
					WHERE status = 'available'
						AND expiry_at > NOW()
				) AS next_code_expiry_at
			FROM predefined_codes
			GROUP BY offer_id
		) inventory ON inventory.offer_id = o.id
	` + suffix
}

func mvpOrderSelectSQL(suffix string) string {
	return `
		SELECT
			ord.id,
			ord.user_id,
			ord.offer_id,
			ord.predefined_code_id,
			ord.order_number,
			ord.status,
			ord.currency_code,
			ord.sale_price_amount,
			ord.paybox_payment_link,
			ord.placed_at,
			ord.verified_at,
			ord.delivered_at,
			ord.failure_reason,
			ord.created_at,
			ord.updated_at,
			o.title,
			o.merchant_name,
			o.redemption_terms,
			o.support_contact,
			(
				SELECT COUNT(*)
				FROM predefined_codes pc
				WHERE pc.offer_id = ord.offer_id
					AND pc.status = 'available'
					AND (pc.expiry_at IS NULL OR pc.expiry_at > NOW())
			) AS available_code_count
	` + suffix
}

func manualPaymentClaimSummarySQL(suffix string) string {
	return `
		SELECT
			mpc.id,
			mpc.order_id,
			mpc.payer_username,
			mpc.claimed_amount,
			mpc.submitted_at,
			mpc.review_status,
			mpc.reviewed_by,
			mpc.reviewed_at,
			mpc.review_note,
			mpc.created_at,
			mpc.updated_at,
			o.order_number,
			o.user_id,
			offers.title,
			offers.merchant_name,
			COALESCE(u.display_name, '') AS buyer_display,
			u.telegram_user_id
		FROM manual_payment_claims mpc
		INNER JOIN orders o ON o.id = mpc.order_id
		INNER JOIN offers ON offers.id = o.offer_id
		INNER JOIN users u ON u.id = o.user_id
	` + suffix
}

func scanMerchantPartner(row interface {
	Scan(dest ...any) error
}) (MerchantPartner, error) {
	var partner MerchantPartner
	err := row.Scan(
		&partner.ID,
		&partner.BusinessName,
		&partner.ContactReference,
		&partner.Status,
		&partner.ApprovalNotes,
		&partner.MerchantDisclosureText,
		&partner.SupportContact,
		&partner.DefaultPaymentLink,
		&partner.CreatedAt,
		&partner.UpdatedAt,
	)
	return partner, err
}

func scanOffer(row interface {
	Scan(dest ...any) error
}) (Offer, error) {
	var offer Offer
	err := row.Scan(
		&offer.ID,
		&offer.MerchantPartnerID,
		&offer.MerchantName,
		&offer.Title,
		&offer.Description,
		&offer.PriceAmount,
		&offer.CurrencyCode,
		&offer.PaymentLink,
		&offer.MerchantDisclosureText,
		&offer.RedemptionTerms,
		&offer.SupportContact,
		&offer.Status,
		&offer.PublishedAt,
		&offer.AvailableCodeCount,
		&offer.NextCodeExpiryAt,
		&offer.CreatedAt,
		&offer.UpdatedAt,
	)
	return offer, err
}

func scanPredefinedCode(row interface {
	Scan(dest ...any) error
}) (PredefinedCode, error) {
	var code PredefinedCode
	err := row.Scan(
		&code.ID,
		&code.OfferID,
		&code.MerchantPartnerID,
		&code.CodeEncrypted,
		&code.CodeMaskedDisplay,
		&code.Status,
		&code.ExpiryAt,
		&code.IssuedBatchReference,
		&code.CreatedAt,
		&code.UpdatedAt,
	)
	return code, err
}

func scanMVPOrder(row interface {
	Scan(dest ...any) error
}) (MVPOrder, error) {
	var order MVPOrder
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.OfferID,
		&order.PredefinedCodeID,
		&order.OrderNumber,
		&order.Status,
		&order.CurrencyCode,
		&order.PriceAmount,
		&order.PayBoxPaymentLink,
		&order.PlacedAt,
		&order.VerifiedAt,
		&order.DeliveredAt,
		&order.FailureReason,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.OfferTitle,
		&order.MerchantName,
		&order.RedemptionTerms,
		&order.SupportContact,
		&order.AvailableCodeCount,
	)
	return order, err
}

func scanManualPaymentClaim(row interface {
	Scan(dest ...any) error
}) (ManualPaymentClaim, error) {
	var claim ManualPaymentClaim
	err := row.Scan(
		&claim.ID,
		&claim.OrderID,
		&claim.PayerUsername,
		&claim.ClaimedAmount,
		&claim.SubmittedAt,
		&claim.ReviewStatus,
		&claim.ReviewedBy,
		&claim.ReviewedAt,
		&claim.ReviewNote,
		&claim.CreatedAt,
		&claim.UpdatedAt,
	)
	return claim, err
}

func scanManualPaymentClaimSummary(row interface {
	Scan(dest ...any) error
}) (ManualPaymentClaim, error) {
	var claim ManualPaymentClaim
	err := row.Scan(
		&claim.ID,
		&claim.OrderID,
		&claim.PayerUsername,
		&claim.ClaimedAmount,
		&claim.SubmittedAt,
		&claim.ReviewStatus,
		&claim.ReviewedBy,
		&claim.ReviewedAt,
		&claim.ReviewNote,
		&claim.CreatedAt,
		&claim.UpdatedAt,
		&claim.OrderNumber,
		&claim.BuyerUserID,
		&claim.OfferTitle,
		&claim.MerchantName,
		&claim.BuyerDisplay,
		&claim.BuyerTelegramID,
	)
	return claim, err
}
