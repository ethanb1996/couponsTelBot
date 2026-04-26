package payments

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type payPalClientOptions struct {
	BaseURL    string
	ClientID   string
	Secret     string
	WebhookID  string
	HTTPClient *http.Client
}

type paypalClient struct {
	baseURL   string
	clientID  string
	secret    string
	webhookID string
	client    *http.Client
}

type payPalCreateCheckoutInput struct {
	OrderID      int64
	OrderNumber  string
	Amount       int64
	CurrencyCode string
	Description  string
	ItemName     string
	ItemSummary  string
	ItemImageURL string
	ReturnURL    string
	CancelURL    string
}

type payPalCheckout struct {
	OrderID     string
	ApprovalURL string
}

type payPalCapture struct {
	OrderID      string
	CaptureID    string
	Status       string
	Amount       int64
	CurrencyCode string
}

type payPalOrderSnapshot struct {
	OrderID      string
	Status       string
	CaptureID    string
	Amount       int64
	CurrencyCode string
}

type payPalWebhookEvent struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Resource  struct {
		ID                string       `json:"id"`
		Status            string       `json:"status"`
		Amount            payPalAmount `json:"amount"`
		SupplementaryData struct {
			RelatedIDs struct {
				OrderID string `json:"order_id"`
			} `json:"related_ids"`
		} `json:"supplementary_data"`
	} `json:"resource"`
}

type payPalAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

func newPayPalClient(options payPalClientOptions) *paypalClient {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &paypalClient{
		baseURL:   strings.TrimRight(options.BaseURL, "/"),
		clientID:  options.ClientID,
		secret:    options.Secret,
		webhookID: options.WebhookID,
		client:    httpClient,
	}
}

func (c *paypalClient) CreateCheckout(ctx context.Context, input payPalCreateCheckoutInput) (payPalCheckout, error) {
	accessToken, err := c.fetchAccessToken(ctx)
	if err != nil {
		return payPalCheckout{}, err
	}

	amountPayload := map[string]any{
		"currency_code": defaultCurrency(input.CurrencyCode, "ILS"),
		"value":         formatMinorUnits(input.Amount),
	}

	purchaseUnit := map[string]any{
		"reference_id": input.OrderNumber,
		"custom_id":    strconv.FormatInt(input.OrderID, 10),
		"invoice_id":   input.OrderNumber,
		"description":  input.Description,
		"amount":       amountPayload,
	}

	if strings.TrimSpace(input.ItemName) != "" {
		amountPayload["breakdown"] = map[string]any{
			"item_total": map[string]any{
				"currency_code": defaultCurrency(input.CurrencyCode, "ILS"),
				"value":         formatMinorUnits(input.Amount),
			},
		}

		item := map[string]any{
			"name":     truncateText(input.ItemName, 127),
			"quantity": "1",
			"category": "DIGITAL_GOODS",
			"unit_amount": map[string]any{
				"currency_code": defaultCurrency(input.CurrencyCode, "ILS"),
				"value":         formatMinorUnits(input.Amount),
			},
		}
		if summary := strings.TrimSpace(input.ItemSummary); summary != "" {
			item["description"] = truncateText(summary, 2048)
		}
		if imageURL := strings.TrimSpace(input.ItemImageURL); imageURL != "" {
			item["image_url"] = imageURL
		}
		purchaseUnit["items"] = []map[string]any{item}
	}

	payload := map[string]any{
		"intent":         "CAPTURE",
		"purchase_units": []map[string]any{purchaseUnit},
		"payment_source": map[string]any{
			"paypal": map[string]any{
				"experience_context": map[string]any{
					"shipping_preference": "NO_SHIPPING",
					"user_action":         "PAY_NOW",
					"return_url":          input.ReturnURL,
					"cancel_url":          input.CancelURL,
				},
			},
		},
	}

	var response struct {
		ID    string `json:"id"`
		Links []struct {
			Href   string `json:"href"`
			Rel    string `json:"rel"`
			Method string `json:"method"`
		} `json:"links"`
	}

	if err := c.doJSON(ctx, http.MethodPost, "/v2/checkout/orders", accessToken, payload, &response); err != nil {
		return payPalCheckout{}, err
	}

	approvalURL := approvalURLFromPayPalLinks(response.Links)

	if strings.TrimSpace(response.ID) == "" || strings.TrimSpace(approvalURL) == "" {
		return payPalCheckout{}, fmt.Errorf("payments: paypal checkout response missing order id or approval url (order_id=%q rels=%s)", strings.TrimSpace(response.ID), summarizePayPalLinks(response.Links))
	}

	return payPalCheckout{
		OrderID:     response.ID,
		ApprovalURL: approvalURL,
	}, nil
}

func approvalURLFromPayPalLinks(links []struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}) string {
	preferredRels := []string{"approve", "payer-action", "approval_url"}
	for _, rel := range preferredRels {
		for _, link := range links {
			if strings.EqualFold(strings.TrimSpace(link.Rel), rel) && strings.TrimSpace(link.Href) != "" {
				return strings.TrimSpace(link.Href)
			}
		}
	}

	for _, link := range links {
		if strings.EqualFold(strings.TrimSpace(link.Method), http.MethodGet) && strings.TrimSpace(link.Href) != "" {
			return strings.TrimSpace(link.Href)
		}
	}

	return ""
}

func summarizePayPalLinks(links []struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}) string {
	if len(links) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(links))
	for _, link := range links {
		parts = append(parts, fmt.Sprintf("%s:%s", strings.TrimSpace(link.Rel), strings.TrimSpace(link.Method)))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

func (c *paypalClient) VerifyAndParseWebhook(ctx context.Context, headers http.Header, body []byte) (payPalWebhookEvent, error) {
	if strings.TrimSpace(c.webhookID) == "" {
		return payPalWebhookEvent{}, fmt.Errorf("%w: missing paypal webhook id", ErrWebhookUnauthorized)
	}

	var rawEvent map[string]any
	if err := json.Unmarshal(body, &rawEvent); err != nil {
		return payPalWebhookEvent{}, err
	}

	accessToken, err := c.fetchAccessToken(ctx)
	if err != nil {
		return payPalWebhookEvent{}, err
	}

	verifyRequest := map[string]any{
		"auth_algo":         headers.Get("PAYPAL-AUTH-ALGO"),
		"cert_url":          headers.Get("PAYPAL-CERT-URL"),
		"transmission_id":   headers.Get("PAYPAL-TRANSMISSION-ID"),
		"transmission_sig":  headers.Get("PAYPAL-TRANSMISSION-SIG"),
		"transmission_time": headers.Get("PAYPAL-TRANSMISSION-TIME"),
		"webhook_id":        c.webhookID,
		"webhook_event":     rawEvent,
	}

	var verifyResponse struct {
		VerificationStatus string `json:"verification_status"`
	}

	if err := c.doJSON(ctx, http.MethodPost, "/v1/notifications/verify-webhook-signature", accessToken, verifyRequest, &verifyResponse); err != nil {
		return payPalWebhookEvent{}, err
	}

	if verifyResponse.VerificationStatus != "SUCCESS" {
		return payPalWebhookEvent{}, ErrWebhookUnauthorized
	}

	var event payPalWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return payPalWebhookEvent{}, err
	}

	return event, nil
}

func (c *paypalClient) CaptureOrder(ctx context.Context, orderID string) (payPalCapture, error) {
	accessToken, err := c.fetchAccessToken(ctx)
	if err != nil {
		return payPalCapture{}, err
	}

	path := "/v2/checkout/orders/" + url.PathEscape(orderID) + "/capture"
	var response struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					ID     string       `json:"id"`
					Status string       `json:"status"`
					Amount payPalAmount `json:"amount"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}

	if err := c.doJSON(ctx, http.MethodPost, path, accessToken, map[string]any{}, &response); err != nil {
		return payPalCapture{}, err
	}

	if len(response.PurchaseUnits) == 0 || len(response.PurchaseUnits[0].Payments.Captures) == 0 {
		return payPalCapture{}, fmt.Errorf("payments: paypal capture response missing capture data")
	}

	capture := response.PurchaseUnits[0].Payments.Captures[0]
	amount, err := parseMinorUnits(capture.Amount.Value)
	if err != nil {
		return payPalCapture{}, err
	}

	return payPalCapture{
		OrderID:      response.ID,
		CaptureID:    capture.ID,
		Status:       normalizePayPalStatus(capture.Status),
		Amount:       amount,
		CurrencyCode: capture.Amount.CurrencyCode,
	}, nil
}

func (c *paypalClient) GetOrder(ctx context.Context, orderID string) (payPalOrderSnapshot, error) {
	accessToken, err := c.fetchAccessToken(ctx)
	if err != nil {
		return payPalOrderSnapshot{}, err
	}

	path := "/v2/checkout/orders/" + url.PathEscape(orderID)
	var response struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Amount   payPalAmount `json:"amount"`
			Payments struct {
				Captures []struct {
					ID     string       `json:"id"`
					Status string       `json:"status"`
					Amount payPalAmount `json:"amount"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}

	if err := c.doJSON(ctx, http.MethodGet, path, accessToken, nil, &response); err != nil {
		return payPalOrderSnapshot{}, err
	}

	snapshot := payPalOrderSnapshot{
		OrderID: response.ID,
		Status:  normalizePayPalOrderStatus(response.Status),
	}
	if strings.TrimSpace(snapshot.OrderID) == "" {
		return payPalOrderSnapshot{}, fmt.Errorf("payments: paypal order lookup missing order id")
	}

	if len(response.PurchaseUnits) == 0 {
		return snapshot, nil
	}

	purchaseUnit := response.PurchaseUnits[0]
	snapshot.CurrencyCode = purchaseUnit.Amount.CurrencyCode
	if amount, err := parseMinorUnits(purchaseUnit.Amount.Value); err == nil {
		snapshot.Amount = amount
	}

	if len(purchaseUnit.Payments.Captures) == 0 {
		return snapshot, nil
	}

	capture := purchaseUnit.Payments.Captures[0]
	snapshot.CaptureID = capture.ID
	snapshot.CurrencyCode = defaultCurrency(capture.Amount.CurrencyCode, snapshot.CurrencyCode)
	amount, err := parseMinorUnits(capture.Amount.Value)
	if err != nil {
		return payPalOrderSnapshot{}, err
	}
	snapshot.Amount = amount

	return snapshot, nil
}

func (c *paypalClient) fetchAccessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en_US")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.clientID+":"+c.secret)))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("payments: paypal oauth failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", fmt.Errorf("payments: paypal oauth response missing access token")
	}

	return response.AccessToken, nil
}

func (c *paypalClient) doJSON(ctx context.Context, method string, path string, accessToken string, payload any, target any) error {
	var requestBody io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("payments: paypal request %s %s failed with status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	if target == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (e payPalWebhookEvent) OrderID() string {
	return e.Resource.ID
}

func (e payPalWebhookEvent) RelatedOrderID() string {
	if strings.TrimSpace(e.Resource.SupplementaryData.RelatedIDs.OrderID) != "" {
		return e.Resource.SupplementaryData.RelatedIDs.OrderID
	}
	return e.Resource.ID
}

func (e payPalWebhookEvent) CaptureID() string {
	return e.Resource.ID
}

func (e payPalWebhookEvent) AmountMinorUnits() int64 {
	amount, err := parseMinorUnits(e.Resource.Amount.Value)
	if err != nil {
		return 0
	}
	return amount
}

func (e payPalWebhookEvent) CurrencyCode() string {
	return e.Resource.Amount.CurrencyCode
}

func parseMinorUnits(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	sign := int64(1)
	if strings.HasPrefix(value, "-") {
		sign = -1
		value = strings.TrimPrefix(value, "-")
	}

	parts := strings.SplitN(value, ".", 3)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}

	fractional := int64(0)
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		if len(frac) == 1 {
			frac += "0"
		}
		if frac != "" {
			fractional, err = strconv.ParseInt(frac, 10, 64)
			if err != nil {
				return 0, err
			}
		}
	}

	return sign * ((whole * 100) + fractional), nil
}

func normalizePayPalStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "COMPLETED":
		return "captured"
	case "APPROVED":
		return "authorized"
	case "PENDING":
		return "pending"
	case "DECLINED", "DENIED", "FAILED":
		return "failed"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func normalizePayPalOrderStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED":
		return "approved"
	case "COMPLETED":
		return "completed"
	case "VOIDED":
		return "voided"
	case "PAYER_ACTION_REQUIRED":
		return "payer_action_required"
	case "CREATED":
		return "created"
	case "SAVED":
		return "saved"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}
