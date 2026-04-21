# Prompt 03: Admin, Inventory, And Minimal Operations

You are a senior Go engineer. Build the smallest possible admin experience inside the Go service.

## Goal
Allow a small team to operate the business manually without a separate frontend app.

## Important Constraint
Use server-rendered HTML templates inside the same Go service.

Do not build:
- a SPA
- React/Vue frontend
- a separate admin deployable

## Authentication
Use simple MVP-safe protection:
- HTTP Basic Auth or an equally small session-based approach
- no complex RBAC
- one operator role is enough

## Admin Features To Implement

### Inventory Source Management
- create and edit coupon sources
- record rights status and notes

### Listing Management
- create listing
- edit listing
- publish listing
- pause listing
- mark listing sold out or expired

### Coupon Inventory Management
- add coupons manually
- support paste/bulk import from text area or CSV upload
- show inventory count per listing
- mark coupon as voided or disputed

### Order And Delivery Review
- list recent orders
- filter by status
- inspect linked payment and delivery status

### Support Case Review
- create case
- view case
- resolve case

## Keep It Minimal
Do not build:
- merchant self-serve
- dashboards with heavy charting
- complicated search infrastructure
- permissions matrix

## Suggested Pages
- `/admin`
- `/admin/sources`
- `/admin/listings`
- `/admin/listings/{id}`
- `/admin/coupons`
- `/admin/orders`
- `/admin/orders/{id}`
- `/admin/support`

## Acceptance Criteria
- operator can ingest coupons
- operator can create a listing
- operator can publish/pause a listing
- operator can inspect which coupon was assigned to which order
- operator can log and resolve a support case
