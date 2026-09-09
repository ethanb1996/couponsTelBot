package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestCaptionedPhotoOfferRequiresPhotoAndCaption(t *testing.T) {
	tests := []struct {
		name    string
		message *tgbotapi.Message
		want    string
		ok      bool
	}{
		{name: "photo with caption", message: &tgbotapi.Message{Photo: []tgbotapi.PhotoSize{{FileID: "photo"}}, Caption: "  Japan Japan\nשובר 100 ₪  "}, want: "Japan Japan\nשובר 100 ₪", ok: true},
		{name: "text only", message: &tgbotapi.Message{Text: "Japan Japan\nשובר 100 ₪"}},
		{name: "caption without photo", message: &tgbotapi.Message{Caption: "Japan Japan\nשובר 100 ₪"}},
		{name: "photo without caption", message: &tgbotapi.Message{Photo: []tgbotapi.PhotoSize{{FileID: "photo"}}}},
		{name: "nil message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := captionedPhotoOffer(tt.message)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("captionedPhotoOffer()=(%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
