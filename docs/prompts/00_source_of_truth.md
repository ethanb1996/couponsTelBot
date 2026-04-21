# Prompt 00: Source Of Truth

You are a senior Go engineer. Build the smallest production-minded MVP needed to start selling pre-bought coupons in Telegram.

## Product TL;DR
The MVP business loop is:

1. buy coupon inventory in advance
2. store it securely
3. list it in Telegram
4. accept payment in `ILS`
5. deliver the coupon to the user in Telegram

## What This MVP Is
- a direct Telegram coupon sales bot
- a controlled inventory system
- a single-service Go application with an embedded admin interface
- a manual-ops-friendly MVP that can start selling quickly

## What This MVP Is Not
- not a coupon discovery marketplace
- not an affiliate business
- not a partner-distribution network
- not a multi-service architecture
- not a separate bot app plus separate admin app
- not a self-serve merchant platform
- not a refund-heavy commerce flow

## Hard Product Rules
- only sell coupons already owned by the business
- every Telegram sales message contains `1` to `3` coupon options
- each option has its own `Buy` button
- clicking a `Buy` button creates an order only for that selected coupon listing
- user must see final-sale / no-refund terms before payment
- payment success must be confirmed by the payment provider before delivery
- coupon must be delivered only after confirmed payment
- no normal refund flow exists in the MVP
- support logging still exists even without refunds

## Incoherences Removed
Implement the simpler model below even if older docs imply something more complex:

- one Go runtime service only
- Telegram bot is the only core user surface
- admin runs inside the same Go service as protected HTML pages
- `coupon_sources` attach to coupon inventory, not to public product strategy
- one `listing` is one sellable SKU
- one `listing` can have many owned `coupons`
- one successful `order` gets exactly one `coupon`
- no user preferences, recommendation engine, discovery feed, or sponsored placements
- no automated refund engine

## Minimal Technical Decisions
- language: Go
- runtime shape: one deployable service
- storage: PostgreSQL
- admin UI: server-rendered HTML templates
- logging: structured logs with `slog`
- HTTP: standard library first; only add a tiny router if it clearly simplifies webhooks
- database access: straightforward SQL, no heavy ORM

## Minimal Domain Model
- `coupon_sources`
- `listings`
- `coupons`
- `orders`
- `payments`
- `coupon_deliveries`
- `support_cases`
- `admin_actions`

Recommended simplification:
- `coupons` belong to `listing_id`
- `coupons` also belong to `source_id`
- `orders` belong to `listing_id` and optionally `coupon_id`
- no `refunds` table in the initial MVP

## Deliverable Standard
Every implementation step should:
- favor simplicity over abstraction
- favor direct SQL over framework magic
- avoid background complexity unless it protects money or delivery
- leave the app sellable, observable, and understandable by a small team

## Definition Of MVP Ready To Sell
- admin can ingest coupon inventory
- admin can publish/pause listings
- bot can send `1` to `3` coupon options per message
- user can press `Buy`
- user can pay in `ILS`
- successful payment leads to coupon delivery in Telegram
- all actions are logged well enough to investigate disputes
