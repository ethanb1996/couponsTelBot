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
	defaultImageBaseURL        = "https://pics.k4a.co.il/share/"
	defaultImportHTTPUserAgent = "couponsTelBot-importer/1.0"
	defaultHTTPTimeout         = 45 * time.Second
	sourceSystemName           = "behatsdaa"
)

var (
	tagPattern        = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern      = regexp.MustCompile(`[\s\p{Zs}]+`)
	numberPattern     = regexp.MustCompile(`\d+(?:[.,]\d{1,2})?`)
	expiryDatePattern = regexp.MustCompile(`\b\d{1,2}[./-]\d{1,2}[./-]\d{2,4}\b`)
)

type Options struct {
	InputPath  string
	PhotosDir  string
	DryRun     bool
	HTTPClient *http.Client
}

type Summary struct {
	SourceCount          int
	ListingCount         int
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
	CategoryID       int64          `json:"categoryId"`
	CategoryName     string         `json:"categoryName"`
	SupplierName     string         `json:"supplierName"`
	Description      string         `json:"description"`
	ShortDescription string         `json:"shortDescription"`
	CategoryHTML     string         `json:"categoryHTML"`
	TermsOfUse       string         `json:"termsOfUse"`
	RedimType        string         `json:"redimType"`
	Prices           []float64      `json:"prices"`
	Images           []sourceImage  `json:"images"`
	Business         sourceBusiness `json:"business"`
}

type sourceImage struct {
	File        string `json:"file"`
	ExternalURL string `json:"externalUrl"`
}

type sourceBusiness struct {
	Name string `json:"name"`
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

	records, err := parseHAR(file)
	if err != nil {
		return Summary{}, err
	}

	if options.DryRun {
		return Summary{
			SourceCount:  len(uniqueSourceNames(records)),
			ListingCount: len(records),
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
		SourceCount:  len(uniqueSourceNames(records)),
		ListingCount: len(records),
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

	if err := tx.Commit(ctx); err != nil {
		return Summary{}, err
	}

	return summary, nil
}

func parseHAR(reader io.Reader) ([]listingRecord, error) {
	var har harEnvelope
	if err := json.NewDecoder(reader).Decode(&har); err != nil {
		return nil, fmt.Errorf("decode HAR: %w", err)
	}

	var payload categoryPayload
	found := false
	for _, entry := range har.Log.Entries {
		if entry.Response.Status != http.StatusOK {
			continue
		}
		if !strings.Contains(entry.Request.URL, "/api/category/GetCategoryById") {
			continue
		}
		if !strings.Contains(strings.ToLower(entry.Response.Content.MimeType), "json") {
			continue
		}
		if strings.TrimSpace(entry.Response.Content.Text) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(entry.Response.Content.Text), &payload); err != nil {
			return nil, fmt.Errorf("decode category payload: %w", err)
		}
		found = true
		break
	}

	if !found {
		return nil, errors.New("no GetCategoryById payload found in HAR")
	}
	if !payload.Status {
		return nil, errors.New("category payload indicates unsuccessful response")
	}

	records := make([]listingRecord, 0)
	for _, category := range payload.Data.SubCategories {
		categoryRecords, err := expandCategory(category)
		if err != nil {
			return nil, err
		}
		records = append(records, categoryRecords...)
	}

	return records, nil
}

func expandCategory(category sourceCategory) ([]listingRecord, error) {
	if category.CategoryID == 0 {
		return nil, errors.New("category id is required")
	}
	if len(category.Prices) == 0 {
		return nil, fmt.Errorf("category %d has no prices", category.CategoryID)
	}

	sourceName := cleanText(firstNonEmpty(category.SupplierName, category.Business.Name))
	if sourceName == "" {
		sourceName = "Imported source"
	}

	merchantName := cleanText(firstNonEmpty(category.Business.Name, category.SupplierName))
	if merchantName == "" {
		merchantName = sourceName
	}

	baseTitle := cleanText(category.CategoryName)
	if baseTitle == "" {
		baseTitle = fmt.Sprintf("Imported listing %d", category.CategoryID)
	}

	description := firstNonEmpty(
		cleanText(category.CategoryHTML),
		cleanText(category.ShortDescription),
		cleanText(category.Description),
	)
	termsSummary := cleanText(category.TermsOfUse)
	redemptionInstructions := cleanText(category.RedimType)
	expirySummary := extractExpirySummary(termsSummary, redemptionInstructions, description)
	photoURL, ext := primaryImage(category.Images)

	records := make([]listingRecord, 0, len(category.Prices))
	for _, price := range category.Prices {
		salePriceAmount := priceToMinorUnits(price)
		if salePriceAmount <= 0 {
			return nil, fmt.Errorf("category %d has invalid price %.2f", category.CategoryID, price)
		}

		title := baseTitle
		if len(category.Prices) > 1 {
			title = fmt.Sprintf("%s (%s)", baseTitle, formatMinorUnits(salePriceAmount))
		}

		importKey := buildImportKey(category.CategoryID, salePriceAmount)
		photoKey := ""
		if photoURL != "" {
			photoKey = buildPhotoKey(category.CategoryID, salePriceAmount, ext)
		}

		couponValueAmount := inferCouponValueAmount(category, salePriceAmount)
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
			FinalSaleDisclosure:    defaultFinalSaleDisclosure,
			PhotoKey:               photoKey,
			PhotoURL:               photoURL,
		})
	}

	return records, nil
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
			) VALUES ($2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $1, 'draft', $13)
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
		defaultCreatedByAdminID,
	).Scan(&created)
	if err != nil {
		return false, fmt.Errorf("upsert listing %s: %w", record.ImportKey, err)
	}

	return created, nil
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
