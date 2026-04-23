# Prompt 05: Payments And Coupon Delivery

You are a senior Go engineer. Implement payment handling and coupon delivery for the MVP.

## Goal
After a user presses `Buy`, the system must:
- create an order
- start payment with PayPal in `ILS`
- wait for provider-confirmed success
- assign exactly one coupon from inventory
- deliver that coupon in Telegram

## Payment Design
Build a small provider adapter interface, but only implement PayPal for MVP.

Keep the interface narrow, for example:
- `CreateCheckout(order)`
- `ParseWebhook(req)`
- `ValidateWebhook(req)`

Use a PayPal-hosted checkout link rather than collecting card details directly.

## Rules
- do not trust client-side payment success
- only trust PayPal-confirmed success
- webhook handling must be idempotent
- delivery must be idempotent
- verify PayPal webhooks with PayPal signature verification, not a shared secret header
- capture approved PayPal orders on the backend, then fulfill only after confirmed capture

## Required Flow
1. user confirms purchase
2. system creates a PayPal checkout order and payment record
3. bot returns a PayPal approval link or button to the user
4. user pays in PayPal with a supported funding source such as card or wallet
5. PayPal webhook is verified by the backend
6. backend captures the approved PayPal order if needed
7. backend records the successful payment and marks the order `paid`
8. system assigns one available coupon transactionally
9. system records coupon delivery
10. bot sends coupon to user
11. order becomes `delivered`

## No-Refund Policy Handling
The user-facing product policy is:
- all sales are final
- no refunds are available

What to implement:
- final-sale acknowledgment stored on the order
- no normal refund endpoint
- no user self-serve refund request flow

What still must exist:
- support case logging
- chargeback/dispute evidence
- manual exception notes if an operator makes a one-off decision outside the normal flow

## Security And Reliability
- encrypt coupon codes at rest
- never expose coupon codes before payment success
- store delivery evidence
- keep PayPal webhook verification strict
- store PayPal order and capture references for reconciliation
- handle duplicate PayPal webhooks without double-delivery

## Acceptance Criteria
- user can complete checkout in `ILS`
- PayPal approval link is created after purchase confirmation
- verified PayPal webhook marks the order paid
- exactly one coupon is assigned to the order
- coupon is delivered in Telegram after payment success
- duplicate webhooks do not double-deliver
- support case can be opened for a bad delivery or invalid coupon complaint
