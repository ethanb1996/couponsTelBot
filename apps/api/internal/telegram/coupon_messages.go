package telegram

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CouponCategory string

const (
	couponCategoryPizza    CouponCategory = "pizza"
	couponCategoryBurger   CouponCategory = "burger"
	couponCategorySushi    CouponCategory = "sushi"
	couponCategoryCafe     CouponCategory = "cafe"
	couponCategoryShawarma CouponCategory = "shawarma"
	couponCategoryOther    CouponCategory = "other"
)

type CouponStatus string

const (
	couponStatusDraft   CouponStatus = "draft"
	couponStatusActive  CouponStatus = "active"
	couponStatusPaused  CouponStatus = "paused"
	couponStatusSoldOut CouponStatus = "sold_out"
	couponStatusExpired CouponStatus = "expired"
)

type CouponDisplayVariant string

const (
	couponDisplayVariantDirect   CouponDisplayVariant = "direct"
	couponDisplayVariantScarcity CouponDisplayVariant = "scarcity"
	couponDisplayVariantFamily   CouponDisplayVariant = "family"
)

type CouponMessageData struct {
	ID                  int64
	BusinessName        string
	AreaLabel           string
	Category            CouponCategory
	Title               string
	OfferDescription    string
	OriginalPriceILS    *int64
	CouponPriceILS      int64
	PayBoxURL           string
	InternalTrackingURL string
	TotalQuantity       *int64
	SoldQuantity        *int64
	AvailableQuantity   *int64
	ValidUntil          *time.Time
	ValidTodayOnly      bool
	RedemptionInfo      string
	Status              CouponStatus
	DisplayVariant      CouponDisplayVariant
}

type TelegramCouponMessage struct {
	Text        string
	ParseMode   string
	ReplyMarkup tgbotapi.InlineKeyboardMarkup
}

type couponMessageConfig struct {
	LowInventoryThreshold  int64
	ScarcityTemplateCutoff int64
	DefaultCTALabel        string
	DefaultRedemptionInfo  string
	SoldOutButtonLabel     string
	ExpiredButtonLabel     string
	HooksByCategory        map[CouponCategory][]string
	VariantCTALabels       map[CouponDisplayVariant]string
}

var couponMessagingConfig = couponMessageConfig{
	LowInventoryThreshold:  10,
	ScarcityTemplateCutoff: 8,
	DefaultCTALabel:        "\U0001F6D2 \u05e8\u05db\u05d9\u05e9\u05d4 \u05d1\u05e4\u05d9\u05d9\u05d1\u05d5\u05e7\u05e1",
	DefaultRedemptionInfo:  "\U0001F4CD \u05d0\u05d7\u05e8\u05d9 \u05d4\u05d0\u05d9\u05e9\u05d5\u05e8 \u05d4\u05e6\u05d9\u05d2\u05d5 \u05d0\u05ea \u05e7\u05d5\u05d3 \u05d4-QR \u05d1\u05d1\u05d9\u05ea \u05d4\u05e2\u05e1\u05e7",
	SoldOutButtonLabel:     "\U0001F514 \u05e2\u05d3\u05db\u05e0\u05d5 \u05d0\u05d5\u05ea\u05d9",
	ExpiredButtonLabel:     "\U0001F50E \u05d4\u05e6\u05d2 \u05e7\u05d5\u05e4\u05d5\u05e0\u05d9\u05dd \u05e4\u05e2\u05d9\u05dc\u05d9\u05dd",
	HooksByCategory: map[CouponCategory][]string{
		couponCategoryPizza: {
			"\U0001F355 \u05e2\u05e8\u05d1 \u05e4\u05d9\u05e6\u05d4?",
			"\U0001F355 \u05e2\u05e8\u05d1 \u05e4\u05d9\u05e6\u05d4 \u05de\u05e9\u05e4\u05d7\u05ea\u05d9?",
		},
		couponCategoryBurger: {
			"\U0001F354 \u05d1\u05d0 \u05dc\u05db\u05dd \u05dc\u05d4\u05ea\u05e4\u05e0\u05e7 \u05d4\u05e2\u05e8\u05d1?",
			"\U0001F354 \u05d4\u05de\u05d1\u05d5\u05e8\u05d2\u05e8 \u05d4\u05d6\u05d4 \u05e1\u05d5\u05d2\u05e8 \u05d0\u05ea \u05d4\u05e4\u05d9\u05e0\u05d4",
		},
		couponCategorySushi: {
			"\U0001F363 \u05d1\u05d0 \u05dc\u05db\u05dd \u05e1\u05d5\u05e9\u05d9 \u05d4\u05e2\u05e8\u05d1?",
			"\U0001F363 \u05e2\u05e8\u05d1 \u05d6\u05d5\u05d2\u05d9 \u05d1\u05dc\u05d9 \u05dc\u05e7\u05e8\u05d5\u05e2 \u05d0\u05ea \u05d4\u05db\u05d9\u05e1",
		},
		couponCategoryCafe: {
			"\u2615 \u05e7\u05e4\u05d4 \u05d5\u05de\u05d0\u05e4\u05d4 \u05dc\u05e4\u05ea\u05d5\u05d7 \u05d0\u05ea \u05d4\u05d9\u05d5\u05dd",
			"\u2615 \u05e2\u05e6\u05d9\u05e8\u05d4 \u05e7\u05d8\u05e0\u05d4 \u05dc\u05e7\u05e4\u05d4 \u05d8\u05d5\u05d1",
		},
		couponCategoryShawarma: {
			"\U0001F959 \u05d0\u05e8\u05d5\u05d7\u05ea \u05e6\u05d4\u05e8\u05d9\u05d9\u05dd \u05de\u05d4\u05d9\u05e8\u05d4 \u05d5\u05de\u05e9\u05ea\u05dc\u05de\u05ea",
			"\U0001F959 \u05e8\u05e2\u05d1\u05d9\u05dd \u05dc\u05de\u05e9\u05d4\u05d5 \u05de\u05d4\u05d9\u05e8?",
		},
		couponCategoryOther: {
			"\U0001F525 \u05d4\u05d8\u05d1\u05d4 \u05de\u05e7\u05d5\u05de\u05d9\u05ea \u05dc\u05d9\u05d3 \u05d4\u05d1\u05d9\u05ea",
			"\U0001F381 \u05e7\u05d5\u05e4\u05d5\u05df \u05d7\u05d3\u05e9 \u05d1\u05d0\u05d6\u05d5\u05e8",
		},
	},
	VariantCTALabels: map[CouponDisplayVariant]string{
		couponDisplayVariantDirect:   "\u05e8\u05db\u05d9\u05e9\u05d4 \u05d1\u05e4\u05d9\u05d9\u05d1\u05d5\u05e7\u05e1",
		couponDisplayVariantScarcity: "\u05e7\u05e0\u05d4 \u05e2\u05db\u05e9\u05d9\u05d5",
		couponDisplayVariantFamily:   "\u05e8\u05db\u05d9\u05e9\u05d4 \u05d1\u05e4\u05d9\u05d9\u05d1\u05d5\u05e7\u05e1",
	},
}

func renderCouponTelegramMessage(coupon CouponMessageData) TelegramCouponMessage {
	return renderCouponTelegramMessageAt(coupon, time.Now())
}

func renderCouponTelegramMessageAt(coupon CouponMessageData, now time.Time) TelegramCouponMessage {
	text := buildCouponMessageText(coupon, now)
	url := getCouponCTAURL(coupon)
	buttonText := ctaLabelForCoupon(coupon)

	message := TelegramCouponMessage{
		Text:      normalizeTelegramText(text),
		ParseMode: tgbotapi.ModeHTML,
	}
	if strings.TrimSpace(url) != "" {
		message.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL(buttonText, url),
			),
		)
	}
	return message
}

func buildCouponMessageText(coupon CouponMessageData, now time.Time) string {
	switch {
	case couponIsExpired(coupon, now):
		return renderExpiredCouponText(coupon)
	case couponIsSoldOut(coupon):
		return renderSoldOutCouponText(coupon)
	default:
		return renderActiveCouponText(coupon, now)
	}
}

func renderActiveCouponText(coupon CouponMessageData, now time.Time) string {
	lines := []string{
		fmt.Sprintf("<b>%s</b>", html.EscapeString(hookForCoupon(coupon))),
		fmt.Sprintf("<b>%s</b>", html.EscapeString(businessLineForCoupon(coupon))),
		html.EscapeString(primaryOfferLine(coupon)),
		renderCouponPrice(coupon),
	}

	if urgency := renderCouponUrgency(coupon, now); urgency != "" {
		lines = append(lines, html.EscapeString(urgency))
	}
	if scarcity := renderCouponScarcity(coupon); scarcity != "" {
		lines = append(lines, html.EscapeString(scarcity))
	}
	if redemption := redemptionInfoForCoupon(coupon); redemption != "" {
		lines = append(lines, html.EscapeString(redemption))
	}

	return strings.Join(lines, "\n\n")
}

func renderSoldOutCouponText(coupon CouponMessageData) string {
	lines := []string{
		"\U0001F3F7\uFE0F <b>\u05d4\u05e7\u05d5\u05e4\u05d5\u05df \u05d0\u05d6\u05dc</b>",
		fmt.Sprintf("<b>%s</b>", html.EscapeString(businessLineForCoupon(coupon))),
		html.EscapeString("\u05d4\u05d4\u05d8\u05d1\u05d4 \u05d4\u05d6\u05d0\u05ea \u05db\u05d1\u05e8 \u05dc\u05d0 \u05d6\u05de\u05d9\u05e0\u05d4. \u05db\u05e9\u05d9\u05e2\u05dc\u05d5 \u05d4\u05d8\u05d1\u05d5\u05ea \u05d7\u05d3\u05e9\u05d5\u05ea \u05e0\u05e2\u05d3\u05db\u05df \u05db\u05d0\u05df."),
	}
	return strings.Join(lines, "\n\n")
}

func renderExpiredCouponText(coupon CouponMessageData) string {
	lines := []string{
		"\u23F0 <b>\u05d4\u05e7\u05d5\u05e4\u05d5\u05df \u05d4\u05e1\u05ea\u05d9\u05d9\u05dd</b>",
		fmt.Sprintf("<b>%s</b>", html.EscapeString(businessLineForCoupon(coupon))),
		html.EscapeString("\u05d4\u05d4\u05d8\u05d1\u05d4 \u05d4\u05d6\u05d0\u05ea \u05db\u05d1\u05e8 \u05dc\u05d0 \u05d1\u05ea\u05d5\u05e7\u05e3."),
	}
	return strings.Join(lines, "\n\n")
}

func couponMessageDataFromOffer(offer store.Offer) CouponMessageData {
	available := offer.AvailableCodeCount
	status := CouponStatus(strings.TrimSpace(offer.Status))
	if status == "" {
		status = couponStatusActive
	}
	if available <= 0 && status == couponStatusActive {
		status = couponStatusSoldOut
	}

	originalPrice := parseCouponOriginalPrice(firstNonEmpty(offer.Description, offer.MerchantDisclosureText), offer.PriceAmount)
	offerDescription := strings.TrimSpace(offer.Description)
	if originalPrice != nil && looksLikePriceAnchor(offerDescription) {
		offerDescription = ""
	}

	return CouponMessageData{
		ID:                offer.ID,
		BusinessName:      strings.TrimSpace(offer.MerchantName),
		Category:          inferCouponCategory(offer),
		Title:             strings.TrimSpace(offer.Title),
		OfferDescription:  offerDescription,
		OriginalPriceILS:  originalPrice,
		CouponPriceILS:    offer.PriceAmount,
		PayBoxURL:         strings.TrimSpace(offer.PaymentLink),
		AvailableQuantity: &available,
		RedemptionInfo:    strings.TrimSpace(offer.RedemptionTerms),
		Status:            status,
	}
}

func inferCouponCategory(offer store.Offer) CouponCategory {
	text := strings.ToLower(strings.Join([]string{
		offer.MerchantName,
		offer.Title,
		offer.Description,
	}, " "))

	switch {
	case containsAny(text, "pizza", "\u05e4\u05d9\u05e6\u05d4"):
		return couponCategoryPizza
	case containsAny(text, "burger", "hamburger", "\u05d1\u05d5\u05e8\u05d2\u05e8", "\u05d4\u05de\u05d1\u05d5\u05e8\u05d2\u05e8"):
		return couponCategoryBurger
	case containsAny(text, "sushi", "poke", "ramen", "\u05e1\u05d5\u05e9\u05d9", "\u05d0\u05e1\u05d9\u05d9\u05ea\u05d9"):
		return couponCategorySushi
	case containsAny(text, "coffee", "cafe", "espresso", "\u05e7\u05e4\u05d4", "\u05d0\u05e1\u05e4\u05e8\u05e1\u05d5"):
		return couponCategoryCafe
	case containsAny(text, "shawarma", "grill", "steak", "\u05e9\u05d5\u05d5\u05d0\u05e8\u05de\u05d4", "\u05d1\u05e9\u05e8", "\u05d2\u05e8\u05d9\u05dc"):
		return couponCategoryShawarma
	default:
		return couponCategoryOther
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func ctaLabelForCoupon(coupon CouponMessageData) string {
	variant := selectCouponTemplate(coupon)
	if label, ok := couponMessagingConfig.VariantCTALabels[variant]; ok && strings.TrimSpace(label) != "" {
		return label
	}
	return couponMessagingConfig.DefaultCTALabel
}

func getCouponCTAURL(coupon CouponMessageData) string {
	if strings.TrimSpace(coupon.InternalTrackingURL) != "" {
		return strings.TrimSpace(coupon.InternalTrackingURL)
	}
	return strings.TrimSpace(coupon.PayBoxURL)
}

func selectCouponTemplate(coupon CouponMessageData) CouponDisplayVariant {
	if coupon.DisplayVariant != "" {
		return coupon.DisplayVariant
	}
	if remaining, ok := couponRemainingQuantity(coupon); ok && remaining <= couponMessagingConfig.ScarcityTemplateCutoff {
		return couponDisplayVariantScarcity
	}
	if coupon.Category == couponCategoryPizza {
		return couponDisplayVariantFamily
	}
	return couponDisplayVariantDirect
}

func hookForCoupon(coupon CouponMessageData) string {
	variant := selectCouponTemplate(coupon)
	if variant == couponDisplayVariantFamily && coupon.Category == couponCategoryPizza {
		return "\U0001F468\u200D\U0001F469\u200D\U0001F467 \u05e2\u05e8\u05d1 \u05de\u05e9\u05e4\u05d7\u05ea\u05d9 \u05d1\u05dc\u05d9 \u05d1\u05d9\u05e9\u05d5\u05dc\u05d9\u05dd"
	}

	hooks := couponMessagingConfig.HooksByCategory[coupon.Category]
	if len(hooks) == 0 {
		hooks = couponMessagingConfig.HooksByCategory[couponCategoryOther]
	}
	if len(hooks) == 0 {
		return "\U0001F525 \u05d4\u05d8\u05d1\u05d4 \u05d7\u05d3\u05e9\u05d4"
	}
	return hooks[0]
}

func businessLineForCoupon(coupon CouponMessageData) string {
	business := strings.TrimSpace(coupon.BusinessName)
	area := strings.TrimSpace(coupon.AreaLabel)
	switch {
	case business == "" && area == "":
		return "\u05e7\u05d5\u05e4\u05d5\u05df \u05e4\u05e2\u05d9\u05dc"
	case business == "":
		return area
	case area == "":
		return business
	default:
		return business + " | " + area
	}
}

func primaryOfferLine(coupon CouponMessageData) string {
	for _, candidate := range []string{coupon.Title, coupon.OfferDescription} {
		candidate = collapseSpaces(strings.TrimSpace(candidate))
		if candidate != "" {
			return candidate
		}
	}
	return "\u05e7\u05d5\u05e4\u05d5\u05df \u05d6\u05de\u05d9\u05df \u05dc\u05e8\u05db\u05d9\u05e9\u05d4"
}

func renderCouponPrice(coupon CouponMessageData) string {
	if coupon.OriginalPriceILS != nil && *coupon.OriginalPriceILS > coupon.CouponPriceILS {
		if selectCouponTemplate(coupon) == couponDisplayVariantScarcity {
			return rtlText(fmt.Sprintf("%s \u2192 %s",
				formatCouponPriceILS(*coupon.OriginalPriceILS),
				formatCouponPriceILS(coupon.CouponPriceILS),
			))
		}
		return rtlText(fmt.Sprintf("\u05d1\u05de\u05e7\u05d5\u05dd <s>%s</s> \u2192 <b>%s</b>",
			formatCouponPriceILS(*coupon.OriginalPriceILS),
			formatCouponPriceILS(coupon.CouponPriceILS),
		))
	}
	return rtlText(fmt.Sprintf("<b>%s</b>", formatCouponPriceILS(coupon.CouponPriceILS)))
}

func renderCouponUrgency(coupon CouponMessageData, now time.Time) string {
	if coupon.ValidTodayOnly {
		return "\U0001F525 \u05d4\u05d9\u05d5\u05dd \u05d1\u05dc\u05d1\u05d3"
	}
	if coupon.ValidUntil == nil || coupon.ValidUntil.IsZero() {
		return ""
	}

	location := coupon.ValidUntil.Location()
	if location == nil {
		location = now.Location()
	}
	localNow := now.In(location)
	localDeadline := coupon.ValidUntil.In(location)

	if sameCalendarDay(localNow, localDeadline) {
		return "\u23F0 \u05ea\u05e7\u05e3 \u05e2\u05d3 " + localDeadline.Format("15:04")
	}
	if localDeadline.After(localNow) && localDeadline.Sub(localNow) <= 24*time.Hour {
		return "\u26A1 \u05ea\u05e7\u05e3 \u05dc-24 \u05e9\u05e2\u05d5\u05ea"
	}
	return ""
}

func sameCalendarDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func renderCouponScarcity(coupon CouponMessageData) string {
	if remaining, ok := couponRemainingQuantity(coupon); ok {
		if remaining <= couponMessagingConfig.LowInventoryThreshold {
			return fmt.Sprintf("\u26A0 \u05e0\u05e9\u05d0\u05e8\u05d5 %d \u05e7\u05d5\u05e4\u05d5\u05e0\u05d9\u05dd", remaining)
		}
	}

	if coupon.TotalQuantity != nil && *coupon.TotalQuantity > 0 {
		return fmt.Sprintf("%d \u05e7\u05d5\u05e4\u05d5\u05e0\u05d9\u05dd \u05d1\u05dc\u05d1\u05d3", *coupon.TotalQuantity)
	}
	return ""
}

func couponRemainingQuantity(coupon CouponMessageData) (int64, bool) {
	if coupon.AvailableQuantity != nil {
		return *coupon.AvailableQuantity, true
	}
	if coupon.TotalQuantity == nil {
		return 0, false
	}
	remaining := *coupon.TotalQuantity
	if coupon.SoldQuantity != nil {
		remaining -= *coupon.SoldQuantity
	}
	return remaining, true
}

func redemptionInfoForCoupon(coupon CouponMessageData) string {
	if value := strings.TrimSpace(coupon.RedemptionInfo); value != "" {
		return value
	}
	return ""
}

func couponIsSoldOut(coupon CouponMessageData) bool {
	if coupon.Status == couponStatusSoldOut {
		return true
	}
	if remaining, ok := couponRemainingQuantity(coupon); ok && remaining <= 0 {
		return true
	}
	return false
}

func couponIsExpired(coupon CouponMessageData, now time.Time) bool {
	if coupon.Status == couponStatusExpired {
		return true
	}
	if coupon.ValidUntil == nil || coupon.ValidUntil.IsZero() {
		return false
	}
	return coupon.ValidUntil.Before(now)
}

func formatCouponPriceILS(amount int64) string {
	shekels := float64(amount) / 100
	switch {
	case amount%100 == 0:
		return fmt.Sprintf("%d\u20AA", amount/100)
	case amount%10 == 0:
		return fmt.Sprintf("%.1f\u20AA", shekels)
	default:
		return fmt.Sprintf("%.2f\u20AA", shekels)
	}
}

var couponPriceAnchorPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*₪?\s*(?:->|→)\s*(\d+(?:\.\d+)?)\s*₪?`)

func parseCouponOriginalPrice(value string, currentPrice int64) *int64 {
	match := couponPriceAnchorPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 3 {
		return nil
	}

	original, ok := parseILSPriceToCents(match[1])
	if !ok {
		return nil
	}
	renderedPrice, ok := parseILSPriceToCents(match[2])
	if !ok || renderedPrice != currentPrice || original <= currentPrice {
		return nil
	}
	return &original
}

func parseILSPriceToCents(value string) (int64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return int64(math.Round(parsed * 100)), true
}

func looksLikePriceAnchor(value string) bool {
	return couponPriceAnchorPattern.MatchString(strings.TrimSpace(value))
}

func rtlText(value string) string {
	return "\u200f" + value
}
