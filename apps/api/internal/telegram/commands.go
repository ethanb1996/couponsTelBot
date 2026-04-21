package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CallbackPrefix is used to encode action and data in callback queries
const (
	CallbackBuyListing     = "buy_listing"
	CallbackConfirmBuy     = "confirm_buy"
	CallbackViewDetails    = "view_details"
	CallbackContactSupport = "contact_support"
)

type BotService struct {
	logger *slog.Logger
	store  *store.Postgres
	botAPI *tgbotapi.BotAPI
	config *config.Config
}

func NewBotService(logger *slog.Logger, repo *store.Postgres, botToken string, cfg *config.Config) (*BotService, error) {
	api, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, err
	}
	return &BotService{
		logger: logger,
		store:  repo,
		botAPI: api,
		config: cfg,
	}, nil
}

// HandleUpdate processes incoming Telegram updates
func (s *BotService) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Handle slash commands
	if update.Message != nil && update.Message.IsCommand() {
		return s.handleCommand(ctx, update.Message)
	}

	// Handle callback queries (button presses)
	if update.CallbackQuery != nil {
		return s.handleCallback(ctx, update.CallbackQuery)
	}

	return nil
}

// handleCommand processes slash commands
func (s *BotService) handleCommand(ctx context.Context, message *tgbotapi.Message) error {
	switch message.Command() {
	case "start":
		return s.handleStart(ctx, message)
	case "help":
		return s.handleHelp(ctx, message)
	default:
		return s.sendMessage(ctx, message.Chat.ID, "Unknown command. Type /help for available commands.")
	}
}

// handleStart sends active listings to the user
func (s *BotService) handleStart(ctx context.Context, message *tgbotapi.Message) error {
	// Get or create user
	user, err := s.getOrCreateUser(ctx, message.From)
	if err != nil {
		s.logger.Error("failed to get or create user", "error", err, "user_id", message.From.ID)
		return s.sendMessage(ctx, message.Chat.ID, "Something went wrong. Please try again.")
	}

	// Fetch active listings
	listings, err := s.store.ListActiveListings(ctx)
	if err != nil {
		s.logger.Error("failed to fetch active listings", "error", err)
		return s.sendMessage(ctx, message.Chat.ID, "Unable to load coupons. Please try again later.")
	}

	if len(listings) == 0 {
		return s.sendMessage(ctx, message.Chat.ID, "No coupons available at the moment. Check back soon!")
	}

	// Show up to 3 listings
	displayCount := len(listings)
	if displayCount > 3 {
		displayCount = 3
	}

	text := "🎉 Welcome to CouponTelBot!\n\nHere are today's best deals:\n\n"

	msg := tgbotapi.NewMessage(message.Chat.ID, "")
	msg.Text = text + s.formatListingsForDisplay(listings[:displayCount])
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = s.createListingsKeyboard(user.ID, listings[:displayCount])

	_, err = s.botAPI.Send(msg)
	if err != nil {
		s.logger.Error("failed to send message", "error", err)
		return err
	}

	return nil
}

// handleHelp sends help message
func (s *BotService) handleHelp(ctx context.Context, message *tgbotapi.Message) error {
	helpText := `<b>How to use CouponTelBot:</b>

/start - See available coupons
/help - Show this message

<b>How to buy:</b>
1. Type /start to see active coupons
2. Press the <b>Buy</b> button on a coupon
3. Review the coupon details
4. Confirm your purchase
5. Complete payment
6. Get your coupon code

<b>Need help?</b>
Use the support button on any coupon detail screen.`

	return s.sendMessage(ctx, message.Chat.ID, helpText)
}

// handleCallback processes inline button callbacks
func (s *BotService) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	// Acknowledge callback
	answer := tgbotapi.NewCallback(callback.ID, "")
	s.botAPI.Request(answer)

	parts := parseCallbackData(callback.Data)
	if len(parts) == 0 {
		return nil
	}

	action := parts[0]

	switch action {
	case CallbackBuyListing:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleBuyListing(ctx, callback, listingID)

	case CallbackViewDetails:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleViewDetails(ctx, callback.Message.Chat.ID, listingID, callback.From.ID)

	case CallbackConfirmBuy:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleConfirmBuy(ctx, callback, listingID)

	default:
		return nil
	}
}

// handleBuyListing shows coupon details before purchase confirmation
func (s *BotService) handleBuyListing(ctx context.Context, callback *tgbotapi.CallbackQuery, listingID int64) error {
	listing, err := s.store.GetListing(ctx, listingID)
	if err != nil {
		s.logger.Error("failed to get listing", "error", err, "listing_id", listingID)
		return nil
	}

	text := s.formatListingDetails(&listing)

	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("✅ Buy Now", formatCallbackData(CallbackConfirmBuy, listing.ID)),
				tgbotapi.NewInlineKeyboardButtonData("← Back", "back_to_listings"),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("📞 Support", formatCallbackData(CallbackContactSupport, listing.ID)),
			},
		},
	}

	_, err = s.botAPI.Send(edit)
	if err != nil {
		s.logger.Error("failed to edit message", "error", err)
	}
	return nil
}

// handleViewDetails shows full coupon details
func (s *BotService) handleViewDetails(ctx context.Context, chatID int64, listingID int64, userID int64) error {
	listing, err := s.store.GetListing(ctx, listingID)
	if err != nil {
		s.logger.Error("failed to get listing", "error", err, "listing_id", listingID)
		return s.sendMessage(ctx, chatID, "Could not load coupon details.")
	}

	text := s.formatListingDetails(&listing)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("✅ Buy Now", formatCallbackData(CallbackConfirmBuy, listing.ID)),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("📞 Support", formatCallbackData(CallbackContactSupport, listing.ID)),
			},
		},
	}

	_, err = s.botAPI.Send(msg)
	return err
}

// handleConfirmBuy creates draft order and initiates checkout
func (s *BotService) handleConfirmBuy(ctx context.Context, callback *tgbotapi.CallbackQuery, listingID int64) error {
	// Get user
	user, err := s.getOrCreateUser(ctx, callback.From)
	if err != nil {
		s.logger.Error("failed to get user", "error", err, "telegram_user_id", callback.From.ID)
		return nil
	}

	// Create draft order
	orderNumber := fmt.Sprintf("ORD-%d-%d", callback.From.ID, time.Now().Unix())
	params := store.CreateDraftOrderParams{
		UserID:                  user.ID,
		ListingID:               listingID,
		OrderNumber:             orderNumber,
		FinalSaleAcknowledgedAt: time.Now(),
	}

	order, err := s.store.CreateDraftOrder(ctx, params)
	if err != nil {
		s.logger.Error("failed to create draft order", "error", err, "user_id", user.ID, "listing_id", listingID)
		msg := "Unable to process order. Please try again."
		if err == store.ErrListingSoldOut {
			msg = "This coupon is sold out. Please choose another."
		}
		return s.sendMessage(ctx, callback.Message.Chat.ID, msg)
	}

	// TODO: Initiate payment checkout with payment provider
	// For now, send checkout link message (placeholder)
	checkoutText := fmt.Sprintf(`✅ Order created: <b>#%s</b>

<b>Next step:</b> Complete payment to get your coupon code.

<i>Payment link would be sent here.</i>`, order.OrderNumber)

	return s.sendMessage(ctx, callback.Message.Chat.ID, checkoutText)
}

// Helper functions

// getOrCreateUser retrieves or creates a user from Telegram data
func (s *BotService) getOrCreateUser(ctx context.Context, from *tgbotapi.User) (*store.User, error) {
	displayName := from.FirstName
	if from.LastName != "" {
		displayName += " " + from.LastName
	}

	user, err := s.store.GetOrCreateUser(ctx, int64(from.ID), displayName, from.UserName, from.LanguageCode)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// formatListingsForDisplay creates a formatted display of multiple listings
func (s *BotService) formatListingsForDisplay(listings []store.Listing) string {
	text := ""
	for i, listing := range listings {
		priceILS := formatPrice(listing.SalePriceAmount)
		expiryText := "Soon"
		if listing.NextCouponExpiryAt != nil {
			expiryText = listing.NextCouponExpiryAt.Format("Jan 2")
		}

		text += fmt.Sprintf(`<b>#%d %s</b>
Coupon: %s
Price: %s
Expires: %s

`, i+1, listing.MerchantName,
			truncate(listing.Title, 30),
			priceILS,
			expiryText)
	}
	return text
}

// formatListingDetails creates detailed coupon information
func (s *BotService) formatListingDetails(listing *store.Listing) string {
	priceILS := formatPrice(listing.SalePriceAmount)
	originalValue := formatPrice(listing.CouponValueAmount)

	expiryText := "Unknown"
	if listing.NextCouponExpiryAt != nil {
		expiryText = listing.NextCouponExpiryAt.Format("2 January 2006")
	}

	text := fmt.Sprintf(`<b>%s - %s</b>

<b>What you get:</b>
%s

<b>Details:</b>
• <b>Coupon Value:</b> %s
• <b>Your Price:</b> %s (You save!)
• <b>Expires:</b> %s
• <b>Quantity Available:</b> %d

<b>How to redeem:</b>
%s

<b>⚠️ Important:</b>
%s

All sales final. No refunds. Please read all terms before purchasing.`,
		listing.MerchantName,
		listing.Title,
		listing.Description,
		originalValue,
		priceILS,
		expiryText,
		listing.AvailableInventoryCount,
		listing.RedemptionInstructions,
		listing.FinalSaleDisclosureText,
	)

	return text
}

// formatPrice converts cents to formatted price string
func formatPrice(cents int64) string {
	shekelAmount := float64(cents) / 100.0
	return fmt.Sprintf("₪%.2f", shekelAmount)
}

// truncate shortens string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// createListingsKeyboard creates inline keyboard for listings
func (s *BotService) createListingsKeyboard(userID int64, listings []store.Listing) *tgbotapi.InlineKeyboardMarkup {
	keyboard := make([][]tgbotapi.InlineKeyboardButton, len(listings))

	for i, listing := range listings {
		keyboard[i] = []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Buy", formatCallbackData(CallbackBuyListing, listing.ID)),
			tgbotapi.NewInlineKeyboardButtonData("Details", formatCallbackData(CallbackViewDetails, listing.ID)),
		}
	}

	// Add support button at the end
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("📞 Support", formatCallbackData(CallbackContactSupport, 0)),
	})

	return &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}
}

// Utility functions for callback data encoding

func formatCallbackData(action string, id int64) string {
	return fmt.Sprintf("%s:%d", action, id)
}

func parseCallbackData(data string) []string {
	// Simple split on colon: "action:id" → ["action", "id"]
	parts := make([]string, 0)
	var current string
	for _, ch := range data {
		if ch == ':' && current != "" {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// sendMessage is a helper to send text messages
func (s *BotService) sendMessage(ctx context.Context, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := s.botAPI.Send(msg)
	return err
}
