# Core Flows

## Flow Scope
This document defines the active MVP flows for:
- offer lifecycle
- predefined code lifecycle
- order lifecycle
- manual payment claim workflow
- Telegram bot interaction
- admin workflow
- support handling

This MVP is direct commerce for merchant-partner offers. It does not depend on automated provider confirmation.

## Offer Lifecycle

State flow:
1. `draft`
2. `active`
3. `paused`
4. `sold_out`
5. `expired`
6. `removed`

Detailed steps:
1. Operator creates a merchant partner.
2. Operator creates an offer tied to that partner.
3. Operator sets price, payment link, merchant disclosure, support contact, and redemption terms.
4. Operator loads predefined codes for the offer.
5. Offer becomes `active` only when required fields and at least one available code exist.
6. Offer moves to `sold_out` when no available codes remain.
7. Offer may be `paused`, `expired`, or `removed` by operator action.

Guardrails:
- an offer must not be active without a merchant owner
- an offer must not be active without a payment link
- an offer must not remain active when there are no deliverable codes

## Predefined Code Lifecycle

State flow:
1. `available`
2. `assigned`
3. `sent`
4. `voided` or `expired`

Detailed steps:
1. Operator imports or creates predefined codes for an offer.
2. Code remains `available` until an order is approved.
3. On approved payment claim, backend assigns one code to the order.
4. After Telegram delivery succeeds, code becomes `sent`.
5. Code may later become `voided` or `expired`.

Guardrails:
- a code must not be assigned before payment verification
- a code must not be assigned to more than one order
- delivery evidence must be stored when a code is sent

## Order Lifecycle

State flow:
1. `draft`
2. `awaiting_payment`
3. `payment_claim_submitted`
4. `payment_verified`
5. `coupon_sent`
6. `payment_rejected`, `cancelled`, or `support_required`

Detailed steps:
1. User selects an offer in Telegram.
2. System creates an order in `draft`.
3. User is shown the payment instructions and PayBox link.
4. Order becomes `awaiting_payment`.
5. User returns and submits a PayBox username.
6. System records the payment claim and order becomes `payment_claim_submitted`.
7. Admin verifies the payment manually.
8. If approved, order becomes `payment_verified`, a code is assigned, and delivery begins.
9. After delivery succeeds, order becomes `coupon_sent`.
10. If the claim is rejected, order becomes `payment_rejected`.
11. If approval succeeds but no code can be assigned, order becomes `support_required`.

Guardrails:
- no order should move to `payment_verified` without an approved payment claim
- no order should move to `coupon_sent` without both a code assignment and delivery record
- repeated approve or reject actions must be idempotent

## Manual Payment Claim Lifecycle

State flow:
1. `pending_review`
2. `verified`
3. `rejected`

Detailed steps:
1. Buyer taps `I paid`.
2. Bot asks for the buyer's PayBox username.
3. Buyer sends the username in chat.
4. System records a `manual_payment_claim`.
5. Bot sends an admin alert with order number, offer, amount, buyer details, and payer username.
6. Admin checks PayBox and approves or rejects the claim.
7. System records the review outcome and reviewer details.

Guardrails:
- only one active pending review claim should exist for an order at a time
- rejected claims should keep the audit history rather than overwrite it

## Telegram Bot Interaction Flow

### Purchase Flow
1. User opens the Telegram bot.
2. Bot sends 1 to 3 active offers.
3. Each offer has its own `View details` and `Buy` path.
4. User opens the offer details and sees:
   - merchant name
   - title and description
   - price in ILS
   - merchant disclosure
   - redemption terms
   - support contact
5. User starts payment.
6. Bot sends the PayBox link and explains the follow-up step:
   - pay through PayBox
   - return to Telegram
   - tap `I paid`
   - submit PayBox username
7. User submits the username.
8. Bot confirms that the claim is under review.
9. After approval, bot delivers the predefined code.
10. If rejected, bot explains that the payment could not be matched and gives retry or support guidance.

### Support Flow
1. User reports a delivery, payment, or redemption issue.
2. Bot creates or links a support case.
3. Admin reviews the order, payment claim, code, and delivery history.
4. Admin responds manually.

## Admin Workflow

### Merchant Setup Workflow
1. Operator creates or updates the merchant partner.
2. Operator records disclosure and support information.
3. Operator confirms the payment link strategy for the merchant or offer.

### Offer Setup Workflow
1. Operator creates the offer.
2. Operator sets:
   - title
   - description
   - price
   - payment link
   - merchant disclosure
   - redemption terms
   - support contact
3. Operator loads predefined codes.
4. Operator previews the offer.
5. Operator publishes the offer.

### Payment Review Workflow
1. User submits a payment claim.
2. Bot notifies the admin.
3. Admin checks PayBox manually.
4. Admin approves or rejects the claim.
5. On approval, system assigns and delivers one code.
6. On rejection, system records the rejection reason and informs the buyer.

### Support Workflow
1. User issue creates a support case.
2. Operator reviews the linked order, claim, code, and delivery evidence.
3. Operator records the investigation outcome.
4. Operator may pause the offer or void codes if there is a merchant-side issue.

## Exception Handling

### Payment Claim Verified But No Available Code Exists
Handling:
- order moves to `support_required`
- support case is created
- operator investigates immediately

### Delivery Fails After Approval
Handling:
- delivery remains `failed`
- code assignment remains auditable
- operator retries or re-sends manually
- order does not move to `coupon_sent` until delivery succeeds

### User Claims The Code Is Invalid
Handling:
- support case is created
- code is reviewed and may be voided
- offer or merchant may be paused if the issue is systemic

## Recommended MVP Flow Set
The flows that should be implemented first are:
1. merchant setup
2. offer publish and pause
3. Telegram purchase flow with PayBox link
4. manual payment claim submission
5. admin approve or reject
6. predefined code assignment and delivery
7. support-case logging and review
