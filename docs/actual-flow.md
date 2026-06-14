# KuponFast Actual Flow

This is the current implemented MVP flow and the source of truth for operations, testing, and future changes.

## Buyer Purchase Flow

1. Buyer opens the Telegram bot and sends `/start`.
2. Bot shows active offers from the database.
3. Buyer taps the offer buy action.
4. Backend creates or resumes an `awaiting_payment` order.
5. Bot sends the PayBox order message with:
   - order number
   - merchant and coupon details
   - amount
   - PayBox payment button
   - support button
6. After a short delay, the bot sends a separate message asking the buyer to upload a PayBox payment screenshot.
7. Buyer uploads the screenshot as a Telegram photo or image document.
8. Backend creates a `manual_payment_claim`, moves the order to `payment_claim_submitted`, and confirms that the request was sent for review.

## PayBox Link Source

The PayBox payment URL comes from the offer record:

- `offers.payment_link` is used when set.
- If an offer is created without its own link, it can inherit `merchant_partners.default_payment_link`.
- The order stores a snapshot in `orders.paybox_payment_link`.

## Admin Approval Flow

1. The bot sends the payment screenshot and claim details to `TELEGRAM_ADMIN_REVIEW_CHAT_ID` when configured.
2. If `TELEGRAM_ADMIN_REVIEW_CHAT_ID` is empty, the bot falls back to direct messages to users listed in `TELEGRAM_ADMIN_USER_IDS`.
3. The admin notification shows the claim id, order number, merchant, offer, amount, buyer display, and buyer Telegram chat id.
4. Inline button callbacks carry only `admin_approve_claim:{claim_id}` or `admin_reject_claim:{claim_id}`.
5. Only Telegram users listed in `TELEGRAM_ADMIN_USER_IDS` may approve or reject, even inside the review chat.
6. On approval:
   - the claim becomes `verified`
   - the order becomes `payment_verified`
   - one available predefined code is assigned
   - a redemption token is created
   - a QR image is rendered
   - the QR is sent to the buyer resolved from `orders.user_id -> users.telegram_user_id`
   - delivery is recorded
   - the order becomes `coupon_sent`
7. Repeated approval after `coupon_sent` is idempotent and does not resend another QR.
8. On rejection:
   - the claim becomes `rejected`
   - the order becomes `payment_rejected`
   - the buyer receives a rejection/support message

The claim id is an internal review identifier. It helps the admin, audit log, and support flow refer to the exact screenshot review request. It is not trusted for QR delivery destination; delivery always resolves the buyer from the database.

## Telegram Admin Setup

Required:

```env
TELEGRAM_ADMIN_USER_IDS=123456789,987654321
```

Optional review chat:

```env
TELEGRAM_ADMIN_REVIEW_CHAT_ID=-1001234567890
```

For a group or supergroup, add the bot to the chat so it can post messages. If the group restricts posting, make the bot an admin. For a channel, the bot must be an admin.

## QR Redemption Flow

1. Buyer shows the delivered QR to the merchant.
2. Merchant scans the QR.
3. Browser opens `GET /api/redemptions/scan/{token}`.
4. Backend records first visibility and shows a Hebrew coupon review page without redeeming.
5. Merchant checks the details and submits `POST /api/redemptions/redeem/{token}`.
6. Backend records redemption exactly once:
   - first valid redeem changes the redemption from `issued` to `redeemed`
   - repeat redeems do not rewrite the original redemption time
   - invalid tokens show an invalid-code page
7. Merchant sees a Hebrew HTML result page:
   - first redeem: coupon accepted
   - repeat redeem: coupon already redeemed
   - invalid token: invalid code
8. The admin review chat is notified for first redeems and repeat redeem attempts.
9. The configured restaurant Telegram chat/channel is notified on first redeem.
10. No automatic customer Telegram message is sent on redemption.

`POST /api/redemptions/redeem` is the JSON-friendly endpoint for API-style redemption.

## Merchant Email After Redemption

On first successful QR redemption, the backend can send the merchant a confirmation email with the order and coupon details.

Email is optional and controlled by Resend config:

```env
RESEND_API_KEY=re_xxxxxxxxx
RESEND_FROM=onboarding@resend.dev
```

Replace `re_xxxxxxxxx` with your real Resend API key before enabling email. For production, use a verified sender/domain in `RESEND_FROM`.

The merchant email address is read from `merchant_partners.contact_reference` when it contains a valid email address. Repeat redeems do not send another merchant email. Email delivery failure is logged but does not roll back the redemption.

## Main Data Tables

- `users`: Telegram buyer identity and destination chat id.
- `merchant_partners`: merchant details, support contact, default PayBox link, optional email in `contact_reference`, and optional restaurant Telegram redemption destination.
- `offers`: Telegram-facing coupon offer and PayBox link.
- `predefined_codes`: code inventory assigned after approval.
- `orders`: buyer order state and PayBox link snapshot.
- `manual_payment_claims`: uploaded PayBox screenshot review requests.
- `coupon_deliveries`: QR/code delivery attempts to buyers.
- `coupon_redemptions`: QR token state, first view, redeem metadata, and restaurant notification status.
- `admin_actions`: approval/rejection audit trail.

## Verification

Run the API test suite from the repo root:

```powershell
$env:GOCACHE='c:\Projects\couponsTelBot\.gocache-local'
go test ./apps/api/...
```
