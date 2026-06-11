package services

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"text/template"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type SMTPRedemptionNotifierOptions struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPRedemptionNotifier struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewSMTPRedemptionNotifier(options SMTPRedemptionNotifierOptions) (*SMTPRedemptionNotifier, error) {
	host := strings.TrimSpace(options.Host)
	from := strings.TrimSpace(options.From)
	if host == "" {
		return nil, nil
	}
	if from == "" {
		return nil, fmt.Errorf("smtp from is required")
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("smtp from must be a valid email address: %w", err)
	}
	port := options.Port
	if port <= 0 {
		port = 587
	}
	return &SMTPRedemptionNotifier{
		host:     host,
		port:     port,
		username: strings.TrimSpace(options.Username),
		password: strings.TrimSpace(options.Password),
		from:     fromAddress.Address,
	}, nil
}

func (n *SMTPRedemptionNotifier) NotifyCouponRedeemed(ctx context.Context, result store.CouponRedemptionScanResult) error {
	if n == nil || !result.FirstScan {
		return nil
	}
	to, ok := redemptionMerchantEmail(result)
	if !ok {
		return nil
	}
	message, err := buildRedemptionEmailMessage(n.from, to, result)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(n.host, fmt.Sprintf("%d", n.port))
	var auth smtp.Auth
	if n.username != "" || n.password != "" {
		auth = smtp.PlainAuth("", n.username, n.password, n.host)
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, n.from, []string{to}, message)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func redemptionMerchantEmail(result store.CouponRedemptionScanResult) (string, bool) {
	address, err := mail.ParseAddress(strings.TrimSpace(result.MerchantContact))
	if err != nil || address.Address == "" {
		return "", false
	}
	return address.Address, true
}

func buildRedemptionEmailMessage(from string, to string, result store.CouponRedemptionScanResult) ([]byte, error) {
	subject := "KuponFast coupon redeemed"
	if strings.TrimSpace(result.OrderNumber) != "" {
		subject = "KuponFast coupon redeemed - " + strings.TrimSpace(result.OrderNumber)
	}

	var body bytes.Buffer
	if err := redemptionEmailTemplate.Execute(&body, result); err != nil {
		return nil, err
	}

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body.String()), nil
}

var redemptionEmailTemplate = template.Must(template.New("redemption-email").Parse(`A coupon was redeemed successfully.

Order: {{.OrderNumber}}
Merchant: {{.MerchantName}}
Coupon: {{.OfferTitle}}
Status: {{.Redemption.Status}}
{{if .Redemption.ScannedAt}}Scanned at: {{.Redemption.ScannedAt.Format "15:04 02/01/2006"}}{{end}}
{{if .BuyerDisplay}}Buyer: {{.BuyerDisplay}}{{end}}
{{if .BuyerTelegramID}}Buyer Telegram ID: {{.BuyerTelegramID}}{{end}}

This email confirms that the QR code was scanned and the coupon was marked as redeemed in KuponFast.
`))
