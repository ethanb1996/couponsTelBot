package apphttp

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type redemptionScanRequest struct {
	RedemptionToken   string `json:"redemption_token"`
	CouponCode        string `json:"coupon_code"`
	MerchantReference string `json:"merchant_reference"`
	ScannerReference  string `json:"scanner_reference"`
}

func redemptionScanHandler(repo *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var input redemptionScanRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
			if err := decoder.Decode(&input); err != nil {
				http.Error(w, "invalid redemption scan payload", http.StatusBadRequest)
				return
			}
			recordRedemptionScan(w, r, repo, input)
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}
}

func redemptionScanByTokenHandler(repo *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/redemptions/scan/"), "/")
		if token == "" || strings.Contains(token, "/") {
			http.NotFound(w, r)
			return
		}

		recordRedemptionScan(w, r, repo, redemptionScanRequest{
			RedemptionToken:   token,
			MerchantReference: strings.TrimSpace(r.URL.Query().Get("merchant_reference")),
			ScannerReference:  strings.TrimSpace(r.URL.Query().Get("scanner_reference")),
		})
	}
}

func recordRedemptionScan(w http.ResponseWriter, r *http.Request, repo *store.Postgres, input redemptionScanRequest) {
	token := strings.TrimSpace(input.RedemptionToken)
	if token == "" {
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

	redemption, err := repo.RecordCouponRedemptionScan(r.Context(), store.RecordCouponRedemptionScanParams{
		RedemptionToken:   token,
		MerchantReference: input.MerchantReference,
		ScannerReference:  input.ScannerReference,
		ScanMetadataJSON:  string(metadata),
	})
	if err != nil {
		http.Error(w, "redemption token was not accepted", http.StatusNotFound)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":             redemption.Status,
		"order_id":           redemption.OrderID,
		"predefined_code_id": redemption.PredefinedCodeID,
		"redemption_token":   redemption.RedemptionToken,
		"merchant_reference": redemption.MerchantReference,
		"scanner_reference":  redemption.ScannerReference,
		"scanned_at":         redemption.ScannedAt,
	})
}
