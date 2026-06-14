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
- The internal review chat set by `TELEGRAM_ADMIN_REVIEW_CHAT_ID` receives the screenshot with approve/reject buttons. If unset, the bot falls back to admin direct messages.
- Only Telegram users listed in `TELEGRAM_ADMIN_USER_IDS` may approve or reject.
- The web admin panel exposes `/admin/audit` for audit logs, with `/admin/payments` kept as a fallback claim queue.
- Approval assigns a predefined code, creates a redemption token, sends a QR code to the buyer, and records delivery.
- Merchant scans the QR code, which calls `/api/redemptions/scan/{token}` and shows a Hebrew coupon review page without redeeming.
- The restaurant clicks the Hebrew redeem button, which posts to `/api/redemptions/redeem/{token}`; first redemption updates state, notifies the admin review chat, and notifies the configured restaurant Telegram chat/channel.
- On first redemption, the API can also send the merchant a confirmation email through Resend when configured.

Set `TELEGRAM_ADMIN_USER_IDS` to a comma-separated list of Telegram user IDs, for example `TELEGRAM_ADMIN_USER_IDS=123456,987654`.
Set `TELEGRAM_ADMIN_REVIEW_CHAT_ID` to a signed Telegram chat id for the internal review chat.
Set `RESEND_API_KEY` and `RESEND_FROM` to enable merchant redemption emails. Replace `re_xxxxxxxxx` with your real Resend API key.

The full implemented flow is documented in [docs/actual-flow.md](../../docs/actual-flow.md).

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

## Admin And Audit

Admin operations keep audit rows for payment claim approval/rejection, support cases, inventory changes, and sensitive internal actions. The embedded admin pages remain available for operational fallback, while the primary review loop is the Telegram admin review chat.
