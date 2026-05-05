package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "inspect listing error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	listingID := flag.Int64("listing-id", 0, "listing id to inspect")
	flag.Parse()
	if *listingID == 0 {
		return fmt.Errorf("listing-id is required")
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

	execMode := strings.TrimSpace(os.Getenv("DATABASE_QUERY_EXEC_MODE"))
	if execMode == "" {
		execMode = "exec"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          dbURL,
		ApplicationName:      "inspect-listing",
		ConnectTimeout:       10 * time.Second,
		MaxConns:             1,
		DefaultQueryExecMode: execMode,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	var (
		id               int64
		status           string
		title            string
		merchantName     string
		totalCoupons     int64
		availableCoupons int64
		assignedCoupons  int64
		deliveredCoupons int64
		voidedCoupons    int64
		disputedCoupons  int64
		nextCouponExpiry *time.Time
	)

	err = db.Pool.QueryRow(ctx, `
		SELECT
			l.id,
			l.status,
			l.title,
			l.merchant_name,
			COUNT(c.id) AS total_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'available' AND c.expiry_at > NOW()) AS available_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'assigned') AS assigned_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'delivered') AS delivered_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'voided') AS voided_coupons,
			COUNT(c.id) FILTER (WHERE c.inventory_status = 'disputed') AS disputed_coupons,
			MIN(c.expiry_at) FILTER (WHERE c.inventory_status = 'available' AND c.expiry_at > NOW()) AS next_coupon_expiry_at
		FROM listings l
		LEFT JOIN coupons c ON c.listing_id = l.id
		WHERE l.id = $1
		GROUP BY l.id
	`, *listingID).Scan(
		&id,
		&status,
		&title,
		&merchantName,
		&totalCoupons,
		&availableCoupons,
		&assignedCoupons,
		&deliveredCoupons,
		&voidedCoupons,
		&disputedCoupons,
		&nextCouponExpiry,
	)
	if err != nil {
		return err
	}

	fmt.Printf("listing id=%d merchant=%q title=%q status=%s total=%d available=%d assigned=%d delivered=%d voided=%d disputed=%d",
		id, merchantName, title, status, totalCoupons, availableCoupons, assignedCoupons, deliveredCoupons, voidedCoupons, disputedCoupons,
	)
	if nextCouponExpiry != nil {
		fmt.Printf(" next_available_expiry=%s", nextCouponExpiry.Format(time.RFC3339))
	}
	fmt.Println()

	rows, err := db.Pool.Query(ctx, `
		SELECT
			id,
			source_id,
			coupon_masked_display,
			inventory_status,
			expiry_at,
			created_at,
			updated_at
		FROM coupons
		WHERE listing_id = $1
		ORDER BY id ASC
	`, *listingID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var couponID int64
		var sourceID int64
		var maskedDisplay string
		var inventoryStatus string
		var expiryAt time.Time
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(&couponID, &sourceID, &maskedDisplay, &inventoryStatus, &expiryAt, &createdAt, &updatedAt); err != nil {
			return err
		}

		fmt.Printf("coupon id=%d source_id=%d masked=%s status=%s expiry=%s created=%s updated=%s\n",
			couponID,
			sourceID,
			maskedDisplay,
			inventoryStatus,
			expiryAt.Format(time.RFC3339),
			createdAt.Format(time.RFC3339),
			updatedAt.Format(time.RFC3339),
		)
	}

	return rows.Err()
}
