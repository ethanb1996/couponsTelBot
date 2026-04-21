# API App

Single Go service for the coupon sales MVP.

It owns:
- Telegram bot webhooks
- payment webhooks
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

## Supabase Notes

- Treat Supabase as hosting, not as a platform dependency.
- Use the database only from the Go service.
- Keep `sslmode=require` on hosted Supabase connection strings.
- If you use a pooled Supabase connection string, leave `DATABASE_QUERY_EXEC_MODE=exec` so the app does not depend on prepared-statement caches that can conflict with transaction pooling.
- If you move to another PostgreSQL host later, the same schema, repository code, and `DATABASE_URL` pattern continue to work.

## Migrations

Migrations live in [migrations](./migrations) as plain SQL files and can be applied with standard PostgreSQL tooling.

PowerShell example:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/migrations/0001_initial_schema.sql
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/migrations/0002_store_invariants.sql
```

That works against local Postgres, Supabase Postgres, or another managed PostgreSQL provider as long as `DATABASE_URL` points at the target database.

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
