package admin

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type importedCouponRow struct {
	MaskedDisplay string
	PlainCode     string
	ExpiryAt      time.Time
}

func boolPtr(value bool) *bool {
	return &value
}

func (h *Handler) operatorID(r *http.Request) string {
	username, _, ok := r.BasicAuth()
	if ok && strings.TrimSpace(username) != "" {
		return username
	}
	return "operator"
}

func (h *Handler) redirectWithError(w http.ResponseWriter, r *http.Request, path string, message string) {
	http.Redirect(w, r, withQueryMessage(path, "error", message), http.StatusSeeOther)
}

func (h *Handler) redirectWithSuccess(w http.ResponseWriter, r *http.Request, path string, message string) {
	http.Redirect(w, r, withQueryMessage(path, "success", message), http.StatusSeeOther)
}

func (h *Handler) serverError(w http.ResponseWriter, message string, err error) {
	h.logger.Error(message, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func parsePathID(path string, prefix string) (int64, error) {
	value := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if value == "" || strings.Contains(value, "/") {
		return 0, errors.New("invalid path id")
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseInt64Field(values map[string][]string, key string) (int64, error) {
	raw := strings.TrimSpace(firstValue(values[key]))
	if raw == "" {
		return 0, errors.New("missing int64 field")
	}
	return strconv.ParseInt(raw, 10, 64)
}

func parseOptionalInt64(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseInt64Default(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCouponImportRequest(r *http.Request) ([]importedCouponRow, error) {
	var rows []importedCouponRow

	if text := strings.TrimSpace(r.FormValue("bulk_import")); text != "" {
		parsed, err := parseCouponCSV(strings.NewReader(text))
		if err != nil {
			return nil, err
		}
		rows = append(rows, parsed...)
	}

	file, _, err := r.FormFile("csv_upload")
	if err == nil {
		defer file.Close()
		parsed, parseErr := parseCouponCSV(file)
		if parseErr != nil {
			return nil, parseErr
		}
		rows = append(rows, parsed...)
	} else if !errors.Is(err, http.ErrMissingFile) {
		return nil, err
	}

	return rows, nil
}

func parseCouponCSV(reader io.Reader) ([]importedCouponRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	rows := make([]importedCouponRow, 0, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		if isImportHeader(record) {
			continue
		}

		row, err := parseImportedCouponRow(record)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func isImportHeader(record []string) bool {
	if len(record) == 0 {
		return false
	}

	first := strings.ToLower(strings.TrimSpace(record[0]))
	switch first {
	case "masked", "masked_display", "masked-display", "code", "plain_code", "expiry", "expiry_at", "expiry-at":
		return true
	default:
		return false
	}
}

func parseImportedCouponRow(record []string) (importedCouponRow, error) {
	switch len(record) {
	case 2:
		expiryAt, err := parseCouponExpiry(record[1])
		if err != nil {
			return importedCouponRow{}, err
		}
		plainCode := strings.TrimSpace(record[0])
		if plainCode == "" {
			return importedCouponRow{}, errors.New("coupon code is required")
		}
		return importedCouponRow{
			MaskedDisplay: deriveMaskedDisplay(plainCode),
			PlainCode:     plainCode,
			ExpiryAt:      expiryAt,
		}, nil
	case 3:
		expiryAt, err := parseCouponExpiry(record[2])
		if err != nil {
			return importedCouponRow{}, err
		}
		plainCode := strings.TrimSpace(record[1])
		if plainCode == "" {
			return importedCouponRow{}, errors.New("coupon code is required")
		}
		maskedDisplay := strings.TrimSpace(record[0])
		if maskedDisplay == "" {
			maskedDisplay = deriveMaskedDisplay(plainCode)
		}
		return importedCouponRow{
			MaskedDisplay: maskedDisplay,
			PlainCode:     plainCode,
			ExpiryAt:      expiryAt,
		}, nil
	default:
		return importedCouponRow{}, errors.New("coupon import rows must be code,expiry_at or masked_display,code,expiry_at")
	}
}

func parseCouponExpiry(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid coupon expiry %q", value)
}

func deriveMaskedDisplay(code string) string {
	code = strings.TrimSpace(code)
	if len(code) <= 4 {
		return "***" + code
	}
	return "***" + code[len(code)-4:]
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func withQueryMessage(path string, key string, value string) string {
	if strings.TrimSpace(value) == "" {
		return path
	}
	return path + "?" + key + "=" + urlQueryEscape(value)
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"!", "%21",
		"#", "%23",
		"&", "%26",
		"+", "%2B",
		"?", "%3F",
	)
	return replacer.Replace(value)
}
