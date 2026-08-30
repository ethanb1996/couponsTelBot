package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                  string
	TelegramBotToken      string
	TelegramWebhookSecret string
	TelegramChannelID     int64
	TelegramAdminUserIDs  []int64
	OffersFile            string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	channelID, err := requiredInt64("TELEGRAM_CHANNEL_ID")
	if err != nil {
		return Config{}, err
	}
	adminIDs, err := int64List("TELEGRAM_ADMIN_USER_IDS")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:                  envDefault("PORT", "8080"),
		TelegramBotToken:      strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramWebhookSecret: strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")),
		TelegramChannelID:     channelID,
		TelegramAdminUserIDs:  adminIDs,
		OffersFile:            envDefault("OFFERS_FILE", filepath.Join("data", "offers.json")),
	}

	var missing []string
	if cfg.TelegramBotToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.TelegramWebhookSecret == "" {
		missing = append(missing, "TELEGRAM_WEBHOOK_SECRET")
	}
	if len(cfg.TelegramAdminUserIDs) == 0 {
		missing = append(missing, "TELEGRAM_ADMIN_USER_IDS")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if current, exists := os.LookupEnv(key); exists && strings.TrimSpace(current) != "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func requiredInt64(key string) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a non-zero integer", key)
	}
	return parsed, nil
}

func int64List(key string) ([]int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	var result []int64
	for _, part := range strings.Split(value, ",") {
		parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must contain comma-separated positive integers", key)
		}
		result = append(result, parsed)
	}
	return result, nil
}
