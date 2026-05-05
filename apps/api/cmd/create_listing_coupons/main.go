package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/jackc/pgx/v5"
)

const (
	defaultSourceName = "Generated Listing Coupon Seed"
	defaultAdminActor = "create_listing_coupons"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "create listing coupons error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dryRun     bool
		expiryDays int
		sourceName string
		adminActor string
	)

	flag.BoolVar(&dryRun, "dry-run", false, "show what would change without writing to the database")
	flag.IntVar(&expiryDays, "expiry-days", 180, "expiry offset in days for generated coupons")
	flag.StringVar(&sourceName, "source-name", defaultSourceName, "coupon source name to use or create")
	flag.StringVar(&adminActor, "admin-actor", defaultAdminActor, "admin actor value for audit entries")
	flag.Parse()

	if expiryDays <= 0 {
		return fmt.Errorf("expiry-days must be greater than zero")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          dbURL,
		ApplicationName:      "create-listing-coupons",
		ConnectTimeout:       10 * time.Second,
		MaxConns:             4,
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

	listings, err := db.ListListingsForAdmin(ctx)
	if err != nil {
		return fmt.Errorf("list listings: %w", err)
	}
	if len(listings) == 0 {
		fmt.Println("No listings found.")
		return nil
	}

	sourceID, sourceCreated, err := ensureGeneratedSource(ctx, db, sourceName)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	expiryAt := now.AddDate(0, 0, expiryDays)

	type listingResult struct {
		listingID       int64
		title           string
		beforeStatus    string
		afterStatus     string
		couponID        int64
		maskedDisplay   string
		createdCoupon   bool
		updatedStatus   bool
		skippedMutation bool
		reusedCoupon    bool
	}

	results := make([]listingResult, 0, len(listings))
	var createdCoupons int
	var reusedCoupons int
	var updatedStatuses int

	for _, listing := range listings {
		targetStatus := desiredListingStatus(listing.Status)
		code, err := generateCouponCode(listing.ID)
		if err != nil {
			return fmt.Errorf("generate coupon code for listing %d: %w", listing.ID, err)
		}

		maskedDisplay := maskCouponCode(code)
		if dryRun {
			existingCouponID, existingMaskedDisplay, alreadySeeded, err := lookupSeededCoupon(ctx, db, listing.ID, sourceID)
			if err != nil {
				return fmt.Errorf("check seeded coupon for listing %d: %w", listing.ID, err)
			}
			results = append(results, listingResult{
				listingID:       listing.ID,
				title:           listing.Title,
				beforeStatus:    listing.Status,
				afterStatus:     targetStatus,
				couponID:        existingCouponID,
				maskedDisplay:   ternary(alreadySeeded, existingMaskedDisplay, maskedDisplay),
				createdCoupon:   !alreadySeeded,
				updatedStatus:   listing.Status != targetStatus,
				skippedMutation: true,
				reusedCoupon:    alreadySeeded,
			})
			if alreadySeeded {
				reusedCoupons++
			} else {
				createdCoupons++
			}
			if listing.Status != targetStatus {
				updatedStatuses++
			}
			continue
		}

		existingCouponID, existingMaskedDisplay, alreadySeeded, err := lookupSeededCoupon(ctx, db, listing.ID, sourceID)
		if err != nil {
			return fmt.Errorf("check seeded coupon for listing %d: %w", listing.ID, err)
		}

		insertedCoupon := store.Coupon{}
		if alreadySeeded {
			reusedCoupons++
			insertedCoupon.ID = existingCouponID
			insertedCoupon.ListingID = listing.ID
			insertedCoupon.CouponMaskedDisplay = existingMaskedDisplay
		} else {
			ciphertext, nonce, err := encryptCouponCode(couponKey, code)
			if err != nil {
				return fmt.Errorf("encrypt coupon code for listing %d: %w", listing.ID, err)
			}

			inserted, err := db.IngestCoupons(ctx, store.IngestCouponsParams{
				ListingID: listing.ID,
				Coupons: []store.CouponInventoryInput{
					{
						SourceID:               sourceID,
						MerchantName:           listing.MerchantName,
						CouponTitle:            listing.Title,
						CouponValueAmount:      listing.CouponValueAmount,
						SalePriceAmount:        store.EffectiveListingPriceAmount(listing.Listing),
						CurrencyCode:           listing.CurrencyCode,
						CouponCodeCiphertext:   ciphertext,
						CouponCodeNonce:        nonce,
						CouponMaskedDisplay:    maskedDisplay,
						ExpiryAt:               expiryAt,
						TransferabilityStatus:  "transferable",
						RightsVerifiedAt:       &now,
						RightsVerificationNote: "Generated automatically in bulk so every listing has seeded inventory.",
						AcquiredCostAmount:     listing.SalePriceAmount,
						AcquiredAt:             &now,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("ingest coupon for listing %d: %w", listing.ID, err)
			}
			if len(inserted) != 1 {
				return fmt.Errorf("expected one inserted coupon for listing %d, got %d", listing.ID, len(inserted))
			}

			createdCoupons++
			insertedCoupon = inserted[0]
			_, _ = db.RecordAdminAction(ctx, store.RecordAdminActionParams{
				AdminActor:      adminActor,
				EntityType:      "coupon",
				EntityID:        insertedCoupon.ID,
				ActionType:      "bulk_create_coupon",
				AfterStateJSON:  fmt.Sprintf(`{"listing_id":%d,"inventory_status":"%s","source_id":%d}`, insertedCoupon.ListingID, insertedCoupon.InventoryStatus, insertedCoupon.SourceID),
				ReasonText:      "generated one seeded coupon for listing inventory",
				BeforeStateJSON: "{}",
			})
		}

		statusUpdated := false
		if listing.Status != targetStatus {
			updatedListing, err := db.UpdateListingStatus(ctx, listing.ID, targetStatus)
			if err != nil {
				return fmt.Errorf("update listing %d status to %s: %w", listing.ID, targetStatus, err)
			}
			updatedStatuses++
			statusUpdated = true
			_, _ = db.RecordAdminAction(ctx, store.RecordAdminActionParams{
				AdminActor:      adminActor,
				EntityType:      "listing",
				EntityID:        updatedListing.ID,
				ActionType:      "bulk_update_listing_status",
				BeforeStateJSON: fmt.Sprintf(`{"status":%q}`, listing.Status),
				AfterStateJSON:  fmt.Sprintf(`{"status":%q}`, updatedListing.Status),
				ReasonText:      "listing activated after generated inventory seed",
			})
		}

		results = append(results, listingResult{
			listingID:     listing.ID,
			title:         listing.Title,
			beforeStatus:  listing.Status,
			afterStatus:   targetStatus,
			couponID:      insertedCoupon.ID,
			maskedDisplay: insertedCoupon.CouponMaskedDisplay,
			createdCoupon: !alreadySeeded,
			updatedStatus: statusUpdated,
			reusedCoupon:  alreadySeeded,
		})
	}

	fmt.Printf("Listings processed: %d\n", len(results))
	fmt.Printf("Source: %s (id=%d, created=%t)\n", sourceName, sourceID, sourceCreated)
	fmt.Printf("Coupons created: %d\n", createdCoupons)
	fmt.Printf("Coupons reused: %d\n", reusedCoupons)
	fmt.Printf("Listing statuses updated: %d\n", updatedStatuses)
	fmt.Printf("Mode: %s\n", ternary(dryRun, "dry-run", "apply"))
	fmt.Printf("Generated expiry: %s\n", expiryAt.Format(time.RFC3339))
	fmt.Println()

	for _, result := range results {
		line := fmt.Sprintf(
			"listing_id=%d status=%s->%s masked=%s title=%q",
			result.listingID,
			result.beforeStatus,
			result.afterStatus,
			result.maskedDisplay,
			result.title,
		)
		if result.couponID != 0 {
			line = fmt.Sprintf("%s coupon_id=%d", line, result.couponID)
		}
		if result.reusedCoupon {
			line = fmt.Sprintf("%s reused=true", line)
		}
		fmt.Println(line)
	}

	return nil
}

func ensureGeneratedSource(ctx context.Context, db *store.Postgres, sourceName string) (int64, bool, error) {
	sources, err := db.ListCouponSources(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("list coupon sources: %w", err)
	}

	for _, source := range sources {
		if strings.EqualFold(strings.TrimSpace(source.SourceName), strings.TrimSpace(sourceName)) {
			return source.ID, false, nil
		}
	}

	source, err := db.CreateCouponSource(ctx, store.CreateCouponSourceParams{
		SourceName:        sourceName,
		SourceType:        "manual_source",
		ContactReference:  "generated locally",
		RightsStatus:      "approved",
		VerificationNotes: "Created automatically for bulk listing coupon generation.",
		RiskRating:        "low",
	})
	if err != nil {
		return 0, false, fmt.Errorf("create coupon source: %w", err)
	}

	return source.ID, true, nil
}

func desiredListingStatus(current string) string {
	if strings.EqualFold(strings.TrimSpace(current), "removed") {
		return "removed"
	}
	return "active"
}

func lookupSeededCoupon(ctx context.Context, db *store.Postgres, listingID int64, sourceID int64) (int64, string, bool, error) {
	var couponID int64
	var maskedDisplay string
	err := db.Pool.QueryRow(ctx, `
		SELECT id, coupon_masked_display
		FROM coupons
		WHERE listing_id = $1
			AND source_id = $2
		ORDER BY id DESC
		LIMIT 1
	`, listingID, sourceID).Scan(&couponID, &maskedDisplay)
	if err == nil {
		return couponID, maskedDisplay, true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, nil
	}
	return 0, "", false, err
}

func generateCouponCode(listingID int64) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const randomLength = 10

	bytes := make([]byte, randomLength)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.Grow(len("AUTO-000000-") + randomLength)
	builder.WriteString("AUTO-")
	builder.WriteString(fmt.Sprintf("%06d-", listingID))
	for _, b := range bytes {
		builder.WriteByte(alphabet[int(b)%len(alphabet)])
	}
	return builder.String(), nil
}

func maskCouponCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) <= 4 {
		return code
	}
	return "AUTO-" + code[len(code)-4:]
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

	ciphertext := gcm.Seal(nil, nonce, []byte(strings.TrimSpace(plainCode)), nil)
	return ciphertext, nonce, nil
}

func ternary[T any](condition bool, whenTrue T, whenFalse T) T {
	if condition {
		return whenTrue
	}
	return whenFalse
}
