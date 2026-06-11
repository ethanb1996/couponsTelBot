package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                    string
	Port                      string
	AppBaseURL                string
	DatabaseURL               string
	DatabaseApplicationName   string
	DatabaseConnectTimeout    time.Duration
	DatabaseMaxConns          int32
	DatabaseMinConns          int32
	DatabaseMaxConnLifetime   time.Duration
	DatabaseMaxConnIdleTime   time.Duration
	DatabaseHealthCheckPeriod time.Duration
	DatabaseQueryExecMode     string
	TelegramBotToken          string
	TelegramWebhookSecret     string
	TelegramAdminUserIDs      []int64
	TelegramAdminReviewChatID int64
	AdminBasicAuthUser        string
	AdminBasicAuthPass        string
	CouponEncryptionKey       string
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPFrom                  string
	OpsSweepInterval          time.Duration
	OpsDeliveryAlertAfter     time.Duration
	OpsReconcileAfter         time.Duration
	OpsCheckoutHoldDuration   time.Duration
	OpsBatchSize              int
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	databaseConnectTimeout, err := durationEnvWithDefault("DATABASE_CONNECT_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	databaseMaxConns, err := int32EnvWithDefault("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}

	databaseMinConns, err := int32EnvWithDefault("DATABASE_MIN_CONNS", 0)
	if err != nil {
		return Config{}, err
	}

	databaseMaxConnLifetime, err := durationEnvWithDefault("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	databaseMaxConnIdleTime, err := durationEnvWithDefault("DATABASE_MAX_CONN_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	databaseHealthCheckPeriod, err := durationEnvWithDefault("DATABASE_HEALTH_CHECK_PERIOD", time.Minute)
	if err != nil {
		return Config{}, err
	}

	opsSweepInterval, err := durationEnvWithDefault("OPS_SWEEP_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}

	opsDeliveryAlertAfter, err := durationEnvWithDefault("OPS_DELIVERY_ALERT_AFTER", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	opsReconcileAfter, err := durationEnvWithDefault("OPS_RECONCILE_AFTER", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}

	opsCheckoutHoldDuration, err := durationEnvWithDefault("OPS_CHECKOUT_HOLD_DURATION", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}

	opsBatchSize, err := intEnvWithDefault("OPS_BATCH_SIZE", 25)
	if err != nil {
		return Config{}, err
	}
	smtpPort, err := intEnvWithDefault("SMTP_PORT", 587)
	if err != nil {
		return Config{}, err
	}

	telegramAdminUserIDs, err := int64ListEnv("TELEGRAM_ADMIN_USER_IDS")
	if err != nil {
		return Config{}, err
	}
	telegramAdminReviewChatID, err := optionalNonZeroInt64Env("TELEGRAM_ADMIN_REVIEW_CHAT_ID")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                    envWithDefault("APP_ENV", "development"),
		Port:                      envWithDefault("PORT", "8080"),
		AppBaseURL:                envWithDefault("APP_BASE_URL", "http://localhost:8080"),
		DatabaseURL:               strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseApplicationName:   envWithDefault("DATABASE_APPLICATION_NAME", "coupons-api"),
		DatabaseConnectTimeout:    databaseConnectTimeout,
		DatabaseMaxConns:          databaseMaxConns,
		DatabaseMinConns:          databaseMinConns,
		DatabaseMaxConnLifetime:   databaseMaxConnLifetime,
		DatabaseMaxConnIdleTime:   databaseMaxConnIdleTime,
		DatabaseHealthCheckPeriod: databaseHealthCheckPeriod,
		DatabaseQueryExecMode:     strings.ToLower(envWithDefault("DATABASE_QUERY_EXEC_MODE", "exec")),
		TelegramBotToken:          strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramWebhookSecret:     strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")),
		TelegramAdminUserIDs:      telegramAdminUserIDs,
		TelegramAdminReviewChatID: telegramAdminReviewChatID,
		AdminBasicAuthUser:        strings.TrimSpace(os.Getenv("ADMIN_BASIC_AUTH_USER")),
		AdminBasicAuthPass:        strings.TrimSpace(os.Getenv("ADMIN_BASIC_AUTH_PASS")),
		CouponEncryptionKey:       strings.TrimSpace(os.Getenv("COUPON_ENCRYPTION_KEY")),
		SMTPHost:                  strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:                  smtpPort,
		SMTPUsername:              strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:              strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		SMTPFrom:                  strings.TrimSpace(os.Getenv("SMTP_FROM")),
		OpsSweepInterval:          opsSweepInterval,
		OpsDeliveryAlertAfter:     opsDeliveryAlertAfter,
		OpsReconcileAfter:         opsReconcileAfter,
		OpsCheckoutHoldDuration:   opsCheckoutHoldDuration,
		OpsBatchSize:              opsBatchSize,
	}

	var missing []string

	required := []struct {
		key   string
		value string
	}{
		{key: "DATABASE_URL", value: cfg.DatabaseURL},
		{key: "TELEGRAM_BOT_TOKEN", value: cfg.TelegramBotToken},
		{key: "TELEGRAM_WEBHOOK_SECRET", value: cfg.TelegramWebhookSecret},
		{key: "ADMIN_BASIC_AUTH_USER", value: cfg.AdminBasicAuthUser},
		{key: "ADMIN_BASIC_AUTH_PASS", value: cfg.AdminBasicAuthPass},
		{key: "COUPON_ENCRYPTION_KEY", value: cfg.CouponEncryptionKey},
	}

	for _, requiredEnv := range required {
		if requiredEnv.value == "" {
			missing = append(missing, requiredEnv.key)
		}
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if err := validateDatabaseConfig(cfg); err != nil {
		return Config{}, err
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

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("parse %s line %d: expected KEY=VALUE", filepath.Base(path), lineNumber)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("parse %s line %d: missing key", filepath.Base(path), lineNumber)
		}

		existingValue, exists := os.LookupEnv(key)
		if exists && strings.TrimSpace(existingValue) != "" {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, filepath.Base(path), err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}

func int32EnvWithDefault(key string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return int32(parsed), nil
}

func intEnvWithDefault(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return parsed, nil
}

func int64ListEnv(key string) ([]int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must contain comma-separated positive integers", key)
		}
		values = append(values, parsed)
	}

	return values, nil
}

func optionalNonZeroInt64Env(key string) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a non-zero integer", key)
	}

	return parsed, nil
}

func durationEnvWithDefault(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return parsed, nil
}

func validateDatabaseConfig(cfg Config) error {
	if cfg.DatabaseApplicationName == "" {
		return fmt.Errorf("DATABASE_APPLICATION_NAME must not be empty")
	}

	if cfg.DatabaseConnectTimeout <= 0 {
		return fmt.Errorf("DATABASE_CONNECT_TIMEOUT must be greater than zero")
	}

	if cfg.DatabaseMaxConns <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be greater than zero")
	}

	if cfg.DatabaseMinConns < 0 {
		return fmt.Errorf("DATABASE_MIN_CONNS must be zero or greater")
	}

	if cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return fmt.Errorf("DATABASE_MIN_CONNS must be less than or equal to DATABASE_MAX_CONNS")
	}

	if cfg.DatabaseMaxConnLifetime <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONN_LIFETIME must be greater than zero")
	}

	if cfg.DatabaseMaxConnIdleTime <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONN_IDLE_TIME must be greater than zero")
	}

	if cfg.DatabaseHealthCheckPeriod <= 0 {
		return fmt.Errorf("DATABASE_HEALTH_CHECK_PERIOD must be greater than zero")
	}

	switch len(cfg.CouponEncryptionKey) {
	case 16, 24, 32:
	default:
		return fmt.Errorf("COUPON_ENCRYPTION_KEY must be 16, 24, or 32 bytes")
	}

	if cfg.OpsSweepInterval <= 0 {
		return fmt.Errorf("OPS_SWEEP_INTERVAL must be greater than zero")
	}

	if cfg.OpsDeliveryAlertAfter <= 0 {
		return fmt.Errorf("OPS_DELIVERY_ALERT_AFTER must be greater than zero")
	}

	if cfg.OpsReconcileAfter <= 0 {
		return fmt.Errorf("OPS_RECONCILE_AFTER must be greater than zero")
	}

	if cfg.OpsCheckoutHoldDuration <= 0 {
		return fmt.Errorf("OPS_CHECKOUT_HOLD_DURATION must be greater than zero")
	}

	if cfg.OpsBatchSize <= 0 {
		return fmt.Errorf("OPS_BATCH_SIZE must be greater than zero")
	}

	if cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP_PORT must be greater than zero")
	}
	if cfg.SMTPHost != "" && cfg.SMTPFrom == "" {
		return fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
	}

	switch cfg.DatabaseQueryExecMode {
	case "", "cache_statement", "cache_describe", "describe_exec", "exec", "simple_protocol":
	default:
		return fmt.Errorf("DATABASE_QUERY_EXEC_MODE must be one of: cache_statement, cache_describe, describe_exec, exec, simple_protocol")
	}

	return nil
}
