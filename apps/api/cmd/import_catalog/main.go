package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/catalogimport"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "catalog import error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	defaultPaymentFeeRate, err := floatEnvWithDefault("PAYMENT_FEE_PERCENT_RATE", 0)
	if err != nil {
		return err
	}
	defaultPaymentFixedFee, err := floatEnvWithDefault("PAYMENT_FIXED_FEE_AMOUNT", 0)
	if err != nil {
		return err
	}

	inputPath := flag.String("input", filepath.Clean("data/GetCategoryById_6982.txt"), "path to HAR export file")
	photosDir := flag.String("photos-dir", filepath.Clean("data/photos"), "directory for downloaded product photos")
	dryRun := flag.Bool("dry-run", false, "parse the HAR and report counts without downloading or writing to the database")
	paymentFeeRate := flag.Float64("payment-fee-rate", defaultPaymentFeeRate, "payment percentage fee as a decimal rate, for example 0.0349 for 3.49%")
	paymentFixedFee := flag.Float64("payment-fixed-fee", defaultPaymentFixedFee, "payment fixed fee in ILS major units, for example 0.49")
	flag.Parse()

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := dbmigrate.LoadDotEnvUpward(workingDir); err != nil {
		return err
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	queryExecMode := strings.TrimSpace(os.Getenv("DATABASE_QUERY_EXEC_MODE"))
	if queryExecMode == "" {
		queryExecMode = "exec"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          databaseURL,
		ApplicationName:      "coupons-import-catalog",
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

	summary, err := catalogimport.Run(ctx, db, catalogimport.Options{
		InputPath:             *inputPath,
		PhotosDir:             *photosDir,
		DryRun:                *dryRun,
		PaymentPercentFeeRate: *paymentFeeRate,
		PaymentFixedFeeAmount: majorUnitsToMinor(*paymentFixedFee),
	})
	if err != nil {
		return err
	}

	if *dryRun {
		fmt.Printf(
			"dry-run ok: %d sources, %d listings, %d preview, %d skipped\n",
			summary.SourceCount,
			summary.ListingCount,
			summary.PreviewListingCount,
			summary.SkippedListingCount,
		)
		return nil
	}

	fmt.Printf(
		"imported %d listings across %d sources (%d created, %d updated, %d preview, %d skipped, %d removed, %d photos downloaded)\n",
		summary.ListingCount,
		summary.SourceCount,
		summary.CreatedListingCount,
		summary.UpdatedListingCount,
		summary.PreviewListingCount,
		summary.SkippedListingCount,
		summary.RemovedListingCount,
		summary.DownloadedPhotoCount,
	)
	return nil
}

func majorUnitsToMinor(value float64) int64 {
	return int64(math.Round(value * 100))
}

func floatEnvWithDefault(key string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid number: %w", key, err)
	}

	return parsed, nil
}
