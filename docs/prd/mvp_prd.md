# MVP PRD

Status: Draft
Owner: PRD Agent

## Product Summary
The MVP is a Telegram-first coupon sales product for users in Israel. The business works directly with small businesses to publish fixed offers, sends buyers to PayBox payment links, verifies payment manually, and delivers predefined coupon codes in Telegram after approval.

This is intentionally a narrow MVP. It is designed around one controlled loop:

1. Create a merchant-approved offer
2. Sell the offer in Telegram
3. Collect payment through PayBox
4. Verify the buyer manually
5. Deliver a predefined code

## Problem Statement
Small local businesses need a lightweight way to sell simple deals in Telegram, and buyers need a faster path than browsing scattered deal sources and unclear redemption terms.

This MVP solves a narrower problem than a marketplace or discovery network. It gives users a direct purchase path to a merchant-approved offer with a simple follow-up verification step.

## Product Goals
1. Validate that Israeli Telegram users will buy fixed merchant-partner offers directly in Telegram.
2. Prove that a semi-manual payment verification flow is good enough for fast go-to-market validation.
3. Measure whether predefined code delivery after approval creates a workable customer experience.
4. Validate that a small team can operate approval and fulfillment manually without chaos.
5. Learn which merchant offer shapes convert and which support issues appear first.

## Non-Goals
- Building a broad coupon discovery network
- Building a multi-merchant marketplace
- Affiliate-first monetization
- Automated provider reconciliation as a required dependency
- Merchant self-serve tooling
- User-to-user coupon marketplace
- Restricted automation against merchant systems

## Target Users

### Primary Users
- Telegram users in Israel who want a fast, simple deal purchase flow
- Users comfortable paying first and waiting for quick manual approval
- Buyers who value clarity over endless browsing

### Secondary Users
- Internal operators managing offers, claims, and support
- Small business partners supplying the offers and code pools

## Core Value Proposition
Users can buy a local offer in Telegram with:
- a clear fixed price in ILS
- a simple PayBox payment flow
- fast manual verification
- direct code delivery in Telegram
- a clear support path

## Assumptions
- Small businesses are willing to authorize offers and provide predefined codes.
- Users will tolerate a manual review step if the bot explains it clearly.
- PayBox is acceptable for the first validation loop.
- The team can manage approvals manually at low launch volume.
- A small number of offers is enough to test demand.

## User Journeys

### Journey 1: User Buys An Offer
1. User opens the Telegram bot.
2. User sees active offers with merchant name, title, price, and key terms.
3. User opens an offer detail.
4. User sees merchant disclosure, redemption terms, and support contact.
5. User taps through to the PayBox link and pays.
6. User returns to Telegram and submits the PayBox username used for payment.
7. Admin verifies payment.
8. The bot delivers a predefined code.

Success condition:
The user pays, gets approved, and receives a usable code with minimal back-and-forth.

### Journey 2: Operator Publishes An Offer
1. Operator creates or updates the merchant partner record.
2. Operator configures the offer and payment link.
3. Operator loads predefined codes.
4. Operator previews and publishes the offer.

Success condition:
Only merchant-approved offers with valid code supply go live.

### Journey 3: User Has A Payment Or Redemption Problem
1. User reports that payment was not approved or the code did not work.
2. Operator reviews the order, claim, and code history.
3. Operator responds manually and records the outcome.

Success condition:
The team can investigate and resolve issues without losing audit history.

## MVP Features

### 1. Telegram Offer Listing Flow
Must have

Description:
- list active offers in Telegram
- show price, merchant name, and key terms
- let the user open detail view and start payment

### 2. Merchant Partner Management
Must have

Description:
- create merchant partner records
- store disclosure and support information
- tie offers to merchants

### 3. Manual PayBox Payment Flow
Must have

Description:
- send a PayBox link
- collect buyer PayBox username after payment
- support manual approval or rejection

### 4. Predefined Code Delivery
Must have

Description:
- store code inventory securely
- assign one code only after approval
- record delivery outcome

### 5. Admin Review Workflow
Must have

Description:
- review payment claims
- approve or reject claims
- pause offers and inspect code availability

### 6. Order And Claim Tracking
Must have

Description:
- record order creation, claim submission, approval result, code assignment, and delivery state

### 7. Support Logging
Must have

Description:
- log payment issues, invalid codes, and delivery issues
- link support cases to the order and code history

## Features Explicitly Excluded From MVP
- Broad coupon discovery feed
- Recommendation engine
- Merchant self-serve portal
- Automated settlement tooling
- Affiliate click-out model
- User-submitted marketplace

## Legal And Trust Review
The direct merchant model is safer than the older resale framing, but it still has important risk areas:
- merchant authorization and disclosure
- payment-claim fraud
- manual review delay
- invalid or expired codes
- support quality

Required controls:
- tie every offer to a merchant partner
- disclose the merchant relationship clearly
- do not auto-deliver before manual approval
- keep order, claim, delivery, and support audit trails

## Monetization Approach
The MVP monetization can be flexible:
- revenue share with merchant
- markup on the offer
- fixed operator fee

The architecture should support any of these without changing the user-facing flow.

## Success Metrics

### Conversion Metrics
- offer view to payment-start rate
- payment-start to claim-submission rate
- claim approval rate
- delivered order rate

### Reliability Metrics
- average approval latency
- successful delivery rate
- complaint rate per 100 orders

### Trust Metrics
- repeat buyer rate
- payment mismatch rate
- invalid code rate

## MVP Success Criteria
The MVP should be considered successful if it demonstrates:

1. Users complete purchases through the Telegram and PayBox loop.
2. Admins can verify and fulfill orders manually with manageable effort.
3. Predefined code delivery works reliably after approval.
4. Complaint volume stays manageable for a small team.
5. Small businesses are willing to keep providing offers.

## Launch Plan

### Phase 0: Setup
- onboard initial merchants
- define disclosure wording
- prepare PayBox links
- define approval and support workflow

### Phase 1: Controlled Launch
- launch a small number of offers
- keep claim volume low
- monitor approval time, delivery, and support

### Phase 2: Optimization
- improve offer copy
- improve admin review speed
- expand only the offer types that convert cleanly

## Open Questions
- what is the best claim review SLA for first launch
- should payment links live at the merchant level or offer level
- what default rejection guidance should the bot send
- when should approval move from Telegram-first to admin-UI-first
