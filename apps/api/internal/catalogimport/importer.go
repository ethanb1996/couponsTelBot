package catalogimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/jackc/pgx/v5"
)

const (
	defaultCurrencyCode        = "ILS"
	defaultSourceType          = "manual_source"
	defaultCreatedByAdminID    = "catalog_import"
	defaultFinalSaleDisclosure = "Imported draft listing. Review transferability, expiry, redemption terms, and final-sale wording before publishing."
	previewFinalSaleDisclosure = "Preview import only. Coupon detail could not be fully extracted. Review and complete this listing before publishing."
	defaultImageBaseURL        = "https://pics.k4a.co.il/share/"
	defaultImportHTTPUserAgent = "couponsTelBot-importer/1.0"
	defaultHTTPTimeout         = 45 * time.Second
	defaultCategoryDetailURL   = "https://back.behatsdaa.org.il/api/category/GetCategoryProducts?categoryId=%d"
	defaultCategoryOrigin      = "https://www.behatsdaa.org.il"
	defaultOrganizationID      = "20"
	sourceSystemName           = "behatsdaa"
)

var (
	tagPattern        = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern      = regexp.MustCompile(`[\s\p{Zs}]+`)
	numberPattern     = regexp.MustCompile(`\d+(?:[.,]\d{1,2})?`)
	expiryDatePattern = regexp.MustCompile(`\b\d{1,2}[./-]\d{1,2}[./-]\d{2,4}\b`)
)

type Options struct {
	InputPath            string
	PhotosDir            string
	DryRun               bool
	HTTPClient           *http.Client
	PayPalPercentFeeRate float64
	PayPalFixedFeeAmount int64
}

type Summary struct {
	SourceCount          int
	ListingCount         int
	PreviewListingCount  int
	SkippedListingCount  int
	RemovedListingCount  int
	DownloadedPhotoCount int
	UpdatedListingCount  int
	CreatedListingCount  int
}

type listingRecord struct {
	ImportKey              string
	SourceName             string
	MerchantName           string
	Title                  string
	Description            string
	CouponValueAmount      int64
	SalePriceAmount        int64
	CurrencyCode           string
	ExpirySummary          string
	TermsSummary           string
	RedemptionInstructions string
	FinalSaleDisclosure    string
	PhotoKey               string
	PhotoURL               string
	Status                 string
}

type parsedCatalog struct {
	Categories         []sourceCategory
	DetailByCategoryID map[int64]sourceCategory
}

type harEnvelope struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	Request struct {
		URL string `json:"url"`
	} `json:"request"`
	Response struct {
		Status  int `json:"status"`
		Content struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"content"`
	} `json:"response"`
}

type categoryPayload struct {
	Status bool `json:"status"`
	Data   struct {
		SubCategories []sourceCategory `json:"subCategories"`
	} `json:"data"`
}

type sourceCategory struct {
	CategoryID       int64             `json:"categoryId"`
	CategoryName     string            `json:"categoryName"`
	SupplierName     string            `json:"supplierName"`
	Description      string            `json:"description"`
	ShortDescription string            `json:"shortDescription"`
	CategoryHTML     string            `json:"categoryHTML"`
	CategoryURL      string            `json:"categoryUrl"`
	TermsOfUse       string            `json:"termsOfUse"`
	RedimType        string            `json:"redimType"`
	CampaignDetails  string            `json:"campaignDetails"`
	Remarks          string            `json:"remarks"`
	MinimumInventory string            `json:"minimumInventoryForSale"`
	MustKnow         string            `json:"mustKnow"`
	Prices           []float64         `json:"prices"`
	Images           []sourceImage     `json:"images"`
	Variants         []sourceVariant   `json:"variants"`
	Locations        []sourceLocation  `json:"locations"`
	DateRanges       []sourceDateRange `json:"dateRanges"`
	Business         sourceBusiness    `json:"business"`
}

type sourceImage struct {
	File        string `json:"file"`
	ExternalURL string `json:"externalUrl"`
}

type sourceBusiness struct {
	Name    string `json:"name"`
	WebSite string `json:"webSite"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

type sourceVariant struct {
	Name          string   `json:"name"`
	BarCode       string   `json:"barCode"`
	ExpireDate    string   `json:"expireDate"`
	EndDate       string   `json:"endDate"`
	OrderLimit    int      `json:"orderLimit"`
	MonthlyLimit  int      `json:"monthlyLimit"`
	YearlyLimit   int      `json:"yearlyLimit"`
	GeneralLimit  int      `json:"generalLimit"`
	Price         float64  `json:"price"`
	GiftCardValue *float64 `json:"giftCardValue"`
	KupaPrice     float64  `json:"kupaPrice"`
}

type sourceLocation struct {
	FriendlyName string `json:"friendlyName"`
	Address      string `json:"address"`
	CityName     string `json:"cityName"`
	OpenHours    string `json:"openHours"`
	Phone        string `json:"phone"`
}

type sourceDateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type categoryDetailPayload struct {
	Status bool           `json:"status"`
	Data   sourceCategory `json:"data"`
}

func Run(ctx context.Context, db *store.Postgres, options Options) (Summary, error) {
	if strings.TrimSpace(options.InputPath) == "" {
		return Summary{}, errors.New("input path is required")
	}
	if strings.TrimSpace(options.PhotosDir) == "" {
		return Summary{}, errors.New("photos dir is required")
	}

	file, err := os.Open(options.InputPath)
	if err != nil {
		return Summary{}, fmt.Errorf("open HAR file: %w", err)
	}
	defer file.Close()

	catalog, err := parseHARCatalog(file)
	if err != nil {
		return Summary{}, err
	}

	detailResolver := newCategoryDetailResolver(nil, catalog.DetailByCategoryID)
	records, rejectedImportKeys, err := buildListingRecords(ctx, catalog.Categories, detailResolver, options)
	if err != nil {
		return Summary{}, err
	}

	if options.DryRun {
		return Summary{
			SourceCount:         len(uniqueSourceNames(records)),
			ListingCount:        len(records),
			PreviewListingCount: countPreviewRecords(records),
			SkippedListingCount: len(rejectedImportKeys),
		}, nil
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	if err := os.MkdirAll(options.PhotosDir, 0o755); err != nil {
		return Summary{}, fmt.Errorf("create photos dir: %w", err)
	}

	summary := Summary{
		SourceCount:         len(uniqueSourceNames(records)),
		ListingCount:        len(records),
		PreviewListingCount: countPreviewRecords(records),
		SkippedListingCount: len(rejectedImportKeys),
	}

	detailResolver = newCategoryDetailResolver(client, catalog.DetailByCategoryID)
	records, rejectedImportKeys, err = buildListingRecords(ctx, catalog.Categories, detailResolver, options)
	if err != nil {
		return Summary{}, err
	}

	photoDownloaded := make(map[string]bool)
	for _, record := range records {
		if record.PhotoKey == "" || record.PhotoURL == "" {
			continue
		}
		downloaded, err := ensurePhoto(ctx, client, options.PhotosDir, record.PhotoKey, record.PhotoURL)
		if err != nil {
			return Summary{}, err
		}
		if downloaded && !photoDownloaded[record.PhotoKey] {
			photoDownloaded[record.PhotoKey] = true
			summary.DownloadedPhotoCount++
		}
	}

	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Summary{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	sourceIDs := make(map[string]int64)
	for _, record := range records {
		sourceID, ok := sourceIDs[record.SourceName]
		if !ok {
			sourceID, err = ensureSource(ctx, tx, record.SourceName)
			if err != nil {
				return Summary{}, err
			}
			sourceIDs[record.SourceName] = sourceID
		}

		created, err := upsertListing(ctx, tx, record, sourceID)
		if err != nil {
			return Summary{}, err
		}
		if created {
			summary.CreatedListingCount++
		} else {
			summary.UpdatedListingCount++
		}
	}

	removedCount, err := suppressIneligibleImportedListings(ctx, tx, rejectedImportKeys)
	if err != nil {
		return Summary{}, err
	}
	summary.RemovedListingCount = removedCount

	if err := tx.Commit(ctx); err != nil {
		return Summary{}, err
	}

	return summary, nil
}

func parseHAR(reader io.Reader) ([]listingRecord, error) {
	catalog, err := parseHARCatalog(reader)
	if err != nil {
		return nil, err
	}
	records, _, err := buildListingRecords(context.Background(), catalog.Categories, newCategoryDetailResolver(nil, catalog.DetailByCategoryID), Options{})
	return records, err
}

func parseHARCatalog(reader io.Reader) (parsedCatalog, error) {
	var har harEnvelope
	if err := json.NewDecoder(reader).Decode(&har); err != nil {
		return parsedCatalog{}, fmt.Errorf("decode HAR: %w", err)
	}

	var payload categoryPayload
	found := false
	detailByCategoryID := make(map[int64]sourceCategory)
	for _, entry := range har.Log.Entries {
		if entry.Response.Status != http.StatusOK {
			continue
		}
		if !strings.Contains(strings.ToLower(entry.Response.Content.MimeType), "json") {
			continue
		}
		if strings.TrimSpace(entry.Response.Content.Text) == "" {
			continue
		}

		switch {
		case strings.Contains(entry.Request.URL, "/api/category/GetCategoryById"):
			if err := json.Unmarshal([]byte(entry.Response.Content.Text), &payload); err != nil {
				return parsedCatalog{}, fmt.Errorf("decode category payload: %w", err)
			}
			found = true
		case strings.Contains(entry.Request.URL, "/api/category/GetCategoryProducts"):
			var detailPayload categoryDetailPayload
			if err := json.Unmarshal([]byte(entry.Response.Content.Text), &detailPayload); err != nil {
				continue
			}
			if !detailPayload.Status || detailPayload.Data.CategoryID == 0 {
				continue
			}
			detailByCategoryID[detailPayload.Data.CategoryID] = detailPayload.Data
		}
	}

	if !found {
		return parsedCatalog{}, errors.New("no GetCategoryById payload found in HAR")
	}
	if !payload.Status {
		return parsedCatalog{}, errors.New("category payload indicates unsuccessful response")
	}

	return parsedCatalog{
		Categories:         payload.Data.SubCategories,
		DetailByCategoryID: detailByCategoryID,
	}, nil
}

func buildListingRecords(ctx context.Context, categories []sourceCategory, detailResolver func(context.Context, int64) (sourceCategory, bool), options Options) ([]listingRecord, []string, error) {
	records := make([]listingRecord, 0)
	rejectedImportKeys := make([]string, 0)
	for _, category := range categories {
		detailCategory, detailAvailable := sourceCategory{}, false
		if detailResolver != nil {
			detailCategory, detailAvailable = detailResolver(ctx, category.CategoryID)
		}
		categoryRecords, categoryRejectedImportKeys, err := expandCategory(category, detailCategory, detailAvailable, options)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, categoryRecords...)
		rejectedImportKeys = append(rejectedImportKeys, categoryRejectedImportKeys...)
	}
	return records, rejectedImportKeys, nil
}

func expandCategory(category sourceCategory, detailCategory sourceCategory, detailAvailable bool, options Options) ([]listingRecord, []string, error) {
	if category.CategoryID == 0 {
		return nil, nil, errors.New("category id is required")
	}
	if len(category.Prices) == 0 {
		return nil, nil, fmt.Errorf("category %d has no prices", category.CategoryID)
	}

	enrichedCategory := mergeCategory(category, detailCategory, detailAvailable)

	sourceName := cleanText(firstNonEmpty(enrichedCategory.SupplierName, enrichedCategory.Business.Name))
	if sourceName == "" {
		sourceName = "Imported source"
	}

	merchantName := cleanText(firstNonEmpty(enrichedCategory.Business.Name, enrichedCategory.SupplierName))
	if merchantName == "" {
		merchantName = sourceName
	}

	baseTitle := cleanText(enrichedCategory.CategoryName)
	if baseTitle == "" {
		baseTitle = fmt.Sprintf("Imported listing %d", category.CategoryID)
	}

	photoURL, ext := primaryImage(enrichedCategory.Images)

	records := make([]listingRecord, 0, len(category.Prices))
	rejectedImportKeys := make([]string, 0)
	for _, price := range category.Prices {
		sourceCostAmount := priceToMinorUnits(price)
		if sourceCostAmount <= 0 {
			return nil, nil, fmt.Errorf("category %d has invalid price %.2f", category.CategoryID, price)
		}

		variant, variantMatched := matchVariantForSalePrice(enrichedCategory.Variants, sourceCostAmount)
		title := buildListingTitle(baseTitle, sourceCostAmount, len(category.Prices), variant, variantMatched)
		description := composeDescription(enrichedCategory, variant)
		termsSummary := composeTermsSummary(enrichedCategory, variant)
		redemptionInstructions := composeRedemptionInstructions(enrichedCategory)
		expirySummary := composeExpirySummary(enrichedCategory, variant, termsSummary, redemptionInstructions, description)

		importKey := buildImportKey(category.CategoryID, sourceCostAmount)
		photoKey := ""
		if photoURL != "" {
			photoKey = buildPhotoKey(category.CategoryID, sourceCostAmount, ext)
		}

		couponValueAmount := inferCouponValueAmount(enrichedCategory, sourceCostAmount)
		if valueFromVariant := inferCouponValueFromVariant(variant, sourceCostAmount); valueFromVariant > 0 {
			couponValueAmount = valueFromVariant
		}
		salePriceAmount, ok := calculateResalePriceAmount(
			couponValueAmount,
			sourceCostAmount,
			options.PayPalPercentFeeRate,
			options.PayPalFixedFeeAmount,
		)
		if !ok || salePriceAmount >= couponValueAmount {
			rejectedImportKeys = append(rejectedImportKeys, importKey)
			continue
		}

		status := "draft"
		finalSaleDisclosure := defaultFinalSaleDisclosure
		if !detailAvailable || (len(enrichedCategory.Variants) > 0 && !variantMatched) {
			status = "preview"
			finalSaleDisclosure = previewFinalSaleDisclosure
		}
		records = append(records, listingRecord{
			ImportKey:              importKey,
			SourceName:             sourceName,
			MerchantName:           merchantName,
			Title:                  title,
			Description:            description,
			CouponValueAmount:      couponValueAmount,
			SalePriceAmount:        salePriceAmount,
			CurrencyCode:           defaultCurrencyCode,
			ExpirySummary:          expirySummary,
			TermsSummary:           termsSummary,
			RedemptionInstructions: redemptionInstructions,
			FinalSaleDisclosure:    finalSaleDisclosure,
			PhotoKey:               photoKey,
			PhotoURL:               photoURL,
			Status:                 status,
		})
	}

	return records, rejectedImportKeys, nil
}

func calculateResalePriceAmount(couponValueAmount int64, sourceCostAmount int64, payPalPercentFeeRate float64, payPalFixedFeeAmount int64) (int64, bool) {
	if couponValueAmount <= 0 || sourceCostAmount < 0 || payPalFixedFeeAmount < 0 {
		return 0, false
	}
	if payPalPercentFeeRate < 0 {
		return 0, false
	}

	denominator := 3 - (2 * payPalPercentFeeRate)
	if denominator <= 0 {
		return 0, false
	}

	numerator := float64(couponValueAmount + (2 * sourceCostAmount) + (2 * payPalFixedFeeAmount))
	salePriceAmount := int64(math.Ceil(numerator / denominator))
	if salePriceAmount <= 0 {
		return 0, false
	}

	return salePriceAmount, true
}

func inferCouponValueAmount(category sourceCategory, salePriceAmount int64) int64 {
	candidates := append(
		extractCandidateValues(category.CategoryName),
		extractCandidateValues(category.ShortDescription)...,
	)
	candidates = append(candidates, extractCandidateValues(category.Description)...)

	best := int64(0)
	for _, candidate := range candidates {
		if candidate < salePriceAmount {
			continue
		}
		if candidate > salePriceAmount*5 {
			continue
		}
		if candidate > best {
			best = candidate
		}
	}
	if best > 0 {
		return best
	}
	return salePriceAmount
}

func extractCandidateValues(text string) []int64 {
	matches := numberPattern.FindAllString(text, -1)
	values := make([]int64, 0, len(matches))
	for _, match := range matches {
		if strings.Contains(match, ".") || strings.Contains(match, ",") {
			normalized := strings.ReplaceAll(match, ",", ".")
			if value, err := strconv.ParseFloat(normalized, 64); err == nil {
				values = append(values, priceToMinorUnits(value))
			}
			continue
		}
		if value, err := strconv.ParseInt(match, 10, 64); err == nil {
			values = append(values, value*100)
		}
	}
	return values
}

func extractExpirySummary(values ...string) string {
	for _, value := range values {
		match := expiryDatePattern.FindString(value)
		if match != "" {
			return "Valid through " + match
		}
	}
	return ""
}

func primaryImage(images []sourceImage) (string, string) {
	for _, image := range images {
		if strings.TrimSpace(image.ExternalURL) != "" {
			parsed, err := url.Parse(strings.TrimSpace(image.ExternalURL))
			if err != nil {
				continue
			}
			ext := path.Ext(parsed.Path)
			if ext == "" {
				ext = ".jpg"
			}
			return parsed.String(), ext
		}
		if strings.TrimSpace(image.File) != "" {
			trimmed := strings.TrimLeft(strings.TrimSpace(image.File), "/")
			ext := path.Ext(trimmed)
			if ext == "" {
				ext = ".jpg"
			}
			return defaultImageBaseURL + trimmed, ext
		}
	}
	return "", ""
}

func ensurePhoto(ctx context.Context, client *http.Client, photosDir string, photoKey string, photoURL string) (bool, error) {
	filePath := filepath.Join(photosDir, photoKey)
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		return false, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat photo %s: %w", photoKey, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, nil)
	if err != nil {
		return false, fmt.Errorf("build photo request %s: %w", photoURL, err)
	}
	req.Header.Set("User-Agent", defaultImportHTTPUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("download photo %s: %w", photoURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("download photo %s: unexpected status %d", photoURL, resp.StatusCode)
	}

	tempPath := filePath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return false, fmt.Errorf("create temp photo %s: %w", photoKey, err)
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write photo %s: %w", photoKey, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("close temp photo %s: %w", photoKey, closeErr)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("move photo %s into place: %w", photoKey, err)
	}

	return true, nil
}

func ensureSource(ctx context.Context, tx pgx.Tx, sourceName string) (int64, error) {
	var sourceID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM coupon_sources
		WHERE LOWER(source_name) = LOWER($1)
			AND source_type = $2
		ORDER BY id ASC
		LIMIT 1
	`, sourceName, defaultSourceType).Scan(&sourceID)
	if err == nil {
		return sourceID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("lookup source %q: %w", sourceName, err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO coupon_sources (
			source_name,
			source_type,
			rights_status,
			risk_rating,
			is_active
		) VALUES ($1, $2, 'unknown', 'medium', TRUE)
		RETURNING id
	`, sourceName, defaultSourceType).Scan(&sourceID)
	if err != nil {
		return 0, fmt.Errorf("create source %q: %w", sourceName, err)
	}
	return sourceID, nil
}

func upsertListing(ctx context.Context, tx pgx.Tx, record listingRecord, sourceID int64) (bool, error) {
	if sourceID == 0 {
		return false, errors.New("source id is required")
	}

	var created bool
	err := tx.QueryRow(ctx, `
		WITH existing AS (
			SELECT id
			FROM listings
			WHERE external_import_key = $1
		), upserted AS (
			INSERT INTO listings (
				merchant_name,
				title,
				description,
				coupon_value_amount,
				sale_price_amount,
				currency_code,
				expiry_summary,
				terms_summary,
				redemption_instructions,
				final_sale_disclosure_text,
				photo_key,
				external_import_key,
				status,
				created_by_admin_id
			) VALUES ($2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $1, $13, $14)
			ON CONFLICT (external_import_key) WHERE external_import_key <> ''
			DO UPDATE SET
				merchant_name = EXCLUDED.merchant_name,
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				coupon_value_amount = EXCLUDED.coupon_value_amount,
				sale_price_amount = EXCLUDED.sale_price_amount,
				currency_code = EXCLUDED.currency_code,
				expiry_summary = EXCLUDED.expiry_summary,
				terms_summary = EXCLUDED.terms_summary,
				redemption_instructions = EXCLUDED.redemption_instructions,
				final_sale_disclosure_text = EXCLUDED.final_sale_disclosure_text,
				photo_key = EXCLUDED.photo_key,
				status = EXCLUDED.status,
				created_by_admin_id = EXCLUDED.created_by_admin_id,
				updated_at = NOW()
			RETURNING id
		)
		SELECT NOT EXISTS (SELECT 1 FROM existing)
	`, record.ImportKey,
		record.MerchantName,
		record.Title,
		record.Description,
		record.CouponValueAmount,
		record.SalePriceAmount,
		record.CurrencyCode,
		record.ExpirySummary,
		record.TermsSummary,
		record.RedemptionInstructions,
		record.FinalSaleDisclosure,
		record.PhotoKey,
		record.Status,
		defaultCreatedByAdminID,
	).Scan(&created)
	if err != nil {
		return false, fmt.Errorf("upsert listing %s: %w", record.ImportKey, err)
	}

	return created, nil
}

func suppressIneligibleImportedListings(ctx context.Context, tx pgx.Tx, importKeys []string) (int, error) {
	if len(importKeys) == 0 {
		return 0, nil
	}

	var affectedCount int
	err := tx.QueryRow(ctx, `
		WITH doomed AS (
			SELECT id
			FROM listings
			WHERE external_import_key = ANY($1::TEXT[])
				AND external_import_key <> ''
		), deleted AS (
			DELETE FROM listings l
			USING doomed d
			WHERE l.id = d.id
				AND NOT EXISTS (
					SELECT 1
					FROM orders o
					WHERE o.listing_id = l.id
				)
			RETURNING l.id
		), updated AS (
			UPDATE listings l
			SET
				status = 'removed',
				updated_at = NOW()
			WHERE l.id IN (
				SELECT d.id
				FROM doomed d
				WHERE d.id NOT IN (SELECT id FROM deleted)
			)
			RETURNING l.id
		)
		SELECT
			(SELECT COUNT(*) FROM deleted) + (SELECT COUNT(*) FROM updated)
	`, importKeys).Scan(&affectedCount)
	if err != nil {
		return 0, fmt.Errorf("suppress ineligible imported listings: %w", err)
	}

	return affectedCount, nil
}

func uniqueSourceNames(records []listingRecord) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, record := range records {
		if _, ok := seen[record.SourceName]; ok {
			continue
		}
		seen[record.SourceName] = struct{}{}
		names = append(names, record.SourceName)
	}
	sort.Strings(names)
	return names
}

func buildImportKey(categoryID int64, salePriceAmount int64) string {
	return fmt.Sprintf("%s:%d:%d", sourceSystemName, categoryID, salePriceAmount)
}

func buildPhotoKey(categoryID int64, salePriceAmount int64, ext string) string {
	if ext == "" {
		ext = ".jpg"
	}
	return fmt.Sprintf("%s_%d_%d%s", sourceSystemName, categoryID, salePriceAmount, ext)
}

func cleanText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "<br />", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	value = strings.ReplaceAll(value, "<br>", "\n")
	value = tagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = spacePattern.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func priceToMinorUnits(value float64) int64 {
	return int64(math.Round(value * 100))
}

func formatMinorUnits(value int64) string {
	major := float64(value) / 100
	text := strconv.FormatFloat(major, 'f', 2, 64)
	text = strings.TrimSuffix(text, "00")
	text = strings.TrimSuffix(text, ".")
	return "ILS " + text
}

func countPreviewRecords(records []listingRecord) int {
	var count int
	for _, record := range records {
		if strings.EqualFold(record.Status, "preview") {
			count++
		}
	}
	return count
}

func newCategoryDetailResolver(client *http.Client, seeded map[int64]sourceCategory) func(context.Context, int64) (sourceCategory, bool) {
	cache := make(map[int64]sourceCategory, len(seeded))
	for categoryID, category := range seeded {
		cache[categoryID] = category
	}

	return func(ctx context.Context, categoryID int64) (sourceCategory, bool) {
		if categoryID == 0 {
			return sourceCategory{}, false
		}
		if category, ok := cache[categoryID]; ok {
			return category, true
		}
		if client == nil {
			return sourceCategory{}, false
		}

		category, ok := fetchCategoryDetail(ctx, client, categoryID)
		if !ok {
			return sourceCategory{}, false
		}
		cache[categoryID] = category
		return category, true
	}
}

func fetchCategoryDetail(ctx context.Context, client *http.Client, categoryID int64) (sourceCategory, bool) {
	requestURL := fmt.Sprintf(defaultCategoryDetailURL, categoryID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return sourceCategory{}, false
	}

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", defaultCategoryOrigin)
	req.Header.Set("Referer", defaultCategoryOrigin+"/")
	req.Header.Set("Native", "true")
	req.Header.Set("OrganizationID", defaultOrganizationID)
	req.Header.Set("User-Agent", defaultImportHTTPUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return sourceCategory{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return sourceCategory{}, false
	}

	var payload categoryDetailPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return sourceCategory{}, false
	}
	if !payload.Status || payload.Data.CategoryID == 0 {
		return sourceCategory{}, false
	}

	return payload.Data, true
}

func mergeCategory(base sourceCategory, detail sourceCategory, detailAvailable bool) sourceCategory {
	if !detailAvailable {
		return base
	}

	merged := detail
	merged.CategoryID = firstNonZeroInt64(detail.CategoryID, base.CategoryID)
	merged.CategoryName = firstNonEmpty(detail.CategoryName, base.CategoryName)
	merged.SupplierName = firstNonEmpty(detail.SupplierName, base.SupplierName)
	merged.Description = firstNonEmpty(detail.Description, base.Description)
	merged.ShortDescription = firstNonEmpty(detail.ShortDescription, base.ShortDescription)
	merged.CategoryHTML = firstNonEmpty(detail.CategoryHTML, base.CategoryHTML)
	merged.CategoryURL = firstNonEmpty(detail.CategoryURL, base.CategoryURL)
	merged.TermsOfUse = firstNonEmpty(detail.TermsOfUse, base.TermsOfUse)
	merged.RedimType = firstNonEmpty(detail.RedimType, base.RedimType)
	merged.CampaignDetails = firstNonEmpty(detail.CampaignDetails, base.CampaignDetails)
	merged.Remarks = firstNonEmpty(detail.Remarks, base.Remarks)
	merged.MinimumInventory = firstNonEmpty(detail.MinimumInventory, base.MinimumInventory)
	merged.MustKnow = firstNonEmpty(detail.MustKnow, base.MustKnow)
	if len(merged.Prices) == 0 {
		merged.Prices = append([]float64(nil), base.Prices...)
	}
	if len(merged.Images) == 0 {
		merged.Images = append([]sourceImage(nil), base.Images...)
	}
	if len(merged.Variants) == 0 {
		merged.Variants = append([]sourceVariant(nil), base.Variants...)
	}
	if len(merged.Locations) == 0 {
		merged.Locations = append([]sourceLocation(nil), base.Locations...)
	}
	if len(merged.DateRanges) == 0 {
		merged.DateRanges = append([]sourceDateRange(nil), base.DateRanges...)
	}
	merged.Business = mergeBusiness(base.Business, detail.Business)
	return merged
}

func mergeBusiness(base sourceBusiness, detail sourceBusiness) sourceBusiness {
	return sourceBusiness{
		Name:    firstNonEmpty(detail.Name, base.Name),
		WebSite: firstNonEmpty(detail.WebSite, base.WebSite),
		Phone:   firstNonEmpty(detail.Phone, base.Phone),
		Address: firstNonEmpty(detail.Address, base.Address),
	}
}

func matchVariantForSalePrice(variants []sourceVariant, salePriceAmount int64) (*sourceVariant, bool) {
	for i := range variants {
		if priceToMinorUnits(variants[i].Price) == salePriceAmount {
			return &variants[i], true
		}
	}
	if len(variants) == 1 {
		return &variants[0], true
	}
	return nil, false
}

func buildListingTitle(baseTitle string, salePriceAmount int64, priceCount int, variant *sourceVariant, variantMatched bool) string {
	if variantMatched && variant != nil {
		if name := cleanText(variant.Name); name != "" {
			return name
		}
	}
	if priceCount > 1 {
		return fmt.Sprintf("%s (%s)", baseTitle, formatMinorUnits(salePriceAmount))
	}
	return baseTitle
}

func composeDescription(category sourceCategory, variant *sourceVariant) string {
	sections := make([]string, 0, 6)
	appendSection := func(label string, value string) {
		value = cleanText(value)
		if value == "" {
			return
		}
		if label == "" {
			sections = append(sections, value)
			return
		}
		sections = append(sections, label+": "+value)
	}

	appendSection("", firstNonEmpty(category.CategoryHTML, category.ShortDescription, category.Description))
	if variant != nil && cleanText(variant.BarCode) != "" {
		appendSection("Variant barcode", variant.BarCode)
	}
	appendSection("Campaign details", category.CampaignDetails)
	appendSection("Remarks", category.Remarks)
	appendSection("Business website", firstNonEmpty(category.Business.WebSite, category.CategoryURL))
	appendSection("Business contact", category.Business.Phone)
	return joinSections(sections)
}

func composeTermsSummary(category sourceCategory, variant *sourceVariant) string {
	sections := make([]string, 0, 6)
	appendSection := func(label string, value string) {
		value = cleanText(value)
		if value == "" {
			return
		}
		if label == "" {
			sections = append(sections, value)
			return
		}
		sections = append(sections, label+": "+value)
	}

	appendSection("", category.TermsOfUse)
	appendSection("Must know", category.MustKnow)
	appendSection("Minimum inventory for sale", category.MinimumInventory)
	if variant != nil {
		appendSection("Purchase limits", formatVariantLimits(*variant))
	}
	return joinSections(sections)
}

func composeRedemptionInstructions(category sourceCategory) string {
	sections := make([]string, 0, 4)
	appendSection := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if label == "" {
			sections = append(sections, value)
			return
		}
		sections = append(sections, label+":\n"+value)
	}

	appendSection("", cleanText(category.RedimType))
	appendSection("Locations", formatLocations(category.Locations))
	return joinSections(sections)
}

func composeExpirySummary(category sourceCategory, variant *sourceVariant, fallbackValues ...string) string {
	if variant != nil {
		if summary := formatDateSummary(variant.ExpireDate); summary != "" {
			return summary
		}
		if summary := formatDateSummary(variant.EndDate); summary != "" {
			return summary
		}
	}
	for _, dateRange := range category.DateRanges {
		if summary := formatDateRangeSummary(dateRange); summary != "" {
			return summary
		}
	}
	if summary := extractExpirySummary(fallbackValues...); summary != "" {
		return summary
	}
	return ""
}

func inferCouponValueFromVariant(variant *sourceVariant, salePriceAmount int64) int64 {
	if variant == nil {
		return 0
	}
	if variant.GiftCardValue != nil && *variant.GiftCardValue > 0 {
		return priceToMinorUnits(*variant.GiftCardValue)
	}
	kupaPrice := priceToMinorUnits(variant.KupaPrice)
	if kupaPrice >= salePriceAmount {
		return kupaPrice
	}
	return 0
}

func formatVariantLimits(variant sourceVariant) string {
	parts := make([]string, 0, 4)
	if variant.OrderLimit > 0 {
		parts = append(parts, fmt.Sprintf("order limit %d", variant.OrderLimit))
	}
	if variant.MonthlyLimit > 0 {
		parts = append(parts, fmt.Sprintf("monthly limit %d", variant.MonthlyLimit))
	}
	if variant.YearlyLimit > 0 {
		parts = append(parts, fmt.Sprintf("yearly limit %d", variant.YearlyLimit))
	}
	if variant.GeneralLimit > 0 {
		parts = append(parts, fmt.Sprintf("general limit %d", variant.GeneralLimit))
	}
	return strings.Join(parts, ", ")
}

func formatLocations(locations []sourceLocation) string {
	if len(locations) == 0 {
		return ""
	}

	lines := make([]string, 0, len(locations))
	for _, location := range locations {
		parts := make([]string, 0, 4)
		if value := cleanText(location.FriendlyName); value != "" {
			parts = append(parts, value)
		}
		if value := cleanText(location.Address); value != "" {
			parts = append(parts, value)
		}
		if value := cleanText(location.CityName); value != "" {
			parts = append(parts, value)
		}
		if value := cleanText(location.OpenHours); value != "" {
			parts = append(parts, "Hours: "+value)
		}
		if value := cleanText(location.Phone); value != "" {
			parts = append(parts, "Phone: "+value)
		}
		if len(parts) == 0 {
			continue
		}
		lines = append(lines, "- "+strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n")
}

func formatDateSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return "Valid through " + parsed.UTC().Format("02/01/2006")
		}
	}
	return ""
}

func formatDateRangeSummary(dateRange sourceDateRange) string {
	start := strings.TrimSpace(dateRange.StartDate)
	end := strings.TrimSpace(dateRange.EndDate)
	if start == "" && end == "" {
		return ""
	}
	if start == "" {
		return formatDateSummary(end)
	}
	if end == "" {
		return formatDateSummary(start)
	}

	startSummary := strings.TrimPrefix(formatDateSummary(start), "Valid through ")
	endSummary := strings.TrimPrefix(formatDateSummary(end), "Valid through ")
	if startSummary == "" || endSummary == "" {
		return ""
	}
	return "Valid from " + startSummary + " through " + endSummary
}

func joinSections(sections []string) string {
	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section != "" {
			filtered = append(filtered, section)
		}
	}
	return strings.Join(filtered, "\n\n")
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
