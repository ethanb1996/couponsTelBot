package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/services"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackBuyListing     = "buy_listing"
	CallbackAnotherDeal    = "another_deal"
	CallbackConfirmBuy     = "confirm_buy"
	CallbackViewDetails    = "view_details"
	CallbackContactSupport = "contact_support"
	CallbackBuyOffer       = "buy_offer"
	CallbackAnotherOffer   = "another_offer"
	CallbackViewOffer      = "view_offer"
)

type BotService struct {
	logger        *slog.Logger
	store         *store.Postgres
	botAPI        *tgbotapi.BotAPI
	config        *config.Config
	payBox        PayBoxFlow
	pendingClaims *pendingPayBoxClaims
}

type PayBoxFlow interface {
	ListActiveOffers(ctx context.Context, limit int) ([]store.Offer, error)
	GetOffer(ctx context.Context, offerID int64) (store.Offer, error)
	StartPayment(ctx context.Context, userID int64, offerID int64) (services.PayBoxPaymentStart, error)
	SubmitPaymentClaim(ctx context.Context, orderID int64, evidence services.PayBoxPaymentEvidence, claimedAmount int64) (store.ManualPaymentClaim, error)
}

type ExpiredCheckoutHoldNotifier interface {
	NotifyExpiredCheckoutHold(ctx context.Context, hold store.ReleasedCheckoutHold) error
}

func NewBotService(logger *slog.Logger, repo *store.Postgres, botToken string, cfg *config.Config) (*BotService, error) {
	api, err := tgbotapi.NewBotAPIWithClient(botToken, tgbotapi.APIEndpoint, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		return nil, err
	}
	return &BotService{
		logger:        logger,
		store:         repo,
		botAPI:        api,
		config:        cfg,
		pendingClaims: newPendingPayBoxClaims(),
	}, nil
}

func (s *BotService) SetPayBoxFlow(payBox PayBoxFlow) {
	s.payBox = payBox
	if s.pendingClaims == nil {
		s.pendingClaims = newPendingPayBoxClaims()
	}
}

func (s *BotService) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message != nil && update.Message.IsCommand() {
		return s.handleCommand(ctx, update.Message)
	}

	if update.Message != nil {
		return s.handleMessage(ctx, update.Message)
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

func (s *BotService) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	if message == nil || message.Chat == nil || message.From == nil || s.pendingClaims == nil || s.payBox == nil {
		return nil
	}

	pending, ok := s.pendingClaims.get(message.Chat.ID)
	if !ok {
		return nil
	}

	evidence, ok := paymentEvidenceFromMessage(message)
	if !ok {
		return s.sendMessage(ctx, message.Chat.ID, "Please upload the PayBox payment screenshot as a photo or image document.")
	}
	evidence.PayerReference = fmt.Sprintf("telegram:%d", message.From.ID)

	claim, err := s.payBox.SubmitPaymentClaim(ctx, pending.orderID, evidence, pending.claimedAmount)
	if err != nil {
		s.logger.Error("failed to submit paybox screenshot claim", "error", err, "order_id", pending.orderID)
		return s.sendMessage(ctx, message.Chat.ID, "Could not submit the screenshot for review. Please try again.")
	}

	s.pendingClaims.delete(message.Chat.ID)
	return s.sendMessage(ctx, message.Chat.ID, fmt.Sprintf("Payment screenshot received. Claim #%d is waiting for admin approval.", claim.ID))
}

func (s *BotService) handleStart(ctx context.Context, message *tgbotapi.Message) error {
	_, err := s.getOrCreateUser(ctx, message.From)
	if err != nil {
		s.logger.Error("failed to get or create user", "error", err, "user_id", message.From.ID)
		return s.sendMessage(ctx, message.Chat.ID, "Something went wrong. Please try again.")
	}

	if s.payBox != nil {
		return s.sendStartOffers(ctx, message.Chat.ID)
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
3. Pay through the PayBox link
4. Upload the payment screenshot here
5. Wait for admin approval
6. Show the QR code to the merchant

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
	case CallbackBuyOffer:
		if len(parts) < 2 {
			return nil
		}
		offerID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleBuyOffer(ctx, callback, offerID)
	case CallbackAnotherOffer:
		if len(parts) < 2 {
			return nil
		}
		currentOfferID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleAnotherOffer(ctx, callback.Message.Chat.ID, currentOfferID)
	case CallbackViewOffer:
		if len(parts) < 2 {
			return nil
		}
		offerID, _ := strconv.ParseInt(parts[1], 10, 64)
		return s.handleViewOffer(ctx, callback.Message.Chat.ID, offerID)
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

func (s *BotService) handleBuyOffer(ctx context.Context, callback *tgbotapi.CallbackQuery, offerID int64) error {
	if s.payBox == nil {
		return s.sendMessage(ctx, callback.Message.Chat.ID, "Payment is temporarily unavailable. Please try again later.")
	}

	user, err := s.getOrCreateUser(ctx, callback.From)
	if err != nil {
		s.logger.Error("failed to get paybox user", "error", err, "telegram_user_id", callback.From.ID)
		return s.sendMessage(ctx, callback.Message.Chat.ID, "Something went wrong. Please try again.")
	}

	started, err := s.payBox.StartPayment(ctx, user.ID, offerID)
	if err != nil {
		s.logger.Error("failed to start paybox payment", "error", err, "user_id", user.ID, "offer_id", offerID)
		return s.sendMessage(ctx, callback.Message.Chat.ID, "Unable to start PayBox payment for this offer right now.")
	}

	if s.pendingClaims == nil {
		s.pendingClaims = newPendingPayBoxClaims()
	}
	s.pendingClaims.set(callback.Message.Chat.ID, pendingPayBoxClaim{
		orderID:       started.OrderID,
		claimedAmount: started.PriceAmount,
	})

	return s.sendPayBoxPaymentInstructions(ctx, callback.Message.Chat.ID, started)
}

func (s *BotService) handleViewOffer(ctx context.Context, chatID int64, offerID int64) error {
	if s.payBox == nil {
		return s.sendMessage(ctx, chatID, "Offers are temporarily unavailable. Please try again later.")
	}

	offer, err := s.payBox.GetOffer(ctx, offerID)
	if err != nil {
		s.logger.Error("failed to get paybox offer", "error", err, "offer_id", offerID)
		return s.sendMessage(ctx, chatID, "Could not load offer details.")
	}

	return s.sendOfferDetails(ctx, chatID, offer)
}

func (s *BotService) handleAnotherOffer(ctx context.Context, chatID int64, currentOfferID int64) error {
	offers, err := s.loadStartOffers(ctx)
	if err != nil {
		s.logger.Error("failed to fetch paybox offers for another deal", "error", err)
		return s.sendMessage(ctx, chatID, "Unable to load another deal right now. Please try again later.")
	}
	if len(offers) == 0 {
		return s.sendMessage(ctx, chatID, "No coupons available at the moment. Check back soon!")
	}

	nextIndex := 0
	for i, offer := range offers {
		if offer.ID == currentOfferID {
			nextIndex = (i + 1) % len(offers)
			break
		}
	}

	return s.sendFeaturedOffer(ctx, chatID, offers, nextIndex)
}

func (s *BotService) handleBuyListing(ctx context.Context, callback *tgbotapi.CallbackQuery, _ int64) error {
	return s.handleLegacyListingBuy(ctx, callback.Message.Chat.ID)
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

func (s *BotService) handleConfirmBuy(ctx context.Context, callback *tgbotapi.CallbackQuery, _ int64) error {
	return s.handleLegacyListingBuy(ctx, callback.Message.Chat.ID)
}

func (s *BotService) handleLegacyListingBuy(ctx context.Context, chatID int64) error {
	if s.payBox != nil {
		return s.sendStartOffers(ctx, chatID)
	}
	return s.sendMessage(ctx, chatID, "This checkout flow is no longer available. Send /start to see the current catalog.")
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
	brandName := strings.TrimSpace(listing.MerchantName)
	if brandName == "" {
		brandName = "הדיל שלך"
	}

	lines := []string{
		fmt.Sprintf("%s <b>%s</b>", listingEmoji(listing), html.EscapeString(brandName)),
		html.EscapeString(listingOfferLine(listing)),
		fmt.Sprintf("במקום %s ← <b>רק %s</b>", formatPrice(listing.CouponValueAmount), formatPrice(store.EffectiveListingPriceAmount(*listing))),
		html.EscapeString(s.listingUrgencyLine(listing)),
	}

	if reassurance := listingReassuranceLine(listing); reassurance != "" {
		lines = append(lines, html.EscapeString(reassurance))
	}

	return strings.Join(lines, "\n")
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

func listingEmoji(listing *store.Listing) string {
	text := strings.ToLower(strings.Join([]string{
		listing.MerchantName,
		listing.Title,
		listing.Description,
	}, " "))

	for _, candidate := range []struct {
		emoji    string
		keywords []string
	}{
		{emoji: "🍔", keywords: []string{"burger", "hamburger", "אגאדיר", "בורגר", "המבורגר"}},
		{emoji: "🍕", keywords: []string{"pizza", "פיצה"}},
		{emoji: "🍣", keywords: []string{"sushi", "סושי", "poke", "פוקי", "ווק", "אסייתי", "ramen"}},
		{emoji: "☕", keywords: []string{"coffee", "cafe", "קפה", "אספרסו"}},
		{emoji: "🥐", keywords: []string{"breakfast", "בוקר", "ארוחת בוקר", "מאפה"}},
		{emoji: "🍰", keywords: []string{"dessert", "קינוח", "גלידה", "וופל", "cake"}},
		{emoji: "🥪", keywords: []string{"sandwich", "כריך", "טוסט", "bagel", "בייגל"}},
		{emoji: "🥩", keywords: []string{"steak", "grill", "בשר", "סטייק", "גריל", "שווארמה"}},
		{emoji: "🍽️", keywords: []string{"restaurant", "meal", "מסעדה", "ארוחה"}},
	} {
		for _, keyword := range candidate.keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				return candidate.emoji
			}
		}
	}

	return "🎟️"
}

func listingOfferLine(listing *store.Listing) string {
	for _, candidate := range []string{listing.Title, listing.Description} {
		if cleaned := cleanListingOfferText(candidate, listing.MerchantName); cleaned != "" {
			return truncate(cleaned, 34)
		}
	}

	return fmt.Sprintf("שובר בשווי %s", formatPrice(listing.CouponValueAmount))
}

func cleanListingOfferText(value, merchantName string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return ""
	}

	for _, separator := range []string{"\r", "\n", "|", "•", "·", ";", "!", "?", "—"} {
		cleaned = strings.ReplaceAll(cleaned, separator, ".")
	}

	for _, segment := range strings.Split(cleaned, ".") {
		normalized := collapseSpaces(stripLatinText(strings.ReplaceAll(strings.TrimSpace(segment), merchantName, "")))
		if normalized == "" || listingOfferSegmentIsNoise(normalized) || !containsHebrew(normalized) {
			continue
		}
		return normalized
	}

	return ""
}

func stripLatinText(value string) string {
	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

func collapseSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func containsHebrew(value string) bool {
	for _, r := range value {
		if r >= 0x0590 && r <= 0x05FF {
			return true
		}
	}

	return false
}

func listingOfferSegmentIsNoise(value string) bool {
	lower := strings.ToLower(value)
	for _, keyword := range []string{
		"תקנון",
		"תנאים",
		"בכפוף",
		"טלפון",
		"אתר",
		"יצירת קשר",
		"פיתוח",
		"פריוויו",
		"לצפייה בלבד",
		"לא כולל",
		"ללא כפל",
		"http",
		"www",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return false
}

func (s *BotService) listingUrgencyLine(listing *store.Listing) string {
	if !canStartCheckout(listing.Status, listing.AvailableInventoryCount) {
		if listing.AvailableInventoryCount == 0 {
			return "⏳ אזל כרגע, שווה לבדוק דיל נוסף"
		}
		return "⏳ כרגע לא זמין לרכישה"
	}

	if listing.NextCouponExpiryAt != nil {
		now := time.Now()
		expiry := listing.NextCouponExpiryAt.In(now.Location())
		if sameDay(now, expiry) {
			return "⏰ תקף להיום בלבד"
		}
		if expiry.Before(now.Add(48 * time.Hour)) {
			return "⏰ תקף עד מחר, לא לפספס"
		}
	}

	switch {
	case listing.AvailableInventoryCount <= 2:
		return "🔥 נשאר מעט, כדאי למהר"
	case listing.AvailableInventoryCount <= 5:
		return "⚡ נחטף מהר היום"
	default:
		return "🔥 לזמן מוגבל"
	}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func listingReassuranceLine(listing *store.Listing) string {
	if !canStartCheckout(listing.Status, listing.AvailableInventoryCount) {
		return ""
	}

	switch listingEmoji(listing) {
	case "🍔", "🍕", "🍣", "☕", "🥐", "🍰", "🥪", "🥩", "🍽️":
		return "טעים, משתלם ופופולרי"
	default:
		return "שווה לנצל עכשיו"
	}
}

func (s *BotService) listingOfferKeyboard(listingID int64, listingStatus string, availableInventoryCount int64, allowAnotherDeal bool) *tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	firstRow := make([]tgbotapi.InlineKeyboardButton, 0, 2)

	if canStartCheckout(listingStatus, availableInventoryCount) {
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
	return s.listingDetailKeyboardWithStatus(listingID, "active", availableInventoryCount)
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

func formatCallbackData(action string, ids ...int64) string {
	parts := make([]string, 0, len(ids)+1)
	parts = append(parts, action)
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ":")
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

func (s *BotService) sendStartOffers(ctx context.Context, chatID int64) error {
	offers, err := s.loadStartOffers(ctx)
	if err != nil {
		s.logger.Error("failed to fetch paybox offers", "error", err)
		return s.sendMessage(ctx, chatID, "Unable to load coupons. Please try again later.")
	}
	if len(offers) == 0 {
		return s.sendMessage(ctx, chatID, "No coupons available at the moment. Check back soon!")
	}

	return s.sendFeaturedOffer(ctx, chatID, offers, 0)
}

func (s *BotService) loadStartOffers(ctx context.Context) ([]store.Offer, error) {
	if s.payBox == nil {
		return nil, nil
	}
	return s.payBox.ListActiveOffers(ctx, 5)
}

func (s *BotService) sendFeaturedOffer(ctx context.Context, chatID int64, offers []store.Offer, index int) error {
	if len(offers) == 0 {
		return s.sendMessage(ctx, chatID, "No coupons available at the moment. Check back soon!")
	}
	if index < 0 || index >= len(offers) {
		index = 0
	}

	offer := offers[index]
	msg := tgbotapi.NewMessage(chatID, normalizeTelegramText(formatOfferSummary(&offer)))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = offerKeyboard(offer.ID, offer.AvailableCodeCount, len(offers) > 1)
	_, err := s.botAPI.Send(msg)
	return err
}

func (s *BotService) sendOfferDetails(ctx context.Context, chatID int64, offer store.Offer) error {
	msg := tgbotapi.NewMessage(chatID, normalizeTelegramText(formatOfferDetails(&offer)))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = offerDetailKeyboard(offer.ID, offer.AvailableCodeCount)
	_, err := s.botAPI.Send(msg)
	return err
}

func (s *BotService) sendPayBoxPaymentInstructions(ctx context.Context, chatID int64, started services.PayBoxPaymentStart) error {
	text := fmt.Sprintf(`<b>PayBox payment ready</b>

<b>Order:</b> %s
<b>Coupon:</b> %s - %s
<b>Amount:</b> %s

Pay with the button below. After payment, upload the PayBox screenshot in this chat.`,
		html.EscapeString(started.OrderNumber),
		html.EscapeString(started.MerchantName),
		html.EscapeString(started.OfferTitle),
		formatPrice(started.PriceAmount),
	)

	if strings.TrimSpace(started.MerchantDisclosureText) != "" {
		text += "\n\n" + html.EscapeString(started.MerchantDisclosureText)
	}
	if strings.TrimSpace(started.NextStepMessage) != "" {
		text += "\n\n" + html.EscapeString(started.NextStepMessage)
	}

	msg := tgbotapi.NewMessage(chatID, normalizeTelegramText(text))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Pay in PayBox", started.PaymentLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Another deal", formatCallbackData(CallbackAnotherOffer, started.OfferID)),
		),
	)

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
	s.logger.Debug("building featured listing keyboard",
		"listing_id", listing.ID,
		"listing_status", listing.Status,
		"available_inventory_count", listing.AvailableInventoryCount,
	)
	keyboard := s.listingOfferKeyboard(listing.ID, listing.Status, listing.AvailableInventoryCount, len(listings) > 1)

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
	s.logger.Debug("building listing detail keyboard",
		"listing_id", listing.ID,
		"listing_status", listing.Status,
		"available_inventory_count", listing.AvailableInventoryCount,
	)
	msg.ReplyMarkup = s.listingDetailKeyboardWithStatus(listing.ID, listing.Status, listing.AvailableInventoryCount)

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
		formatPrice(store.EffectiveListingPriceAmount(*listing)),
		html.EscapeString(availability),
	)
}

func formatOfferSummary(offer *store.Offer) string {
	available := fmt.Sprintf("%d coupons", offer.AvailableCodeCount)
	if offer.AvailableCodeCount == 1 {
		available = "1 coupon"
	}

	return fmt.Sprintf("<b>%s</b>\n%s\n\n<b>Price:</b> %s\n<b>Available:</b> %s",
		html.EscapeString(offer.MerchantName),
		html.EscapeString(truncate(firstNonEmpty(offer.Title, offer.Description), 80)),
		formatPrice(offer.PriceAmount),
		html.EscapeString(available),
	)
}

func formatOfferDetails(offer *store.Offer) string {
	lines := []string{
		fmt.Sprintf("<b>%s</b>", html.EscapeString(offer.MerchantName)),
		html.EscapeString(offer.Title),
		fmt.Sprintf("<b>Price:</b> %s", formatPrice(offer.PriceAmount)),
	}
	if strings.TrimSpace(offer.Description) != "" {
		lines = append(lines, html.EscapeString(offer.Description))
	}
	if strings.TrimSpace(offer.RedemptionTerms) != "" {
		lines = append(lines, "<b>Redemption:</b> "+html.EscapeString(offer.RedemptionTerms))
	}
	if strings.TrimSpace(offer.MerchantDisclosureText) != "" {
		lines = append(lines, html.EscapeString(offer.MerchantDisclosureText))
	}
	return strings.Join(lines, "\n")
}

func offerKeyboard(offerID int64, availableCodeCount int64, allowAnotherDeal bool) *tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	firstRow := make([]tgbotapi.InlineKeyboardButton, 0, 2)

	if availableCodeCount > 0 {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("Buy", formatCallbackData(CallbackBuyOffer, offerID)))
	}
	if allowAnotherDeal {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("Another deal", formatCallbackData(CallbackAnotherOffer, offerID)))
	}
	if len(firstRow) == 0 {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("Details", formatCallbackData(CallbackViewOffer, offerID)))
	}

	rows = append(rows, firstRow)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Details", formatCallbackData(CallbackViewOffer, offerID)),
	))
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func offerDetailKeyboard(offerID int64, availableCodeCount int64) *tgbotapi.InlineKeyboardMarkup {
	row := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if availableCodeCount > 0 {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("Buy", formatCallbackData(CallbackBuyOffer, offerID)))
	}
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Another deal", formatCallbackData(CallbackAnotherOffer, offerID)))
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{row}}
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

func (s *BotService) SendPhotoMessage(ctx context.Context, telegramUserID int64, photo []byte, filename string, caption string) (int64, error) {
	msg := tgbotapi.NewPhoto(telegramUserID, tgbotapi.FileBytes{
		Name:  filename,
		Bytes: photo,
	})
	msg.Caption = normalizeTelegramText(caption)
	msg.ParseMode = tgbotapi.ModeHTML
	sent, err := s.botAPI.Send(msg)
	if err != nil {
		return 0, err
	}
	return int64(sent.MessageID), nil
}

func (s *BotService) SendPostDeliveryOffer(ctx context.Context, telegramUserID int64) error {
	if s == nil {
		return nil
	}
	if telegramUserID == 0 {
		return fmt.Errorf("telegram user id is required")
	}

	listings, err := s.loadStartListings(ctx)
	if err != nil {
		return err
	}
	if len(listings) == 0 {
		return nil
	}

	message := "Want another coupon? Here is a fresh deal:"
	if err := s.sendMessage(ctx, telegramUserID, message); err != nil {
		return err
	}

	return s.sendFeaturedListing(ctx, telegramUserID, listings, 0)
}

func (s *BotService) NotifyExpiredCheckoutHold(ctx context.Context, hold store.ReleasedCheckoutHold) error {
	if s == nil {
		return nil
	}
	if hold.TelegramUserID == 0 {
		return fmt.Errorf("telegram user id is required")
	}

	message := "<b>Your checkout window expired.</b>\n" +
		"We released that reserved coupon because payment was not completed in time.\n\n" +
		"Here is another available deal:"
	if err := s.sendMessage(ctx, hold.TelegramUserID, message); err != nil {
		return err
	}

	listings, err := s.loadStartListings(ctx)
	if err != nil {
		return err
	}
	if len(listings) == 0 {
		return s.sendMessage(ctx, hold.TelegramUserID, "No coupons are available right now. Send /start again soon.")
	}

	return s.sendFeaturedListing(ctx, hold.TelegramUserID, listings, 0)
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

func (s *BotService) listingDetailKeyboardWithStatus(listingID int64, listingStatus string, availableInventoryCount int64) *tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 1)
	firstRow := make([]tgbotapi.InlineKeyboardButton, 0, 2)

	if canStartCheckout(listingStatus, availableInventoryCount) {
		firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05e7\u05d1\u05dc \u05e7\u05d5\u05e4\u05d5\u05df", formatCallbackData(CallbackConfirmBuy, listingID)))
	}
	firstRow = append(firstRow, tgbotapi.NewInlineKeyboardButtonData("\U0001F449 \u05d3\u05d9\u05dc \u05d0\u05d7\u05e8", formatCallbackData(CallbackAnotherDeal, listingID)))
	rows = append(rows, firstRow)

	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func canStartCheckout(listingStatus string, availableInventoryCount int64) bool {
	return strings.EqualFold(strings.TrimSpace(listingStatus), "active") && availableInventoryCount > 0
}

func filterBrowsableListings(listings []store.Listing, isDevelopment bool) []store.Listing {
	filtered := make([]store.Listing, 0, len(listings))
	for _, listing := range listings {
		if listing.CouponValueAmount <= store.EffectiveListingPriceAmount(listing) {
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

func paymentEvidenceFromMessage(message *tgbotapi.Message) (services.PayBoxPaymentEvidence, bool) {
	if len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		messageID := int64(message.MessageID)
		return services.PayBoxPaymentEvidence{
			ScreenshotFileID:            photo.FileID,
			ScreenshotUniqueID:          photo.FileUniqueID,
			ScreenshotTelegramMessageID: &messageID,
			ScreenshotCaption:           message.Caption,
		}, true
	}

	if message.Document != nil && strings.HasPrefix(strings.ToLower(message.Document.MimeType), "image/") {
		messageID := int64(message.MessageID)
		return services.PayBoxPaymentEvidence{
			ScreenshotFileID:            message.Document.FileID,
			ScreenshotUniqueID:          message.Document.FileUniqueID,
			ScreenshotTelegramMessageID: &messageID,
			ScreenshotCaption:           message.Caption,
		}, true
	}

	return services.PayBoxPaymentEvidence{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type pendingPayBoxClaim struct {
	orderID       int64
	claimedAmount int64
}

type pendingPayBoxClaims struct {
	mu     sync.Mutex
	claims map[int64]pendingPayBoxClaim
}

func newPendingPayBoxClaims() *pendingPayBoxClaims {
	return &pendingPayBoxClaims{claims: make(map[int64]pendingPayBoxClaim)}
}

func (p *pendingPayBoxClaims) set(chatID int64, claim pendingPayBoxClaim) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims[chatID] = claim
}

func (p *pendingPayBoxClaims) get(chatID int64) (pendingPayBoxClaim, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	claim, ok := p.claims[chatID]
	return claim, ok
}

func (p *pendingPayBoxClaims) delete(chatID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.claims, chatID)
}
