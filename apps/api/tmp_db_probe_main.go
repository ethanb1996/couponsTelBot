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
  if err := dbmigrate.LoadDotEnvUpward(wd); err != nil { panic(err) }
  dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
  execMode := strings.TrimSpace(os.Getenv("DATABASE_QUERY_EXEC_MODE"))
  if execMode == "" { execMode = "exec" }
  ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
  defer cancel()
  db, err := store.Open(ctx, store.Options{DatabaseURL: dbURL, ApplicationName: "debug-state", ConnectTimeout: 10*time.Second, MaxConns: 1, MinConns: 0, MaxConnLifetime: 30*time.Minute, MaxConnIdleTime: 5*time.Minute, HealthCheckPeriod: time.Minute, DefaultQueryExecMode: execMode})
  if err != nil { panic(err) }
  defer db.Close()
  var listings, active, imported, coupons, available int64
  must := func(err error) { if err != nil { panic(err) } }
  must(db.Pool.QueryRow(ctx, `select count(*) from listings`).Scan(&listings))
  must(db.Pool.QueryRow(ctx, `select count(*) from listings where status='active'`).Scan(&active))
  must(db.Pool.QueryRow(ctx, `select count(*) from listings where external_import_key <> ''`).Scan(&imported))
  must(db.Pool.QueryRow(ctx, `select count(*) from coupons`).Scan(&coupons))
  must(db.Pool.QueryRow(ctx, `select count(*) from coupons where inventory_status='available' and expiry_at > now()`).Scan(&available))
  fmt.Printf("listings=%d active=%d imported=%d coupons=%d available=%d\n", listings, active, imported, coupons, available)
  rows, err := db.Pool.Query(ctx, `select id, title, status, external_import_key, photo_key from listings order by id desc limit 10`)
  must(err)
  defer rows.Close()
  for rows.Next() {
    var id int64
    var title, status, key, photo string
    must(rows.Scan(&id, &title, &status, &key, &photo))
    fmt.Printf("id=%d status=%s key=%s photo=%s title=%s\n", id, status, key, photo, title)
  }
}
