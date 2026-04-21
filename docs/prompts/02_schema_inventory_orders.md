# Prompt 02: Schema, Inventory, And Orders

You are a senior Go engineer. Implement the minimal PostgreSQL schema and store layer for the coupon sales MVP.

## Goal
Create the smallest correct schema that supports:
- owned coupon inventory
- sellable listings
- orders
- payments
- coupon delivery
- support logging

## Prefer This Simplified Model
Use this model even if older docs suggest something more elaborate:

- `coupon_sources`
- `listings`
- `coupons`
- `orders`
- `payments`
- `coupon_deliveries`
- `support_cases`
- `admin_actions`

## Important Relationship Decisions
- one `listing` is one sellable SKU
- one `listing` can have many `coupons`
- one `coupon_source` can supply many `coupons`
- one `listing` can produce many `orders`
- one successful `order` gets exactly one `coupon`
- `support_cases` attach primarily to `order_id`, optionally to `coupon_id`

## Required Data Behaviors
- coupons are ingested before sale
- coupons move through statuses like:
  - `available`
  - `reserved`
  - `assigned`
  - `delivered`
  - `expired`
  - `voided`
  - `disputed`
- orders move through statuses like:
  - `draft`
  - `pending_payment`
  - `paid`
  - `delivery_pending`
  - `delivered`
  - `failed`
  - `cancelled`
  - `disputed`
- payments are separate from orders
- deliveries are separate from payments

## Must-Have Business Invariants
- a coupon cannot be delivered before payment success
- a coupon cannot be assigned to more than one order
- a sold-out listing must not keep accepting new purchases
- final-sale acknowledgment must be stored with the order
- no `refunds` table in the initial MVP

## What To Build
- SQL migrations
- indexes and uniqueness constraints where needed
- store/repository methods for:
  - create source
  - create listing
  - ingest coupons for a listing
  - list active listings
  - create draft order
  - mark order pending payment
  - record payment event
  - assign one available coupon transactionally
  - record delivery event
  - create support case

## Transaction Requirement
Coupon assignment must happen transactionally and safely under concurrency.

Preferred behavior:
- lock one available coupon row for the chosen listing
- assign it to the order
- prevent duplicate assignment

## Acceptance Criteria
- schema migrations apply cleanly
- store layer can create inventory and listings
- store layer can create orders and payments
- store layer can assign one coupon safely
- sold-out behavior is detectable from the database state
