# Modules

## Module Overview
The MVP should be organized into a small set of modules with clear ownership boundaries. The goal is not to maximize separation for its own sake, but to make operations, debugging, and future iteration easier without adding distributed-system overhead.

The recommended MVP modules are:
- bot
- api
- admin
- offer governance
- analytics and audit
- background jobs

Modules intentionally excluded from MVP:
- payments
- fulfillment automation
- merchant self-serve
- scraping or browser automation

## 1. Bot Module

### Responsibilities
- receive Telegram updates through webhook handlers
- manage onboarding flow
- capture user preferences such as category interest
- return offer lists and offer detail views
- collect user feedback such as "worked" or "didn't work"
- redirect users to merchant or partner destinations

### What It Owns
- Telegram-specific message formatting
- Telegram command handling
- Telegram session or conversational state that is safe to keep lightweight

### What It Does Not Own
- canonical offer data
- publishing approval decisions
- supplier verification logic
- direct database writes outside approved API pathways

### Inputs
- Telegram webhook events
- active offer data from API
- preference and feedback endpoints from API

### Outputs
- Telegram messages
- feedback events
- click events

### Failure Points
- Telegram API delivery failures
- invalid bot state transitions
- broken deep links or malformed message formatting

## 2. API Module

### Responsibilities
- act as the main business logic layer
- expose endpoints for bot and admin surfaces
- validate offer lifecycle changes
- enforce required metadata and status rules
- manage categories, source labels, reports, and support states
- provide read models for active offers and internal review queues

### What It Owns
- offer lifecycle rules
- validation logic
- source-of-truth APIs
- authorization rules for admin versus user-facing operations

### What It Does Not Own
- long-running campaign management outside MVP scope
- merchant-side automation
- direct Telegram rendering concerns

### Inputs
- bot requests
- admin requests
- job runner invocations

### Outputs
- validated offer records
- issue queues
- analytics events
- audit entries

### Failure Points
- invalid state transitions
- schema drift
- deployment bugs affecting all surfaces

## 3. Admin Module

### Responsibilities
- create, edit, preview, publish, pause, expire, and remove offers
- review invalid-offer reports
- inspect source provenance and disclosure metadata
- review supplier quality signals
- provide operators with a manageable workflow for small-team operations

### What It Owns
- operator user interface
- internal review flows
- publish/pause/remove actions routed through API

### What It Does Not Own
- core business rules
- direct database writes
- external supplier integrations

### Inputs
- API read models for offers, reports, and source metadata

### Outputs
- operator actions sent to API

### Failure Points
- unclear operator workflows causing bad publishing decisions
- insufficient validation at publish time
- accidental operator mistakes due to weak UX

## 4. Offer Governance Module

### Responsibilities
- represent the trust and compliance layer around offers
- store source type, provenance notes, rights checks, and disclosure state
- manage publication statuses such as draft, approved, published, paused, expired, and removed
- track invalid-offer reports and moderation outcomes

### What It Owns
- offer review checklist model
- publication state machine
- report-to-resolution workflow
- policy-oriented metadata used to minimize risk

### What It Does Not Own
- Telegram interaction logic
- merchant acquisition strategy
- payment or refund workflows

### Inputs
- operator review actions
- user issue reports
- scheduled expiry checks

### Outputs
- trusted active-offer set for bot delivery
- audit-ready offer history
- paused or removed content decisions

### Failure Points
- incomplete source documentation
- ambiguous disclosure state
- inconsistent status transitions leading to stale offers

## 5. Analytics and Audit Module

### Responsibilities
- capture offer views, clicks, joins, and feedback
- track invalid-offer reports and resolution times
- maintain audit logs of operator actions and status changes
- surface source quality and supplier concentration metrics

### What It Owns
- event model for MVP metrics
- audit log entries for sensitive actions
- operational reporting inputs

### What It Does Not Own
- user-facing recommendation logic beyond basic reporting support
- external BI complexity not needed for MVP

### Inputs
- bot events
- admin actions
- API state changes

### Outputs
- dashboards or reports for operators
- metrics used in roadmap decisions
- recovery context for incidents

### Failure Points
- missing events causing bad product decisions
- audit gaps making disputes harder to resolve
- analytics coupling that slows core user flows

## 6. Background Jobs Module

### Responsibilities
- expire offers automatically based on timestamps
- schedule future publish times when needed
- run stale-offer checks
- generate periodic summary metrics
- trigger non-blocking operational notifications

### What It Owns
- safe internal automation only
- retries for internal asynchronous tasks

### What It Does Not Own
- third-party website automation
- fulfillment automation
- merchant-side checkout actions

### Inputs
- database state
- scheduler triggers
- API-issued jobs

### Outputs
- offer status updates
- notifications
- aggregate metrics

### Failure Points
- missed expiry jobs
- duplicate scheduling
- background task backlog

## Integration Boundaries

### Bot -> API
- Bot may read active offers and submit user actions.
- Bot may not decide compliance state or bypass validation.

### Admin -> API
- Admin may trigger operator actions.
- Admin may not mutate the database directly.

### API -> Database
- API is the only write path for business entities in normal operation.
- Direct scripts should be reserved for migrations and controlled maintenance.

### Background Jobs -> API or Database
- Jobs may perform approved internal lifecycle actions.
- Jobs may not introduce new business rules outside the API/domain layer.

### Analytics -> Core Flows
- Analytics should observe the system, not become a hard dependency for publishing or Telegram response generation.

## Explicit MVP Exclusions

### Payments Module
Why excluded:
- direct payment handling introduces refund, chargeback, fraud, and consumer-protection complexity too early

Deferred until:
- there is clear legal approval, explicit supply rights, and proven user demand for direct commerce

### Fulfillment Module
Why excluded:
- the MVP is discovery and partner distribution, not owned coupon delivery or merchant-side fulfillment

Deferred until:
- the business moves beyond click-out and approved distribution models

### Merchant Self-Serve Module
Why excluded:
- too much operational and policy complexity for MVP

Deferred until:
- partner demand and internal workflows are stable enough to automate safely

### Scraping / Automation Module
Why excluded:
- directly conflicts with the product's legal and platform-risk constraints

Deferred until:
- not planned in current strategy; compliant feeds or formal integrations should be preferred instead
