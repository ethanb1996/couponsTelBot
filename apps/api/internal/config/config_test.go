package config

import (
	"strings"
	"testing"
)

func TestLoadReportsMissingEnvironmentVariablesInStableOrder(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("PAYMENT_PROVIDER_NAME", "")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_SECRET", "")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "")
	t.Setenv("COUPON_ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected config load to fail")
	}

	expected := "missing required environment variables: DATABASE_URL, TELEGRAM_BOT_TOKEN, TELEGRAM_WEBHOOK_SECRET, PAYMENT_PROVIDER_NAME, PAYMENT_PROVIDER_SECRET, PAYMENT_PROVIDER_WEBHOOK_SECRET, ADMIN_BASIC_AUTH_USER, ADMIN_BASIC_AUTH_PASS, COUPON_ENCRYPTION_KEY"
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
	t.Setenv("PAYMENT_PROVIDER_NAME", "mockpay")
	t.Setenv("PAYMENT_PROVIDER_SECRET", "provider-secret")
	t.Setenv("PAYMENT_PROVIDER_WEBHOOK_SECRET", "provider-webhook-secret")
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
}
