# System Overview

Status: Draft
Owner: Architect Agent

## Objective
Define an MVP-first architecture for a Telegram-based coupon discovery product in Israel that:
- is simple for a small team to operate
- supports manual verification and compliant offer publishing
- avoids risky automation dependencies
- keeps trust, auditability, and fast issue handling at the center

The architecture is intentionally optimized for coupon discovery and partner distribution, not direct coupon resale. It should support learning quickly while keeping the highest-risk commerce behaviors out of scope.

## Architectural Principles
- Telegram is the primary user surface.
- Manual review is a feature, not a temporary hack.
- Every published offer must have a clear source and audit trail.
- Internal automation is acceptable; external risky automation is not.
- Keep modules loosely coupled enough to evolve, but not so fragmented that MVP operations become hard to run.
- Prefer a small number of dependable services over a microservice-heavy design.

## MVP System Shape
The MVP should use a modular monolith backend with a few clearly separated surfaces:

1. Telegram delivery surface for users
2. Admin surface for operators
3. API/backend as the system of record and business rules layer
4. Data store for offers, source metadata, feedback, and audit history
5. Background jobs for safe internal tasks such as expiry handling and notifications

This provides enough separation for clarity without introducing distributed-system complexity too early.

## High-Level Flow

### Offer Intake and Publishing
1. Operator identifies a candidate offer from a compliant source.
2. Operator reviews source, expiry, restrictions, and publishing rights.
3. Operator creates or updates the offer in the admin interface.
4. API validates required metadata and stores the offer in the database.
5. Offer is marked `draft`, `approved`, `published`, `paused`, `expired`, or `removed`.
6. When published, the bot or channel formatter sends the offer to the Telegram delivery surface.

### User Consumption
1. User starts the bot or joins the channel.
2. User receives onboarding and optionally selects preferences.
3. Bot/API returns current active offers filtered by category or audience rules.
4. User taps through to merchant or partner destination, or sends feedback.
5. API records engagement, click events, and issue reports.

### Feedback and Issue Handling
1. User marks an offer as invalid or unclear.
2. API records the issue and adds it to an internal review queue.
3. Operator reviews the report in admin.
4. Operator updates, pauses, or removes the offer.
5. Bot content and offer state stay aligned through the shared backend status model.

## Proposed Deployment Topology

### User-Facing Surfaces
- Telegram Bot
- Optional Telegram Channel used for broadcast-style offer distribution
- Admin Web UI for internal operators only

### Backend Layer
- Single backend application exposing:
  - bot webhook handlers
  - admin APIs
  - internal services for offer lifecycle, moderation, analytics, and audit logging

### Data Layer
- Relational database as the source of truth
- Optional cache only if needed later; not required for MVP

### Internal Processing
- Job runner or scheduled worker for:
  - offer expiry transitions
  - scheduled publishing
  - stale-offer checks
  - summary analytics rollups

### External Dependencies
- Telegram Bot API
- approved partner or affiliate links
- analytics/logging stack

Explicitly excluded from MVP architecture:
- browser automation against merchant systems
- scraping pipelines against protected sites
- direct payment orchestration
- coupon fulfillment automation against third-party checkouts

## Core Domains

### 1. Offer Catalog
Maintains the canonical record of every offer, including:
- title and merchant name
- category
- offer summary
- restrictions
- expiry data
- source type
- source provenance notes
- publication status

### 2. Offer Governance
Handles the safety and trust layer:
- operator review state
- rights/disclosure checks
- publish/pause/remove decisions
- invalid-offer reports
- audit trail of changes

### 3. Telegram Experience
Handles user-facing flows:
- onboarding
- category preferences
- offer listing
- offer detail formatting
- feedback collection
- click-out links

### 4. Ops and Insights
Supports the internal feedback loop:
- defect tracking
- source quality metrics
- basic engagement reporting
- support review

## Integration Boundaries

### Telegram Bot <-> Backend API
Boundary:
- Telegram sends webhook updates to the backend.
- Backend decides all business logic and state transitions.

Reason:
- keeps Telegram-specific handling thin
- allows the same offer state to drive bot and admin behavior

### Admin UI <-> Backend API
Boundary:
- Admin never writes directly to the database.
- All changes go through backend validation and audit logging.

Reason:
- preserves consistent workflows
- reduces accidental state corruption

### Backend API <-> Database
Boundary:
- Database is the source of truth for offers, states, reports, and audit entries.
- Business rules live in the application layer, not in admin-only scripts.

Reason:
- easier to evolve safely
- clearer operational debugging

### Backend <-> External Offer Sources
Boundary:
- MVP does not integrate directly into merchant systems for automated retrieval or fulfillment.
- External sources are represented as metadata and outbound links, not deeply coupled workflows.

Reason:
- reduces ToS, legal, and fragility risk
- keeps supply-side failures from breaking the core app

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
- users do not receive responses or new published offers

Mitigation:
- retryable outbound delivery
- structured error logging
- ability to republish or re-send from admin

### Bad Offer Data Entered by Operators
Examples:
- missing expiry
- unclear restrictions
- wrong destination link
- incorrect source labeling

Impact:
- direct trust damage and support load

Mitigation:
- required-field validation
- publish checklist
- preview before publish
- quick pause/remove flow

### Stale Offer Remaining Published
Examples:
- expiry job fails
- operator misses manual removal
- partner changes the destination unexpectedly

Impact:
- invalid offers stay visible too long

Mitigation:
- expiry timestamps enforced in backend queries
- scheduled audits for near-expiry and expired content
- user report queue

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
- too many offers to review manually
- too many user issue reports
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
- affiliate links stop working
- partner changes offer terms
- source quality degrades suddenly

Impact:
- clicks or redemptions fail despite the app functioning correctly

Mitigation:
- source-quality scoring
- partner/source audit history
- fast pause/remove controls
- no hard dependency on one supplier for core functionality

## Operational Simplicity Decisions
- Use one backend application for MVP instead of splitting services.
- Keep Telegram and admin on the same business rules layer.
- Do not build payments or fulfillment orchestration into MVP.
- Do not build automated merchant integrations into MVP.
- Store offer state centrally and derive all user-facing views from it.
- Favor explicit status transitions over hidden side effects.

## Deferred Architecture
These are intentionally deferred until later phases:
- self-serve merchant portal
- direct payments and refund orchestration
- marketplace workflows
- user-to-user inventory submission with public listing
- exclusive partner APIs
- any direct resale engine

## Recommended MVP Technology Shape
The exact stack can vary, but the architectural shape should be:
- one backend service for bot logic, admin APIs, and core domain logic
- one admin web app
- one relational database
- one job runner or scheduler
- Telegram as the primary delivery client

This keeps the architecture aligned with the MVP product strategy: low-risk, auditable, manual-first, and fast to iterate.
