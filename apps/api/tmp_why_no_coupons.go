package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/dbmigrate"
	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func main() {
	wd, _ := os.Getwd()
	_ = dbmigrate.LoadDotEnvUpward(wd)
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	execMode := strings.TrimSpace(os.Getenv("DATABASE_QUERY_EXEC_MODE"))
	if execMode == "" {
		execMode = "exec"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := store.Open(ctx, store.Options{DatabaseURL: dbURL, ApplicationName: "why-no-coupons", ConnectTimeout: 10 * time.Second, MaxConns: 1, DefaultQueryExecMode: execMode})
	if err != nil {
		panic(err)
	}
	defer db.Close()

	queries := []struct{ label, sql string }{
		{"all_imported", `select count(*) from listings where external_import_key <> ''`},
		{"active_imported", `select count(*) from listings where external_import_key <> '' and status = 'active'`},
		{"discounted_imported", `select count(*) from listings where external_import_key <> '' and coupon_value_amount > coalesce(nullif(resell_price_amount, 0), sale_price_amount)`},
		{"resell_populated_imported", `select count(*) from listings where external_import_key <> '' and resell_price_amount > 0`},
		{"resell_mismatch_imported", `select count(*) from listings where external_import_key <> '' and resell_price_amount <> sale_price_amount`},
		{"active_discounted_imported", `select count(*) from listings where external_import_key <> '' and status = 'active' and coupon_value_amount > coalesce(nullif(resell_price_amount, 0), sale_price_amount)`},
		{"active_discounted_with_inventory", `
      select count(*)
      from listings l
      where l.status = 'active'
        and l.coupon_value_amount > coalesce(nullif(l.resell_price_amount, 0), l.sale_price_amount)
        and exists (
          select 1 from coupons c
          where c.listing_id = l.id
            and c.inventory_status = 'available'
            and c.expiry_at > now()
        )`},
	}

	for _, q := range queries {
		var count int64
		if err := db.Pool.QueryRow(ctx, q.sql).Scan(&count); err != nil {
			panic(err)
		}
		fmt.Printf("%s=%d\n", q.label, count)
	}

	rows, err := db.Pool.Query(ctx, `
    select l.id, l.status, l.merchant_name, l.title, l.coupon_value_amount, l.sale_price_amount, l.resell_price_amount,
      count(c.id) filter (where c.inventory_status='available' and c.expiry_at > now()) as available
    from listings l
    left join coupons c on c.listing_id = l.id
    where l.coupon_value_amount > coalesce(nullif(l.resell_price_amount, 0), l.sale_price_amount)
    group by l.id
    order by available desc, l.id
    limit 15
  `)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, value, sale, resell, available int64
		var status, merchant, title string
		if err := rows.Scan(&id, &status, &merchant, &title, &value, &sale, &resell, &available); err != nil {
			panic(err)
		}
		fmt.Printf("id=%d status=%s available=%d value=%d sale=%d resell=%d merchant=%s title=%s\n", id, status, available, value, sale, resell, merchant, title)
	}
}
