package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

const sandboxSourceName = "Sandbox Test Source"

type candidateListing struct {
	ID                int64
	Status            string
	MerchantName      string
	Title             string
	CouponValueAmount int64
	SalePriceAmount   int64
	ResellPriceAmount int64
	AvailableCoupons  int64
	OrderCount        int64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "prepare sandbox checkout error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	listingIDFlag := flag.Int64("listing-id", 0, "specific listing id to prepare; defaults to the best discounted draft/preview candidate with no orders")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := dbmigrate.LoadDotEnvUpward(wd); err != nil {
		return err
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	queryExecMode := strings.TrimSpace(os.Getenv("DATABASE_QUERY_EXEC_MODE"))
	if queryExecMode == "" {
		queryExecMode = "exec"
	}
	couponKey := strings.TrimSpace(os.Getenv("COUPON_ENCRYPTION_KEY"))
	if couponKey == "" {
		return fmt.Errorf("COUPON_ENCRYPTION_KEY is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          dbURL,
		ApplicationName:      "prepare-sandbox-checkout",
		ConnectTimeout:       10 * time.Second,
		MaxConns:             2,
		MinConns:             0,
		MaxConnLifetime:      30 * time.Minute,
		MaxConnIdleTime:      5 * time.Minute,
		HealthCheckPeriod:    time.Minute,
		DefaultQueryExecMode: queryExecMode,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	candidates, err := listCandidates(ctx, db)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no discounted listings found to prepare")
	}

	fmt.Println("Candidate listings:")
	for _, candidate := range candidates {
		fmt.Printf(
			"- id=%d status=%s available=%d orders=%d value=%s source=%s resell=%s merchant=%s title=%s\n",
			candidate.ID,
			candidate.Status,
			candidate.AvailableCoupons,
			candidate.OrderCount,
			formatMinorUnits(candidate.CouponValueAmount),
			formatMinorUnits(candidate.SalePriceAmount),
			formatMinorUnits(store.EffectiveListingPriceAmount(store.Listing{
				SalePriceAmount:   candidate.SalePriceAmount,
				ResellPriceAmount: candidate.ResellPriceAmount,
			})),
			candidate.MerchantName,
			candidate.Title,
		)
	}

	selectedID := *listingIDFlag
	if selectedID == 0 {
		for _, candidate := range candidates {
			if candidate.OrderCount == 0 {
				selectedID = candidate.ID
				break
			}
		}
	}
	if selectedID == 0 {
		return fmt.Errorf("could not choose a listing automatically; rerun with -listing-id")
	}

	listing, err := db.GetListing(ctx, selectedID)
	if err != nil {
		return fmt.Errorf("load selected listing %d: %w", selectedID, err)
	}

	sourceID, err := ensureSandboxSource(ctx, db)
	if err != nil {
		return err
	}

	if listing.Status != "active" {
		listing, err = db.UpdateListingStatus(ctx, listing.ID, "active")
		if err != nil {
			return fmt.Errorf("publish listing %d: %w", listing.ID, err)
		}
	}

	now := time.Now().UTC()
	plainCode := fmt.Sprintf("SANDBOX-%d", now.Unix())
	maskedDisplay := "SANDBOX-" + plainCode[len(plainCode)-4:]
	ciphertext, nonce, err := encryptCouponCode(couponKey, plainCode)
	if err != nil {
		return fmt.Errorf("encrypt sandbox coupon code: %w", err)
	}

	expiryAt := now.Add(45 * 24 * time.Hour)
	inserted, err := db.IngestCoupons(ctx, store.IngestCouponsParams{
		ListingID: listing.ID,
		Coupons: []store.CouponInventoryInput{
			{
				SourceID:               sourceID,
				MerchantName:           listing.MerchantName,
				CouponTitle:            listing.Title,
				CouponValueAmount:      listing.CouponValueAmount,
				SalePriceAmount:        store.EffectiveListingPriceAmount(listing),
				CurrencyCode:           listing.CurrencyCode,
				CouponCodeCiphertext:   ciphertext,
				CouponCodeNonce:        nonce,
				CouponMaskedDisplay:    maskedDisplay,
				ExpiryAt:               expiryAt,
				TransferabilityStatus:  "transferable",
				RightsVerificationNote: "Sandbox checkout test coupon prepared automatically for development verification.",
				AcquiredCostAmount:     listing.SalePriceAmount,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("ingest sandbox coupon for listing %d: %w", listing.ID, err)
	}
	if len(inserted) != 1 {
		return fmt.Errorf("expected exactly one inserted sandbox coupon, got %d", len(inserted))
	}

	verifiedListing, err := db.GetListing(ctx, listing.ID)
	if err != nil {
		return fmt.Errorf("reload listing %d after setup: %w", listing.ID, err)
	}

	activeListings, err := db.ListActiveListings(ctx)
	if err != nil {
		return fmt.Errorf("list active listings after setup: %w", err)
	}
	isVisible := false
	for _, activeListing := range activeListings {
		if activeListing.ID == listing.ID {
			isVisible = true
			break
		}
	}

	fmt.Println()
	fmt.Println("Prepared sandbox checkout listing:")
	fmt.Printf("- listing_id=%d\n", verifiedListing.ID)
	fmt.Printf("- status=%s\n", verifiedListing.Status)
	fmt.Printf("- merchant=%s\n", verifiedListing.MerchantName)
	fmt.Printf("- title=%s\n", verifiedListing.Title)
	fmt.Printf("- coupon_value=%s\n", formatMinorUnits(verifiedListing.CouponValueAmount))
	fmt.Printf("- source_sale_price=%s\n", formatMinorUnits(verifiedListing.SalePriceAmount))
	fmt.Printf("- resell_price=%s\n", formatMinorUnits(store.EffectiveListingPriceAmount(verifiedListing)))
	fmt.Printf("- available_inventory=%d\n", verifiedListing.AvailableInventoryCount)
	fmt.Printf("- source_id=%d\n", sourceID)
	fmt.Printf("- coupon_id=%d\n", inserted[0].ID)
	fmt.Printf("- coupon_masked_display=%s\n", inserted[0].CouponMaskedDisplay)
	fmt.Printf("- coupon_expiry=%s\n", inserted[0].ExpiryAt.Format(time.RFC3339))
	fmt.Printf("- appears_in_active_listings=%t\n", isVisible)

	fmt.Println()
	fmt.Println("Sandbox checkout test path:")
	fmt.Println("1. In Telegram, send /start to the bot.")
	fmt.Printf("2. Open listing %d (%s).\n", verifiedListing.ID, verifiedListing.Title)
	fmt.Println("3. Tap the payment button and complete the PayPal sandbox flow.")
	fmt.Println("4. Return to Telegram and confirm the coupon delivery message appears.")
	fmt.Println("5. In admin, verify the order moved through pending_payment to delivered.")

	return nil
}

func listCandidates(ctx context.Context, db *store.Postgres) ([]candidateListing, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT
			l.id,
			l.status,
			l.merchant_name,
			l.title,
			l.coupon_value_amount,
			l.sale_price_amount,
			l.resell_price_amount,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'available' AND c.expiry_at > NOW()) AS available_coupons,
			COUNT(DISTINCT o.id) AS order_count
		FROM listings l
		LEFT JOIN coupons c ON c.listing_id = l.id
		LEFT JOIN orders o ON o.listing_id = l.id
		WHERE l.coupon_value_amount > COALESCE(NULLIF(l.resell_price_amount, 0), l.sale_price_amount)
			AND l.status IN ('draft', 'preview', 'active')
		GROUP BY l.id
		ORDER BY
			CASE l.status
				WHEN 'draft' THEN 0
				WHEN 'preview' THEN 1
				ELSE 2
			END,
			(l.coupon_value_amount - COALESCE(NULLIF(l.resell_price_amount, 0), l.sale_price_amount)) DESC,
			l.id ASC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]candidateListing, 0)
	for rows.Next() {
		var candidate candidateListing
		if err := rows.Scan(
			&candidate.ID,
			&candidate.Status,
			&candidate.MerchantName,
			&candidate.Title,
			&candidate.CouponValueAmount,
			&candidate.SalePriceAmount,
			&candidate.ResellPriceAmount,
			&candidate.AvailableCoupons,
			&candidate.OrderCount,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func ensureSandboxSource(ctx context.Context, db *store.Postgres) (int64, error) {
	sources, err := db.ListCouponSources(ctx)
	if err != nil {
		return 0, fmt.Errorf("list coupon sources: %w", err)
	}
	for _, source := range sources {
		if strings.EqualFold(strings.TrimSpace(source.SourceName), sandboxSourceName) {
			return source.ID, nil
		}
	}

	source, err := db.CreateCouponSource(ctx, store.CreateCouponSourceParams{
		SourceName:        sandboxSourceName,
		SourceType:        "manual_source",
		ContactReference:  "local sandbox",
		RightsStatus:      "approved",
		VerificationNotes: "Created automatically for development sandbox checkout testing.",
		RiskRating:        "low",
	})
	if err != nil {
		return 0, fmt.Errorf("create sandbox coupon source: %w", err)
	}
	return source.ID, nil
}

func encryptCouponCode(key string, plainCode string) ([]byte, []byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plainCode), nil)
	return ciphertext, nonce, nil
}

func formatMinorUnits(amount int64) string {
	return fmt.Sprintf("%.2f ILS", float64(amount)/100)
}
