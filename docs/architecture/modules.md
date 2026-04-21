# Modules

## Module Overview
The MVP should be organized around a direct coupon sale flow, not a discovery marketplace. The recommended modules are:
- bot
- api
- admin
- inventory and catalog
- order and payment
- delivery and support
- analytics and audit
- background jobs

Modules intentionally excluded from MVP:
- merchant self-serve
- broad marketplace features
- scraping or browser automation
- automated refund engine

## 1. Bot Module

### Responsibilities
- receive Telegram updates through webhook handlers
- show coupon listings and detail views
- present pre-purchase terms including final-sale and no-refund disclosure
- start payment flow
- deliver purchased coupons after confirmed payment
- collect complaint or support messages

### What It Owns
- Telegram-specific message formatting
- Telegram command handling
- lightweight conversational state

### What It Does Not Own
- canonical coupon inventory
- payment confirmation rules
- supplier verification logic
- direct database writes outside approved API pathways

### Inputs
- Telegram webhook events
- active listing data from API
- payment and delivery endpoints from API

### Outputs
- Telegram messages
- checkout intents
- support events

### Failure Points
- Telegram API delivery failures
- invalid bot state transitions
- coupon delivery message failure

## 2. API Module

### Responsibilities
- act as the main business logic layer
- expose endpoints for bot and admin surfaces
- validate listing, coupon, order, payment, and delivery lifecycle changes
- enforce required metadata and status rules
- manage complaint and support states

### What It Owns
- inventory lifecycle rules
- order lifecycle rules
- payment lifecycle rules
- delivery gating rules
- authorization rules for admin versus user-facing operations

### What It Does Not Own
- raw card handling
- merchant-side automation
- Telegram rendering concerns

### Inputs
- bot requests
- admin requests
- job runner invocations
- payment-provider callbacks

### Outputs
- validated inventory and listings
- validated orders and payments
- delivery triggers
- audit entries

### Failure Points
- invalid state transitions
- duplicate order creation
- deployment bugs affecting all surfaces

## 3. Admin Module

### Responsibilities
- create coupon inventory entries
- create, edit, preview, publish, pause, expire, and remove listings
- inspect source provenance and disclosure metadata
- inspect coupon assignment and delivery history
- review complaints and support cases

### What It Owns
- operator user interface
- internal review flows
- publish and pause actions routed through API

### What It Does Not Own
- core business rules
- direct database writes
- external supplier integrations

### Inputs
- API read models for listings, coupons, orders, payments, and support cases

### Outputs
- operator actions sent to API

### Failure Points
- bad inventory entry
- weak pre-publish validation
- operator mistakes during assignment review

## 4. Inventory and Catalog Module

### Responsibilities
- store pre-bought coupon inventory
- represent Telegram-facing listings
- store source provenance, rights checks, and disclosure state
- manage listing status and inventory status

### What It Owns
- inventory intake checklist
- listing state machine
- coupon inventory state machine

### What It Does Not Own
- Telegram interaction logic
- payment execution
- supplier acquisition strategy

### Inputs
- operator intake actions
- expiry checks
- delivery results

### Outputs
- active listing set for bot delivery
- assigned inventory for paid orders
- audit-ready inventory history

### Failure Points
- incomplete source documentation
- overselling inventory
- stale or expired inventory left active

## 5. Order and Payment Module

### Responsibilities
- create orders from buy attempts
- start payment with an ILS-capable provider
- record payment success or failure
- ensure coupon assignment only happens after successful payment
- record final-sale / no-refund acknowledgment

### What It Owns
- order lifecycle
- payment lifecycle
- price snapshot at time of sale
- provider reference data

### What It Does Not Own
- raw card data
- Telegram rendering logic
- supplier verification

### Inputs
- buy requests from bot
- payment-provider callbacks
- admin actions for manual review

### Outputs
- order records
- payment records
- delivery trigger on successful payment
- dispute evidence

### Failure Points
- payment succeeds but callback is delayed
- duplicate charges
- chargeback pressure despite no-refund policy

## 6. Delivery and Support Module

### Responsibilities
- assign coupon inventory to a paid order
- deliver the coupon to the user in Telegram
- track delivery timestamp and evidence
- log user complaints and support cases
- support manual review of invalid coupon claims

### What It Owns
- assignment-to-delivery workflow
- support-case records
- delivery confirmation state

### What It Does Not Own
- payment-provider state
- supplier acquisition logic

### Inputs
- paid orders
- support requests
- admin review actions

### Outputs
- delivered coupon messages
- support-case status changes
- disputed or voided inventory markers

### Failure Points
- assigned coupon not delivered
- duplicate delivery
- complaint handling without enough evidence

## 7. Analytics and Audit Module

### Responsibilities
- capture listing views, purchases, payment outcomes, delivery outcomes, and complaints
- track complaint resolution times
- maintain audit logs of operator actions and status changes
- surface source quality, conversion, and dispute metrics

### What It Owns
- event model for MVP metrics
- audit log entries for sensitive actions
- operational reporting inputs

### What It Does Not Own
- recommendation engines
- external BI complexity not needed for MVP

### Inputs
- bot events
- admin actions
- API state changes

### Outputs
- dashboards or reports for operators
- metrics used in product decisions
- recovery context for incidents

### Failure Points
- missing payment or delivery events
- audit gaps during disputes
- analytics coupling that slows core flows

## 8. Background Jobs Module

### Responsibilities
- expire listings and inventory automatically based on timestamps
- reconcile delayed payment events
- run undelivered-order checks
- generate periodic summary metrics
- trigger non-blocking operational notifications

### What It Owns
- safe internal automation only
- retries for internal asynchronous tasks

### What It Does Not Own
- third-party website automation
- automated refund execution
- merchant-side checkout actions

### Inputs
- database state
- scheduler triggers
- API-issued jobs

### Outputs
- listing and inventory status updates
- notifications
- aggregate metrics

### Failure Points
- missed expiry jobs
- undetected paid-but-undelivered orders
- background task backlog

## Integration Boundaries

### Bot -> API
- Bot may read active listings and submit buy or support actions.
- Bot may not decide inventory, payment, or compliance state.

### Admin -> API
- Admin may trigger operator actions.
- Admin may not mutate the database directly.

### API -> Database
- API is the only normal write path for business entities.
- Direct scripts should be reserved for migrations and controlled maintenance.

### Payment Provider -> API
- Payment provider confirms payment state.
- Coupon delivery must never rely on client-side success alone.

### Background Jobs -> API or Database
- Jobs may perform approved internal lifecycle actions.
- Jobs may not invent new business rules outside the API/domain layer.

### Analytics -> Core Flows
- Analytics should observe the system, not become a hard dependency for order completion or coupon delivery.

## Explicit MVP Exclusions

### Merchant Self-Serve Module
Why excluded:
- too much operational and policy complexity for MVP

Deferred until:
- partner demand and internal workflows are stable enough to automate safely

### Scraping / Automation Module
Why excluded:
- directly conflicts with the product's legal and platform-risk constraints

Deferred until:
- not planned in current strategy; compliant sourcing should be preferred instead

### Automated Refund Module
Why excluded:
- the stated MVP policy is that no refunds are available

Deferred until:
- only if the business later changes policy or is required to support structured refund operations
