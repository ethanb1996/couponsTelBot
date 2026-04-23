# Bazario Coupon Bot

Agent-driven repo scaffold for a Telegram-based coupon commerce MVP.

## Goals
- Fast MVP delivery
- Clear product and architecture ownership
- Legal-risk-aware product shaping
- Reusable Codex agent setup

## Core roles
- `prd` agent: defines roadmap and requirements
- `architect` agent: designs the system and implementation shape
- `orchestrator` agent: coordinates both, resolves conflicts, and converts strategy into execution

## First steps
1. Read `PROJECT_CONTEXT.md`
2. Review `.codex/agents/`
3. Start with `docs/prd/mvp_prd.md`
4. Convert approved scope into `docs/architecture/`
5. Implement inside `apps/`

## Database Approach

The MVP starts on Supabase PostgreSQL for managed hosting, but the backend stays portable:

- the Go service connects through a standard `DATABASE_URL`
- SQL migrations remain plain PostgreSQL files
- the repo does not depend on Supabase Auth, Realtime, Storage, or Edge Functions

## Payments

The initial checkout flow uses PayPal-hosted payment links and verified PayPal webhooks, while still keeping payment state and coupon delivery in the single Go service.

See [apps/api/README.md](apps/api/README.md) for local setup, Supabase SSL guidance, PayPal env vars, webhook setup expectations, migration commands, and optional `DATABASE_*` pool settings.

## Go Commands

The Go module for the backend lives in `apps/api`.

- From the repo root, run the API with `go run ./apps/api/cmd/server`.
- For module maintenance commands such as `go mod tidy`, run them inside `apps/api`.
