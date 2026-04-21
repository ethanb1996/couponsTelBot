# Prompt 06: Ops Hardening And Launch Readiness

You are a senior Go engineer. Harden the minimal coupon sales MVP just enough to start selling safely.

## Goal
Add only the operational protections that matter for:
- money
- coupon delivery
- support handling
- debugging

## Required Hardening

### Background Jobs
- expire old listings and expired coupons
- detect paid-but-undelivered orders
- reconcile delayed payment callbacks if needed

### Auditability
- log admin actions for inventory changes, listing changes, and support outcomes
- keep enough evidence to investigate disputes

### Observability
- structured logs
- request IDs where practical
- clear error paths for payment and delivery failures

### Security
- protect admin routes
- keep payment secrets out of logs
- keep coupon secrets encrypted at rest

### Supportability
- minimal operator runbook for:
  - invalid coupon complaint
  - paid but not delivered
  - duplicate charge claim
  - chargeback review

## Testing Priorities
Focus on a small number of high-value tests:
- assign exactly one coupon under concurrency
- payment webhook idempotency
- delivery idempotency
- sold-out listing behavior
- final-sale acknowledgment persistence

## Launch Checklist
Before calling the MVP sellable, confirm:
- at least one listing can be created and published
- bot message shows `1` to `3` coupon options with `Buy` buttons
- payment provider accepts `ILS`
- successful payment triggers delivery
- operator can inspect order, payment, coupon, and delivery history
- support case can be logged
- no-refund terms are displayed before payment

## Keep Scope Tight
Do not add:
- refund automation
- merchant portal
- analytics warehouse
- recommendation engine
- multi-region deployment

## Acceptance Criteria
- the system can survive normal operator mistakes
- paid-but-undelivered orders are detectable quickly
- the team has enough logs and admin tooling to investigate complaints
- the MVP is narrow enough to sell without a large ops team
