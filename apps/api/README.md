# API App

Single Go service for the coupon sales MVP.

It owns:
- Telegram bot webhooks
- payment webhooks
- PayPal checkout handoff and verified payment callbacks
- embedded admin pages
- inventory, listing, order, payment, and delivery state

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

## PayPal

The MVP payment flow uses PayPal-hosted checkout links.

- The bot reserves one coupon before showing the PayPal approval link.
- The user sees a checkout-ready message only after that reservation succeeds.
- The reservation is temporary and expires automatically if payment is not confirmed in time.
- Users pay on PayPal with whatever funding sources the merchant account exposes, such as card or wallet.
- The backend verifies PayPal webhook signatures through PayPal's verification API.
- Coupon delivery happens only after a verified PayPal success event for the same reserved order.
- If a late payment arrives after the checkout hold has already expired, the payment is recorded and escalated for manual refund or support review instead of auto-delivering a different coupon.

Required payment settings:

- `PAYMENT_PROVIDER_NAME=paypal`
- `PAYMENT_PROVIDER_CLIENT_ID`
- `PAYMENT_PROVIDER_SECRET`
- `PAYMENT_PROVIDER_BASE_URL`
- `PAYMENT_PROVIDER_WEBHOOK_ID`

PayPal base URL values:

- sandbox: `https://api-m.sandbox.paypal.com`
- live: `https://api-m.paypal.com`

Webhook expectations:

- subscribe the PayPal webhook to the events needed for hosted checkout fulfillment
- include at least `CHECKOUT.ORDER.APPROVED` and `PAYMENT.CAPTURE.COMPLETED`
- point the webhook at `/webhooks/payments/paypal`
- configure the webhook ID in `PAYMENT_PROVIDER_WEBHOOK_ID`

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
- `OPS_RECONCILE_AFTER` controls when stale PayPal checkouts are rechecked for delayed callbacks.
- `OPS_CHECKOUT_HOLD_DURATION` controls how long a reserved coupon stays on hold while the user completes PayPal checkout.
- `OPS_BATCH_SIZE` controls how many orders each ops cycle inspects per category.

Checkout-hold behavior:

- creating a checkout now reserves exactly one `available` coupon and marks it `reserved`
- sold-out listings no longer show buy actions in Telegram
- stale `pending_payment` orders older than `OPS_CHECKOUT_HOLD_DURATION` are cancelled automatically
- releasing a stale hold returns the coupon to `available`, clears `orders.coupon_id`, and stores `failure_reason=checkout_hold_expired`

Admin operations now keep audit rows for source changes, listing changes, coupon inventory changes, and support outcomes. The dashboard also highlights:

- paid-but-undelivered orders
- pending payment reconciliations
- recent admin actions
