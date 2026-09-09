package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	logger          *slog.Logger
	api             *tgbotapi.BotAPI
	catalog         *Catalog
	channel         int64
	channelUsername string
	menuMu          sync.Mutex
}

func NewBot(logger *slog.Logger, token string, channelID int64, catalog *Catalog) (*Bot, error) {
	api, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		return nil, err
	}
	chat, err := api.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: channelID}})
	if err != nil {
		return nil, fmt.Errorf("get menu channel: %w", err)
	}
	channelUsername := strings.TrimPrefix(strings.TrimSpace(chat.UserName), "@")
	if channelUsername == "" {
		return nil, fmt.Errorf("menu channel must be public and have a username for direct-message links")
	}
	bot := &Bot{logger: logger, api: api, catalog: catalog, channel: channelID, channelUsername: channelUsername}
	_, _ = api.Request(tgbotapi.NewDeleteMyCommands())
	if err := bot.syncChannelMenu(); err != nil {
		logger.Warn("channel menu is not connected yet", "error", err, "channel_id", channelID)
	}
	return bot, nil
}

func (b *Bot) HandleUpdate(_ context.Context, update tgbotapi.Update) error {
	if update.ChannelPost != nil {
		return b.captureChannelPost(update.ChannelPost)
	}
	if update.EditedChannelPost != nil {
		return b.captureChannelPost(update.EditedChannelPost)
	}
	return nil
}

func (b *Bot) captureChannelPost(message *tgbotapi.Message) error {
	if message == nil || message.Chat == nil || int64(message.Chat.ID) != b.channel {
		return nil
	}
	if message.MessageID == b.catalog.MenuMessageID() {
		return nil
	}
	text := strings.TrimSpace(message.Text)
	if text == "" {
		text = strings.TrimSpace(message.Caption)
	}
	if text == "" {
		return nil
	}
	published := time.Unix(int64(message.Date), 0)
	if message.Date == 0 {
		published = time.Now()
	}
	offer, duplicate, err := b.catalog.Add(text, message.MessageID, published)
	if err != nil {
		return err
	}
	b.logger.Info("captured channel offer", "offer_id", offer.ID, "title", offer.Title, "duplicate_refreshed", duplicate)
	return b.syncChannelMenu()
}

func (b *Bot) syncChannelMenu() error {
	b.menuMu.Lock()
	defer b.menuMu.Unlock()

	text, keyboard := b.channelMenu()
	messageID := b.catalog.MenuMessageID()
	if messageID != 0 {
		edit := tgbotapi.NewEditMessageTextAndMarkup(b.channel, messageID, text, keyboard)
		edit.ParseMode = tgbotapi.ModeHTML
		_, err := b.api.Request(edit)
		if err == nil || isMessageNotModified(err) {
			return b.pinChannelMenu(messageID)
		}
		if !isMissingMenuMessage(err) {
			return err
		}
		if err := b.catalog.SetMenuMessageID(0); err != nil {
			return err
		}
	}

	message := tgbotapi.NewMessage(b.channel, text)
	message.ParseMode = tgbotapi.ModeHTML
	// Telegram rejects an empty inline keyboard (the zero-value markup is
	// serialized as null). Publish the empty-state menu without reply markup;
	// buttons are added as soon as the first offer is captured.
	if len(keyboard.InlineKeyboard) > 0 {
		message.ReplyMarkup = keyboard
	}
	sent, err := b.api.Send(message)
	if err != nil {
		return err
	}
	if err := b.catalog.SetMenuMessageID(sent.MessageID); err != nil {
		return err
	}
	return b.pinChannelMenu(sent.MessageID)
}

func (b *Bot) channelMenu() (string, tgbotapi.InlineKeyboardMarkup) {
	offers := b.catalog.List()
	text := "<b>תפריט הקופונים</b>\n\nההצעות האחרונות יופיעו כאן."
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(offers))
	if len(offers) > 0 {
		text = fmt.Sprintf("<b>ההצעות האחרונות (%d)</b>\n\nבחרו קופון ונחזור אליכם להשלמת הרכישה:", len(offers))
		for _, offer := range offers {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL(offerButtonLabel(offer.Text), channelDirectLink(b.channelUsername, offer)),
			))
		}
	}
	return text, tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func isMessageNotModified(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func isMissingMenuMessage(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "message to edit not found") ||
		strings.Contains(message, "message can't be edited") ||
		strings.Contains(message, "message_id_invalid")
}

func channelDirectLink(channelUsername string, offer Offer) string {
	draft := "היי, אני מעוניין/ת בקופון: " + offerButtonLabel(offer.Text)
	return fmt.Sprintf("https://t.me/%s?direct&text=%s", strings.TrimPrefix(channelUsername, "@"), url.QueryEscape(draft))
}

func (b *Bot) pinChannelMenu(messageID int) error {
	_, err := b.api.Request(tgbotapi.PinChatMessageConfig{
		ChatID:              b.channel,
		MessageID:           messageID,
		DisableNotification: true,
	})
	return err
}
