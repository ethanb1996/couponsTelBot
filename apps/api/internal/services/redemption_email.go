package services

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"net/mail"
	"strings"
	texttemplate "text/template"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	"github.com/resend/resend-go/v3"
)

const resendSampleAPIKey = "re_xxxxxxxxx"

type ResendRedemptionNotifierOptions struct {
	APIKey string
	From   string
}

type resendEmailSender interface {
	SendWithContext(ctx context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
}

type ResendRedemptionNotifier struct {
	from   string
	sender resendEmailSender
}

func NewResendRedemptionNotifier(options ResendRedemptionNotifierOptions) (*ResendRedemptionNotifier, error) {
	apiKey := strings.TrimSpace(options.APIKey)
	from := strings.TrimSpace(options.From)
	if apiKey == "" && from == "" {
		return nil, nil
	}
	if apiKey == "" {
		return nil, fmt.Errorf("resend api key is required")
	}
	if apiKey == resendSampleAPIKey {
		return nil, fmt.Errorf("replace %s with your real Resend API key", resendSampleAPIKey)
	}
	if from == "" {
		return nil, fmt.Errorf("resend from is required")
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("resend from must be a valid email address: %w", err)
	}

	client := resend.NewClient(apiKey)
	return &ResendRedemptionNotifier{
		from:   fromAddress.String(),
		sender: client.Emails,
	}, nil
}

func (n *ResendRedemptionNotifier) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionConfirmResult) error {
	if n == nil || !result.FirstRedeem {
		return nil
	}
	to, ok := redemptionMerchantEmail(result)
	if !ok {
		return nil
	}
	request, err := buildRedemptionEmailRequest(n.from, to, result)
	if err != nil {
		return err
	}
	_, err = n.sender.SendWithContext(ctx, request)
	return err
}

func redemptionMerchantEmail(result store.CouponRedemptionConfirmResult) (string, bool) {
	address, err := mail.ParseAddress(strings.TrimSpace(result.Preview.MerchantContact))
	if err != nil || address.Address == "" {
		return "", false
	}
	return address.Address, true
}

func buildRedemptionEmailRequest(from string, to string, result store.CouponRedemptionConfirmResult) (*resend.SendEmailRequest, error) {
	text, err := buildRedemptionEmailText(result)
	if err != nil {
		return nil, err
	}
	html, err := buildRedemptionEmailHTML(result)
	if err != nil {
		return nil, err
	}
	return &resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: redemptionEmailSubject(result),
		Html:    html,
		Text:    text,
	}, nil
}

func redemptionEmailSubject(result store.CouponRedemptionConfirmResult) string {
	subject := "KuponFast coupon redeemed"
	if strings.TrimSpace(result.Preview.OrderNumber) != "" {
		subject = "KuponFast coupon redeemed - " + strings.TrimSpace(result.Preview.OrderNumber)
	}
	return subject
}

func buildRedemptionEmailText(result store.CouponRedemptionConfirmResult) (string, error) {
	var body bytes.Buffer
	if err := redemptionEmailTextTemplate.Execute(&body, result); err != nil {
		return "", err
	}
	return body.String(), nil
}

func buildRedemptionEmailHTML(result store.CouponRedemptionConfirmResult) (string, error) {
	var body bytes.Buffer
	if err := redemptionEmailHTMLTemplate.Execute(&body, result); err != nil {
		return "", err
	}
	return body.String(), nil
}

var redemptionEmailTextTemplate = texttemplate.Must(texttemplate.New("redemption-email-text").Parse(`A coupon was redeemed successfully.

Order: {{.Preview.OrderNumber}}
Merchant: {{.Preview.MerchantName}}
Coupon: {{.Preview.OfferTitle}}
Status: {{.Preview.Redemption.Status}}
{{if .Preview.Redemption.RedeemedAt}}Redeemed at: {{.Preview.Redemption.RedeemedAt.Format "15:04 02/01/2006"}}{{end}}
{{if .Preview.BuyerDisplay}}Buyer: {{.Preview.BuyerDisplay}}{{end}}
{{if .Preview.BuyerTelegramID}}Buyer Telegram ID: {{.Preview.BuyerTelegramID}}{{end}}

This email confirms that the QR code was scanned and the coupon was marked as redeemed in KuponFast.
`))

var redemptionEmailHTMLTemplate = htmltemplate.Must(htmltemplate.New("redemption-email-html").Parse(`<!doctype html>
<html>
<body>
  <p>A coupon was redeemed successfully.</p>
  <table>
    <tr><td><strong>Order</strong></td><td>{{.Preview.OrderNumber}}</td></tr>
    <tr><td><strong>Merchant</strong></td><td>{{.Preview.MerchantName}}</td></tr>
    <tr><td><strong>Coupon</strong></td><td>{{.Preview.OfferTitle}}</td></tr>
    <tr><td><strong>Status</strong></td><td>{{.Preview.Redemption.Status}}</td></tr>
    {{if .Preview.Redemption.RedeemedAt}}<tr><td><strong>Redeemed at</strong></td><td>{{.Preview.Redemption.RedeemedAt.Format "15:04 02/01/2006"}}</td></tr>{{end}}
    {{if .Preview.BuyerDisplay}}<tr><td><strong>Buyer</strong></td><td>{{.Preview.BuyerDisplay}}</td></tr>{{end}}
    {{if .Preview.BuyerTelegramID}}<tr><td><strong>Buyer Telegram ID</strong></td><td>{{.Preview.BuyerTelegramID}}</td></tr>{{end}}
  </table>
  <p>This email confirms that the QR code was scanned and the coupon was marked as redeemed in KuponFast.</p>
</body>
</html>`))
