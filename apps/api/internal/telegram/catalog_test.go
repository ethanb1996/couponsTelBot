package telegram

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCatalogKeepsLatestEightDistinctOffers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "offers.json")
	catalog, err := NewCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 9; i++ {
		_, _, err := catalog.Add(fmt.Sprintf("Coupon %d\nhttps://example.com/coupon/%d?utm_source=telegram", i, i), i, time.Unix(int64(i), 0))
		if err != nil {
			t.Fatal(err)
		}
	}
	offers := catalog.List()
	if len(offers) != 8 {
		t.Fatalf("expected 8 offers, got %d", len(offers))
	}
	if offers[0].Title != "Coupon 9" || offers[7].Title != "Coupon 2" {
		t.Fatalf("unexpected order: %#v", offers)
	}
}

func TestCatalogRefreshesDuplicateURL(t *testing.T) {
	catalog, err := NewCatalog(filepath.Join(t.TempDir(), "offers.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, duplicate, err := catalog.Add("Pizza Place\n50% off\nhttps://EXAMPLE.com/deal/?utm_source=one", 10, time.Now())
	if err != nil || duplicate {
		t.Fatalf("unexpected first add: duplicate=%v err=%v", duplicate, err)
	}
	second, duplicate, err := catalog.Add("Pizza Place!\n50% OFF\nhttps://example.com/deal?utm_source=two", 11, time.Now())
	if err != nil || !duplicate {
		t.Fatalf("expected duplicate refresh: duplicate=%v err=%v", duplicate, err)
	}
	if first.ID != second.ID || len(catalog.List()) != 1 || catalog.List()[0].Title != "Pizza Place!" {
		t.Fatalf("duplicate was not refreshed: %#v", catalog.List())
	}
}

func TestCatalogKeepsDifferentOffersFromSameRestaurant(t *testing.T) {
	catalog, err := NewCatalog(filepath.Join(t.TempDir(), "offers.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = catalog.Add("🔥 Pizza Place\n1+1 on pizza", 1, time.Now())
	_, duplicate, err := catalog.Add("Pizza Place\n30% off pasta", 2, time.Now())
	if err != nil || duplicate || len(catalog.List()) != 2 {
		t.Fatalf("expected distinct offers, got duplicate=%v offers=%#v err=%v", duplicate, catalog.List(), err)
	}
}

func TestCatalogKeepsDifferentOffersThatShareAHomepage(t *testing.T) {
	catalog, err := NewCatalog(filepath.Join(t.TempDir(), "offers.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = catalog.Add("Pizza Place\n1+1 on pizza\nhttps://example.com", 1, time.Now())
	_, duplicate, err := catalog.Add("Pizza Place\n30% off pasta\nhttps://example.com", 2, time.Now())
	if err != nil || duplicate || len(catalog.List()) != 2 {
		t.Fatalf("expected distinct homepage offers, got duplicate=%v offers=%#v err=%v", duplicate, catalog.List(), err)
	}
}

func TestOfferButtonLabel(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "restaurant and voucher price",
			text: "🍣 קופון חדש ל־Japan Japan\nשובר בשווי 100 ₪ ב־79 ₪ בלבד — חיסכון של 21 ₪ 🎉\nלמימוש חד-פעמי",
			want: "Japan Japan · 100 ₪ ב־79 ₪",
		},
		{name: "percentage", text: "מבצע ב־Burger Bar\n25% הנחה על כל התפריט", want: "Burger Bar · 25% הנחה על כל התפריט"},
		{name: "one plus one", text: "דיל ב־Sushi House\n1+1 על כל הרולים", want: "Sushi House · 1+1 על כל הרולים"},
		{name: "fallback", text: "🍕 Family Pizza", want: "Family Pizza"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := offerButtonLabel(tt.text)
			if got != tt.want {
				t.Fatalf("offerButtonLabel()=%q, want %q", got, tt.want)
			}
			if utf8.RuneCountInString(got) > 40 {
				t.Fatalf("label is too long: %q", got)
			}
		})
	}
}

func TestOfferButtonLabelPreservesRestaurantAndEssenceWhenLong(t *testing.T) {
	got := offerButtonLabel("קופון חדש ל־A Very Long Restaurant Name\nA very long essential offer with several unnecessary words")
	if !strings.HasPrefix(got, "A Very Long Resta… · ") || !strings.Contains(got, "essent") {
		t.Fatalf("unexpected compact label: %q", got)
	}
	if utf8.RuneCountInString(got) > 40 {
		t.Fatalf("label is too long: %q", got)
	}
}

func TestCatalogPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "offers.json")
	catalog, _ := NewCatalog(path)
	_, _, _ = catalog.Add("Persisted offer", 3, time.Now())
	if err := catalog.SetMenuMessageID(99); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.List()) != 1 || reloaded.List()[0].Title != "Persisted offer" {
		t.Fatalf("unexpected reloaded catalog: %#v", reloaded.List())
	}
	if reloaded.MenuMessageID() != 99 {
		t.Fatalf("expected persisted menu message id, got %d", reloaded.MenuMessageID())
	}
}

func TestParseOfferStartPayload(t *testing.T) {
	if id, ok := parseOfferStartPayload("offer_abc123"); !ok || id != "abc123" {
		t.Fatalf("unexpected payload parse: id=%q ok=%v", id, ok)
	}
	if _, ok := parseOfferStartPayload("other_abc123"); ok {
		t.Fatal("expected unrelated payload to be rejected")
	}
}

func TestIsMessageNotModified(t *testing.T) {
	if !isMessageNotModified(fmt.Errorf("Bad Request: message is not modified")) {
		t.Fatal("expected Telegram's unchanged-message response to be recognized")
	}
	if isMessageNotModified(fmt.Errorf("Bad Request: message to edit not found")) {
		t.Fatal("expected unrelated edit error to remain an error")
	}
}

func TestIsMissingMenuMessage(t *testing.T) {
	for _, message := range []string{
		"Bad Request: message to edit not found",
		"Bad Request: message can't be edited",
		"Bad Request: MESSAGE_ID_INVALID",
	} {
		if !isMissingMenuMessage(fmt.Errorf("%s", message)) {
			t.Fatalf("expected missing-menu error to be recognized: %q", message)
		}
	}
	if isMissingMenuMessage(fmt.Errorf("network timeout")) {
		t.Fatal("expected transient error not to replace the menu")
	}
}
