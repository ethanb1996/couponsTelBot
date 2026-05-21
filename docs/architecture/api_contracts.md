# API Contracts

Status: Draft

## Purpose
Define the target interaction contracts for the manual PayBox MVP.

These are target contracts for the next refactor phase. Current runtime code may still expose older PayPal-oriented behavior.

## Telegram Bot Callback Contracts

### `view_offer:{offer_id}`
Purpose:
- open the offer detail screen

Required behavior:
- load the offer by id
- show merchant name, title, price, disclosure, redemption terms, and support contact

### `start_payment:{offer_id}`
Purpose:
- create or resume an order and show payment instructions

Required behavior:
- create an order in `awaiting_payment` if needed
- snapshot offer price and payment link
- send the PayBox link and explain the claim submission step

### `submit_payment_claim:{order_id}`
Purpose:
- transition the bot into claim collection mode for one order

Required behavior:
- ask the buyer for the PayBox username used to pay
- associate the next free-text answer with the order

### `admin_approve_claim:{claim_id}`
Purpose:
- approve a pending manual payment claim

Required behavior:
- mark claim `verified`
- move order to `payment_verified`
- assign one available predefined code
- send the code to the user
- record delivery and admin audit entries

### `admin_reject_claim:{claim_id}`
Purpose:
- reject a pending manual payment claim

Required behavior:
- mark claim `rejected`
- move order to `payment_rejected`
- notify the buyer that the payment could not be matched
- preserve full audit history

## Target Bot Interaction Payloads

### Start Payment Response
Suggested fields:
- `order_id`
- `order_number`
- `offer_id`
- `offer_title`
- `merchant_name`
- `price_amount`
- `currency_code`
- `payment_link`
- `merchant_disclosure_text`
- `redemption_terms`
- `support_contact`
- `next_step_message`

### Payment Claim Submission
Suggested fields:
- `order_id`
- `payer_username`
- `claimed_amount`
- `submitted_at`

### Admin Claim Alert
Suggested fields:
- `claim_id`
- `order_id`
- `order_number`
- `buyer_telegram_user_id`
- `buyer_display_name`
- `offer_title`
- `merchant_name`
- `price_amount`
- `payer_username`
- `submitted_at`

## Target Admin Actions

### Approve Claim
Input:
- `claim_id`
- `review_note` optional

Output:
- updated claim
- updated order
- assigned code reference
- delivery result

### Reject Claim
Input:
- `claim_id`
- `review_note`

Output:
- updated claim
- updated order
- buyer notification result

## Suggested Backend Service Methods

### `CreateAwaitingPaymentOrder`
Input:
- `user_id`
- `offer_id`

Output:
- `order`

Rules:
- only active offers may start payment
- sold-out offers must fail clearly

### `SubmitManualPaymentClaim`
Input:
- `order_id`
- `payer_username`
- `claimed_amount` optional

Output:
- `manual_payment_claim`

Rules:
- order must be in `awaiting_payment`
- creating a claim moves the order to `payment_claim_submitted`

### `ApproveManualPaymentClaim`
Input:
- `claim_id`
- `reviewed_by`
- `review_note`

Output:
- `order`
- `manual_payment_claim`
- `predefined_code`
- `coupon_delivery`

Rules:
- approval must be idempotent
- if no code is available, move the order to `support_required`

### `RejectManualPaymentClaim`
Input:
- `claim_id`
- `reviewed_by`
- `review_note`

Output:
- `order`
- `manual_payment_claim`

Rules:
- rejection must be idempotent
- buyer should be able to contact support or retry through a new claim path

## Suggested HTTP Or Internal API Surface

The exact transport can vary, but the target API surface should include:
- offer listing and detail reads for the bot
- order creation for payment start
- payment claim submission
- admin claim approval and rejection
- support case creation and lookup

Recommended route or method groups:
- `/bot/offers`
- `/bot/orders`
- `/bot/payment-claims`
- `/admin/payment-claims`
- `/admin/offers`
- `/admin/support-cases`

## Idempotency Rules
- approving the same claim twice must not double-assign codes
- rejecting the same claim twice must not change a previously terminal outcome
- delivering the same order twice must not send multiple codes unless an explicit support resend flow exists
