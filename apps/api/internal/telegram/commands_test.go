package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func TestCreateListingsKeyboardHidesBuyForSoldOutListing(t *testing.T) {
	service := &BotService{}
	keyboard := service.createListingsKeyboard(1, []store.Listing{
		{ID: 10, Title: "Available", AvailableInventoryCount: 1},
		{ID: 11, Title: "Sold Out", AvailableInventoryCount: 0},
	})

	if keyboard == nil || len(keyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected keyboard rows, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 2 || firstRow[0].Text != "Buy" || firstRow[1].Text != "Details" {
		t.Fatalf("unexpected first row: %#v", firstRow)
	}

	secondRow := keyboard.InlineKeyboard[1]
	if len(secondRow) != 1 || secondRow[0].Text != "Details" {
		t.Fatalf("expected sold-out row to show details only, got %#v", secondRow)
	}
}

func TestListingDetailKeyboardHidesBuyNowWhenSoldOut(t *testing.T) {
	service := &BotService{}
	keyboard := service.listingDetailKeyboard(12, 0, true)

	if keyboard == nil || len(keyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected keyboard rows, got %#v", keyboard)
	}

	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) != 1 || firstRow[0].Text != "Back" {
		t.Fatalf("expected sold-out detail row to show only Back, got %#v", firstRow)
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

func TestFormatListingsForDisplayShowsPreviewStatusInDevelopment(t *testing.T) {
	service := &BotService{
		config: &config.Config{AppEnv: "development"},
	}

	text := service.formatListingsForDisplay([]store.Listing{
		{MerchantName: "Cafe", Title: "Breakfast coupon", SalePriceAmount: 3500, AvailableInventoryCount: 0},
	})

	if !strings.Contains(text, "Status: Preview only") {
		t.Fatalf("expected preview status in development listing summary, got %q", text)
	}
}

func TestFormatHoldDurationUsesMinutes(t *testing.T) {
	if got := formatHoldDuration(10 * time.Minute); got != "10 minutes" {
		t.Fatalf("expected humanized minutes, got %q", got)
	}
}
