# Buy Flow E2E Verification In Development

## Purpose

This document turns the current development buy flow into concrete end-to-end checks.

It answers two practical questions:

1. Which paths should be verified as real E2E tests
2. Which paths can be simulated with HTTP requests instead of only using the Telegram chat UI

## Short Answer

You can verify the flow in three different ways.

1. Real E2E through Telegram chat plus PayPal sandbox
This is the best way to verify the true user journey.

2. Hybrid E2E by posting fake Telegram webhook updates to the API
This works for bot entry, listing navigation, and checkout start, as long as the app is running and you know the Telegram webhook secret.

3. Code-level integration tests with stubs
This is the best fit for late-payment, delivery-failure, and payment-provider edge cases that are hard to reproduce from the outside.

## What Can Be Simulated By API Requests

### Can simulate with HTTP

1. Telegram inbound updates by posting JSON to `POST /webhooks/telegram`
2. Telegram callback presses such as `buy_listing:<id>`, `view_details:<id>`, and `confirm_buy:<id>`
3. PayPal browser return page with `GET /payments/paypal/return`
4. PayPal browser cancel page with `GET /payments/paypal/cancel`

### Cannot realistically simulate with a simple fake HTTP request

1. PayPal webhook success and failure callbacks to `POST /webhooks/payments/paypal`

Reason:
The backend verifies PayPal webhook signatures against PayPal before it trusts the event. A random local `POST` will be rejected unless it carries a real PayPal-signed payload.

### Practical rule

1. Use real Telegram plus real PayPal sandbox for the happy path
2. Use fake Telegram webhook requests for bot-side branching and state-machine checks
3. Use Go integration tests or controlled app changes for strict PayPal webhook edge cases

## Current Dev Assumptions

These are true in the current repository setup.

1. `APP_ENV=development`
2. PayPal points to sandbox
3. The checkout-hold duration defaults to 10 minutes because `OPS_CHECKOUT_HOLD_DURATION` is not set in `.env`
4. In development, `/start` can show discounted `draft`, `preview`, and `active` listings
5. Checkout still only starts for `active` listings with available inventory

## Recommended Verification Order

Run the tests in this order.

1. Real happy-path purchase
2. API-simulated bot navigation
3. Preview or inactive checkout rejection
4. Hold expiry after abandoned checkout
5. Late-payment escalation
6. Payment failure and delivery failure edge cases

## Preconditions

Before running any of these checks:

1. Start the API from the repo root

```powershell
go run ./apps/api/cmd/server
```

2. Make sure `APP_BASE_URL` points at a public URL that PayPal can call back into
3. Make sure the Telegram bot webhook is configured to hit `/webhooks/telegram`
4. Make sure the PayPal sandbox webhook is configured to hit `/webhooks/payments/paypal`
5. Prepare a deterministic active listing with inventory

```powershell
go run ./apps/api/tmp_prepare_sandbox_checkout.go -listing-id 0
```

That helper prints a ready-to-use listing. In the current dev DB it prepared listing `89`, but do not hardcode that forever. Re-run the helper and use whatever listing it prints.

## Test 1: Real Happy Path

This is the primary E2E test.

### Goal

Verify the full user path:

1. discover listing
2. start checkout
3. pay in PayPal sandbox
4. receive coupon in Telegram
5. confirm order and coupon state in admin

### Steps

1. Run the sandbox-prep helper
2. Open the Telegram bot as a real user
3. Send `/start`
4. Find the prepared active listing
5. Tap the buy CTA
6. Complete the PayPal sandbox payment
7. Return to Telegram
8. Wait for coupon delivery
9. Open admin and inspect the order

### Expected result

1. A new order is created
2. One coupon is reserved before payment
3. The order moves to `pending_payment`
4. PayPal confirms the payment
5. The reserved coupon moves to `assigned`
6. The coupon delivery record is written
7. The order moves to `delivered`
8. The user receives the coupon code in Telegram

### Verify in admin

Check:

1. `/admin/orders`
2. `/admin/orders/{id}`
3. `/admin/coupons`

Confirm:

1. order status is `delivered`
2. latest payment is `captured` or `authorized`
3. assigned coupon matches the order
4. coupon delivery exists with Telegram message metadata

## Test 2: Simulate `/start` With A Telegram Webhook Request

This is useful when you want to exercise the bot flow without typing in the Telegram chat.

### Important note

The Telegram webhook handler responds with `202 Accepted` and processes the update asynchronously.

That means:

1. the HTTP response only tells you the webhook was accepted
2. you must verify the outcome in logs, admin, or in the real Telegram account tied to the fake update payload

### PowerShell example

Set the base URL and secret first:

```powershell
$baseUrl = "http://localhost:8080"
$secret = $env:TELEGRAM_WEBHOOK_SECRET
```

Post a `/start` command:

```powershell
$payload = @"
{
  "update_id": 1001,
  "message": {
    "message_id": 1,
    "date": 1710000000,
    "chat": { "id": 123456789, "type": "private" },
    "from": {
      "id": 123456789,
      "is_bot": false,
      "first_name": "Dev",
      "username": "devuser",
      "language_code": "en"
    },
    "text": "/start",
    "entities": [
      { "offset": 0, "length": 6, "type": "bot_command" }
    ]
  }
}
"@

Invoke-RestMethod `
  -Method Post `
  -Uri "$baseUrl/webhooks/telegram" `
  -Headers @{
    "X-Telegram-Bot-Api-Secret-Token" = $secret
    "Content-Type" = "application/json"
  } `
  -Body $payload
```

### Expected result

1. The bot creates or refreshes the Telegram user
2. The bot loads development-visible listings
3. The bot sends a featured listing message to Telegram for `chat.id=123456789`

## Test 3: Simulate A Buy Or Confirm Callback

This is the most useful API-driven test because it exercises order creation and checkout start.

### Goal

Verify that a callback like `confirm_buy:<listing_id>`:

1. creates the draft order
2. reserves one coupon
3. creates a PayPal checkout
4. marks the order `pending_payment`

### PowerShell example

Replace `89` with the listing prepared by the helper.

```powershell
$listingId = 89
$payload = @"
{
  "update_id": 1002,
  "callback_query": {
    "id": "cb-1002",
    "from": {
      "id": 123456789,
      "is_bot": false,
      "first_name": "Dev",
      "username": "devuser",
      "language_code": "en"
    },
    "message": {
      "message_id": 10,
      "date": 1710000001,
      "chat": { "id": 123456789, "type": "private" }
    },
    "data": "confirm_buy:$listingId"
  }
}
"@

Invoke-RestMethod `
  -Method Post `
  -Uri "$baseUrl/webhooks/telegram" `
  -Headers @{
    "X-Telegram-Bot-Api-Secret-Token" = $secret
    "Content-Type" = "application/json"
  } `
  -Body $payload
```

### Expected result

1. A draft order is created
2. One coupon is moved from `available` to `reserved`
3. PayPal checkout is created
4. The order becomes `pending_payment`
5. The bot sends a PayPal approval link to the Telegram user

## Test 4: Preview Or Inactive Listing Should Not Start Checkout

This is a development-specific branch because `/start` may show `preview` or `draft` listings.

### Goal

Verify that a non-active listing can be browsed but cannot be purchased.

### How to run it

1. Pick a `draft` or `preview` listing ID from admin
2. Send a `confirm_buy:<listing_id>` callback payload like the previous test

### Expected result

1. No successful checkout is started
2. No valid order is created for purchase completion
3. The user gets the preview/inactive message

## Test 5: User Cancels Or Abandons Checkout

This verifies the hold-expiry path.

### Goal

Verify that a reserved coupon is returned to inventory when the user does not complete payment.

### How to run it

1. Start checkout from Telegram or by the callback webhook simulation
2. Do not complete payment
3. Optionally open the cancel page

```powershell
Invoke-WebRequest "$baseUrl/payments/paypal/cancel?order_number=TEST"
```

4. Wait slightly longer than the checkout hold duration
5. Check admin state

### Expected result

1. The browser cancel page renders, but it does not itself release the reservation
2. The ops loop later cancels the order
3. The order becomes `cancelled`
4. `failure_reason` becomes `checkout_hold_expired`
5. The reserved coupon returns to `available`

## Test 6: Late Payment After Hold Expiry

This is a high-value edge case.

### Goal

Verify that the system does not auto-assign a different coupon after the original hold expired.

### Best way to test

This is difficult as a black-box external E2E test.

Recommended options:

1. use a Go integration test with a stubbed payment gateway
2. use a controlled dev scenario where a real PayPal webhook arrives after the hold already expired

### Expected result

1. payment is recorded
2. coupon is not auto-delivered
3. a support case is created for manual review

## Test 7: Simulating PayPal Webhooks

### Raw fake HTTP request

Not recommended.

Reason:

1. the backend verifies the webhook signature with PayPal
2. fake payloads will be rejected unless they are real PayPal events

### Realistic options

1. trigger a real sandbox payment and let PayPal send the webhook
2. rely on PayPal sandbox webhook resend tools if available in the sandbox dashboard
3. cover the branch in Go integration tests using a stub gateway

## Test 8: Delivery Failure

This is also difficult as a pure external E2E test.

### Goal

Verify that failed Telegram delivery creates a support case and does not silently mark success.

### Best way to test

1. integration test with a stub deliverer that returns an error
2. or temporarily break bot delivery in a safe dev-only environment

### Expected result

1. delivery record is created with `failed`
2. support case is opened
3. the order does not become cleanly delivered

## Recommended Minimum E2E Suite

If we want a practical dev verification suite with the highest signal, start with these five:

1. real Telegram plus real PayPal sandbox happy path
2. fake Telegram `/start` webhook
3. fake Telegram `confirm_buy:<active_listing>` callback
4. fake Telegram `confirm_buy:<preview_or_draft_listing>` callback
5. abandoned checkout leading to hold expiry and inventory release

## Important Notes Found During Analysis

These are worth verifying explicitly while testing.

1. The featured-card `Buy` action already starts checkout. It does not force a separate legal-confirmation screen first.
2. The cancel page is only informational. Reservation release happens later through the hold-expiry sweep.
3. There is at least one dev-data inconsistency where a listing is marked `sold_out` while still having an `available` coupon. The status gate still blocks checkout, but it is worth cleaning this data up.
4. Payment-failure branches deserve special scrutiny because they may leave a coupon reserved unless another cleanup path releases it later.

## Suggested Next Step

After this document, the best follow-up is:

1. automate Tests 2, 3, 4, and 5 as scriptable dev checks
2. keep Test 1 as a manual release-gate E2E
3. add Go integration tests for late payment, payment failure, and delivery failure
