# Telegram Bot Flow Architecture: Direct Merchant Offers (Manual PayBox V1)

## Context
- Business model: fixed offers from small businesses that work with us directly
- Payment method: PayBox link
- Verification model: buyer submits PayBox username, admin verifies manually
- Fulfillment model: bot sends one predefined coupon code after approval

## 1. Global Configuration
- Business name: `Bazario`
- Support contact: `@admin`
- Payment link source: offer-level PayBox link or merchant default PayBox link

## 2. User Flow

### Step 1: Onboarding (`/start`)
Trigger:
- user sends `/start`

Bot behavior:
- greet the user
- explain that the bot sells fixed local offers from small businesses
- show 1 to 3 active offers

Buttons per offer:
- `View details`
- `Buy`

### Step 2: Offer Detail
Trigger:
- user taps `View details` or `Buy`

Bot message should include:
- merchant name
- offer title
- short description
- price in ILS
- merchant disclosure text
- redemption terms
- support contact

Buttons:
- `Pay with PayBox`
- `Another offer`
- `Support`

### Step 3: Payment Instructions
Trigger:
- user taps `Pay with PayBox`

Bot behavior:
- create or reuse an order in `awaiting_payment`
- send the payment link
- explain the two-step process

Bot message:
1. Pay through the PayBox link
2. Return here and tap `I paid`
3. Send the PayBox username used for the payment
4. Wait for manual verification

Buttons:
- `Open PayBox`
- `I paid`
- `Cancel`

### Step 4: Payment Claim Collection
Trigger:
- user taps `I paid`

Bot behavior:
- ask for the PayBox username used for the payment
- store the next free-text message as the claim input

Bot message:
- ask the user to send the exact PayBox username used for the payment
- explain that the team will verify it manually

### Step 5: Waiting State
Trigger:
- user sends the PayBox username

Bot behavior:
- create a `manual_payment_claim`
- move the order to `payment_claim_submitted`
- confirm that the claim is under review
- notify the admin

Bot message:
- thank the user
- say the payment is now under review
- explain that the code will be sent here after approval

## 3. Admin Flow

### Step 6: Admin Alert
Trigger:
- a manual payment claim is submitted

Admin message should include:
- buyer Telegram handle and id
- order number
- merchant name
- offer title
- amount
- payer PayBox username
- submit timestamp

Buttons:
- `Approve and send code`
- `Reject claim`

### Step 7: Approval And Fulfillment
Trigger:
- admin taps `Approve and send code`

System behavior:
- mark the claim `verified`
- move the order to `payment_verified`
- assign one available predefined code
- send the code to the user
- record delivery evidence

User message:
- confirm payment approval
- send the predefined code
- include redemption instructions and support contact

Admin message:
- confirm that the code was assigned and sent

### Step 8: Rejection
Trigger:
- admin taps `Reject claim`

System behavior:
- mark the claim `rejected`
- move the order to `payment_rejected`
- notify the user

User message:
- explain that the payment could not be matched
- ask the user to retry or contact support

## 4. Support Flow

### Post-Purchase Support
Trigger:
- user taps `Support` or sends a problem message

System behavior:
- create or update a support case
- link the case to the order, claim, and code when available
- route the case to admin review

## 5. Guardrails
- only active offers can be purchased
- payment does not grant fulfillment until admin approval
- no code is assigned before payment verification
- approval and rejection actions must be idempotent
- if payment is approved but no code is available, move the order to `support_required`
