package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const offerCallbackPrefix = "offer:"

type Bot struct {
	logger   *slog.Logger
	api      *tgbotapi.BotAPI
	catalog  *Catalog
	channel  int64
	adminIDs []int64
}

func NewBot(logger *slog.Logger, token string, channelID int64, adminIDs []int64, catalog *Catalog) (*Bot, error) {
	api, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		return nil, err
	}
	bot := &Bot{logger: logger, api: api, catalog: catalog, channel: channelID, adminIDs: adminIDs}
	commands := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Show the latest coupon offers"},
		tgbotapi.BotCommand{Command: "offers", Description: "Show the latest coupon offers"},
	)
	if _, err := api.Request(commands); err != nil {
		logger.Warn("failed to set bot commands", "error", err)
	}
	return bot, nil
}

func (b *Bot) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.ChannelPost != nil {
		return b.captureChannelPost(update.ChannelPost)
	}
	if update.EditedChannelPost != nil {
		return b.captureChannelPost(update.EditedChannelPost)
	}
	if update.CallbackQuery != nil {
		return b.handleCallback(ctx, update.CallbackQuery)
	}
	if update.Message != nil && update.Message.IsCommand() {
		switch update.Message.Command() {
		case "start", "offers":
			return b.sendMenu(update.Message.Chat.ID)
		}
	}
	return nil
}

func (b *Bot) captureChannelPost(message *tgbotapi.Message) error {
	if message == nil || message.Chat == nil || int64(message.Chat.ID) != b.channel {
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
	if err == nil {
		b.logger.Info("captured channel offer", "offer_id", offer.ID, "title", offer.Title, "duplicate_refreshed", duplicate)
	}
	return err
}

func (b *Bot) sendMenu(chatID int64) error {
	offers := b.catalog.List()
	if len(offers) == 0 {
		_, err := b.api.Send(tgbotapi.NewMessage(chatID, "עדיין אין קופונים בתפריט. ההצעות יופיעו כאן אחרי פרסום חדש בערוץ."))
		return err
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(offers))
	for _, offer := range offers {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(offer.Title, offerCallbackPrefix+offer.ID),
		))
	}
	message := tgbotapi.NewMessage(chatID, "<b>8 ההצעות האחרונות</b>\n\nבחרו קופון ונחזור אליכם להשלמת הרכישה:")
	message.ParseMode = tgbotapi.ModeHTML
	message.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, err := b.api.Send(message)
	return err
}

func (b *Bot) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	if callback == nil || callback.Message == nil || callback.From == nil || !strings.HasPrefix(callback.Data, offerCallbackPrefix) {
		return nil
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, "ההתעניינות נשלחה"))
	offer, ok := b.catalog.Get(strings.TrimPrefix(callback.Data, offerCallbackPrefix))
	if !ok {
		_, err := b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "ההצעה כבר אינה בין שמונה ההצעות האחרונות. שלחו /offers לרשימה המעודכנת."))
		return err
	}

	buyerText := fmt.Sprintf("<b>ההתעניינות נשלחה ✅</b>\n\nאני מעוניין/ת בקופון:\n<b>%s</b>\n\nנציג יחזור אליך כאן להשלמת הרכישה.", html.EscapeString(offer.Title))
	message := tgbotapi.NewMessage(callback.Message.Chat.ID, buyerText)
	message.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(message); err != nil {
		return err
	}
	return b.notifyAdmins(ctx, offer, callback.From)
}

func (b *Bot) notifyAdmins(_ context.Context, offer Offer, user *tgbotapi.User) error {
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if name == "" {
		name = "Telegram user"
	}
	text := fmt.Sprintf(`<b>התעניינות חדשה בקופון</b>

קופון: <b>%s</b>
לקוח: <a href="tg://user?id=%d">%s</a>
Telegram ID: <code>%d</code>`, html.EscapeString(offer.Title), user.ID, html.EscapeString(name), user.ID)
	if user.UserName != "" {
		text += "\nUsername: @" + html.EscapeString(user.UserName)
	}
	var firstErr error
	for _, adminID := range b.adminIDs {
		message := tgbotapi.NewMessage(adminID, text)
		message.ParseMode = tgbotapi.ModeHTML
		if _, err := b.api.Send(message); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
