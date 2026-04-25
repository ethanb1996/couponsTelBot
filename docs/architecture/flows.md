# Core Flows

## Flow Scope
This document defines the active MVP flows for:
- coupon lifecycle
- order lifecycle
- Telegram bot interaction
- admin workflow
- refund handling

This MVP is direct commerce. It does not defer orders or payments.

## Coupon Lifecycle

State flow:
1. `available`
2. `reserved`
3. `assigned`
4. `delivered`
5. `used`, `expired`, `voided`, or `disputed`

Detailed steps:
1. Operator acquires coupon inventory from an approved source.
2. Operator verifies value, expiry, transferability, and sale price.
3. Coupon is stored securely as `available`.
4. When a user starts checkout, the system reserves one specific coupon and marks it `reserved`.
5. If checkout is abandoned or expires, the coupon returns to `available`.
6. After successful payment, the same reserved coupon is finalized to the order and becomes `assigned`.
7. After Telegram delivery succeeds, coupon becomes `delivered`.
8. Later, coupon may move to:
   - `used` if confirmed redeemed
   - `expired` if no longer valid
   - `voided` if invalidated internally
   - `disputed` if user complains

Guardrails:
- a coupon must not be sold unless the team already holds it
- a coupon must not be delivered before payment success
- a coupon must not be assigned to more than one order

## Order Lifecycle

State flow:
1. `draft`
2. `pending_payment`
3. `paid`
4. `delivery_pending`
5. `delivered`
6. `failed`, `cancelled`, or `disputed`

Detailed steps:
1. User selects a listing in Telegram.
2. System creates an order in `draft` and reserves one coupon.
3. User is shown the final-sale and no-refund terms.
4. User proceeds to payment and order becomes `pending_payment`.
5. If checkout creation fails or the hold expires, the order becomes `failed` or `cancelled` and the reserved coupon is released.
6. Payment provider confirms success and order becomes `paid`.
7. Reserved coupon finalization begins and order becomes `delivery_pending`.
8. Coupon is sent in Telegram and order becomes `delivered`.
9. If payment fails, order becomes `failed`.
10. If a serious post-delivery problem occurs, order may become `disputed`.

Guardrails:
- no order should move to `paid` without provider confirmation
- no order should move to `delivered` without a coupon assignment and delivery record

## Telegram Bot Interaction Flow

### Purchase Flow
1. User opens the Telegram bot.
2. Bot sends a message containing 1 to 3 coupon listings.
3. Each coupon in the message has its own `Buy` button.
4. User presses the `Buy` button for one specific coupon.
5. Bot shows the selected coupon detail and purchase terms:
   - merchant name
   - coupon value
   - sale price in ILS
   - expiry
   - redemption instructions
   - final-sale disclosure
   - explicit statement that no refunds are available
   - sold-out notice when no inventory is available
6. User confirms purchase.
7. Bot reserves one coupon, creates a PayPal checkout order, and sends the PayPal approval link.
8. Bot states that the coupon is reserved only for the checkout-hold window.
9. User completes payment in PayPal with a supported funding source.
10. Backend verifies the PayPal webhook and captures the approved order if needed.
11. On confirmed capture, system finalizes the reserved coupon and sends it to the user in Telegram.
12. Bot confirms delivery and gives support contact path.

Message rule:
- every promotional or catalog message from the bot should contain between 1 and 3 coupon options
- each option should be individually purchasable from the same message via its own `Buy` button
- pressing one `Buy` button should create an order only for that selected coupon, not for the whole message
- if a listing has zero available coupons, purchase buttons should be hidden and only non-purchase actions remain

### Complaint Flow
1. User reports coupon invalid, delivery problem, or payment problem.
2. Bot creates a support event.
3. Admin reviews the linked order, payment, coupon, and delivery records.
4. Admin responds to the user manually.

## Admin Workflow

### Inventory Intake Workflow
1. Operator acquires coupon inventory.
2. Operator enters:
   - merchant name
   - source
   - coupon value
   - sale price
   - expiry
   - coupon code
   - transferability notes
3. Operator verifies the coupon is sellable.
4. Coupon becomes `available`.

### Listing Workflow
1. Operator creates a listing tied to available inventory.
2. Operator defines:
   - listing title
   - description
   - price
   - expiry summary
   - redemption instructions
   - final-sale / no-refund text
3. Operator previews the listing.
4. Operator publishes the listing to Telegram.

### Delivery Review Workflow
1. System records a paid order.
2. System assigns coupon inventory.
3. Delivery event is sent to Telegram.
4. Operator can inspect paid, assigned, and delivered records if something fails.

### Support Workflow
1. User complaint creates a support case.
2. Operator reviews order, payment, coupon, and delivery evidence.
3. Operator records investigation outcome.
4. Operator may mark coupon or source as disputed or voided if needed.

## Refund Handling

### Stated MVP Policy
The MVP policy is explicit:
- all sales are final
- no refunds are available

This must be shown before payment.

### System Handling
Because no refunds are part of the stated MVP policy:
- there is no normal refund flow
- there is no automated refund engine
- there is no user-facing refund request path in the primary product flow

### Internal Exception Handling
Even without refunds as policy, the system must still support:
- logging payment issues
- logging invalid coupon complaints
- logging duplicate charge investigations
- manual escalation if the business chooses to make a one-off exception

Any exception should be handled manually outside the normal product flow and recorded in support and audit records.

## Failure and Exception Handling

### Payment Succeeds but Coupon Cannot Be Assigned
Handling:
- payment is recorded successfully
- no alternate coupon is auto-assigned
- order is escalated for manual refund or support review
- support case is opened
- operator investigates inventory state immediately

### Coupon Assigned but Telegram Delivery Fails
Handling:
- delivery remains `failed`
- operator retries or manually re-sends
- order does not move to `delivered` until delivery succeeds

### User Claims Coupon Is Invalid
Handling:
- support case is created
- coupon is reviewed and may become `disputed`
- source quality is re-evaluated

### Duplicate Charge or Chargeback
Handling:
- payment issue is logged
- operator reviews PayPal order, capture, and delivery evidence
- case is handled manually because no automated refund path exists

## Recommended MVP Flow Set
The flows that should actually be implemented first are:
1. inventory intake
2. listing publish and pause
3. Telegram 1-to-3 coupon message and purchase flow
4. payment confirmation
5. coupon assignment and delivery
6. support-case logging and manual investigation
