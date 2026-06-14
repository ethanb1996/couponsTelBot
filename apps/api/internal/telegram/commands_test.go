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

func TestBotServiceUsesReviewChatForAdminNotifications(t *testing.T) {
	service := &BotService{
		adminUserIDs:      []int64{111, 222},
		adminUserIDSet:    buildAdminUserIDSet([]int64{111, 222}),
		adminReviewChatID: -1001234567890,
	}

	recipients := service.adminNotificationRecipients()
	if len(recipients) != 1 || recipients[0] != -1001234567890 {
		t.Fatalf("expected review chat recipient, got %#v", recipients)
	}
	if !service.isTelegramAdmin(&tgbotapi.User{ID: 111}) {
		t.Fatal("expected configured admin user to be authorized")
	}
	if service.isTelegramAdmin(&tgbotapi.User{ID: 333}) {
		t.Fatal("did not expect review-chat membership to grant admin rights")
	}
}

func TestBotServiceFallsBackToAdminDMRecipients(t *testing.T) {
	service := &BotService{adminUserIDs: []int64{111, 222}}

	recipients := service.adminNotificationRecipients()
	if len(recipients) != 2 || recipients[0] != 111 || recipients[1] != 222 {
		t.Fatalf("expected admin DM recipients, got %#v", recipients)
	}
}

func TestFormatAdminPaymentClaimMessageIncludesBuyerChatID(t *testing.T) {
	message := formatAdminPaymentClaimMessage(store.ManualPaymentClaim{
		ID:                       901,
		OrderID:                  501,
		OrderNumber:              "PB-501",
		PayerUsername:            "telegram:1111",
		ClaimedAmount:            4900,
		PaymentScreenshotCaption: "paid",
		BuyerDisplay:             "Buyer One",
		BuyerTelegramID:          1111,
		MerchantName:             "Cafe",
		OfferTitle:               "Breakfast",
	})

	for _, want := range []string{
		"Claim: #901",
		"Order: PB-501 (#501)",
		"Buyer Telegram chat ID: <code>1111</code>",
		"Offer: Cafe - Breakfast",
		"Caption: paid",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected admin message to contain %q, got %q", want, message)
		}
	}
}

func TestFormatAdminCouponRedemptionConfirmMessage(t *testing.T) {
	redeemedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	restaurantChatID := int64(-1001234567890)
	message := formatAdminCouponRedemptionMessage(store.CouponRedemptionConfirmResult{
		FirstRedeem:                  true,
		RestaurantNotificationChatID: &restaurantChatID,
		Preview: store.CouponRedemptionPreview{
			OrderNumber:          "PB-20260609180338",
			MerchantName:         "הרובע י\"ב",
			OfferTitle:           "פיצה משפחתית + תוספת",
			AmountPaid:           5900,
			PaymentStatusSummary: "אושר במערכת KuponFast",
			BuyerDisplay:         "Buyer One",
			BuyerTelegramID:      1111,
			Redemption:           store.CouponRedemption{Status: "redeemed", RedeemedAt: &redeemedAt},
		},
	})

	for _, want := range []string{
		"מימוש קופון",
		"PB-20260609180338",
		"הרובע י&#34;ב",
		"פיצה משפחתית + תוספת",
		"אושר במערכת KuponFast",
		"Telegram ID: <code>1111</code>",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected redemption message to contain %q, got %q", want, message)
		}
	}
}

func TestFormatAdminCouponRedemptionConfirmMessageForRepeat(t *testing.T) {
	redeemedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	message := formatAdminCouponRedemptionMessage(store.CouponRedemptionConfirmResult{
		FirstRedeem: false,
		Preview: store.CouponRedemptionPreview{
			OrderNumber: "PB-20260609180338",
			Redemption:  store.CouponRedemption{Status: "redeemed", RedeemedAt: &redeemedAt},
		},
	})

	if !strings.Contains(message, "ניסיון מימוש חוזר") {
		t.Fatalf("expected repeat redeem title, got %q", message)
	}
	if strings.Contains(message, "Telegram ID") {
		t.Fatalf("did not expect missing buyer id line, got %q", message)
	}
}

func TestFormatRestaurantCouponRedemptionMessage(t *testing.T) {
	redeemedAt := time.Date(2026, 6, 9, 18, 15, 0, 0, time.UTC)
	message := formatRestaurantCouponRedemptionMessage(store.CouponRedemptionConfirmResult{
		FirstRedeem: true,
		Preview: store.CouponRedemptionPreview{
			OrderNumber:          "PB-20260609180338",
			MerchantName:         "הרובע י\"ב",
			OfferTitle:           "פיצה משפחתית + תוספת",
			AmountPaid:           5900,
			PaymentStatusSummary: "אושר במערכת KuponFast",
			Redemption:           store.CouponRedemption{Status: "redeemed", RedeemedAt: &redeemedAt},
		},
	})

	for _, want := range []string{
		"קופון מומש בהצלחה",
		"PB-20260609180338",
		"סכום ששולם",
		"אושר במערכת KuponFast",
		"המימוש נרשם במערכת KuponFast",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected restaurant message to contain %q, got %q", want, message)
		}
	}
}

func TestRenderCouponTelegramMessageRendersDirectCoupon(t *testing.T) {
	original := int64(7400)
	total := int64(20)
	sold := int64(0)

	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:     "La Capriza",
		AreaLabel:        "Ashdod",
		Category:         couponCategoryPizza,
		Title:            "Family pizza + topping",
		OriginalPriceILS: &original,
		CouponPriceILS:   5900,
		PayBoxURL:        "https://paybox.example/lacapriza",
		TotalQuantity:    &total,
		SoldQuantity:     &sold,
		ValidTodayOnly:   true,
		RedemptionInfo:   "Show the QR at the counter",
		Status:           couponStatusActive,
		DisplayVariant:   couponDisplayVariantDirect,
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	if msg.ParseMode != tgbotapi.ModeHTML {
		t.Fatalf("expected HTML parse mode, got %q", msg.ParseMode)
	}
	if !strings.Contains(msg.Text, "<s>74₪</s>") || !strings.Contains(msg.Text, "59₪") {
		t.Fatalf("expected price anchor in message, got %q", msg.Text)
	}
	if len(msg.ReplyMarkup.InlineKeyboard) != 1 || len(msg.ReplyMarkup.InlineKeyboard[0]) != 1 {
		t.Fatalf("expected one CTA button, got %#v", msg.ReplyMarkup)
	}
	if msg.ReplyMarkup.InlineKeyboard[0][0].URL == nil || *msg.ReplyMarkup.InlineKeyboard[0][0].URL != "https://paybox.example/lacapriza" {
		t.Fatalf("expected paybox url button, got %#v", msg.ReplyMarkup.InlineKeyboard[0][0].URL)
	}
}

func TestRenderCouponTelegramMessageUsesLiveScarcityCouponCopy(t *testing.T) {
	available := int64(8)
	original := int64(7400)

	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:      "לקפריזה | רובע י\"ב",
		Category:          couponCategoryPizza,
		Title:             "פיצה משפחתית + תוספת",
		OriginalPriceILS:  &original,
		CouponPriceILS:    5900,
		PayBoxURL:         "https://paybox.example/lacapriza",
		AvailableQuantity: &available,
		Status:            couponStatusActive,
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	for _, want := range []string{
		"🍕 ערב פיצה?",
		"לקפריזה | רובע י&#34;ב",
		"פיצה משפחתית + תוספת",
		"74₪ → 59₪",
		"⚠ נשארו 8 קופונים",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("expected %q in message, got %q", want, msg.Text)
		}
	}
	if strings.Contains(msg.Text, "👇 קנה עכשיו") {
		t.Fatalf("did not expect duplicated CTA in body, got %q", msg.Text)
	}
	if msg.ReplyMarkup.InlineKeyboard[0][0].Text != couponMessagingConfig.VariantCTALabels[couponDisplayVariantScarcity] {
		t.Fatalf("expected scarcity CTA label, got %q", msg.ReplyMarkup.InlineKeyboard[0][0].Text)
	}
}

func TestRenderCouponTelegramMessageDoesNotFakeScarcityWhenQuantityUnknown(t *testing.T) {
	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:   "Cafe",
		Category:       couponCategoryCafe,
		Title:          "Coffee and pastry",
		CouponPriceILS: 2900,
		PayBoxURL:      "https://paybox.example/cafe",
		Status:         couponStatusActive,
		DisplayVariant: couponDisplayVariantDirect,
		RedemptionInfo: "Use at the counter",
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	if strings.Contains(msg.Text, "\u05e0\u05e9\u05d0\u05e8\u05d5") {
		t.Fatalf("did not expect fake scarcity line, got %q", msg.Text)
	}
}

func TestRenderCouponTelegramMessageRendersPriceWithoutOriginalPrice(t *testing.T) {
	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:   "Cafe",
		Category:       couponCategoryCafe,
		Title:          "Coffee and pastry",
		CouponPriceILS: 2900,
		PayBoxURL:      "https://paybox.example/cafe",
		Status:         couponStatusActive,
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	if strings.Contains(msg.Text, "<s>") {
		t.Fatalf("did not expect struck-through original price, got %q", msg.Text)
	}
	if !strings.Contains(msg.Text, "29₪") {
		t.Fatalf("expected coupon price in message, got %q", msg.Text)
	}
}

func TestRenderCouponTelegramMessageRendersSoldOutState(t *testing.T) {
	available := int64(0)
	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:      "La Capriza",
		Category:          couponCategoryPizza,
		Title:             "Family pizza + topping",
		CouponPriceILS:    5900,
		PayBoxURL:         "https://paybox.example/lacapriza",
		AvailableQuantity: &available,
		Status:            couponStatusSoldOut,
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	if !strings.Contains(msg.Text, "\u05d0\u05d6\u05dc") {
		t.Fatalf("expected sold-out copy, got %q", msg.Text)
	}
}

func TestRenderCouponTelegramMessageRendersExpiredState(t *testing.T) {
	expiredAt := time.Date(2026, 5, 25, 20, 0, 0, 0, time.UTC)
	msg := renderCouponTelegramMessageAt(CouponMessageData{
		BusinessName:   "La Capriza",
		Category:       couponCategoryPizza,
		Title:          "Family pizza + topping",
		CouponPriceILS: 5900,
		PayBoxURL:      "https://paybox.example/lacapriza",
		ValidUntil:     &expiredAt,
		Status:         couponStatusActive,
	}, time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC))

	if !strings.Contains(msg.Text, "\u05d4\u05e1\u05ea\u05d9\u05d9\u05dd") {
		t.Fatalf("expected expired copy, got %q", msg.Text)
	}
}

func TestCouponMessageDataFromOfferUsesAvailableInventoryAsRealScarcity(t *testing.T) {
	offer := store.Offer{
		ID:                 77,
		MerchantName:       "Pizza House",
		Title:              "Family pizza",
		Description:        "One large pie",
		PriceAmount:        5900,
		PaymentLink:        "https://paybox.example/pizza",
		AvailableCodeCount: 4,
		Status:             "active",
	}

	coupon := couponMessageDataFromOffer(offer)
	if coupon.AvailableQuantity == nil || *coupon.AvailableQuantity != 4 {
		t.Fatalf("expected available quantity from offer, got %#v", coupon.AvailableQuantity)
	}
	if coupon.Status != couponStatusActive {
		t.Fatalf("expected active status, got %q", coupon.Status)
	}
	if coupon.OriginalPriceILS != nil {
		t.Fatalf("did not expect original price without price anchor metadata, got %#v", coupon.OriginalPriceILS)
	}
}

func TestCouponMessageDataFromOfferParsesPriceAnchorMetadata(t *testing.T) {
	offer := store.Offer{
		MerchantName:       "לקפריזה | רובע י\"ב",
		Title:              "פיצה משפחתית + תוספת",
		Description:        "74₪ → 59₪",
		PriceAmount:        5900,
		PaymentLink:        "https://paybox.example/lacapriza",
		AvailableCodeCount: 8,
		Status:             "active",
	}

	coupon := couponMessageDataFromOffer(offer)
	if coupon.OriginalPriceILS == nil || *coupon.OriginalPriceILS != 7400 {
		t.Fatalf("expected parsed original price, got %#v", coupon.OriginalPriceILS)
	}
	if coupon.OfferDescription != "" {
		t.Fatalf("expected price-anchor metadata to stay out of offer description, got %q", coupon.OfferDescription)
	}
}

func TestBotServiceSupportChatURLPrefersTelegramAdminID(t *testing.T) {
	service := &BotService{adminUserIDs: []int64{8319213106}}

	if got := service.supportChatURL("@admin"); got != "tg://user?id=8319213106" {
		t.Fatalf("expected telegram direct chat url, got %q", got)
	}
}

func TestBotServiceSupportChatURLFallsBackToUsername(t *testing.T) {
	service := &BotService{}

	if got := service.supportChatURL("@adminsupport"); got != "https://t.me/adminsupport" {
		t.Fatalf("expected username support url, got %q", got)
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
