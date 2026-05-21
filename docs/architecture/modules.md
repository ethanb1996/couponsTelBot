# Modules

## Module Overview
The MVP should be organized around a direct merchant-partner sales flow, not a resale marketplace. The recommended modules are:
- bot
- api
- admin
- merchant partners and offers
- manual payment review
- delivery and support
- analytics and audit
- background jobs

Modules intentionally excluded from MVP:
- merchant self-serve
- broad marketplace features
- scraping or browser automation
- automated provider reconciliation as the primary path
- automated refund engine

## 1. Bot Module

### Responsibilities
- receive Telegram updates through webhook handlers
- show active offers and detail views
- present payment instructions and merchant disclosure
- collect buyer-submitted PayBox usernames
- notify the buyer of review, approval, rejection, and support status
- deliver predefined codes after approval

### What It Owns
- Telegram-specific message formatting
- Telegram command and callback handling
- lightweight conversational state for payment claim collection

### What It Does Not Own
- canonical offer data
- payment verification rules
- admin approval rules
- direct database writes outside approved backend pathways

## 2. API Module

### Responsibilities
- act as the main business logic layer
- expose endpoints and service methods for bot and admin surfaces
- validate offer, order, payment-claim, and delivery transitions
- enforce required metadata and audit rules
- manage support states

### What It Owns
- offer lifecycle rules
- order lifecycle rules
- payment-claim lifecycle rules
- code assignment and delivery gating rules
- admin authorization rules

### What It Does Not Own
- provider-side payment truth in v1
- Telegram rendering concerns
- merchant acquisition strategy

## 3. Admin Module

### Responsibilities
- create merchant partners
- create, edit, preview, publish, pause, expire, and remove offers
- inspect code availability and delivery history
- approve or reject payment claims
- review complaints and support cases

### What It Owns
- operator user interface
- internal review flows
- publish, approval, and support actions routed through API

### What It Does Not Own
- core business rules
- direct database writes

## 4. Merchant Partners And Offers Module

### Responsibilities
- store partner records
- store Telegram-facing offers
- track payment links, disclosure, support contact, and redemption terms
- manage offer availability based on code supply

### What It Owns
- merchant partner state
- offer state machine
- pre-publish validation

### What It Does Not Own
- Telegram conversation logic
- payment review execution

## 5. Manual Payment Review Module

### Responsibilities
- record buyer-submitted PayBox usernames
- create manual payment claims
- route claims to admins
- store approval or rejection outcomes

### What It Owns
- payment-claim state machine
- admin review actions
- review audit data

### What It Does Not Own
- provider webhook logic as a required dependency
- final delivery rendering

## 6. Delivery And Support Module

### Responsibilities
- assign a predefined code to an approved order
- deliver the code to the buyer in Telegram
- track delivery timestamp and evidence
- log user complaints and support cases

### What It Owns
- assignment-to-delivery workflow
- support-case records
- delivery confirmation state

### What It Does Not Own
- merchant acquisition logic
- payment verification rules before approval

## 7. Analytics And Audit Module

### Responsibilities
- capture offer views, order starts, claim submissions, approvals, rejections, deliveries, and complaints
- maintain audit logs of operator actions and status changes
- surface approval latency, rejection rate, and fulfillment health metrics

### What It Owns
- event model for MVP metrics
- audit entries for sensitive actions
- operational reporting inputs

### What It Does Not Own
- recommendation systems
- analytics dependencies that block order completion

## 8. Background Jobs Module

### Responsibilities
- expire offers and predefined codes automatically when needed
- remind operators about stale pending claims
- flag approved-but-undelivered orders
- generate lightweight summary metrics

### What It Owns
- safe internal automation only
- retries for non-critical asynchronous tasks

### What It Does Not Own
- provider payment truth
- merchant-side automation
- automated refund execution

## Integration Boundaries

### Bot -> API
- Bot may read active offers and submit order or claim actions.
- Bot may not decide approval, code assignment, or support resolution state.

### Admin -> API
- Admin may approve or reject payment claims and manage offers.
- Admin may not mutate the database directly.

### API -> Database
- API is the only normal write path for merchant-partner, offer, order, claim, delivery, and support entities.

### Background Jobs -> API Or Database
- Jobs may perform approved internal lifecycle actions.
- Jobs may not invent business rules outside the domain layer.

## Explicit MVP Exclusions

### Merchant Self-Serve Module
Why excluded:
- adds operational and product complexity too early

### Automated Provider Reconciliation Module
Why excluded:
- not required for the fastest manual PayBox launch

### Marketplace Module
Why excluded:
- the MVP is validating direct merchant offers, not multi-sided supply
