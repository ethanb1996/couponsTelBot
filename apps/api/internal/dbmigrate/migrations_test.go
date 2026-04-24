package dbmigrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMigrationsPairsUpAndDownFiles(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "0001_initial_schema.sql"), "CREATE TABLE example (id INT);")
	writeTestFile(t, filepath.Join(dir, "0001_initial_schema.down.sql"), "DROP TABLE example;")
	writeTestFile(t, filepath.Join(dir, "0002_indexes.sql"), "CREATE INDEX idx ON example (id);")

	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	if migrations[0].Version != "0001" || migrations[0].Name != "initial_schema" {
		t.Fatalf("unexpected first migration: %+v", migrations[0])
	}

	if migrations[0].DownSQL == "" {
		t.Fatalf("expected first migration to have down SQL")
	}

	if migrations[1].Version != "0002" || migrations[1].DownSQL != "" {
		t.Fatalf("unexpected second migration: %+v", migrations[1])
	}
}

func TestLoadMigrationsRejectsMissingUpFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "0003_only_down.down.sql"), "DROP TABLE nope;")

	if _, err := LoadMigrations(dir); err == nil {
		t.Fatalf("expected error for missing up migration")
	}
}

func TestStatusMarksUnknownAppliedMigration(t *testing.T) {
	appliedAt := time.Unix(1, 0).UTC()
	rows := statusRowsForTest([]Migration{
		{Version: "0001", Name: "initial", Checksum: "abc"},
	}, []AppliedMigration{
		{Version: "0001", Name: "initial", Checksum: "abc", AppliedAt: appliedAt},
		{Version: "0009", Name: "legacy", Checksum: "zzz", AppliedAt: appliedAt},
	})

	if len(rows) != 2 {
		t.Fatalf("expected 2 status rows, got %d", len(rows))
	}

	if rows[0].State != "applied" {
		t.Fatalf("expected first row to be applied, got %s", rows[0].State)
	}

	if rows[1].State != "missing_local_file" {
		t.Fatalf("expected second row to be missing_local_file, got %s", rows[1].State)
	}
}

func TestStatusMarksChecksumDrift(t *testing.T) {
	appliedAt := time.Unix(1, 0).UTC()
	rows := statusRowsForTest([]Migration{
		{Version: "0001", Name: "initial", Checksum: "local"},
	}, []AppliedMigration{
		{Version: "0001", Name: "initial", Checksum: "remote", AppliedAt: appliedAt},
	})

	if len(rows) != 1 {
		t.Fatalf("expected 1 status row, got %d", len(rows))
	}

	if rows[0].State != "drifted" || !rows[0].Drift {
		t.Fatalf("expected drifted row, got %+v", rows[0])
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func statusRowsForTest(migrations []Migration, applied []AppliedMigration) []StatusRow {
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

	return rows
}
