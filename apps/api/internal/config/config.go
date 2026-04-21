package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	AppEnv                        string
	Port                          string
	AppBaseURL                    string
	DatabaseURL                   string
	TelegramBotToken              string
	TelegramWebhookSecret         string
	PaymentProviderName           string
	PaymentProviderSecret         string
	PaymentProviderWebhookSecret  string
	AdminBasicAuthUser            string
	AdminBasicAuthPass            string
	CouponEncryptionKey           string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:                       envWithDefault("APP_ENV", "development"),
		Port:                         envWithDefault("PORT", "8080"),
		AppBaseURL:                   envWithDefault("APP_BASE_URL", "http://localhost:8080"),
		DatabaseURL:                  strings.TrimSpace(os.Getenv("DATABASE_URL")),
		TelegramBotToken:             strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramWebhookSecret:        strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")),
		PaymentProviderName:          strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER_NAME")),
		PaymentProviderSecret:        strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER_SECRET")),
		PaymentProviderWebhookSecret: strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER_WEBHOOK_SECRET")),
		AdminBasicAuthUser:           strings.TrimSpace(os.Getenv("ADMIN_BASIC_AUTH_USER")),
		AdminBasicAuthPass:           strings.TrimSpace(os.Getenv("ADMIN_BASIC_AUTH_PASS")),
		CouponEncryptionKey:          strings.TrimSpace(os.Getenv("COUPON_ENCRYPTION_KEY")),
	}

	var missing []string

	required := map[string]string{
		"DATABASE_URL":                  cfg.DatabaseURL,
		"TELEGRAM_BOT_TOKEN":            cfg.TelegramBotToken,
		"TELEGRAM_WEBHOOK_SECRET":       cfg.TelegramWebhookSecret,
		"PAYMENT_PROVIDER_NAME":         cfg.PaymentProviderName,
		"PAYMENT_PROVIDER_SECRET":       cfg.PaymentProviderSecret,
		"PAYMENT_PROVIDER_WEBHOOK_SECRET": cfg.PaymentProviderWebhookSecret,
		"ADMIN_BASIC_AUTH_USER":         cfg.AdminBasicAuthUser,
		"ADMIN_BASIC_AUTH_PASS":         cfg.AdminBasicAuthPass,
		"COUPON_ENCRYPTION_KEY":         cfg.CouponEncryptionKey,
	}

	for key, value := range required {
		if value == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.AppEnv) {
	case "production":
		return slog.LevelInfo
	case "test":
		return slog.LevelWarn
	default:
		return slog.LevelDebug
	}
}

func envWithDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
