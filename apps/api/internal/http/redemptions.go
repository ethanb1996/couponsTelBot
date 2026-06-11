package apphttp

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type redemptionRecorder interface {
	RecordCouponRedemptionScan(ctx context.Context, params store.RecordCouponRedemptionScanParams) (store.CouponRedemptionScanResult, error)
}

type redemptionNotifier interface {
	NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionScanResult) error
}

type redemptionNotifierChain []redemptionNotifier

func (chain redemptionNotifierChain) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionScanResult) error {
	var firstErr error
	for _, notifier := range chain {
		if notifier == nil {
			continue
		}
		if err := notifier.NotifyCouponRedeemed(ctx, result); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type redemptionScanRequest struct {
	RedemptionToken   string `json:"redemption_token"`
	CouponCode        string `json:"coupon_code"`
	MerchantReference string `json:"merchant_reference"`
	ScannerReference  string `json:"scanner_reference"`
}

func redemptionScanHandler(repo redemptionRecorder, notifier redemptionNotifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var input redemptionScanRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
			if err := decoder.Decode(&input); err != nil {
				http.Error(w, "invalid redemption scan payload", http.StatusBadRequest)
				return
			}
			recordRedemptionScan(w, r, repo, notifier, input, redemptionResponseJSON)
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}
}

func redemptionScanByTokenHandler(repo redemptionRecorder, notifier redemptionNotifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/redemptions/scan/"), "/")
		if token == "" || strings.Contains(token, "/") {
			renderRedemptionHTML(w, http.StatusNotFound, redemptionHTMLData{Title: "קוד לא תקין", Heading: "קוד לא תקין", Message: "לא ניתן לאמת את קוד הקופון."})
			return
		}

		mode := redemptionResponseHTML
		if r.Method == http.MethodPost {
			mode = redemptionResponseJSON
		}
		recordRedemptionScan(w, r, repo, notifier, redemptionScanRequest{
			RedemptionToken:   token,
			MerchantReference: strings.TrimSpace(r.URL.Query().Get("merchant_reference")),
			ScannerReference:  strings.TrimSpace(r.URL.Query().Get("scanner_reference")),
		}, mode)
	}
}

type redemptionResponseMode string

const (
	redemptionResponseJSON redemptionResponseMode = "json"
	redemptionResponseHTML redemptionResponseMode = "html"
)

func recordRedemptionScan(w http.ResponseWriter, r *http.Request, repo redemptionRecorder, notifier redemptionNotifier, input redemptionScanRequest, mode redemptionResponseMode) {
	token := strings.TrimSpace(input.RedemptionToken)
	if token == "" {
		if mode == redemptionResponseHTML {
			renderRedemptionHTML(w, http.StatusBadRequest, redemptionHTMLData{Title: "קוד לא תקין", Heading: "קוד לא תקין", Message: "חסר קוד מימוש."})
			return
		}
		http.Error(w, "redemption_token is required", http.StatusBadRequest)
		return
	}

	metadata, err := json.Marshal(map[string]string{
		"coupon_code": strings.TrimSpace(input.CouponCode),
		"user_agent":  strings.TrimSpace(r.UserAgent()),
		"remote_addr": strings.TrimSpace(r.RemoteAddr),
		"method":      strings.TrimSpace(r.Method),
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	result, err := repo.RecordCouponRedemptionScan(r.Context(), store.RecordCouponRedemptionScanParams{
		RedemptionToken:   token,
		MerchantReference: input.MerchantReference,
		ScannerReference:  input.ScannerReference,
		ScanMetadataJSON:  string(metadata),
	})
	if err != nil {
		if mode == redemptionResponseHTML {
			renderRedemptionHTML(w, http.StatusNotFound, redemptionHTMLData{Title: "קוד לא תקין", Heading: "קוד לא תקין", Message: "קוד הקופון לא נמצא או שאינו תקף."})
			return
		}
		http.Error(w, "redemption token was not accepted", http.StatusNotFound)
		return
	}

	if notifier != nil {
		if err := notifier.NotifyCouponRedeemed(r.Context(), result); err != nil {
			slog.Default().Warn("failed to notify coupon redemption",
				"error", err,
				"order_id", result.Redemption.OrderID,
				"redemption_id", result.Redemption.ID,
			)
		}
	}

	if mode == redemptionResponseHTML {
		renderRedemptionResultHTML(w, result)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":             result.Redemption.Status,
		"first_scan":         result.FirstScan,
		"order_id":           result.Redemption.OrderID,
		"order_number":       result.OrderNumber,
		"merchant_name":      result.MerchantName,
		"offer_title":        result.OfferTitle,
		"predefined_code_id": result.Redemption.PredefinedCodeID,
		"redemption_token":   result.Redemption.RedemptionToken,
		"merchant_reference": result.Redemption.MerchantReference,
		"scanner_reference":  result.Redemption.ScannerReference,
		"scanned_at":         result.Redemption.ScannedAt,
	})
}

type redemptionHTMLData struct {
	Title       string
	Heading     string
	Message     string
	StatusLabel string
	OrderNumber string
	Merchant    string
	Offer       string
	ScannedAt   string
}

func renderRedemptionResultHTML(w http.ResponseWriter, result store.CouponRedemptionScanResult) {
	data := redemptionHTMLData{
		Title:       "קופון מומש בהצלחה",
		Heading:     "הקופון מומש בהצלחה",
		Message:     "אפשר לכבד את הקופון עבור הלקוח.",
		StatusLabel: "מומש",
		OrderNumber: result.OrderNumber,
		Merchant:    result.MerchantName,
		Offer:       result.OfferTitle,
	}
	if !result.FirstScan {
		data.Title = "הקופון כבר מומש"
		data.Heading = "הקופון כבר מומש"
		data.Message = "הקופון נסרק בעבר ואין לממש אותו שוב."
		data.StatusLabel = "כבר מומש"
	}
	if result.Redemption.ScannedAt != nil {
		data.ScannedAt = result.Redemption.ScannedAt.Local().Format("15:04 02/01/2006")
	}
	renderRedemptionHTML(w, http.StatusOK, data)
}

func renderRedemptionHTML(w http.ResponseWriter, statusCode int, data redemptionHTMLData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = redemptionPageTemplate.Execute(w, data)
}

var redemptionPageTemplate = template.Must(template.New("redemption").Parse(`<!doctype html>
<html lang="he" dir="rtl">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body { margin: 0; font-family: Arial, sans-serif; background: #f6f7f9; color: #17202a; }
    main { max-width: 520px; margin: 0 auto; min-height: 100vh; display: grid; place-items: center; padding: 24px; box-sizing: border-box; }
    section { width: 100%; background: #fff; border: 1px solid #d8dee7; border-radius: 8px; padding: 24px; box-sizing: border-box; }
    h1 { margin: 0 0 12px; font-size: 28px; }
    p { margin: 0 0 18px; color: #485465; line-height: 1.5; }
    dl { margin: 0; display: grid; gap: 10px; }
    div.row { display: flex; justify-content: space-between; gap: 16px; border-top: 1px solid #edf0f4; padding-top: 10px; }
    dt { color: #687386; }
    dd { margin: 0; font-weight: 700; text-align: left; direction: ltr; }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>{{.Heading}}</h1>
      <p>{{.Message}}</p>
      <dl>
        {{if .StatusLabel}}<div class="row"><dt>סטטוס</dt><dd>{{.StatusLabel}}</dd></div>{{end}}
        {{if .OrderNumber}}<div class="row"><dt>הזמנה</dt><dd>{{.OrderNumber}}</dd></div>{{end}}
        {{if .Merchant}}<div class="row"><dt>בית עסק</dt><dd>{{.Merchant}}</dd></div>{{end}}
        {{if .Offer}}<div class="row"><dt>קופון</dt><dd>{{.Offer}}</dd></div>{{end}}
        {{if .ScannedAt}}<div class="row"><dt>זמן סריקה</dt><dd>{{.ScannedAt}}</dd></div>{{end}}
      </dl>
    </section>
  </main>
</body>
</html>`))
