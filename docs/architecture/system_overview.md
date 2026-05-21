# System Overview

Status: Draft
Owner: Architect Agent

## Objective
Define an MVP-first architecture for a Telegram-based coupon sales product in Israel that:
- sells fixed offers from small businesses that work with us directly
- uses PayBox as the initial payment channel
- verifies payment manually through buyer-submitted PayBox usernames
- delivers predefined coupon codes after operator approval
- keeps auditability, trust, and operational simplicity at the center

The architecture is intentionally optimized for a narrow manual-first loop:

1. create merchant partner records
2. publish predefined offers in Telegram
3. accept payment through merchant-approved PayBox links
4. collect a payment claim from the buyer
5. verify payment manually
6. deliver a predefined code in Telegram

## Architectural Principles
- Telegram is the primary user surface.
- Merchant authorization is a feature, not a nice-to-have.
- Every sellable offer must have a clear merchant owner and audit trail.
- Manual operations are acceptable in MVP if they reduce launch complexity.
- Prefer a small number of dependable modules over automation-heavy design.
- Separate user-visible offer state from payment review state and fulfillment state.

## MVP System Shape
The MVP should use a modular monolith backend with a few clearly separated surfaces:

1. Telegram bot for buyers
2. Admin workflow surface for operators
3. Backend as the system of record and business rules layer
4. Relational database for users, merchant partners, offers, orders, payment claims, codes, and support history
5. Lightweight background jobs only for non-critical cleanup and reminders

## High-Level Flow

### Offer Setup
1. Operator creates a merchant partner record.
2. Operator creates a fixed offer tied to that merchant.
3. Operator adds merchant disclosure text, redemption terms, support contact, and PayBox payment link.
4. Operator loads predefined coupon codes for that offer.
5. Offer is marked `draft`, `active`, `paused`, `sold_out`, `expired`, or `removed`.

### User Purchase Flow
1. User opens the bot and views active offers.
2. User opens an offer detail view.
3. User sees price, merchant disclosure, redemption terms, support contact, and payment instructions.
4. User taps through to the PayBox link and completes payment outside Telegram.
5. User returns to the bot and submits their PayBox username.
6. Backend records a manual payment claim.
7. Bot alerts the admin for manual review.
8. Admin verifies the payment and approves or rejects the claim.
9. After approval, backend assigns one predefined code and the bot delivers it to the user.

### Issue Handling
1. User reports an invalid code, payment mismatch, or redemption issue.
2. API records a support case linked to the order, payment claim, and code when available.
3. Operator reviews the full history in admin.
4. Operator responds manually and records the outcome.

## Proposed Deployment Topology

### User-Facing Surfaces
- Telegram Bot
- Optional Telegram channel for acquisition or announcement
- Admin web UI for setup, audit, and support history

### Backend Layer
- Single backend application exposing:
  - Telegram webhook handlers
  - admin APIs
  - payment-claim review actions
  - internal services for offers, codes, orders, support, and audit logging

### Data Layer
- Relational database as the source of truth
- No cache required for MVP

### Internal Processing
- Job runner or scheduler for:
  - expiring offers and codes
  - reminding operators about pending payment claims
  - flagging orders stuck without review or fulfillment

### External Dependencies
- Telegram Bot API
- PayBox links managed by the business or merchant
- logging and analytics stack

Explicitly excluded from MVP architecture:
- automated provider webhooks as the primary payment path
- merchant self-serve portals
- third-party marketplace workflows
- browser automation against merchant systems
- automated refund engine

## Core Domains

### 1. Merchant Partners
Maintains the canonical record of each small business, including:
- business name
- contact reference
- status
- approval notes
- disclosure requirements

### 2. Offers And Codes
Maintains the sale catalog:
- fixed offer title and description
- price in ILS
- payment link
- redemption terms
- predefined code pool
- publication and availability status

### 3. Orders And Payment Claims
Tracks the purchase loop:
- user starts purchase
- order enters awaiting-payment state
- buyer submits PayBox username
- claim enters manual review
- order is approved or rejected

### 4. Delivery And Support
Handles fulfillment and issue review:
- assign one predefined code per approved order
- send code in Telegram
- store delivery evidence
- log support cases

## Integration Boundaries

### Telegram Bot <-> Backend API
Boundary:
- Telegram sends user updates to the backend.
- Backend decides all business rules and status transitions.

Reason:
- keeps bot logic thin
- makes admin and bot rely on the same source of truth

### Admin UI <-> Backend API
Boundary:
- Admin never writes directly to the database.
- All approval, rejection, publish, and support actions go through backend validation and audit logging.

Reason:
- preserves consistent workflows
- reduces accidental state corruption

### Backend API <-> Database
Boundary:
- Database is the source of truth for merchant partners, offers, codes, orders, payment claims, deliveries, and support cases.
- Business rules live in the application layer.

Reason:
- easier to evolve safely
- clearer operational debugging

### Backend <-> Payment Channel
Boundary:
- Backend does not authoritatively verify payment from a provider webhook in v1.
- Backend records buyer claims and admin review outcomes.

Reason:
- fastest path to launch
- keeps the initial payment loop simple and manual

## Failure Points

### Unverified Payment Claim
Examples:
- buyer enters wrong PayBox username
- buyer claims to have paid but cannot be matched

Impact:
- manual support load and delayed fulfillment

Mitigation:
- clear claim instructions
- pending-review queue
- reject and retry workflow

### Admin Approval Delay
Examples:
- operator is offline
- pending claims queue grows too fast

Impact:
- users wait too long after payment

Mitigation:
- launch with low volume
- admin Telegram alerts
- reminders for stale claims

### Offer Or Code Misconfiguration
Examples:
- wrong PayBox link
- no available predefined codes
- unclear redemption terms

Impact:
- broken purchase flow or poor user trust

Mitigation:
- required-field validation
- pre-publish checklist
- sold-out or paused state controls

### Single Backend Failure
Examples:
- bad deployment
- config error
- application crash

Impact:
- bot and admin flow can both fail

Mitigation:
- simple deployment
- health checks
- rollback path

## Operational Simplicity Decisions
- Use one backend application for MVP.
- Keep Telegram and admin on the same business rules layer.
- Build around manual payment verification instead of provider automation.
- Keep offer setup and support logging in the initial version.
- Store order, claim, fulfillment, and support state centrally.
- Favor explicit state transitions over hidden side effects.

## Deferred Architecture
These are intentionally deferred until later phases:
- merchant self-serve portal
- automated provider reconciliation
- marketplace workflows
- user-submitted offers
- complex revenue-share settlement tooling

## Recommended MVP Technology Shape
- one backend service for bot logic, admin APIs, and core domain logic
- one admin web app
- one relational database
- one lightweight job runner or scheduler
- Telegram as the primary delivery client

This keeps the architecture aligned with the MVP strategy: direct merchant offers, manual-first verification, auditable fulfillment, and fast iteration.
