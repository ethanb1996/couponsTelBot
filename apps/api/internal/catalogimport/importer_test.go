package catalogimport

import (
	"strings"
	"testing"
)

func TestParseHARExpandsMultiPriceCategory(t *testing.T) {
	const sampleHAR = `{
		"log": {
			"entries": [
				{
					"request": {
						"url": "https://back.behatsdaa.org.il/api/category/GetCategoryById?categoryId=6982"
					},
					"response": {
						"status": 200,
						"content": {
							"mimeType": "application/json",
							"text": "{\"status\":true,\"data\":{\"subCategories\":[{\"categoryId\":4244,\"categoryName\":\"Gift Voucher 150\",\"supplierName\":\"Supplier A\",\"description\":\"Starts at 74\",\"shortDescription\":\"\",\"categoryHTML\":\"<p>Main description</p>\",\"termsOfUse\":\"Valid until 28/02/2026\",\"redimType\":\"Show at checkout\",\"prices\":[74,148],\"images\":[{\"file\":\"NewUploads/example.jpg\",\"externalUrl\":\"\"}],\"business\":{\"name\":\"Merchant A\"}}]}}"
						}
					}
				}
			]
		}
	}`

	records, err := parseHAR(strings.NewReader(sampleHAR))
	if err != nil {
		t.Fatalf("parseHAR returned error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 listing records, got %d", len(records))
	}

	if records[0].SalePriceAmount != 7400 {
		t.Fatalf("expected first sale price 7400, got %d", records[0].SalePriceAmount)
	}

	if records[0].CouponValueAmount != 15000 {
		t.Fatalf("expected coupon value 15000, got %d", records[0].CouponValueAmount)
	}

	if records[0].PhotoKey == "" || !strings.HasSuffix(records[0].PhotoKey, ".jpg") {
		t.Fatalf("expected jpg photo key, got %q", records[0].PhotoKey)
	}

	if records[0].ExpirySummary != "Valid through 28/02/2026" {
		t.Fatalf("unexpected expiry summary %q", records[0].ExpirySummary)
	}
}

func TestInferCouponValueFallsBackToSalePrice(t *testing.T) {
	category := sourceCategory{
		CategoryName:     "Breakfast package",
		ShortDescription: "Morning meal",
		Description:      "Starts at 105",
	}

	value := inferCouponValueAmount(category, 10500)
	if value != 10500 {
		t.Fatalf("expected fallback sale price, got %d", value)
	}
}

func TestCleanTextStripsHTML(t *testing.T) {
	got := cleanText("<div>Hello<br />world&nbsp;&amp; friends</div>")
	if got != "Hello world & friends" {
		t.Fatalf("unexpected cleaned text %q", got)
	}
}
