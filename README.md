# Bazario Coupon Bot

Agent-driven repo scaffold for a Telegram-based coupon commerce MVP.

## Goals
- Fast MVP delivery
- Clear product and architecture ownership
- Low-complexity go-to-market validation
- Reusable Codex agent setup

## Product Direction
The current target product is a Telegram-first coupon sales flow for small businesses in Israel:
- fixed merchant-partner offers
- PayBox payment links
- manual payment verification
- predefined code delivery in Telegram

This repo still contains legacy automated payment and resale-oriented implementation work. The target architecture docs now describe the manual PayBox v1 operating model that should guide future refactors.

## Core roles
- `prd` agent: defines roadmap and requirements
- `architect` agent: designs the system and implementation shape
- `orchestrator` agent: coordinates both, resolves conflicts, and converts strategy into execution

## First steps
1. Read `PROJECT_CONTEXT.md`
2. Review `.codex/agents/`
3. Start with `docs/prd/mvp_prd.md`
4. Review `docs/architecture/`
5. Implement inside `apps/`

## Database Approach

The MVP starts on Supabase PostgreSQL for managed hosting, but the backend stays portable:

- the Go service connects through a standard `DATABASE_URL`
- SQL migrations remain plain PostgreSQL files
- the repo does not depend on Supabase Auth, Realtime, Storage, or Edge Functions

## Payments

The target v1 flow uses merchant-approved PayBox payment links with manual buyer claim submission and manual admin verification before coupon delivery.

Some runtime code still reflects an older PayPal/webhook design. Treat the architecture docs as the source of truth for the next refactor phase.

See [apps/api/README.md](/abs/path/c:/Projects/couponsTelBot/apps/api/README.md) for backend setup details.

## Go Commands

The Go module for the backend lives in `apps/api`.

- From the repo root, run the API with `go run ./apps/api/cmd/server`.
- From the repo root, check or apply database migrations with `go run ./apps/api/cmd/migrate status` and `go run ./apps/api/cmd/migrate up`.
- For module maintenance commands such as `go mod tidy`, run them inside `apps/api`.
