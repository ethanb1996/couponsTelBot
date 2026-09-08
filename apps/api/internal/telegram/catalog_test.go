package telegram

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
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
	first, duplicate, err := catalog.Add("Old title\nhttps://EXAMPLE.com/deal/?utm_source=one", 10, time.Now())
	if err != nil || duplicate {
		t.Fatalf("unexpected first add: duplicate=%v err=%v", duplicate, err)
	}
	second, duplicate, err := catalog.Add("New title\nhttps://example.com/deal?utm_source=two", 11, time.Now())
	if err != nil || !duplicate {
		t.Fatalf("expected duplicate refresh: duplicate=%v err=%v", duplicate, err)
	}
	if first.ID != second.ID || len(catalog.List()) != 1 || catalog.List()[0].Title != "New title" {
		t.Fatalf("duplicate was not refreshed: %#v", catalog.List())
	}
}

func TestCatalogDeduplicatesNormalizedTitleWithoutURL(t *testing.T) {
	catalog, err := NewCatalog(filepath.Join(t.TempDir(), "offers.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = catalog.Add("🔥 Pizza Deal!\nFirst copy", 1, time.Now())
	_, duplicate, err := catalog.Add("Pizza Deal\nUpdated copy", 2, time.Now())
	if err != nil || !duplicate || len(catalog.List()) != 1 {
		t.Fatalf("expected title duplicate, got duplicate=%v offers=%#v err=%v", duplicate, catalog.List(), err)
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
