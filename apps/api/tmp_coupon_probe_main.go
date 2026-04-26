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
  if execMode == "" { execMode = "exec" }
  ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
  defer cancel()
  db, err := store.Open(ctx, store.Options{DatabaseURL: dbURL, ApplicationName: "debug-coupons", ConnectTimeout: 10*time.Second, MaxConns: 1, DefaultQueryExecMode: execMode})
  if err != nil { panic(err) }
  defer db.Close()
  rows, err := db.Pool.Query(ctx, `select c.id, c.inventory_status, c.expiry_at, l.title from coupons c join listings l on l.id = c.listing_id order by c.id desc`)
  if err != nil { panic(err) }
  defer rows.Close()
  for rows.Next() {
    var id int64
    var status, title string
    var expiry time.Time
    if err := rows.Scan(&id, &status, &expiry, &title); err != nil { panic(err) }
    fmt.Printf("coupon id=%d status=%s expiry=%s title=%s\n", id, status, expiry.UTC().Format(time.RFC3339), title)
  }
}
