package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	// Load from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Insert test merchant
	var merchantID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO merchant_partners (business_name, contact_reference, status, merchant_disclosure_text, support_contact)
		VALUES ('Bazario Test', 'test_contact', 'active', 'This offer is sold on behalf of Bazario Test', '@admin')
		RETURNING id
	`).Scan(&merchantID)
	if err != nil {
		log.Fatalf("Failed to insert merchant: %v", err)
	}
	fmt.Printf("✓ Created merchant partner ID: %d\n", merchantID)

	// Insert test offer
	var offerID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO offers (merchant_partner_id, merchant_name, title, description, price_amount, currency_code, payment_link, merchant_disclosure_text, redemption_terms, support_contact, status, published_at)
		VALUES ($1, 'Bazario Test', 'Test Coupon 20% Off', '20% discount on all products', 5000, 'ILS', 'https://paybox.money/p/test-link', 'This offer is sold on behalf of Bazario Test', 'Valid for 30 days. Redeem online at bazario.test', '@admin', 'active', NOW())
		RETURNING id
	`, merchantID).Scan(&offerID)
	if err != nil {
		log.Fatalf("Failed to insert offer: %v", err)
	}
	fmt.Printf("✓ Created offer ID: %d\n", offerID)

	// Insert 5 test predefined codes
	encryptedCode := strings.Repeat("fc", 32)
	for i := 1; i <= 5; i++ {
		var codeID int64
		err = conn.QueryRow(ctx, `
			INSERT INTO predefined_codes (offer_id, merchant_partner_id, code_encrypted, code_masked_display, status, expiry_at, issued_batch_reference)
			VALUES ($1, $2, decode($3, 'hex'), $4, 'available', NOW() + INTERVAL '30 days', 'batch_001')
			RETURNING id
		`, offerID, merchantID, encryptedCode, fmt.Sprintf("BAZARIO-2025-%04d", i)).Scan(&codeID)
		if err != nil {
			log.Fatalf("Failed to insert code %d: %v", i, err)
		}
		fmt.Printf("  • Created predefined code %d: BAZARIO-2025-%04d\n", codeID, i)
	}

	fmt.Println("\n✓ Test data setup complete!")
	fmt.Printf("\nReady for live test:\n")
	fmt.Printf("  • Merchant: Bazario Test (ID: %d)\n", merchantID)
	fmt.Printf("  • Offer: Test Coupon 20% Off (ID: %d)\n", offerID)
	fmt.Printf("  • Admin Telegram ID: 8319213106\n")
	fmt.Printf("  • Payment link: https://paybox.money/p/test-link\n")
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Start API server: go run ./apps/api/cmd/server/main.go\n")
	fmt.Printf("  2. Send /start to bot to see the offer\n")
	fmt.Printf("  3. Open admin: http://localhost:8080/admin/orders\n")
}
