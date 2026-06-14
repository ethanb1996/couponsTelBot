package apphttp

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type redemptionStore interface {
	GetCouponRedemptionPreview(ctx context.Context, token string) (store.CouponRedemptionPreview, error)
	ConfirmCouponRedemption(ctx context.Context, params store.ConfirmCouponRedemptionParams) (store.CouponRedemptionConfirmResult, error)
}

type redemptionNotifier interface {
	NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionConfirmResult) error
}

type redemptionNotifierChain []redemptionNotifier

func (chain redemptionNotifierChain) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionConfirmResult) error {
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

func redemptionScanHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func redemptionScanByTokenHandler(repo redemptionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		token := tokenFromPath(r.URL.Path, "/api/redemptions/scan/")
		if token == "" {
			renderInvalidRedemptionHTML(w, http.StatusNotFound)
			return
		}

		preview, err := repo.GetCouponRedemptionPreview(r.Context(), token)
		if err != nil {
			renderInvalidRedemptionHTML(w, http.StatusNotFound)
			return
		}

		renderRedemptionPreviewHTML(w, preview)
	}
}

func redemptionRedeemHandler(repo redemptionStore, notifier redemptionNotifier, fallbackRestaurantChatID int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		var input redemptionScanRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid redemption payload", http.StatusBadRequest)
			return
		}

		result, err := confirmRedemption(r, repo, notifier, input, fallbackRestaurantChatID)
		if err != nil {
			http.Error(w, "redemption token was not accepted", http.StatusNotFound)
			return
		}

		WriteJSON(w, http.StatusOK, redemptionJSONPayload(result))
	}
}

func redemptionRedeemByTokenHandler(repo redemptionStore, notifier redemptionNotifier, fallbackRestaurantChatID int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		token := tokenFromPath(r.URL.Path, "/api/redemptions/redeem/")
		if token == "" {
			renderInvalidRedemptionHTML(w, http.StatusNotFound)
			return
		}

		result, err := confirmRedemption(r, repo, notifier, redemptionScanRequest{
			RedemptionToken:   token,
			MerchantReference: strings.TrimSpace(r.FormValue("merchant_reference")),
			ScannerReference:  strings.TrimSpace(r.FormValue("scanner_reference")),
		}, fallbackRestaurantChatID)
		if err != nil {
			renderInvalidRedemptionHTML(w, http.StatusNotFound)
			return
		}

		renderRedemptionConfirmHTML(w, result)
	}
}

func confirmRedemption(r *http.Request, repo redemptionStore, notifier redemptionNotifier, input redemptionScanRequest, fallbackRestaurantChatID int64) (store.CouponRedemptionConfirmResult, error) {
	token := strings.TrimSpace(input.RedemptionToken)
	if token == "" {
		return store.CouponRedemptionConfirmResult{}, fmt.Errorf("redemption token is required")
	}

	metadata, err := json.Marshal(map[string]string{
		"coupon_code": strings.TrimSpace(input.CouponCode),
		"user_agent":  strings.TrimSpace(r.UserAgent()),
		"remote_addr": strings.TrimSpace(r.RemoteAddr),
		"method":      strings.TrimSpace(r.Method),
	})
	if err != nil {
		return store.CouponRedemptionConfirmResult{}, err
	}

	result, err := repo.ConfirmCouponRedemption(r.Context(), store.ConfirmCouponRedemptionParams{
		RedemptionToken:                      token,
		MerchantReference:                    strings.TrimSpace(input.MerchantReference),
		ScannerReference:                     strings.TrimSpace(input.ScannerReference),
		RedeemMetadataJSON:                   string(metadata),
		RestaurantNotificationFallbackChatID: fallbackRestaurantChatID,
	})
	if err != nil {
		return store.CouponRedemptionConfirmResult{}, err
	}

	if notifier != nil {
		if err := notifier.NotifyCouponRedeemed(r.Context(), result); err != nil {
			slog.Default().Warn("failed to notify coupon redemption",
				"error", err,
				"order_id", result.Preview.Redemption.OrderID,
				"redemption_id", result.Preview.Redemption.ID,
			)
		}
	}
	return result, nil
}

func tokenFromPath(path string, prefix string) string {
	token := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if token == "" || strings.Contains(token, "/") || token == path {
		return ""
	}
	return token
}

func redemptionJSONPayload(result store.CouponRedemptionConfirmResult) map[string]any {
	preview := result.Preview
	return map[string]any{
		"status":                          preview.Redemption.Status,
		"first_redeem":                    result.FirstRedeem,
		"order_number":                    preview.OrderNumber,
		"merchant_name":                   preview.MerchantName,
		"offer_title":                     preview.OfferTitle,
		"amount_paid":                     preview.AmountPaid,
		"currency_code":                   preview.CurrencyCode,
		"payment_status_summary":          preview.PaymentStatusSummary,
		"merchant_reference":              preview.Redemption.MerchantReference,
		"scanner_reference":               preview.Redemption.ScannerReference,
		"redeemed_at":                     preview.Redemption.RedeemedAt,
		"restaurant_notification_chat_id": result.RestaurantNotificationChatID,
	}
}

type redemptionHTMLData struct {
	Title            string
	Heading          string
	Message          string
	StatusLabel      string
	OrderNumber      string
	Merchant         string
	Offer            string
	AmountPaid       string
	PaymentStatus    string
	ApprovalTime     string
	BuyerDisplay     string
	RedemptionTerms  string
	RedeemedAt       string
	RedeemAction     string
	ShowRedeemButton bool
}

func renderRedemptionPreviewHTML(w http.ResponseWriter, preview store.CouponRedemptionPreview) {
	if preview.Redemption.Status == "redeemed" {
		renderRedemptionHTML(w, http.StatusOK, alreadyRedeemedHTMLData(preview))
		return
	}
	if preview.PredefinedCodeExpiryAt != nil && preview.PredefinedCodeExpiryAt.Before(time.Now()) {
		data := previewHTMLData(preview)
		data.Title = "הקופון פג תוקף"
		data.Heading = "הקופון פג תוקף"
		data.Message = "לא ניתן לממש קופון שפג תוקפו. יש לפנות ל-KuponFast לפני כיבוד הקופון."
		data.StatusLabel = "פג תוקף"
		data.ShowRedeemButton = false
		renderRedemptionHTML(w, http.StatusOK, data)
		return
	}
	renderRedemptionHTML(w, http.StatusOK, previewHTMLData(preview))
}

func renderRedemptionConfirmHTML(w http.ResponseWriter, result store.CouponRedemptionConfirmResult) {
	if !result.FirstRedeem {
		renderRedemptionHTML(w, http.StatusOK, alreadyRedeemedHTMLData(result.Preview))
		return
	}
	data := previewHTMLData(result.Preview)
	data.Title = "קופון מומש בהצלחה"
	data.Heading = "קופון מומש בהצלחה"
	data.Message = "המימוש נרשם במערכת KuponFast. אפשר לכבד את הקופון עבור הלקוח."
	data.StatusLabel = "מומש"
	data.RedeemedAt = formatRedemptionTime(result.Preview.Redemption.RedeemedAt)
	data.ShowRedeemButton = false
	renderRedemptionHTML(w, http.StatusOK, data)
}

func renderInvalidRedemptionHTML(w http.ResponseWriter, status int) {
	renderRedemptionHTML(w, status, redemptionHTMLData{
		Title:       "קוד לא תקין",
		Heading:     "קוד לא תקין",
		Message:     "לא ניתן לאמת את קוד הקופון. אין לכבד את הקופון בלי בדיקה מול KuponFast.",
		StatusLabel: "לא תקין",
	})
}

func previewHTMLData(preview store.CouponRedemptionPreview) redemptionHTMLData {
	return redemptionHTMLData{
		Title:            "בדיקת קופון",
		Heading:          "בדיקת קופון",
		Message:          "נא לוודא שהפרטים מתאימים להזמנה לפני המימוש.",
		StatusLabel:      "תקין",
		OrderNumber:      preview.OrderNumber,
		Merchant:         preview.MerchantName,
		Offer:            preview.OfferTitle,
		AmountPaid:       formatAmount(preview.AmountPaid, preview.CurrencyCode),
		PaymentStatus:    preview.PaymentStatusSummary,
		ApprovalTime:     formatRedemptionTime(preview.ApprovalTime),
		BuyerDisplay:     preview.BuyerDisplay,
		RedemptionTerms:  preview.RedemptionTerms,
		RedeemAction:     "/api/redemptions/redeem/" + template.URLQueryEscaper(preview.Redemption.RedemptionToken),
		ShowRedeemButton: true,
	}
}

func alreadyRedeemedHTMLData(preview store.CouponRedemptionPreview) redemptionHTMLData {
	return redemptionHTMLData{
		Title:       "הקופון כבר מומש",
		Heading:     "הקופון כבר מומש",
		Message:     "הקופון הזה מומש בעבר ולא ניתן לממש אותו שוב.",
		StatusLabel: "כבר מומש",
		OrderNumber: preview.OrderNumber,
		Merchant:    preview.MerchantName,
		Offer:       preview.OfferTitle,
		AmountPaid:  formatAmount(preview.AmountPaid, preview.CurrencyCode),
		RedeemedAt:  formatRedemptionTime(preview.Redemption.RedeemedAt),
	}
}

func formatAmount(amount int64, currency string) string {
	if amount <= 0 {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(currency), "ILS") || strings.TrimSpace(currency) == "" {
		return fmt.Sprintf("%.2f ₪", float64(amount)/100.0)
	}
	return fmt.Sprintf("%.2f %s", float64(amount)/100.0, strings.ToUpper(strings.TrimSpace(currency)))
}

func formatRedemptionTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Local().Format("15:04 02/01/2006")
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
    main { max-width: 560px; margin: 0 auto; min-height: 100vh; display: grid; place-items: center; padding: 24px; box-sizing: border-box; }
    section { width: 100%; background: #fff; border: 1px solid #d8dee7; border-radius: 8px; padding: 24px; box-sizing: border-box; }
    h1 { margin: 0 0 12px; font-size: 28px; }
    p { margin: 0 0 18px; color: #485465; line-height: 1.5; }
    dl { margin: 0; display: grid; gap: 10px; }
    .row { display: flex; justify-content: space-between; gap: 16px; border-top: 1px solid #edf0f4; padding-top: 10px; }
    dt { color: #687386; }
    dd { margin: 0; font-weight: 700; text-align: left; direction: ltr; }
    .he { direction: rtl; text-align: right; }
    form { margin-top: 22px; }
    button { width: 100%; border: 0; border-radius: 8px; background: #13795b; color: #fff; font-size: 20px; font-weight: 700; padding: 14px 16px; cursor: pointer; }
    button:active { transform: translateY(1px); }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>{{.Heading}}</h1>
      <p>{{.Message}}</p>
      <dl>
        {{if .StatusLabel}}<div class="row"><dt>סטטוס</dt><dd class="he">{{.StatusLabel}}</dd></div>{{end}}
        {{if .OrderNumber}}<div class="row"><dt>מספר הזמנה</dt><dd>{{.OrderNumber}}</dd></div>{{end}}
        {{if .Merchant}}<div class="row"><dt>בית עסק</dt><dd class="he">{{.Merchant}}</dd></div>{{end}}
        {{if .Offer}}<div class="row"><dt>קופון</dt><dd class="he">{{.Offer}}</dd></div>{{end}}
        {{if .AmountPaid}}<div class="row"><dt>סכום ששולם</dt><dd>{{.AmountPaid}}</dd></div>{{end}}
        {{if .PaymentStatus}}<div class="row"><dt>סטטוס תשלום</dt><dd class="he">{{.PaymentStatus}}</dd></div>{{end}}
        {{if .ApprovalTime}}<div class="row"><dt>שעת אישור</dt><dd>{{.ApprovalTime}}</dd></div>{{end}}
        {{if .BuyerDisplay}}<div class="row"><dt>לקוח</dt><dd class="he">{{.BuyerDisplay}}</dd></div>{{end}}
        {{if .RedemptionTerms}}<div class="row"><dt>תנאי מימוש</dt><dd class="he">{{.RedemptionTerms}}</dd></div>{{end}}
        {{if .RedeemedAt}}<div class="row"><dt>מומש בתאריך</dt><dd>{{.RedeemedAt}}</dd></div>{{end}}
      </dl>
      {{if .ShowRedeemButton}}
      <form method="post" action="{{.RedeemAction}}">
        <button type="submit">ממש קופון</button>
      </form>
      {{end}}
    </section>
  </main>
</body>
</html>`))
