package telegram

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestListingOfferKeyboardHidesGetCouponForSoldOutListing(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingOfferKeyboard(11, "active", 0, true)

	if keyboard == nil || len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected two keyboard rows, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 1 {
		t.Fatalf("expected one button for sold-out listing, got %#v", firstRow)
	}
	if firstRow[0].CallbackData == nil || *firstRow[0].CallbackData != "another_deal:11" {
		t.Fatalf("expected another-deal callback, got %#v", firstRow[0].CallbackData)
	}

	secondRow := keyboard.InlineKeyboard[1]
	if len(secondRow) != 1 || secondRow[0].CallbackData == nil || *secondRow[0].CallbackData != "view_details:11" {
		t.Fatalf("expected details button row, got %#v", secondRow)
	}
}

func TestListingDetailKeyboardHidesPrimaryCTAWhenSoldOut(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingDetailKeyboardWithStatus(12, "active", 0)

	if keyboard == nil || len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf("expected one keyboard row, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 1 {
		t.Fatalf("expected one action in first row, got %#v", firstRow)
	}
	if firstRow[0].CallbackData == nil || *firstRow[0].CallbackData != "another_deal:12" {
		t.Fatalf("expected another-deal callback, got %#v", firstRow[0].CallbackData)
	}
}

func TestListingDetailKeyboardUsesTwoCTAsWhenAvailable(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingDetailKeyboardWithStatus(13, "active", 2)

	if keyboard == nil || len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf("expected one keyboard row, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 2 {
		t.Fatalf("expected two buttons for active listing, got %#v", firstRow)
	}
	if firstRow[0].Text != "👉 קבל קופון" {
		t.Fatalf("expected get-coupon CTA, got %q", firstRow[0].Text)
	}
	if firstRow[0].CallbackData == nil || *firstRow[0].CallbackData != "confirm_buy:13" {
		t.Fatalf("expected confirm-buy callback, got %#v", firstRow[0].CallbackData)
	}
	if firstRow[1].CallbackData == nil || *firstRow[1].CallbackData != "another_deal:13" {
		t.Fatalf("expected another-deal callback, got %#v", firstRow[1].CallbackData)
	}
}

func TestListingOfferKeyboardHidesGetCouponForUnpublishedListingWithInventory(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingOfferKeyboard(13, "draft", 2, true)

	if keyboard == nil || len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected two keyboard rows, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 1 {
		t.Fatalf("expected one button for unpublished listing, got %#v", firstRow)
	}
	if firstRow[0].CallbackData == nil || *firstRow[0].CallbackData != "another_deal:13" {
		t.Fatalf("expected another-deal callback, got %#v", firstRow[0].CallbackData)
	}
}

func TestFormatListingDetailsBuildsShortHighConversionMessage(t *testing.T) {
	service := &BotService{}
	text := service.formatListingDetails(&store.Listing{
		MerchantName:            "אגאדיר",
		Status:                  "active",
		Title:                   "שובר 150₪ לארוחה",
		Description:             "Preview only. development mode. www.example.com",
		CouponValueAmount:       15000,
		SalePriceAmount:         7400,
		ResellPriceAmount:       10500,
		AvailableInventoryCount: 6,
	})

	lines := strings.Split(text, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected exactly five lines, got %d in %q", len(lines), text)
	}
	if !strings.Contains(lines[0], "🍔") || !strings.Contains(lines[0], "אגאדיר") {
		t.Fatalf("expected brand headline with emoji, got %q", lines[0])
	}
	if !strings.Contains(lines[2], "150.0") || !strings.Contains(lines[2], "105.0") {
		t.Fatalf("expected price anchor line, got %q", lines[2])
	}
	for _, forbidden := range []string{"Preview only", "development mode", "www."} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected noisy detail copy to be removed, got %q", text)
		}
	}
}

func TestFormatListingDetailsUsesNonBurgerEmojiWhenRelevant(t *testing.T) {
	service := &BotService{}
	text := service.formatListingDetails(&store.Listing{
		MerchantName:            "פיצה האט",
		Status:                  "active",
		Title:                   "פיצה משפחתית",
		CouponValueAmount:       9000,
		SalePriceAmount:         4800,
		ResellPriceAmount:       6900,
		AvailableInventoryCount: 1,
	})

	if !strings.HasPrefix(text, "🍕") {
		t.Fatalf("expected pizza emoji, got %q", text)
	}
	if strings.Contains(text, "🍔") {
		t.Fatalf("did not expect burger emoji, got %q", text)
	}
}

func TestFormatListingDetailsShowsUnavailableCopyWithoutLegalNoise(t *testing.T) {
	service := &BotService{}
	text := service.formatListingDetails(&store.Listing{
		MerchantName:            "Cafe",
		Status:                  "draft",
		Title:                   "Breakfast coupon",
		Description:             "Use for one meal. Final sale. Contact support.",
		CouponValueAmount:       5000,
		SalePriceAmount:         2200,
		ResellPriceAmount:       3500,
		AvailableInventoryCount: 0,
		RedemptionInstructions:  "Show the code to the cashier.",
		FinalSaleDisclosureText: "All sales final.",
	})

	if !strings.Contains(text, "⏳") {
		t.Fatalf("expected unavailable urgency line, got %q", text)
	}
	for _, forbidden := range []string{"Final sale", "Contact support", "Show the code", "Preview only"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected legal and support noise to be removed, got %q", text)
		}
	}
}

func TestFormatFeaturedListingCaptionShowsPreviewStatusInDevelopment(t *testing.T) {
	service := &BotService{
		config: &config.Config{AppEnv: "development"},
	}

	text := service.formatFeaturedListingCaption(&store.Listing{
		MerchantName:            "Cafe",
		Title:                   "Breakfast coupon",
		CouponValueAmount:       5000,
		SalePriceAmount:         2200,
		ResellPriceAmount:       3500,
		AvailableInventoryCount: 0,
	})

	if !strings.Contains(text, "תצוגה בלבד") {
		t.Fatalf("expected preview status in featured caption, got %q", text)
	}
	if !strings.Contains(text, "50.0") || !strings.Contains(text, "35.0") || !strings.Contains(stripBidiControls(text), "₪") {
		t.Fatalf("expected formatted prices in caption, got %q", text)
	}
}

func TestFormatPriceUsesShekelSuffix(t *testing.T) {
	if got := stripBidiControls(formatPrice(5950)); !strings.Contains(got, "59.5") || !strings.Contains(got, "₪") {
		t.Fatalf("expected shekel suffix price, got %q", got)
	}
	if got := stripBidiControls(formatPrice(5900)); !strings.Contains(got, "59.0") || !strings.Contains(got, "₪") {
		t.Fatalf("expected integer shekel suffix price, got %q", got)
	}
}

func TestFormatHoldDurationUsesMinutes(t *testing.T) {
	if got := formatHoldDuration(10 * time.Minute); got != "10 minutes" {
		t.Fatalf("expected humanized minutes, got %q", got)
	}
}

func TestTruncatePreservesUTF8ForHebrew(t *testing.T) {
	input := "שובר זוגי לארוחת בוקר מפנקת"
	got := truncate(input, 10)

	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 output, got %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) != 13 {
		t.Fatalf("expected 10 runes plus ellipsis, got %q", got)
	}
}

func TestNormalizeTelegramTextRemovesInvalidUTF8(t *testing.T) {
	input := string([]byte{'a', 0xff, 'b'})
	got := normalizeTelegramText(input)

	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 output, got %q", got)
	}
	if got != "ab" {
		t.Fatalf("expected invalid byte to be removed, got %q", got)
	}
}

func TestAdminPaymentClaimKeyboardUsesClaimCallbacks(t *testing.T) {
	keyboard := adminPaymentClaimKeyboard(901)

	if keyboard == nil || len(keyboard.InlineKeyboard) != 1 || len(keyboard.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected one row with approve/reject buttons, got %#v", keyboard)
	}
	if keyboard.InlineKeyboard[0][0].CallbackData == nil || *keyboard.InlineKeyboard[0][0].CallbackData != "admin_approve_claim:901" {
		t.Fatalf("unexpected approve callback: %#v", keyboard.InlineKeyboard[0][0].CallbackData)
	}
	if keyboard.InlineKeyboard[0][1].CallbackData == nil || *keyboard.InlineKeyboard[0][1].CallbackData != "admin_reject_claim:901" {
		t.Fatalf("unexpected reject callback: %#v", keyboard.InlineKeyboard[0][1].CallbackData)
	}
}

func TestBotServiceChecksTelegramAdminAllowlist(t *testing.T) {
	service := &BotService{adminUserIDSet: buildAdminUserIDSet([]int64{111, 222})}

	if !service.isTelegramAdmin(&tgbotapi.User{ID: 111}) {
		t.Fatal("expected user 111 to be admin")
	}
	if service.isTelegramAdmin(&tgbotapi.User{ID: 333}) {
		t.Fatal("did not expect user 333 to be admin")
	}
}

func TestFilterBrowsableListingsDevelopmentAllowsDraftActiveAndPreviewDiscounts(t *testing.T) {
	filtered := filterBrowsableListings([]store.Listing{
		{ID: 1, Status: "draft", CouponValueAmount: 10000, SalePriceAmount: 6000, ResellPriceAmount: 6500},
		{ID: 2, Status: "active", CouponValueAmount: 12000, SalePriceAmount: 7000, ResellPriceAmount: 7600},
		{ID: 3, Status: "preview", CouponValueAmount: 9000, SalePriceAmount: 5000, ResellPriceAmount: 5900},
		{ID: 4, Status: "sold_out", CouponValueAmount: 10000, SalePriceAmount: 6000, ResellPriceAmount: 6500},
		{ID: 5, Status: "active", CouponValueAmount: 10000, SalePriceAmount: 6000, ResellPriceAmount: 10000},
	}, true)

	if len(filtered) != 3 || filtered[0].ID != 1 || filtered[1].ID != 2 || filtered[2].ID != 3 {
		t.Fatalf("expected discounted draft/active/preview listings only, got %#v", filtered)
	}
}

func TestFilterBrowsableListingsProductionAllowsOnlyActiveDiscounts(t *testing.T) {
	filtered := filterBrowsableListings([]store.Listing{
		{ID: 1, Status: "draft", CouponValueAmount: 10000, SalePriceAmount: 6000, ResellPriceAmount: 6500},
		{ID: 2, Status: "active", CouponValueAmount: 12000, SalePriceAmount: 7000, ResellPriceAmount: 7600},
		{ID: 3, Status: "preview", CouponValueAmount: 9000, SalePriceAmount: 5000, ResellPriceAmount: 5900},
		{ID: 4, Status: "active", CouponValueAmount: 10000, SalePriceAmount: 6000, ResellPriceAmount: 10000},
	}, false)

	if len(filtered) != 1 || filtered[0].ID != 2 {
		t.Fatalf("expected only active discounted listings in production, got %#v", filtered)
	}
}

func stripBidiControls(value string) string {
	value = strings.ReplaceAll(value, "\u2066", "")
	value = strings.ReplaceAll(value, "\u2069", "")
	return value
}
