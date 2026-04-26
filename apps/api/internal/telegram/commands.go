package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/payments"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackBuyListing     = "buy_listing"
	CallbackAnotherDeal    = "another_deal"
	CallbackConfirmBuy     = "confirm_buy"
	CallbackViewDetails    = "view_details"
	CallbackContactSupport = "contact_support"
)

type BotService struct {
	logger          *slog.Logger
	store           *store.Postgres
	botAPI          *tgbotapi.BotAPI
	config          *config.Config
	checkoutStarter CheckoutStarter
}

type CheckoutStarter interface {
	StartCheckout(ctx context.Context, order store.Order, listing store.Listing) (payments.CheckoutLink, error)
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

func (s *BotService) SetCheckoutStarter(checkoutStarter CheckoutStarter) {
	s.checkoutStarter = checkoutStarter
}

func (s *BotService) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message != nil && update.Message.IsCommand() {
		return s.handleCommand(ctx, update.Message)
	}

	if update.CallbackQuery != nil {
		return s.handleCallback(ctx, update.CallbackQuery)
	}

	return nil
}

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

func (s *BotService) handleStart(ctx context.Context, message *tgbotapi.Message) error {
	_, err := s.getOrCreateUser(ctx, message.From)
	if err != nil {
		s.logger.Error("failed to get or create user", "error", err, "user_id", message.From.ID)
		return s.sendMessage(ctx, message.Chat.ID, "Something went wrong. Please try again.")
	}

	listings, err := s.loadStartListings(ctx)
	if err != nil {
		s.logger.Error("failed to fetch start listings", "error", err)
		return s.sendMessage(ctx, message.Chat.ID, "Unable to load coupons. Please try again later.")
	}

	if len(listings) == 0 {
		return s.sendMessage(ctx, message.Chat.ID, "No coupons available at the moment. Check back soon!")
	}

	return s.sendFeaturedListing(ctx, message.Chat.ID, listings, 0)
}

func (s *BotService) handleHelp(ctx context.Context, message *tgbotapi.Message) error {
	helpText := `<b>How to use CouponTelBot:</b>

/start - See available coupons`
	if s.isDevelopmentMode() {
		helpText += `
(development also shows preview catalog items without inventory)`
	}

	helpText += `
/help - Show this message

<b>How to buy:</b>
1. Type /start to see available coupons
2. Press the coupon button on a deal
3. Review the coupon details
4. Confirm your purchase
5. Complete payment
6. Get your coupon code

<b>Need help?</b>
Use the support button on any coupon detail screen.`

	return s.sendMessage(ctx, message.Chat.ID, helpText)
}

func (s *BotService) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	answer := tgbotapi.NewCallback(callback.ID, "")
	s.botAPI.Request(answer)

	parts := parseCallbackData(callback.Data)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case CallbackBuyListing:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleBuyListing(ctx, callback, listingID)
	case CallbackAnotherDeal:
		if len(parts) < 2 {
			return nil
		}
		currentListingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleAnotherDeal(ctx, callback.Message.Chat.ID, currentListingID)
	case CallbackViewDetails:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleViewDetails(ctx, callback.Message.Chat.ID, listingID)
	case CallbackConfirmBuy:
		if len(parts) < 2 {
			return nil
		}
		listingID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleConfirmBuy(ctx, callback, listingID)
	case CallbackContactSupport:
		return s.handleContactSupport(ctx, callback.Message.Chat.ID)
	default:
		return nil
	}
}

func (s *BotService) handleBuyListing(ctx context.Context, callback *tgbotapi.CallbackQuery, listingID int64) error {
	listing, err := s.store.GetListing(ctx, listingID)
	if err != nil {
		s.logger.Error("failed to get listing", "error", err, "listing_id", listingID)
		return nil
	}

	return s.sendListingDetails(ctx, callback.Message.Chat.ID, listing)
}

func (s *BotService) handleViewDetails(ctx context.Context, chatID int64, listingID int64) error {
	listing, err := s.store.GetListing(ctx, listingID)
	if err != nil {
		s.logger.Error("failed to get listing", "error", err, "listing_id", listingID)
		return s.sendMessage(ctx, chatID, "Could not load coupon details.")
	}

	return s.sendListingDetails(ctx, chatID, listing)
}

func (s *BotService) handleAnotherDeal(ctx context.Context, chatID int64, currentListingID int64) error {
	listings, err := s.loadStartListings(ctx)
	if err != nil {
		s.logger.Error("failed to fetch listings for another deal", "error", err)
		return s.sendMessage(ctx, chatID, "Unable to load another deal right now. Please try again later.")
	}
	if len(listings) == 0 {
		return s.sendMessage(ctx, chatID, "No coupons available at the moment. Check back soon!")
	}

	nextIndex := 0
	for i, listing := range listings {
		if listing.ID == currentListingID {
			nextIndex = (i + 1) % len(listings)
			break
		}
	}

	return s.sendFeaturedListing(ctx, chatID, listings, nextIndex)
}

func (s *BotService) handleConfirmBuy(ctx context.Context, callback *tgbotapi.CallbackQuery, listingID int64) error {
	if s.checkoutStarter == nil {
		s.logger.Error("checkout starter is not configured")
		return s.sendMessage(ctx, callback.Message.Chat.ID, "Payment is temporarily unavailable. Please try again later.")
	}

	user, err := s.getOrCreateUser(ctx, callback.From)
	if err != nil {
		s.logger.Error("failed to get user", "error", err, "telegram_user_id", callback.From.ID)
		return nil
	}

	listing, err := s.store.GetListing(ctx, listingID)
	if err != nil {
		s.logger.Error("failed to get listing for checkout", "error", err, "listing_id", listingID)
		return s.sendMessage(ctx, callback.Message.Chat.ID, "Unable to load this coupon right now. Please try again.")
	}

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

	checkout, err := s.checkoutStarter.StartCheckout(ctx, order, listing)
	if err != nil {
		s.logger.Error("failed to start paypal checkout", "error", err, "order_id", order.ID, "listing_id", listingID)
		return s.sendMessage(ctx, callback.Message.Chat.ID, "We could not start the PayPal checkout. Please try again in a moment.")
	}

	checkoutText := fmt.Sprintf(`<b>Checkout ready:</b> %s

<b>Coupon:</b> %s - %s
<b>Amount:</b> %s

This coupon is reserved for %s while you complete payment.

Tap the PayPal button below to complete payment. We will deliver the coupon in Telegram after PayPal confirms the payment.`,
		order.OrderNumber,
		listing.MerchantName,
		listing.Title,
		formatPrice(order.SalePriceAmount),
		formatHoldDuration(s.config.OpsCheckoutHoldDuration),
	)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, normalizeTelegramText(checkoutText))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Pay with PayPal", checkout.ApprovalURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Support", formatCallbackData(CallbackContactSupport, listing.ID)),
		),
	)

	_, err = s.botAPI.Send(msg)
	return err
}

func (s *BotService) handleContactSupport(ctx context.Context, chatID int64) error {
	return s.sendMessage(ctx, chatID, "Support is available here. Send your order number and what went wrong, and we will review it manually.")
}

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

func (s *BotService) formatListingDetails(listing *store.Listing) string {
	priceILS := formatPrice(listing.SalePriceAmount)
	originalValue := formatPrice(listing.CouponValueAmount)

	expiryText := "Unknown"
	if listing.NextCouponExpiryAt != nil {
		expiryText = listing.NextCouponExpiryAt.Format("2 January 2006")
	}

	availabilityText := fmt.Sprintf("%d", listing.AvailableInventoryCount)
	purchaseNotice := "Tap Continue to Payment to reserve this coupon for checkout."
	if listing.AvailableInventoryCount == 0 {
		if s.isDevelopmentMode() {
			availabilityText = "Preview only"
			purchaseNotice = "This catalog item is visible in development preview mode. Add coupon inventory before it can be purchased."
		} else {
			availabilityText = "Sold out"
			purchaseNotice = "This coupon is currently sold out. You can still review the details or contact support."
		}
	}

	return fmt.Sprintf(`<b>%s - %s</b>

<b>What you get:</b>
%s

<b>Details:</b>
- <b>Coupon Value:</b> %s
- <b>Your Price:</b> %s
- <b>Expires:</b> %s
- <b>Quantity Available:</b> %s

<b>How to redeem:</b>
%s

<b>Important:</b>
%s

%s

All sales final. No refunds. Please read all terms before purchasing.`,
		html.EscapeString(listing.MerchantName),
		html.EscapeString(listing.Title),
		html.EscapeString(listing.Description),
		originalValue,
		priceILS,
		expiryText,
		availabilityText,
		html.EscapeString(listing.RedemptionInstructions),
		html.EscapeString(listing.FinalSaleDisclosureText),
		purchaseNotice,
	)
}

func formatPrice(cents int64) string {
	shekelAmount := float64(cents) / 100.0
	var formatted string
	if cents%100 == 0 {
		formatted = fmt.Sprintf("%.1f", shekelAmount)
	} else if cents%10 == 0 {
		formatted = fmt.Sprintf("%.1f", shekelAmount)
	} else {
		formatted = fmt.Sprintf("%.2f", shekelAmount)
	}

	// Isolate the amount as an LTR run so it keeps the expected visual order
	// inside surrounding Hebrew/RTL text.
	return "\u2066" + formatted + "\u00a0\u20aa" + "\u2069"
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func (s *BotService) listingOfferKeyboard(listingID int64, availableInventoryCount int64, allowAnotherDeal bool) *tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	firstRow := make([]tgbotapi.InlineKeyboardButton, 0, 2)

	if availableInventoryCount > 0 {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05e7\u05d1\u05dc \u05e7\u05d5\u05e4\u05d5\u05df", formatCallbackData(CallbackBuyListing, listingID)))
	}
	if allowAnotherDeal {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05d3\u05d9\u05dc \u05d0\u05d7\u05e8", formatCallbackData(CallbackAnotherDeal, listingID)))
	}
	if len(firstRow) == 0 {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\u05e8\u05e7 \u05dc\u05d4\u05e6\u05d9\u05e5", formatCallbackData(CallbackViewDetails, listingID)))
	}

	rows = append(rows, firstRow)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("\u05e4\u05e8\u05d8\u05d9\u05dd \u05de\u05dc\u05d0\u05d9\u05dd", formatCallbackData(CallbackViewDetails, listingID)),
	))
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (s *BotService) listingDetailKeyboard(listingID int64, availableInventoryCount int64) *tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	firstRow := make([]tgbotapi.InlineKeyboardButton, 0, 2)

	if availableInventoryCount > 0 {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05d4\u05de\u05e9\u05da \u05dc\u05ea\u05e9\u05dc\u05d5\u05dd", formatCallbackData(CallbackConfirmBuy, listingID)))
	}
	firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05d3\u05d9\u05dc \u05d0\u05d7\u05e8", formatCallbackData(CallbackAnotherDeal, listingID)))
	rows = append(rows, firstRow)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("\u05ea\u05de\u05d9\u05db\u05d4", formatCallbackData(CallbackContactSupport, listingID)),
	})

	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatHoldDuration(value time.Duration) string {
	if value%time.Minute == 0 {
		minutes := int(value / time.Minute)
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	return value.String()
}

func formatCallbackData(action string, id int64) string {
	return fmt.Sprintf("%s:%d", action, id)
}

func parseCallbackData(data string) []string {
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

func (s *BotService) sendMessage(ctx context.Context, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, normalizeTelegramText(text))
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := s.botAPI.Send(msg)
	return err
}

func (s *BotService) sendFeaturedListing(ctx context.Context, chatID int64, listings []store.Listing, index int) error {
	if len(listings) == 0 {
		return s.sendMessage(ctx, chatID, "No coupons available at the moment. Check back soon!")
	}
	if index < 0 || index >= len(listings) {
		index = 0
	}

	listing := listings[index]
	caption := normalizeTelegramText(s.formatFeaturedListingCaption(&listing))
	keyboard := s.listingOfferKeyboard(listing.ID, listing.AvailableInventoryCount, len(listings) > 1)

	if photoPath, ok := localListingPhotoPath(listing.PhotoKey); ok {
		msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(photoPath))
		msg.Caption = caption
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err := s.botAPI.Send(msg)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, caption)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	_, err := s.botAPI.Send(msg)
	return err
}

func (s *BotService) sendListingDetails(ctx context.Context, chatID int64, listing store.Listing) error {
	if photoPath, ok := localListingPhotoPath(listing.PhotoKey); ok {
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(photoPath))
		photo.Caption = normalizeTelegramText(s.formatListingPhotoCaption(&listing))
		photo.ParseMode = tgbotapi.ModeHTML
		if _, err := s.botAPI.Send(photo); err != nil {
			s.logger.Error("failed to send listing detail photo", "error", err, "listing_id", listing.ID)
		}
	}

	msg := tgbotapi.NewMessage(chatID, normalizeTelegramText(s.formatListingDetails(&listing)))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = s.listingDetailKeyboard(listing.ID, listing.AvailableInventoryCount)

	_, err := s.botAPI.Send(msg)
	if err != nil {
		s.logger.Error("failed to send listing detail message", "error", err, "listing_id", listing.ID)
	}
	return err
}

func (s *BotService) formatFeaturedListingCaption(listing *store.Listing) string {
	availability := fmt.Sprintf("%d \u05e7\u05d5\u05e4\u05d5\u05e0\u05d9\u05dd", listing.AvailableInventoryCount)
	if listing.AvailableInventoryCount == 0 {
		if s.isDevelopmentMode() {
			availability = "\u05ea\u05e6\u05d5\u05d2\u05d4 \u05d1\u05dc\u05d1\u05d3"
		} else {
			availability = "\u05d0\u05d6\u05dc \u05d4\u05de\u05dc\u05d0\u05d9"
		}
	}

	offerName := strings.TrimSpace(listing.Title)
	if offerName == "" {
		offerName = strings.TrimSpace(listing.MerchantName)
	}

	return fmt.Sprintf("<b>\U0001F525 \u05d3\u05d9\u05dc \u05d7\u05dd \u05e2\u05db\u05e9\u05d9\u05d5!</b>\n\n"+
		"\U0001F39F\uFE0F <b>%s</b>\n"+
		"%s\n\n"+
		"\U0001F48E \u05d1\u05de\u05e7\u05d5\u05dd: %s\n"+
		"\U0001F4A5 \u05e2\u05db\u05e9\u05d9\u05d5: %s\n"+
		"\u23F3 \u05e0\u05e9\u05d0\u05e8\u05d5: %s",
		html.EscapeString(listing.MerchantName),
		html.EscapeString(truncate(offerName, 40)),
		formatPrice(listing.CouponValueAmount),
		formatPrice(listing.SalePriceAmount),
		html.EscapeString(availability),
	)
}

func (s *BotService) formatListingPhotoCaption(listing *store.Listing) string {
	return fmt.Sprintf("<b>%s</b>\n%s", html.EscapeString(listing.MerchantName), html.EscapeString(truncate(listing.Title, 80)))
}

func (s *BotService) loadStartListings(ctx context.Context) ([]store.Listing, error) {
	if !s.isDevelopmentMode() {
		listings, err := s.store.ListActiveListings(ctx)
		if err != nil {
			return nil, err
		}
		return filterBrowsableListings(listings, false), nil
	}

	summaries, err := s.store.ListListingsForAdmin(ctx)
	if err != nil {
		return nil, err
	}

	listings := make([]store.Listing, 0, len(summaries))
	for _, summary := range summaries {
		listings = append(listings, summary.Listing)
	}

	return filterBrowsableListings(listings, true), nil
}

func (s *BotService) isDevelopmentMode() bool {
	return s != nil && s.config != nil && strings.EqualFold(strings.TrimSpace(s.config.AppEnv), "development")
}

func (s *BotService) SendHTMLMessage(ctx context.Context, telegramUserID int64, text string) (int64, error) {
	msg := tgbotapi.NewMessage(telegramUserID, normalizeTelegramText(text))
	msg.ParseMode = tgbotapi.ModeHTML
	sent, err := s.botAPI.Send(msg)
	if err != nil {
		return 0, err
	}
	return int64(sent.MessageID), nil
}

func normalizeTelegramText(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	return strings.ToValidUTF8(text, "")
}

func localListingPhotoPath(photoKey string) (string, bool) {
	photoKey = strings.TrimSpace(photoKey)
	if photoKey == "" || strings.Contains(photoKey, "/") || strings.Contains(photoKey, `\`) {
		return "", false
	}

	path := filepath.Join("data", "photos", photoKey)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}

	return path, true
}

func filterBrowsableListings(listings []store.Listing, isDevelopment bool) []store.Listing {
	filtered := make([]store.Listing, 0, len(listings))
	for _, listing := range listings {
		if listing.CouponValueAmount <= listing.SalePriceAmount {
			continue
		}
		if isDevelopment {
			if !strings.EqualFold(listing.Status, "draft") &&
				!strings.EqualFold(listing.Status, "active") &&
				!strings.EqualFold(listing.Status, "preview") {
				continue
			}
		} else if !strings.EqualFold(listing.Status, "active") {
			continue
		}
		filtered = append(filtered, listing)
	}
	return filtered
}
