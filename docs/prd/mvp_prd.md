# MVP PRD

Status: Draft
Owner: PRD Agent

## Product Summary
The MVP is a Telegram-first coupon resale product for users in Israel. The business pre-buys coupon inventory, lists that inventory inside Telegram, accepts payment in ILS through an approved payment provider, and delivers the coupon to the user after successful payment.

This is intentionally a narrower MVP than a discovery marketplace. It is designed around one controlled loop:

1. Hold inventory before sale
2. Sell inventory in Telegram
3. Take payment in ILS
4. Deliver coupon immediately

## Problem Statement
Users who want discounted fast-food and similar coupons do not have a fast, simple way to buy a ready-to-use coupon inside Telegram. The current experience usually requires searching across multiple channels, validating whether the deal is still good, and figuring out how to redeem it.

This MVP solves a smaller problem than full coupon discovery. It gives users a direct purchase path to pre-bought inventory that the seller already controls.

## Product Goals
1. Validate that Israeli Telegram users will buy pre-bought coupons directly in Telegram.
2. Prove that a simple inventory-sale flow converts better than a broader discovery experience for the first wedge.
3. Measure whether immediate post-payment coupon delivery creates a compelling user experience.
4. Validate margin between coupon acquisition cost and resale price after payment fees.
5. Operate with a small number of controlled coupon types and a small number of suppliers.

## Non-Goals
- Building a broad coupon discovery network
- Affiliate-first monetization
- Sponsored placement marketplace
- User-to-user coupon marketplace
- Merchant self-serve tooling
- Multi-country expansion
- Any sourcing flow that depends on restricted automation, captcha bypassing, or anti-bot evasion

## Target Users

### Primary Users
- Telegram users in Israel who want a fast buy flow for discounted coupons
- Price-sensitive users who value speed and convenience more than deep browsing
- Users willing to buy a clearly defined coupon product with final-sale terms

### Secondary Users
- Internal operators managing inventory, listings, payments, and delivery
- Approved suppliers providing pre-bought coupon inventory

## User Pains
- Buying discounted coupons is fragmented and inconvenient
- Users cannot quickly tell which coupon is immediately usable
- Searching for a coupon often takes too much effort
- Users want a simple purchase and delivery experience inside Telegram

## Core Value Proposition
Users can buy a pre-bought coupon in Telegram with:
- a simple product listing
- clear price in ILS
- fast payment flow
- immediate coupon delivery after successful payment
- no need to search across multiple channels

## Assumptions
- The team can acquire coupon inventory before sale at a price that leaves margin after fees.
- There are coupon types that can be lawfully held and resold with acceptable rights clarity.
- Users will tolerate an all-sales-final model if the coupon, price, and delivery terms are explicit before payment.
- A payment provider that accepts ILS can be integrated without excessive friction.
- Inventory can be kept small enough that the team can verify each item before listing.

## User Journeys

### Journey 1: User Buys a Coupon
1. User opens the Telegram bot.
2. User sees a list of available coupons with merchant name, coupon value, sale price, and key terms.
3. User selects a coupon listing.
4. User sees:
   merchant name
   coupon value
   sale price in ILS
   expiry
   redemption instructions
   explicit final-sale disclosure stating no refunds are available
5. User chooses to buy.
6. User is redirected to or shown the payment flow with a provider that accepts ILS.
7. Payment succeeds.
8. Coupon is delivered to the user in Telegram.

Success condition:
The user completes payment and receives the coupon without manual intervention.

### Journey 2: Operator Lists Inventory
1. Operator acquires coupon inventory.
2. Operator verifies validity, expiry, transferability, and sale price.
3. Operator stores the coupon securely in admin.
4. Operator creates a Telegram listing linked to available inventory.
5. Listing becomes available for sale.

Success condition:
Only inventory that is actually held and verified is offered for sale.

### Journey 3: User Has a Post-Purchase Problem
1. User reports that a coupon is invalid or unclear.
2. Operator reviews the case manually.
3. Operator checks coupon record, source history, and delivery record.
4. Operator responds to the user.

Success condition:
The team can investigate and document the issue even though the stated policy is that refunds are not available.

## MVP Features

### 1. Telegram Product Listing Flow
Must have

Description:
- list available coupons in Telegram
- show price, value, expiry, and key terms
- allow user to open detail view and buy

### 2. Controlled Coupon Inventory
Must have

Description:
- pre-bought coupon inventory stored before sale
- secure coupon storage
- inventory count or availability tracking

### 3. ILS Payment Flow
Must have

Description:
- payment provider integration that accepts ILS
- successful payment confirmation before coupon delivery
- payment status tracking

### 4. Coupon Delivery Flow
Must have

Description:
- deliver coupon to user only after payment succeeds
- store delivery timestamp and delivery evidence

### 5. Admin Inventory Workflow
Must have

Description:
- create inventory entries
- create listings
- pause listings
- mark inventory as sold, voided, expired, or disputed

### 6. Order and Payment Tracking
Must have

Description:
- record order creation, payment result, coupon assignment, and delivery state

### 7. Final-Sale Disclosure
Must have

Description:
- explicit statement before payment that all sales are final
- explicit statement that no refunds are available

### 8. Support Logging
Must have

Description:
- allow operators to log complaints, failures, and delivery issues even if no refund path exists

## Features Explicitly Excluded from MVP
- Broad coupon discovery feed
- Category preference engine
- Affiliate click-out monetization
- Sponsored listing marketplace
- User-submitted coupon marketplace
- Automated sourcing from protected merchant systems
- Wallets or stored balance
- Partial refunds, refunds on demand, or automated refund tooling

## Legal / Platform Risk Review
This MVP intentionally accepts a riskier commerce posture than the previous discovery-first version. The main risk concentrations are:
- direct resale rights and transferability
- payment disputes and chargebacks
- user trust damage when a delivered coupon fails
- legal and consumer-risk exposure from a no-refund policy

Required controls:
- only sell inventory already in hand
- verify transferability before listing
- disclose the business role clearly
- disclose final-sale / no-refund terms clearly before payment
- keep audit records of inventory acquisition, payment, delivery, and support complaints

## Safer Alternatives Considered

### Safer Alternative: Discovery and Click-Out Model
Why safer:
- no inventory holding
- no direct payment risk
- lower refund and chargeback exposure

Why not chosen for this MVP:
- the goal is now to test direct Telegram sales with controlled inventory

### Safer Alternative: Partner Distribution Without Resale
Why safer:
- fewer transferability and resale-rights issues

Why not chosen for this MVP:
- it introduces more dependency on partners and less control over product experience

## Monetization Approach
The MVP monetization model is direct resale margin:

1. Buy coupon inventory at cost
2. List coupon inventory at resale price
3. Collect payment in ILS
4. Deliver coupon
5. Keep the spread after fees and losses

Core economic variables:
- inventory acquisition cost
- payment provider fee
- invalid inventory loss
- support overhead
- resale price

## Refund Policy
The MVP policy is explicit:
- all sales are final
- no refunds are available

This policy must be shown before payment, not after delivery.

Even with this policy, the system still needs internal support handling for:
- invalid coupon complaints
- wrong delivery
- duplicate charge investigation
- fraud review

## Success Metrics

### Conversion Metrics
- listing view to purchase rate
- payment completion rate
- delivered order rate

### Unit Economics Metrics
- gross margin per coupon sold
- payment fee rate
- invalid inventory loss rate

### Reliability Metrics
- successful delivery rate
- post-purchase complaint rate
- coupon failure rate

### Trust Metrics
- repeat buyer rate
- complaint rate per 100 orders
- chargeback or payment dispute rate

## MVP Success Criteria
The MVP should be considered successful if it demonstrates:

1. Users complete direct coupon purchases inside the Telegram flow.
2. Coupons can be delivered immediately after successful payment with high reliability.
3. Unit economics are positive after payment fees and inventory losses.
4. Complaint and dispute rates remain manageable for a small team.
5. The business can run on a small set of controlled inventory without operational chaos.

## Launch Plan

### Phase 0: Setup
- identify transferable coupon inventory
- choose ILS-capable payment provider
- define final-sale terms and no-refund disclosure
- define delivery and support logging workflow

### Phase 1: Controlled Launch
- launch a small number of coupon SKUs
- limit inventory
- monitor payment completion, delivery, and complaints

### Phase 2: Optimization
- refine listing formats
- improve delivery reliability
- expand only the coupon types with acceptable margins and failure rates

## Open Questions
- which exact coupon types are safest to pre-buy and resell
- which ILS payment provider is best for this product
- what pre-purchase wording is required for the no-refund policy
- what manual exception policy should exist for obvious delivery failures even if refunds are not generally available
