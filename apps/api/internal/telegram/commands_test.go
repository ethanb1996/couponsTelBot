package telegram

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestListingOfferKeyboardHidesGetCouponForSoldOutListing(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingOfferKeyboard(11, 0, true)

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

func TestListingDetailKeyboardHidesContinueToPaymentWhenSoldOut(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingDetailKeyboard(12, 0)

	if keyboard == nil || len(keyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected keyboard rows, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 1 {
		t.Fatalf("expected one action in first row, got %#v", firstRow)
	}
	if firstRow[0].CallbackData == nil || *firstRow[0].CallbackData != "another_deal:12" {
		t.Fatalf("expected another-deal callback, got %#v", firstRow[0].CallbackData)
	}
}

func TestFormatListingDetailsIncludesSoldOutNotice(t *testing.T) {
	service := &BotService{}
	text := service.formatListingDetails(&store.Listing{
		MerchantName:            "Cafe",
		Title:                   "Breakfast coupon",
		Description:             "Use for one meal.",
		CouponValueAmount:       5000,
		SalePriceAmount:         3500,
		RedemptionInstructions:  "Show the code to the cashier.",
		FinalSaleDisclosureText: "All sales final.",
	})

	if !strings.Contains(text, "Quantity Available:</b> Sold out") {
		t.Fatalf("expected sold-out quantity copy, got %q", text)
	}
	if !strings.Contains(text, "currently sold out") {
		t.Fatalf("expected sold-out notice, got %q", text)
	}
}

func TestFormatListingDetailsUsesPreviewNoticeInDevelopment(t *testing.T) {
	service := &BotService{
		config: &config.Config{AppEnv: "development"},
	}
	text := service.formatListingDetails(&store.Listing{
		MerchantName:            "Cafe",
		Title:                   "Breakfast coupon",
		Description:             "Use for one meal.",
		CouponValueAmount:       5000,
		SalePriceAmount:         3500,
		RedemptionInstructions:  "Show the code to the cashier.",
		FinalSaleDisclosureText: "All sales final.",
	})

	if !strings.Contains(text, "Quantity Available:</b> Preview only") {
		t.Fatalf("expected preview quantity copy, got %q", text)
	}
	if !strings.Contains(text, "visible in development preview mode") {
		t.Fatalf("expected development preview notice, got %q", text)
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
		SalePriceAmount:         3500,
		AvailableInventoryCount: 0,
	})

	if !strings.Contains(text, "תצוגה בלבד") {
		t.Fatalf("expected preview status in featured caption, got %q", text)
	}
	if !strings.Contains(text, "50.0") || !strings.Contains(text, "35.0") || !strings.Contains(text, "₪") {
		t.Fatalf("expected formatted prices in caption, got %q", text)
	}
}

func TestFormatPriceUsesShekelSuffix(t *testing.T) {
	if got := formatPrice(5950); stripBidiControls(got) != "59.5 ₪" {
		t.Fatalf("expected shekel suffix price, got %q", got)
	}
	if got := formatPrice(5900); stripBidiControls(got) != "59.0 ₪" {
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

func TestFilterBrowsableListingsDevelopmentAllowsDraftActiveAndPreviewDiscounts(t *testing.T) {
	filtered := filterBrowsableListings([]store.Listing{
		{ID: 1, Status: "draft", CouponValueAmount: 10000, SalePriceAmount: 6000},
		{ID: 2, Status: "active", CouponValueAmount: 12000, SalePriceAmount: 7000},
		{ID: 3, Status: "preview", CouponValueAmount: 9000, SalePriceAmount: 5000},
		{ID: 4, Status: "sold_out", CouponValueAmount: 10000, SalePriceAmount: 6000},
		{ID: 5, Status: "active", CouponValueAmount: 10000, SalePriceAmount: 10000},
	}, true)

	if len(filtered) != 3 || filtered[0].ID != 1 || filtered[1].ID != 2 || filtered[2].ID != 3 {
		t.Fatalf("expected discounted draft/active/preview listings only, got %#v", filtered)
	}
}

func TestFilterBrowsableListingsProductionAllowsOnlyActiveDiscounts(t *testing.T) {
	filtered := filterBrowsableListings([]store.Listing{
		{ID: 1, Status: "draft", CouponValueAmount: 10000, SalePriceAmount: 6000},
		{ID: 2, Status: "active", CouponValueAmount: 12000, SalePriceAmount: 7000},
		{ID: 3, Status: "preview", CouponValueAmount: 9000, SalePriceAmount: 5000},
		{ID: 4, Status: "active", CouponValueAmount: 10000, SalePriceAmount: 10000},
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
