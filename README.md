# KuponFast Coupon Bot

Agent-driven repo scaffold for a Telegram-based coupon commerce MVP.

## Goals
- Fast MVP delivery
- Clear current-flow documentation
- Low-complexity go-to-market validation

## Current Product
The current product is a Telegram-first coupon sales flow for small businesses in Israel:

- fixed merchant-partner offers
- PayBox payment links
- manual payment screenshot approval by an internal Telegram admin chat
- QR delivery to the buyer after approval
- merchant QR scan redemption
- optional merchant email confirmation after first successful redemption

The source of truth for the implemented flow is [docs/actual-flow.md](docs/actual-flow.md).

## Database Approach

The MVP starts on Supabase PostgreSQL for managed hosting, but the backend stays portable:

- the Go service connects through a standard `DATABASE_URL`
- SQL migrations remain plain PostgreSQL files
- the repo does not depend on Supabase Auth, Realtime, Storage, or Edge Functions

## Payments

The current flow uses merchant-approved PayBox links. Buyers upload a payment screenshot in Telegram, admins approve or reject from Telegram, and approved buyers receive a QR code.

See [apps/api/README.md](apps/api/README.md) for backend setup details.

## Go Commands

The Go module for the backend lives in `apps/api`.

- From the repo root, run the API with `go run ./apps/api/cmd/server`.
- From the repo root, check or apply database migrations with `go run ./apps/api/cmd/migrate status` and `go run ./apps/api/cmd/migrate up`.
- For module maintenance commands such as `go mod tidy`, run them inside `apps/api`.
