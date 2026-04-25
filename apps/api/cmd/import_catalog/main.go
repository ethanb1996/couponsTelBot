package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	inputPath := flag.String("input", filepath.Clean("data/GetCategoryById_6982.txt"), "path to HAR export file")
	photosDir := flag.String("photos-dir", filepath.Clean("data/photos"), "directory for downloaded product photos")
	dryRun := flag.Bool("dry-run", false, "parse the HAR and report counts without downloading or writing to the database")
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
		InputPath: *inputPath,
		PhotosDir: *photosDir,
		DryRun:    *dryRun,
	})
	if err != nil {
		return err
	}

	if *dryRun {
		fmt.Printf("dry-run ok: %d sources, %d listings\n", summary.SourceCount, summary.ListingCount)
		return nil
	}

	fmt.Printf(
		"imported %d listings across %d sources (%d created, %d updated, %d photos downloaded)\n",
		summary.ListingCount,
		summary.SourceCount,
		summary.CreatedListingCount,
		summary.UpdatedListingCount,
		summary.DownloadedPhotoCount,
	)
	return nil
}
