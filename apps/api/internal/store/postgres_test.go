package store

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestBuildPoolConfigAppliesPortableDatabaseOptions(t *testing.T) {
	config, err := buildPoolConfig(Options{
		DatabaseURL:          "postgres://postgres:postgres@localhost:5432/coupons?sslmode=disable",
		ApplicationName:      "bazario-api",
		ConnectTimeout:       15 * time.Second,
		MaxConns:             8,
		MinConns:             2,
		MaxConnLifetime:      45 * time.Minute,
		MaxConnIdleTime:      10 * time.Minute,
		HealthCheckPeriod:    30 * time.Second,
		DefaultQueryExecMode: "exec",
	})
	if err != nil {
		t.Fatalf("expected pool config build to succeed: %v", err)
	}

	if got := config.ConnConfig.RuntimeParams["application_name"]; got != "bazario-api" {
		t.Fatalf("expected application_name runtime param, got %q", got)
	}

	if config.ConnConfig.ConnectTimeout != 15*time.Second {
		t.Fatalf("expected connect timeout override, got %s", config.ConnConfig.ConnectTimeout)
	}

	if config.MaxConns != 8 {
		t.Fatalf("expected max conns override, got %d", config.MaxConns)
	}

	if config.MinConns != 2 {
		t.Fatalf("expected min conns override, got %d", config.MinConns)
	}

	if config.MaxConnLifetime != 45*time.Minute {
		t.Fatalf("expected max conn lifetime override, got %s", config.MaxConnLifetime)
	}

	if config.MaxConnIdleTime != 10*time.Minute {
		t.Fatalf("expected max conn idle time override, got %s", config.MaxConnIdleTime)
	}

	if config.HealthCheckPeriod != 30*time.Second {
		t.Fatalf("expected health check period override, got %s", config.HealthCheckPeriod)
	}

	if config.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("expected query exec mode override, got %v", config.ConnConfig.DefaultQueryExecMode)
	}
}

func TestBuildPoolConfigRejectsUnsupportedQueryExecMode(t *testing.T) {
	_, err := buildPoolConfig(Options{
		DatabaseURL:          "postgres://postgres:postgres@localhost:5432/coupons?sslmode=disable",
		DefaultQueryExecMode: "prepared",
	})
	if err == nil {
		t.Fatal("expected pool config build to fail")
	}

	if got := err.Error(); got != `unsupported query exec mode "prepared"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildPoolConfigRequiresDatabaseURL(t *testing.T) {
	_, err := buildPoolConfig(Options{})
	if err == nil {
		t.Fatal("expected pool config build to fail")
	}

	if got := err.Error(); got != "database url is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDatabaseConnectionWarningsForSupabaseHostedSSL(t *testing.T) {
	warnings := DatabaseConnectionWarnings(
		"postgres://postgres:secret@db.project-ref.supabase.co:5432/postgres?sslmode=disable",
		"exec",
	)

	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %d (%v)", len(warnings), warnings)
	}

	expected := "Supabase hosted Postgres should use sslmode=require or stronger in DATABASE_URL"
	if warnings[0] != expected {
		t.Fatalf("unexpected warning\nwant: %s\ngot:  %s", expected, warnings[0])
	}
}

func TestDatabaseConnectionWarningsForSupabasePoolerExecMode(t *testing.T) {
	warnings := DatabaseConnectionWarnings(
		"postgres://postgres.project-ref:secret@aws-0-eu-central-1.pooler.supabase.com:6543/postgres?sslmode=require",
		"cache_statement",
	)

	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %d (%v)", len(warnings), warnings)
	}

	expected := "Supabase pooler connections should use DATABASE_QUERY_EXEC_MODE=exec"
	if warnings[0] != expected {
		t.Fatalf("unexpected warning\nwant: %s\ngot:  %s", expected, warnings[0])
	}
}

func TestDatabaseConnectionWarningsRemainQuietForLocalPostgres(t *testing.T) {
	warnings := DatabaseConnectionWarnings(
		"postgres://postgres:postgres@localhost:5432/coupons?sslmode=disable",
		"",
	)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
