# API App

Single Go service for the coupon sales MVP.

It owns:
- Telegram bot webhooks
- manual PayBox payment claims
- embedded admin pages
- offer, predefined code, order, payment claim, QR delivery, and redemption state

## Database

The MVP uses standard PostgreSQL connectivity from the Go backend only.

- Supabase is the initial managed PostgreSQL host for fast MVP setup.
- The app still connects through a normal `DATABASE_URL`.
- Repository code and SQL migrations stay portable to any ordinary PostgreSQL host later.
- Telegram clients and browsers never connect to the database directly.

## Local Development

Run the service from the repo root so `apps/api` can pick up the shared `.env` file automatically.

1. Copy `.env.example` to `.env`.
2. Set `DATABASE_URL` to either:
   - a local PostgreSQL URL such as `postgres://postgres:postgres@127.0.0.1:5432/coupons?sslmode=disable`
   - or the Supabase Postgres connection string for your project
3. Fill the remaining app secrets in `.env`.

The API accepts a plain PostgreSQL URL and uses `pgxpool` directly. No ORM or Supabase SDK is required.

## PayBox MVP Flow

The MVP payment flow is manual PayBox approval:

- `/start` shows the active offer catalog.
- Buyer taps Buy and receives the merchant PayBox payment link.
- Buyer uploads the PayBox payment screenshot in the Telegram chat.
- Admin reviews pending claims at `/admin/payments`.
- Approval assigns a predefined code, sends a QR code to the buyer, and records delivery.
- Merchant scans the QR code, which calls `/api/redemptions/scan/{token}` and records redemption in the database.

## Supabase Notes

- Treat Supabase as hosting, not as a platform dependency.
- Use the database only from the Go service.
- Keep `sslmode=require` on hosted Supabase connection strings.
- If you use a pooled Supabase connection string, leave `DATABASE_QUERY_EXEC_MODE=exec` so the app does not depend on prepared-statement caches that can conflict with transaction pooling.
- On startup, the API logs a warning if a Supabase URL is missing strict SSL or if a Supabase pooler URL is paired with a non-`exec` query mode.
- If you move to another PostgreSQL host later, the same schema, repository code, and `DATABASE_URL` pattern continue to work.

## Migrations

Migrations live in [migrations](./migrations) as plain SQL files and can be applied either with standard PostgreSQL tooling or with the built-in migration command.

PowerShell example:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/migrations/0001_initial_schema.sql
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/migrations/0002_store_invariants.sql
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/migrations/0003_ops_hardening_indexes.sql
```

That works against local Postgres, Supabase Postgres, or another managed PostgreSQL provider as long as `DATABASE_URL` points at the target database.

Built-in migration command:

```powershell
go run ./apps/api/cmd/migrate status
go run ./apps/api/cmd/migrate up
go run ./apps/api/cmd/migrate down
```

Notes:

- `status` shows whether each local migration is `pending`, `applied`, `drifted`, or missing locally.
- `up` creates `schema_migrations` if needed and applies any pending migrations in order.
- `down` reverts only the most recently applied local migration using the matching `.down.sql` file.
- This is safe to use against Supabase with the current pooled connection string because the runner uses normal PostgreSQL transactions and table locks, not session-level migration locks.

## Catalog Import

To import external catalog listings from a HAR export and download their primary product photos into `data/photos`, run:

```powershell
go run ./apps/api/cmd/import_catalog -input data/GetCategoryById_6982.txt -photos-dir data/photos
```

If you want the importer to compute a resale price from the source cost while covering payment fees, pass the fee inputs:

```powershell
go run ./apps/api/cmd/import_catalog `
  -input data/GetCategoryById_6982.txt `
  -photos-dir data/photos `
  -payment-fee-rate 0.0349 `
  -payment-fixed-fee 0.49
```

You can also set these in `.env` so the importer uses them by default:

- `PAYMENT_FEE_PERCENT_RATE=0.0349`
- `PAYMENT_FIXED_FEE_AMOUNT=0.49`

The importer uses this formula:

- `profit = (coupon_value_amount - sale_price_amount) / 2`
- `sale_price_amount - source_cost_amount - payment_fee = profit`

That resolves to:

- `sale_price_amount = (coupon_value_amount + 2*source_cost_amount + 2*payment_fixed_fee) / (3 - 2*payment_fee_rate)`

The importer stores the original HAR supplier price in `sale_price_amount` and the computed customer-facing price in `resell_price_amount`.

If the computed sale price is greater than or equal to `coupon_value_amount`, the importer skips that listing and removes any matching previously imported listing from sale.

Use `-dry-run` to validate the HAR and see how many sources/listings would be imported without touching the database or downloading files.

## Optional Database Settings

In addition to `DATABASE_URL`, the app supports these portable `pgxpool` settings:

- `DATABASE_APPLICATION_NAME`
- `DATABASE_CONNECT_TIMEOUT`
- `DATABASE_MAX_CONNS`
- `DATABASE_MIN_CONNS`
- `DATABASE_MAX_CONN_LIFETIME`
- `DATABASE_MAX_CONN_IDLE_TIME`
- `DATABASE_HEALTH_CHECK_PERIOD`
- `DATABASE_QUERY_EXEC_MODE`

`DATABASE_QUERY_EXEC_MODE` accepts `cache_statement`, `cache_describe`, `describe_exec`, `exec`, or `simple_protocol`.

## Ops Hardening

The API now runs a small in-process ops loop for launch readiness:

- `OPS_SWEEP_INTERVAL` controls how often the service sweeps expired coupons and listing statuses.
- `OPS_DELIVERY_ALERT_AFTER` controls when paid-but-undelivered orders are escalated into support cases.
- `OPS_BATCH_SIZE` controls how many orders each ops cycle inspects per category.

Admin operations now keep audit rows for source changes, listing changes, coupon inventory changes, and support outcomes. The dashboard also highlights:

- paid-but-undelivered orders
- pending payment reconciliations
- recent admin actions
