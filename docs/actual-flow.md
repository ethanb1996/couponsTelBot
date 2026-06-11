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
4. Backend records the scan:
   - first valid scan changes the redemption from `issued` to `redeemed`
   - repeat scans do not re-redeem the coupon or rewrite the original scan time
   - invalid tokens show an invalid-code page
5. Merchant sees a simple Hebrew HTML result page:
   - first scan: coupon accepted
   - repeat scan: coupon already redeemed
   - invalid token: invalid code
6. The admin review chat is notified for both first scans and repeat scans.
7. No automatic customer Telegram message is sent on redemption.

`POST /api/redemptions/scan` remains JSON-friendly for API-style scans.

## Merchant Email After Redemption

On first successful QR redemption, the backend can send the merchant a confirmation email with the order and coupon details.

Email is optional and controlled by SMTP config:

```env
SMTP_HOST=smtp-relay.brevo.com
SMTP_PORT=587
SMTP_USERNAME=your-smtp-user
SMTP_PASSWORD=your-smtp-key
SMTP_FROM=your-verified-sender@example.com
```

The merchant email address is read from `merchant_partners.contact_reference` when it contains a valid email address. Repeat scans do not send another merchant email. Email delivery failure is logged but does not roll back the redemption.

For the pilot, Brevo is the preferred free provider because the current implementation already uses SMTP and Brevo offers a free daily sending allowance.

## Main Data Tables

- `users`: Telegram buyer identity and destination chat id.
- `merchant_partners`: merchant details, support contact, default PayBox link, optional email in `contact_reference`.
- `offers`: Telegram-facing coupon offer and PayBox link.
- `predefined_codes`: code inventory assigned after approval.
- `orders`: buyer order state and PayBox link snapshot.
- `manual_payment_claims`: uploaded PayBox screenshot review requests.
- `coupon_deliveries`: QR/code delivery attempts to buyers.
- `coupon_redemptions`: QR token state and scan metadata.
- `admin_actions`: approval/rejection audit trail.

## Verification

Run the API test suite from the repo root:

```powershell
$env:GOCACHE='c:\Projects\couponsTelBot\.gocache-local'
go test ./apps/api/...
```
