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
	if err := bot.syncChannelMenu(); err != nil {
		logger.Warn("channel menu is not connected yet", "error", err, "channel_id", channelID)
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
		case "start":
			if offerID, ok := parseOfferStartPayload(update.Message.CommandArguments()); ok {
				return b.sendInterest(update.Message.Chat.ID, offerID, update.Message.From)
			}
			return b.sendMenu(update.Message.Chat.ID)
		case "offers":
			return b.sendMenu(update.Message.Chat.ID)
		}
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
	return b.sendInterest(callback.Message.Chat.ID, strings.TrimPrefix(callback.Data, offerCallbackPrefix), callback.From)
}

func (b *Bot) sendInterest(chatID int64, offerID string, user *tgbotapi.User) error {
	offer, ok := b.catalog.Get(offerID)
	if !ok {
		_, err := b.api.Send(tgbotapi.NewMessage(chatID, "ההצעה כבר אינה בין שמונה ההצעות האחרונות. שלחו /offers לרשימה המעודכנת."))
		return err
	}

	buyerText := fmt.Sprintf("<b>ההתעניינות נשלחה ✅</b>\n\nאני מעוניין/ת בקופון:\n<b>%s</b>\n\nנציג יחזור אליך כאן להשלמת הרכישה.", html.EscapeString(offer.Title))
	message := tgbotapi.NewMessage(chatID, buyerText)
	message.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(message); err != nil {
		return err
	}
	return b.notifyAdmins(context.Background(), offer, user)
}

func (b *Bot) syncChannelMenu() error {
	text, keyboard := b.channelMenu()
	messageID := b.catalog.MenuMessageID()
	if messageID != 0 {
		edit := tgbotapi.NewEditMessageTextAndMarkup(b.channel, messageID, text, keyboard)
		edit.ParseMode = tgbotapi.ModeHTML
		if _, err := b.api.Request(edit); err == nil {
			return b.pinChannelMenu(messageID)
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
		text = "<b>8 ההצעות האחרונות</b>\n\nבחרו קופון ונחזור אליכם להשלמת הרכישה:"
		for _, offer := range offers {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL(offer.Title, b.offerDeepLink(offer.ID)),
			))
		}
	}
	return text, tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Bot) offerDeepLink(offerID string) string {
	return fmt.Sprintf("https://t.me/%s?start=offer_%s", b.api.Self.UserName, offerID)
}

func parseOfferStartPayload(payload string) (string, bool) {
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, "offer_") {
		return "", false
	}
	offerID := strings.TrimPrefix(payload, "offer_")
	return offerID, offerID != ""
}

func (b *Bot) pinChannelMenu(messageID int) error {
	_, err := b.api.Request(tgbotapi.PinChatMessageConfig{
		ChatID:              b.channel,
		MessageID:           messageID,
		DisableNotification: true,
	})
	return err
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
