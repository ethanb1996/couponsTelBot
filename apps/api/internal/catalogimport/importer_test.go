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
				},
				{
					"request": {
						"url": "https://back.behatsdaa.org.il/api/category/GetCategoryProducts?categoryId=4244"
					},
					"response": {
						"status": 200,
						"content": {
							"mimeType": "application/json",
							"text": "{\"status\":true,\"data\":{\"categoryId\":4244,\"categoryName\":\"Gift Voucher 150\",\"supplierName\":\"Supplier A\",\"termsOfUse\":\"Valid until 28/02/2026\",\"redimType\":\"Show at checkout\",\"variants\":[{\"name\":\"Gift Voucher 74\",\"price\":74,\"kupaPrice\":150},{\"name\":\"Gift Voucher 148\",\"price\":148,\"kupaPrice\":300}],\"business\":{\"name\":\"Merchant A\",\"webSite\":\"https://merchant.example.com\"}}}"
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
		t.Fatalf("expected first source sale price 7400, got %d", records[0].SalePriceAmount)
	}

	if records[0].ResellPriceAmount != 9934 {
		t.Fatalf("expected first resale price 9934, got %d", records[0].ResellPriceAmount)
	}

	if records[0].CouponValueAmount != 15000 {
		t.Fatalf("expected coupon value 15000, got %d", records[0].CouponValueAmount)
	}

	if records[0].Title != "Gift Voucher 74" {
		t.Fatalf("expected variant title, got %q", records[0].Title)
	}

	if records[0].Status != "draft" {
		t.Fatalf("expected draft status when detail exists, got %q", records[0].Status)
	}

	if records[0].PhotoKey == "" || !strings.HasSuffix(records[0].PhotoKey, ".jpg") {
		t.Fatalf("expected jpg photo key, got %q", records[0].PhotoKey)
	}

	if records[0].ExpirySummary != "Valid through 28/02/2026" {
		t.Fatalf("unexpected expiry summary %q", records[0].ExpirySummary)
	}
}

func TestExpandCategoryComputesResalePriceWithDynamicProfitAndPayPalFees(t *testing.T) {
	category := sourceCategory{
		CategoryID:   4244,
		CategoryName: "Gift Voucher 150",
		SupplierName: "Supplier A",
		Prices:       []float64{74},
		Business: sourceBusiness{
			Name: "Merchant A",
		},
		Variants: []sourceVariant{
			{
				Name:      "Gift Voucher 74",
				Price:     74,
				KupaPrice: 150,
			},
		},
	}

	records, rejectedImportKeys, err := expandCategory(category, category, true, Options{
		PayPalPercentFeeRate: 0.0349,
		PayPalFixedFeeAmount: 49,
	})
	if err != nil {
		t.Fatalf("expandCategory returned error: %v", err)
	}

	if len(rejectedImportKeys) != 0 {
		t.Fatalf("expected no rejected import keys, got %v", rejectedImportKeys)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 listing record, got %d", len(records))
	}

	if records[0].CouponValueAmount != 15000 {
		t.Fatalf("expected coupon value 15000, got %d", records[0].CouponValueAmount)
	}

	if records[0].SalePriceAmount != 7400 {
		t.Fatalf("expected source sale price 7400, got %d", records[0].SalePriceAmount)
	}

	if records[0].ResellPriceAmount != 10204 {
		t.Fatalf("expected computed resale price 10204, got %d", records[0].ResellPriceAmount)
	}
}

func TestExpandCategoryRejectsListingsWhoseResalePriceReachesCouponValue(t *testing.T) {
	category := sourceCategory{
		CategoryID:   66294,
		CategoryName: "Pizza Deal 30",
		SupplierName: "Supplier A",
		Prices:       []float64{29},
		Business: sourceBusiness{
			Name: "Merchant A",
		},
		Variants: []sourceVariant{
			{
				Name:      "Pizza Deal 29",
				Price:     29,
				KupaPrice: 30,
			},
		},
	}

	records, rejectedImportKeys, err := expandCategory(category, category, true, Options{
		PayPalPercentFeeRate: 0.0349,
		PayPalFixedFeeAmount: 49,
	})
	if err != nil {
		t.Fatalf("expandCategory returned error: %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("expected no listing records, got %d", len(records))
	}

	if len(rejectedImportKeys) != 1 {
		t.Fatalf("expected 1 rejected import key, got %d", len(rejectedImportKeys))
	}

	if rejectedImportKeys[0] != "behatsdaa:66294:2900" {
		t.Fatalf("unexpected rejected import key %q", rejectedImportKeys[0])
	}
}

func TestParseHARMarksMissingDetailAsPreview(t *testing.T) {
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
							"text": "{\"status\":true,\"data\":{\"subCategories\":[{\"categoryId\":66294,\"categoryName\":\"Pizza Deal 60\",\"supplierName\":\"Supplier A\",\"description\":\"Starts at 29\",\"shortDescription\":\"\",\"categoryHTML\":\"<p>Main description</p>\",\"termsOfUse\":\"Valid until 28/02/2026\",\"redimType\":\"Show at checkout\",\"prices\":[29,34],\"images\":[{\"file\":\"NewUploads/example.jpg\",\"externalUrl\":\"\"}],\"business\":{\"name\":\"Merchant A\"}}]}}"
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

	for _, record := range records {
		if record.Status != "preview" {
			t.Fatalf("expected preview status for missing detail, got %q", record.Status)
		}
		if record.FinalSaleDisclosure != previewFinalSaleDisclosure {
			t.Fatalf("expected preview disclosure, got %q", record.FinalSaleDisclosure)
		}
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

func TestCalculateResalePriceAmount(t *testing.T) {
	salePriceAmount, ok := calculateResalePriceAmount(15000, 7400, 0.0349, 49)
	if !ok {
		t.Fatal("expected resale price calculation to succeed")
	}

	if salePriceAmount != 10204 {
		t.Fatalf("expected sale price 10204, got %d", salePriceAmount)
	}
}

func TestCleanTextStripsHTML(t *testing.T) {
	got := cleanText("<div>Hello<br />world&nbsp;&amp; friends</div>")
	if got != "Hello world & friends" {
		t.Fatalf("unexpected cleaned text %q", got)
	}
}
