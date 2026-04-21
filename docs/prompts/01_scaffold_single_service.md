# Prompt 01: Scaffold Single Go Service

You are a senior Go engineer. Implement the minimal project skeleton for the MVP described in `00_source_of_truth.md`.

## Goal
Create one Go service that will own:
- Telegram bot webhook handling
- payment webhooks
- admin pages
- inventory, order, and delivery logic

## Important Constraint
Do not create separate deployable services for `bot`, `admin`, or `api`.

Use one service in `apps/api` as the only runtime application for the MVP.

## Required Project Shape
Use a structure close to:

```text
apps/api/
  cmd/server/main.go
  internal/config/
  internal/http/
  internal/telegram/
  internal/payments/
  internal/store/
  internal/services/
  internal/admin/
  internal/security/
  migrations/
  templates/
```

## What To Build In This Step
- Go module and entrypoint
- environment-based config
- HTTP server bootstrap
- structured logging
- database connection bootstrap
- health endpoint
- stub route registration for:
  - `/healthz`
  - `/webhooks/telegram`
  - `/webhooks/payments/{provider}`
  - `/admin`

## Config To Support
At minimum:
- `PORT`
- `DATABASE_URL`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_WEBHOOK_SECRET`
- `PAYMENT_PROVIDER_NAME`
- `PAYMENT_PROVIDER_SECRET`
- `PAYMENT_PROVIDER_WEBHOOK_SECRET`
- `ADMIN_BASIC_AUTH_USER`
- `ADMIN_BASIC_AUTH_PASS`
- `COUPON_ENCRYPTION_KEY`

## Simplicity Rules
- standard library first
- avoid dependency-heavy frameworks
- avoid DI frameworks
- avoid CQRS/event-bus patterns
- avoid separate frontend build toolchain

## Acceptance Criteria
- service starts locally
- config validation fails fast on missing required env vars
- database connection is created on startup
- health endpoint works
- webhook and admin route groups exist, even if some handlers are stubbed
- logs are structured and readable
