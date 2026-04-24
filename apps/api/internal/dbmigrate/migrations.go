package dbmigrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationTableName = "schema_migrations"

type Migration struct {
	Version  string
	Name     string
	UpPath   string
	DownPath string
	UpSQL    string
	DownSQL  string
	Checksum string
}

type AppliedMigration struct {
	Version   string
	Name      string
	Checksum  string
	AppliedAt time.Time
}

type StatusRow struct {
	Version   string
	Name      string
	State     string
	AppliedAt *time.Time
	Drift     bool
}

func FindMigrationsDir(startDir string) (string, error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		for _, candidate := range []string{
			filepath.Join(current, "apps", "api", "migrations"),
			filepath.Join(current, "migrations"),
		} {
			info, statErr := os.Stat(candidate)
			if statErr == nil && info.IsDir() {
				return candidate, nil
			}
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return "", statErr
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		current = parent
	}
}

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	migrations := map[string]*Migration{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		switch {
		case strings.HasSuffix(name, ".down.sql"):
			version, migrationName, ok := parseMigrationName(strings.TrimSuffix(name, ".down.sql"))
			if !ok {
				continue
			}
			migration := ensureMigration(migrations, version, migrationName)
			migration.DownPath = filepath.Join(dir, name)
		case strings.HasSuffix(name, ".sql"):
			version, migrationName, ok := parseMigrationName(strings.TrimSuffix(name, ".sql"))
			if !ok {
				continue
			}
			migration := ensureMigration(migrations, version, migrationName)
			migration.UpPath = filepath.Join(dir, name)
		}
	}

	ordered := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if migration.UpPath == "" {
			return nil, fmt.Errorf("missing up migration for version %s", migration.Version)
		}

		upSQL, err := os.ReadFile(migration.UpPath)
		if err != nil {
			return nil, err
		}
		migration.UpSQL = string(upSQL)

		if migration.DownPath != "" {
			downSQL, err := os.ReadFile(migration.DownPath)
			if err != nil {
				return nil, err
			}
			migration.DownSQL = string(downSQL)
		}

		sum := sha256.Sum256(upSQL)
		migration.Checksum = hex.EncodeToString(sum[:])
		ordered = append(ordered, *migration)
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Version < ordered[j].Version
	})

	return ordered, nil
}

func Status(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) ([]StatusRow, error) {
	if err := ensureMigrationTable(ctx, pool); err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(ctx, pool)
	if err != nil {
		return nil, err
	}

	appliedByVersion := make(map[string]AppliedMigration, len(applied))
	for _, item := range applied {
		appliedByVersion[item.Version] = item
	}

	rows := make([]StatusRow, 0, len(migrations)+len(applied))
	knownVersions := make(map[string]struct{}, len(migrations))
	for _, migration := range migrations {
		knownVersions[migration.Version] = struct{}{}

		row := StatusRow{
			Version: migration.Version,
			Name:    migration.Name,
			State:   "pending",
		}

		if appliedMigration, ok := appliedByVersion[migration.Version]; ok {
			row.State = "applied"
			row.AppliedAt = &appliedMigration.AppliedAt
			if appliedMigration.Checksum != "" && appliedMigration.Checksum != migration.Checksum {
				row.State = "drifted"
				row.Drift = true
			}
		}

		rows = append(rows, row)
	}

	for _, appliedMigration := range applied {
		if _, ok := knownVersions[appliedMigration.Version]; ok {
			continue
		}

		appliedAt := appliedMigration.AppliedAt
		rows = append(rows, StatusRow{
			Version:   appliedMigration.Version,
			Name:      appliedMigration.Name,
			State:     "missing_local_file",
			AppliedAt: &appliedAt,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Version < rows[j].Version
	})

	return rows, nil
}

func Up(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (int, error) {
	if err := ensureMigrationTable(ctx, pool); err != nil {
		return 0, err
	}

	applied, err := appliedMigrations(ctx, pool)
	if err != nil {
		return 0, err
	}

	appliedByVersion := make(map[string]AppliedMigration, len(applied))
	for _, item := range applied {
		appliedByVersion[item.Version] = item
	}

	appliedCount := 0
	for _, migration := range migrations {
		if existing, ok := appliedByVersion[migration.Version]; ok {
			if existing.Checksum != "" && existing.Checksum != migration.Checksum {
				return appliedCount, fmt.Errorf("migration %s checksum mismatch between database and local file", migration.Version)
			}
			continue
		}

		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return appliedCount, err
		}

		if err := applyUpMigration(ctx, tx, migration); err != nil {
			_ = tx.Rollback(ctx)
			return appliedCount, err
		}

		if err := tx.Commit(ctx); err != nil {
			return appliedCount, err
		}

		appliedCount++
	}

	return appliedCount, nil
}

func Down(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (*Migration, error) {
	if err := ensureMigrationTable(ctx, pool); err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(ctx, pool)
	if err != nil {
		return nil, err
	}

	if len(applied) == 0 {
		return nil, nil
	}

	appliedByVersion := make(map[string]AppliedMigration, len(applied))
	for _, item := range applied {
		appliedByVersion[item.Version] = item
	}

	var target *Migration
	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if _, ok := appliedByVersion[migration.Version]; ok {
			target = &migration
			break
		}
	}

	if target == nil {
		return nil, errors.New("database has applied migrations that do not exist locally")
	}
	if strings.TrimSpace(target.DownSQL) == "" {
		return nil, fmt.Errorf("migration %s has no down migration", target.Version)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}

	if err := applyDownMigration(ctx, tx, *target); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return target, nil
}

func ensureMigrationTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, migrationTableName))
	return err
}

func appliedMigrations(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT version, name, checksum, applied_at
		FROM %s
		ORDER BY applied_at ASC, version ASC
	`, migrationTableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []AppliedMigration
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Name, &item.Checksum, &item.AppliedAt); err != nil {
			return nil, err
		}
		applied = append(applied, item)
	}

	return applied, rows.Err()
}

func applyUpMigration(ctx context.Context, tx pgx.Tx, migration Migration) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s IN ACCESS EXCLUSIVE MODE`, migrationTableName)); err != nil {
		return err
	}

	var existingChecksum string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT checksum FROM %s WHERE version = $1`, migrationTableName), migration.Version).Scan(&existingChecksum)
	if err == nil {
		if existingChecksum != migration.Checksum {
			return fmt.Errorf("migration %s checksum mismatch between database and local file", migration.Version)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if strings.TrimSpace(migration.UpSQL) == "" {
		return fmt.Errorf("migration %s is empty", migration.Version)
	}

	if _, err := tx.Exec(ctx, migration.UpSQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.Version, err)
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (version, name, checksum)
		VALUES ($1, $2, $3)
	`, migrationTableName), migration.Version, migration.Name, migration.Checksum); err != nil {
		return err
	}

	return nil
}

func applyDownMigration(ctx context.Context, tx pgx.Tx, migration Migration) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s IN ACCESS EXCLUSIVE MODE`, migrationTableName)); err != nil {
		return err
	}

	var existingChecksum string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT checksum FROM %s WHERE version = $1`, migrationTableName), migration.Version).Scan(&existingChecksum)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("migration %s is not applied", migration.Version)
		}
		return err
	}

	if existingChecksum != "" && existingChecksum != migration.Checksum {
		return fmt.Errorf("migration %s checksum mismatch between database and local file", migration.Version)
	}

	if _, err := tx.Exec(ctx, migration.DownSQL); err != nil {
		return fmt.Errorf("revert migration %s: %w", migration.Version, err)
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE version = $1`, migrationTableName), migration.Version); err != nil {
		return err
	}

	return nil
}

func ensureMigration(migrations map[string]*Migration, version string, name string) *Migration {
	if migration, ok := migrations[version]; ok {
		return migration
	}

	migration := &Migration{
		Version: version,
		Name:    name,
	}
	migrations[version] = migration
	return migration
}

func parseMigrationName(base string) (string, string, bool) {
	version, name, ok := strings.Cut(base, "_")
	if !ok || strings.TrimSpace(version) == "" || strings.TrimSpace(name) == "" {
		return "", "", false
	}

	return version, name, true
}
