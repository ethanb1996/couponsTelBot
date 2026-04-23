# System Overview

Status: Draft
Owner: Architect Agent

## Objective
Define an MVP-first architecture for a Telegram-based coupon resale product in Israel that:
- is simple for a small team to operate
- supports pre-bought inventory, payment, and delivery
- avoids risky automation dependencies
- keeps trust, auditability, and fast issue handling at the center

The architecture is intentionally optimized for a narrow direct-sale loop:

1. store pre-bought coupon inventory
2. list inventory in Telegram
3. accept payment in ILS
4. deliver the coupon to the user

## Architectural Principles
- Telegram is the primary user surface.
- Controlled inventory is a feature, not a temporary workaround.
- Every sellable coupon must have a clear source and audit trail.
- Internal automation is acceptable; external risky automation is not.
- Keep modules loosely coupled enough to evolve, but not so fragmented that MVP operations become hard to run.
- Prefer a small number of dependable services over a microservice-heavy design.

## MVP System Shape
The MVP should use a modular monolith backend with a few clearly separated surfaces:

1. Telegram delivery surface for users
2. Admin surface for operators
3. API/backend as the system of record and business rules layer
4. Data store for users, coupons, orders, payments, and audit history
5. Background jobs for safe internal tasks such as payment reconciliation and expiry handling

This provides enough separation for clarity without introducing distributed-system complexity too early.

## High-Level Flow

### Inventory Intake and Listing
1. Operator acquires coupon inventory from an approved source.
2. Operator verifies value, expiry, transferability, and resale metadata.
3. Operator stores the coupon securely in the admin interface.
4. API validates inventory and stores it in the database.
5. Operator creates a Telegram listing linked to available inventory.
6. Listing is marked `draft`, `active`, `paused`, `sold_out`, `expired`, or `removed`.

### User Purchase Flow
1. User starts the bot and views active coupon listings.
2. User opens a listing detail view.
3. User sees sale price, coupon value, expiry, delivery terms, and explicit no-refund policy.
4. User confirms purchase and receives a PayPal checkout link.
5. User completes payment in PayPal in ILS.
6. Backend verifies the PayPal webhook and captures the approved PayPal order.
7. Backend assigns one coupon from inventory to the order.
8. Bot delivers the coupon to the user in Telegram.

### Feedback and Issue Handling
1. User reports an invalid coupon, delivery issue, or payment issue.
2. API records the issue and adds it to an internal review queue.
3. Operator reviews the report in admin.
4. Operator checks coupon, order, payment, and delivery history.
5. Operator responds to the user and records the outcome.

## Proposed Deployment Topology

### User-Facing Surfaces
- Telegram Bot
- Optional Telegram Channel used for acquisition or announcement
- Admin Web UI for internal operators only

### Backend Layer
- Single backend application exposing:
  - bot webhook handlers
  - admin APIs
  - internal services for inventory, order, payment, delivery, and audit logging

### Data Layer
- Relational database as the source of truth
- Optional cache only if needed later; not required for MVP

### Internal Processing
- Job runner or scheduled worker for:
  - inventory expiry transitions
  - payment reconciliation
  - undelivered-order checks
  - summary analytics rollups

### External Dependencies
- Telegram Bot API
- PayPal as the initial payment provider that accepts ILS
- analytics/logging stack

Explicitly excluded from MVP architecture:
- browser automation against merchant systems
- scraping pipelines against protected sites
- third-party coupon acquisition automation
- automated refund engine

## Core Domains

### 1. Listing Catalog
Maintains the canonical record of every listing, including:
- title and merchant name
- coupon value
- sale price
- expiry data
- restrictions
- source provenance notes
- listing status

### 2. Inventory Governance
Handles the safety and trust layer:
- operator review state
- rights/disclosure checks
- listing/pause/remove decisions
- complaint handling
- audit trail of changes

### 3. Telegram Experience
Handles user-facing flows:
- listing view
- listing detail view
- checkout entry point
- post-payment coupon delivery
- complaint submission

### 4. Ops and Insights
Supports the internal feedback loop:
- inventory quality tracking
- payment and delivery reporting
- complaint metrics
- support review

## Integration Boundaries

### Telegram Bot <-> Backend API
Boundary:
- Telegram sends webhook updates to the backend.
- Backend decides all business logic and state transitions.

Reason:
- keeps Telegram-specific handling thin
- allows the same listing, order, and delivery state to drive bot and admin behavior

### Admin UI <-> Backend API
Boundary:
- Admin never writes directly to the database.
- All changes go through backend validation and audit logging.

Reason:
- preserves consistent workflows
- reduces accidental state corruption

### Backend API <-> Database
Boundary:
- Database is the source of truth for listings, coupons, orders, payments, reports, and audit entries.
- Business rules live in the application layer, not in admin-only scripts.

Reason:
- easier to evolve safely
- clearer operational debugging

### Backend <-> External Offer Sources
Boundary:
- MVP does not integrate directly into merchant systems for automated acquisition.
- External sources are represented as supplier records and manual inventory intake.

Reason:
- reduces ToS, legal, and fragility risk
- keeps supplier-side failures from breaking the core app

### Backend <-> Payment Provider
Boundary:
- backend creates PayPal checkout orders and approval links
- PayPal owns card and wallet handling, including card and Apple Pay flows exposed by the merchant account
- backend verifies PayPal webhooks and trusts only PayPal-confirmed payment success for coupon delivery

Reason:
- reduces PCI scope
- keeps delivery gated behind authoritative payment state

### Backend <-> Analytics / Logging
Boundary:
- product events and operational logs are emitted asynchronously where possible
- analytics failure should not block publishing or user flows

Reason:
- observability matters, but should not take down the product

## Failure Points

### Telegram Delivery Failure
Examples:
- webhook outage
- Telegram API errors
- malformed message payloads

Impact:
- users do not receive responses or delivered coupons

Mitigation:
- retryable outbound delivery
- structured error logging
- ability to re-send a delivered coupon from admin when appropriate

### Bad Inventory Data Entered by Operators
Examples:
- missing expiry
- wrong coupon value
- already-used inventory
- incorrect source labeling

Impact:
- direct trust damage and support load

Mitigation:
- required-field validation
- intake checklist
- inventory preview
- quick pause/remove flow

### Payment Success but Coupon Delivery Fails
Examples:
- bot send error
- assignment logic failure
- coupon marked sold but not delivered

Impact:
- direct purchase failure and complaint risk

Mitigation:
- make delivery idempotent
- store assignment and delivery state separately
- alert on paid but undelivered orders

### Single Backend Failure
Examples:
- application crash
- bad deployment
- configuration error

Impact:
- bot, admin, and operational tools can all be affected

Mitigation:
- simple deployment with health checks
- rollback path
- minimal moving parts in MVP

### Database Failure or Corruption
Examples:
- connectivity issues
- migration mistakes
- accidental destructive edits

Impact:
- core system of record becomes unavailable

Mitigation:
- backups
- controlled migrations
- admin writes only through API
- audit history for recovery context

### Human Operations Bottleneck
Examples:
- too much inventory to verify manually
- too many user complaints
- insufficient support coverage

Impact:
- quality drops before the product is technically broken

Mitigation:
- narrow category scope
- limit publishing volume
- instrument operator workload
- expand only after stable throughput is proven

### Supplier or Partner Failure
Examples:
- supplier inventory is invalid
- source quality degrades suddenly
- transferability assumptions prove wrong

Impact:
- users buy invalid or unusable coupons despite the app functioning correctly

Mitigation:
- source-quality scoring
- partner/source audit history
- fast pause/remove controls
- no hard dependency on one supplier for core functionality

### Chargebacks and No-Refund Pressure
Examples:
- user disputes a valid delivered purchase
- user claims coupon failed
- payment provider allows chargeback even when policy says no refunds

Impact:
- financial loss and account risk with payment provider

Mitigation:
- clear pre-purchase disclosure
- store delivery evidence
- store PayPal order and capture references
- store coupon assignment evidence
- keep complaint and dispute logs

## Operational Simplicity Decisions
- Use one backend application for MVP instead of splitting services.
- Keep Telegram and admin on the same business rules layer.
- Build payment and delivery into MVP, because they are the core loop.
- Do not build automated merchant integrations into MVP.
- Store coupon, order, payment, and delivery state centrally.
- Favor explicit status transitions over hidden side effects.

## Deferred Architecture
These are intentionally deferred until later phases:
- self-serve merchant portal
- automated refund orchestration
- marketplace workflows
- user-to-user inventory submission with public listing
- exclusive partner APIs
- complex partner acquisition automation

## Recommended MVP Technology Shape
The exact stack can vary, but the architectural shape should be:
- one backend service for bot logic, admin APIs, and core domain logic
- one admin web app
- one relational database
- one job runner or scheduler
- Telegram as the primary delivery client

This keeps the architecture aligned with the MVP product strategy: low-risk, auditable, manual-first, and fast to iterate.
