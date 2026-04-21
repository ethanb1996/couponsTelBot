# Prompt 05: Payments And Coupon Delivery

You are a senior Go engineer. Implement payment handling and coupon delivery for the MVP.

## Goal
After a user presses `Buy`, the system must:
- create an order
- start payment with a provider that accepts `ILS`
- wait for provider-confirmed success
- assign exactly one coupon from inventory
- deliver that coupon in Telegram

## Payment Design
Build a small provider adapter interface, but only implement one provider for MVP.

Keep the interface narrow, for example:
- `CreateCheckout(order)`
- `ParseWebhook(req)`
- `ValidateWebhook(req)`

## Rules
- do not trust client-side payment success
- only trust provider-confirmed success
- webhook handling must be idempotent
- delivery must be idempotent

## Required Flow
1. user confirms purchase
2. system creates payment record
3. system redirects user to provider checkout or returns provider payment URL
4. provider webhook confirms payment
5. system marks order `paid`
6. system assigns one available coupon transactionally
7. system records coupon delivery
8. bot sends coupon to user
9. order becomes `delivered`

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
- keep payment webhook verification strict

## Acceptance Criteria
- user can complete checkout in `ILS`
- payment webhook marks the order paid
- exactly one coupon is assigned to the order
- coupon is delivered in Telegram after payment success
- duplicate webhooks do not double-deliver
- support case can be opened for a bad delivery or invalid coupon complaint
