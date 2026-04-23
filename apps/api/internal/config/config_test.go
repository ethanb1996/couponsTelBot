package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadReportsMissingEnvironmentVariablesInStableOrder(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("PAYMENT_PROVIDER_NAME", "")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "")
	t.Setenv("COUPON_ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	expected := "missing required environment variables: DATABASE_URL, TELEGRAM_BOT_TOKEN, TELEGRAM_WEBHOOK_SECRET, PAYMENT_PROVIDER_NAME, PAYMENT_PROVIDER_CLIENT_ID, PAYMENT_PROVIDER_SECRET, PAYMENT_PROVIDER_BASE_URL, PAYMENT_PROVIDER_WEBHOOK_ID, ADMIN_BASIC_AUTH_USER, ADMIN_BASIC_AUTH_PASS, COUPON_ENCRYPTION_KEY"
	if err.Error() != expected {
		t.Fatalf("unexpected error message\nwant: %s\ngot:  %s", expected, err.Error())
	}
}

func TestLoadAppliesDefaultsForOptionalEnvironmentVariables(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "paypal")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://api-m.sandbox.paypal.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config load to succeed: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("expected default APP_ENV, got %q", cfg.AppEnv)
	}

	if cfg.Port != "8080" {
		t.Fatalf("expected default PORT, got %q", cfg.Port)
	}

	if cfg.AppBaseURL != "http://localhost:8080" {
		t.Fatalf("expected default APP_BASE_URL, got %q", cfg.AppBaseURL)
	}

	if cfg.DatabaseApplicationName != "coupons-api" {
		t.Fatalf("expected default DATABASE_APPLICATION_NAME, got %q", cfg.DatabaseApplicationName)
	}

	if cfg.DatabaseConnectTimeout != 10*time.Second {
		t.Fatalf("expected default DATABASE_CONNECT_TIMEOUT, got %s", cfg.DatabaseConnectTimeout)
	}

	if cfg.DatabaseMaxConns != 10 {
		t.Fatalf("expected default DATABASE_MAX_CONNS, got %d", cfg.DatabaseMaxConns)
	}

	if cfg.DatabaseMinConns != 0 {
		t.Fatalf("expected default DATABASE_MIN_CONNS, got %d", cfg.DatabaseMinConns)
	}

	if cfg.DatabaseMaxConnLifetime != 30*time.Minute {
		t.Fatalf("expected default DATABASE_MAX_CONN_LIFETIME, got %s", cfg.DatabaseMaxConnLifetime)
	}

	if cfg.DatabaseMaxConnIdleTime != 5*time.Minute {
		t.Fatalf("expected default DATABASE_MAX_CONN_IDLE_TIME, got %s", cfg.DatabaseMaxConnIdleTime)
	}

	if cfg.DatabaseHealthCheckPeriod != time.Minute {
		t.Fatalf("expected default DATABASE_HEALTH_CHECK_PERIOD, got %s", cfg.DatabaseHealthCheckPeriod)
	}

	if cfg.DatabaseQueryExecMode != "exec" {
		t.Fatalf("expected default DATABASE_QUERY_EXEC_MODE, got %q", cfg.DatabaseQueryExecMode)
	}
}

func TestLoadAppliesExplicitDatabaseSettings(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("DATABASE_APPLICATION_NAME", "bazario-api")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "20s")
	t.Setenv("DATABASE_MAX_CONNS", "12")
	t.Setenv("DATABASE_MIN_CONNS", "2")
	t.Setenv("DATABASE_MAX_CONN_LIFETIME", "45m")
	t.Setenv("DATABASE_MAX_CONN_IDLE_TIME", "10m")
	t.Setenv("DATABASE_HEALTH_CHECK_PERIOD", "30s")
	t.Setenv("DATABASE_QUERY_EXEC_MODE", "EXEC")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "paypal")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://api-m.sandbox.paypal.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config load to succeed: %v", err)
	}

	if cfg.DatabaseApplicationName != "bazario-api" {
		t.Fatalf("expected explicit DATABASE_APPLICATION_NAME, got %q", cfg.DatabaseApplicationName)
	}

	if cfg.DatabaseConnectTimeout != 20*time.Second {
		t.Fatalf("expected explicit DATABASE_CONNECT_TIMEOUT, got %s", cfg.DatabaseConnectTimeout)
	}

	if cfg.DatabaseMaxConns != 12 {
		t.Fatalf("expected explicit DATABASE_MAX_CONNS, got %d", cfg.DatabaseMaxConns)
	}

	if cfg.DatabaseMinConns != 2 {
		t.Fatalf("expected explicit DATABASE_MIN_CONNS, got %d", cfg.DatabaseMinConns)
	}

	if cfg.DatabaseMaxConnLifetime != 45*time.Minute {
		t.Fatalf("expected explicit DATABASE_MAX_CONN_LIFETIME, got %s", cfg.DatabaseMaxConnLifetime)
	}

	if cfg.DatabaseMaxConnIdleTime != 10*time.Minute {
		t.Fatalf("expected explicit DATABASE_MAX_CONN_IDLE_TIME, got %s", cfg.DatabaseMaxConnIdleTime)
	}

	if cfg.DatabaseHealthCheckPeriod != 30*time.Second {
		t.Fatalf("expected explicit DATABASE_HEALTH_CHECK_PERIOD, got %s", cfg.DatabaseHealthCheckPeriod)
	}

	if cfg.DatabaseQueryExecMode != "exec" {
		t.Fatalf("expected normalized DATABASE_QUERY_EXEC_MODE, got %q", cfg.DatabaseQueryExecMode)
	}
}

func TestLoadRejectsInvalidDatabaseSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "not-a-duration")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "paypal")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://api-m.sandbox.paypal.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	if !strings.Contains(err.Error(), "DATABASE_CONNECT_TIMEOUT must be a valid duration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidDatabaseQueryExecMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("DATABASE_QUERY_EXEC_MODE", "prepared")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "paypal")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://api-m.sandbox.paypal.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	expected := "DATABASE_QUERY_EXEC_MODE must be one of: cache_statement, cache_describe, describe_exec, exec, simple_protocol"
	if err.Error() != expected {
		t.Fatalf("unexpected error message\nwant: %s\ngot:  %s", expected, err.Error())
	}
}

func TestLoadRejectsNonPayPalProvider(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "mockpay")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://api-m.sandbox.paypal.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	if got := err.Error(); got != "PAYMENT_PROVIDER_NAME must be paypal" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnexpectedPayPalBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "telegram-secret")
	t.Setenv("PAYMENT_PROVIDER_NAME", "paypal")
	t.Setenv("PAYMENT_PROVIDER_CLIENT_ID", "paypal-client-id")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_BASE_URL", "https://example.com")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_ID", "provider-webhook-id")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "password")
	t.Setenv("COUPON_ENCRYPTION_KEY", strings.Repeat("a", 32))

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	expected := "PAYMENT_PROVIDER_BASE_URL must be https://api-m.sandbox.paypal.com or https://api-m.paypal.com"
	if err.Error() != expected {
		t.Fatalf("unexpected error message\nwant: %s\ngot:  %s", expected, err.Error())
	}
}

func TestLoadReadsDotEnvFromCurrentWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	dotEnvPath := filepath.Join(tempDir, ".env")
	dotEnv := strings.Join([]string{
		"APP_ENV=development",
		"PORT=8181",
		"APP_BASE_URL=http://localhost:8181",
		`DATABASE_URL="postgres://postgres:postgres@localhost:5432/coupons?sslmode=disable"`,
		"TELEGRAM_BOT_TOKEN=dotenv-bot-token",
		"TELEGRAM_WEBHOOK_SECRET=dotenv-telegram-secret",
		"PAYMENT_PROVIDER_NAME=paypal",
		"PAYMENT_PROVIDER_CLIENT_ID=dotenv-paypal-client-id",
		"PAYMENT_PROVIDER_SECRET=dotenv-provider-secret",
		"PAYMENT_PROVIDER_BASE_URL=https://api-m.sandbox.paypal.com",
		"PAYMENT_PROVIDER_WEBHOOK_ID=dotenv-provider-webhook-id",
		"ADMIN_BASIC_AUTH_USER=dotenv-admin",
		"ADMIN_BASIC_AUTH_PASS=dotenv-password",
		"COUPON_ENCRYPTION_KEY=" + strings.Repeat("a", 32),
		"",
	}, "\n")
	if err := os.WriteFile(dotEnvPath, []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	keys := []string{
		"APP_ENV",
		"PORT",
		"APP_BASE_URL",
		"DATABASE_URL",
		"DATABASE_APPLICATION_NAME",
		"DATABASE_CONNECT_TIMEOUT",
		"DATABASE_MAX_CONNS",
		"DATABASE_MIN_CONNS",
		"DATABASE_MAX_CONN_LIFETIME",
		"DATABASE_MAX_CONN_IDLE_TIME",
		"DATABASE_HEALTH_CHECK_PERIOD",
		"DATABASE_QUERY_EXEC_MODE",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_WEBHOOK_SECRET",
		"PAYMENT_PROVIDER_NAME",
		"PAYMENT_PROVIDER_CLIENT_ID",
		"PAYMENT_PROVIDER_SECRET",
		"PAYMENT_PROVIDER_BASE_URL",
		"PAYMENT_PROVIDER_WEBHOOK_ID",
		"ADMIN_BASIC_AUTH_USER",
		"ADMIN_BASIC_AUTH_PASS",
		"COUPON_ENCRYPTION_KEY",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	t.Setenv("PORT", "9191")

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDir)
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config load to succeed from .env: %v", err)
	}

	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/coupons?sslmode=disable" {
		t.Fatalf("expected DATABASE_URL from .env, got %q", cfg.DatabaseURL)
	}

	if cfg.Port != "9191" {
		t.Fatalf("expected explicit environment variable to override .env, got %q", cfg.Port)
	}

	if cfg.TelegramBotToken != "dotenv-bot-token" {
		t.Fatalf("expected TELEGRAM_BOT_TOKEN from .env, got %q", cfg.TelegramBotToken)
	}

	if cfg.DatabaseQueryExecMode != "exec" {
		t.Fatalf("expected default DATABASE_QUERY_EXEC_MODE alongside .env load, got %q", cfg.DatabaseQueryExecMode)
	}
}
