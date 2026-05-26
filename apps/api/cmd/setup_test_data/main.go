package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/jackc/pgx/v5"
)

const (
	liveTestPartnerName      = "לקפריזה"
	liveTestOfferMerchant    = "לקפריזה | רובע י\"ב"
	liveTestOfferTitle       = "פיצה משפחתית + תוספת"
	liveTestOfferPriceAnchor = "74₪ → 59₪"
	liveTestOfferPriceAmount = int64(5900)
	liveTestPaymentLink      = "https://paybox.money/p/test-link"
	liveTestSupportContact   = "@admin"
	liveTestTargetCodes      = 8
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to start transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		UPDATE offers
		SET status = 'paused', updated_at = NOW()
		WHERE merchant_name = 'Bazario Test'
			OR title = 'Test Coupon 20% Off'
	`); err != nil {
		log.Fatalf("Failed to pause legacy test offers: %v", err)
	}

	merchantID, err := ensureMerchantPartner(ctx, tx)
	if err != nil {
		log.Fatalf("Failed to ensure merchant partner: %v", err)
	}

	offerID, err := ensureOffer(ctx, tx, merchantID)
	if err != nil {
		log.Fatalf("Failed to ensure offer: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE offers
		SET status = 'paused', updated_at = NOW()
		WHERE merchant_name = $1
			AND title = $2
			AND id <> $3
	`, liveTestOfferMerchant, liveTestOfferTitle, offerID); err != nil {
		log.Fatalf("Failed to pause duplicate live-test offers: %v", err)
	}

	availableCodes, err := normalizeAvailableCodes(ctx, tx, merchantID, offerID)
	if err != nil {
		log.Fatalf("Failed to normalize predefined codes: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Failed to commit setup: %v", err)
	}

	fmt.Println("Live test coupon updated successfully.")
	fmt.Printf("Merchant partner ID: %d\n", merchantID)
	fmt.Printf("Offer ID: %d\n", offerID)
	fmt.Printf("Available predefined codes: %d\n", availableCodes)
	fmt.Printf("Admin Telegram IDs: %v\n", cfg.TelegramAdminUserIDs)
	fmt.Printf("Offer copy: %s / %s / %s\n", liveTestOfferMerchant, liveTestOfferTitle, liveTestOfferPriceAnchor)
	fmt.Printf("Payment link: %s\n", liveTestPaymentLink)
}

func ensureMerchantPartner(ctx context.Context, tx pgx.Tx) (int64, error) {
	var merchantID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM merchant_partners
		WHERE business_name = $1
		ORDER BY id DESC
		LIMIT 1
	`, liveTestPartnerName).Scan(&merchantID)
	if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE merchant_partners
			SET
				status = 'active',
				merchant_disclosure_text = $2,
				support_contact = $3,
				default_payment_link = $4,
				updated_at = NOW()
			WHERE id = $1
		`,
			merchantID,
			"הקופון נמכר דרך הבוט באישור בית העסק.",
			liveTestSupportContact,
			liveTestPaymentLink,
		)
		return merchantID, err
	}
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO merchant_partners (
			business_name,
			contact_reference,
			status,
			merchant_disclosure_text,
			support_contact,
			default_payment_link
		) VALUES ($1, $2, 'active', $3, $4, $5)
		RETURNING id
	`,
		liveTestPartnerName,
		"live-test-lacapriza",
		"הקופון נמכר דרך הבוט באישור בית העסק.",
		liveTestSupportContact,
		liveTestPaymentLink,
	).Scan(&merchantID)
	return merchantID, err
}

func ensureOffer(ctx context.Context, tx pgx.Tx, merchantID int64) (int64, error) {
	var offerID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM offers
		WHERE merchant_name = $1
			AND title = $2
		ORDER BY id DESC
		LIMIT 1
	`, liveTestOfferMerchant, liveTestOfferTitle).Scan(&offerID)
	if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE offers
			SET
				merchant_partner_id = $2,
				merchant_name = $3,
				title = $4,
				description = $5,
				price_amount = $6,
				currency_code = 'ILS',
				payment_link = $7,
				merchant_disclosure_text = $8,
				redemption_terms = '',
				support_contact = $9,
				status = 'active',
				published_at = COALESCE(published_at, NOW()),
				updated_at = NOW()
			WHERE id = $1
		`,
			offerID,
			merchantID,
			liveTestOfferMerchant,
			liveTestOfferTitle,
			liveTestOfferPriceAnchor,
			liveTestOfferPriceAmount,
			liveTestPaymentLink,
			"הטבה לרובע י\"ב באשדוד.",
			liveTestSupportContact,
		)
		return offerID, err
	}
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}

	err = tx.QueryRow(ctx, `
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
		) VALUES ($1, $2, $3, $4, $5, 'ILS', $6, $7, '', $8, 'active', NOW())
		RETURNING id
	`,
		merchantID,
		liveTestOfferMerchant,
		liveTestOfferTitle,
		liveTestOfferPriceAnchor,
		liveTestOfferPriceAmount,
		liveTestPaymentLink,
		"הטבה לרובע י\"ב באשדוד.",
		liveTestSupportContact,
	).Scan(&offerID)
	return offerID, err
}

func normalizeAvailableCodes(ctx context.Context, tx pgx.Tx, merchantID, offerID int64) (int64, error) {
	if _, err := tx.Exec(ctx, `
		WITH ranked AS (
			SELECT id
			FROM predefined_codes
			WHERE offer_id = $1
				AND status = 'available'
				AND (expiry_at IS NULL OR expiry_at > NOW())
			ORDER BY id ASC
			OFFSET $2
		)
		UPDATE predefined_codes
		SET
			status = 'expired',
			updated_at = NOW()
		WHERE id IN (SELECT id FROM ranked)
	`, offerID, liveTestTargetCodes); err != nil {
		return 0, err
	}

	var availableCount int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM predefined_codes
		WHERE offer_id = $1
			AND status = 'available'
			AND (expiry_at IS NULL OR expiry_at > NOW())
	`, offerID).Scan(&availableCount); err != nil {
		return 0, err
	}

	for i := availableCount + 1; i <= liveTestTargetCodes; i++ {
		encryptedHex := fmt.Sprintf("%064x", i)
		if _, err := tx.Exec(ctx, `
			INSERT INTO predefined_codes (
				offer_id,
				merchant_partner_id,
				code_encrypted,
				code_masked_display,
				status,
				expiry_at,
				issued_batch_reference
			) VALUES (
				$1,
				$2,
				decode($3, 'hex'),
				$4,
				'available',
				NOW() + INTERVAL '30 days',
				'live-test-lacapriza'
			)
		`,
			offerID,
			merchantID,
			encryptedHex,
			fmt.Sprintf("LCAPRIZA-2026-%04d", i),
		); err != nil {
			return 0, err
		}
	}

	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM predefined_codes
		WHERE offer_id = $1
			AND status = 'available'
			AND (expiry_at IS NULL OR expiry_at > NOW())
	`, offerID).Scan(&availableCount); err != nil {
		return 0, err
	}

	return availableCount, nil
}
