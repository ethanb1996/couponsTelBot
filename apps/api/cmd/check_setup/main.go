package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/config"
	"github.com/jackc/pgx/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT
			o.id,
			o.status,
			o.merchant_name,
			o.title,
			o.description,
			o.price_amount,
			o.published_at,
			COUNT(pc.id) FILTER (
				WHERE pc.status = 'available'
					AND (pc.expiry_at IS NULL OR pc.expiry_at > NOW())
			) AS available_codes
		FROM offers o
		LEFT JOIN predefined_codes pc ON pc.offer_id = o.id
		GROUP BY o.id
		ORDER BY o.published_at DESC NULLS LAST, o.id DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Failed to query offers: %v", err)
	}
	defer rows.Close()

	fmt.Println("Top offers in database:")
	for rows.Next() {
		var (
			id             int64
			status         string
			merchantName   string
			title          string
			description    string
			priceAmount    int64
			publishedAt    any
			availableCodes int64
		)
		if err := rows.Scan(&id, &status, &merchantName, &title, &description, &priceAmount, &publishedAt, &availableCodes); err != nil {
			log.Fatalf("Failed to scan offer: %v", err)
		}
		fmt.Printf("ID=%d status=%s available=%d merchant=%q title=%q description=%q price=%d published=%v\n",
			id, status, availableCodes, merchantName, title, description, priceAmount, publishedAt)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("Rows error: %v", err)
	}
}
