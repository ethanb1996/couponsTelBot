package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
	t.Setenv("TELEGRAM_CHANNEL_ID", "-4448924956")
	t.Setenv("OFFERS_FILE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.TelegramChannelID != -4448924956 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.OffersFile != filepath.Join("data", "offers.json") {
		t.Fatalf("unexpected offers file: %q", cfg.OffersFile)
	}
}

func TestLoadRejectsMissingBotSettings(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("TELEGRAM_CHANNEL_ID", "-1")
	_, err := Load()
	if err == nil {
		t.Fatal("expected missing settings error")
	}
}
