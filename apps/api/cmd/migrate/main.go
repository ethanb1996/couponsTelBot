package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command := "status"
	if len(os.Args) > 1 {
		command = strings.ToLower(strings.TrimSpace(os.Args[1]))
	}

	if command != "status" && command != "up" && command != "down" {
		return fmt.Errorf("unsupported command %q (expected status, up, or down)", command)
	}

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

	migrationsDir, err := dbmigrate.FindMigrationsDir(workingDir)
	if err != nil {
		return fmt.Errorf("locate migrations directory: %w", err)
	}

	migrations, err := dbmigrate.LoadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := store.Open(ctx, store.Options{
		DatabaseURL:          databaseURL,
		ApplicationName:      "coupons-migrate",
		ConnectTimeout:       10 * time.Second,
		MaxConns:             1,
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

	switch command {
	case "status":
		rows, err := dbmigrate.Status(ctx, db.Pool, migrations)
		if err != nil {
			return err
		}
		printStatus(rows)
	case "up":
		applied, err := dbmigrate.Up(ctx, db.Pool, migrations)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s)\n", applied)
	case "down":
		migration, err := dbmigrate.Down(ctx, db.Pool, migrations)
		if err != nil {
			return err
		}
		if migration == nil {
			fmt.Println("no applied migrations to revert")
			return nil
		}
		fmt.Printf("reverted migration %s_%s\n", migration.Version, migration.Name)
	}

	return nil
}

func printStatus(rows []dbmigrate.StatusRow) {
	if len(rows) == 0 {
		fmt.Println("no migrations found")
		return
	}

	for _, row := range rows {
		appliedAt := "-"
		if row.AppliedAt != nil {
			appliedAt = row.AppliedAt.UTC().Format(time.RFC3339)
		}

		fmt.Printf("%s\t%s\t%s\t%s\n", row.Version, row.State, appliedAt, row.Name)
	}
}
