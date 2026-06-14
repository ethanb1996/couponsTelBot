# Restaurant Redemption Confirmation

Status: planned feature spec

## Goal

Improve the end of the coupon flow so the restaurant has clear confidence about:

- which coupon was presented
- which order it belongs to
- what amount was paid
- whether the coupon is still valid
- when the coupon was actually redeemed
- which restaurant chat/channel received the confirmation

All Telegram messages in this feature must be in Hebrew.

## Current Baseline

The current implemented flow is:

1. Customer chooses a coupon in Telegram.
2. Customer pays through a fixed PayBox link.
3. Customer uploads a PayBox payment screenshot.
4. Admin approves the claim from Telegram.
5. Bot creates a secure QR code and sends it to the customer.
6. Customer shows QR at the restaurant.
7. Restaurant scans QR.
8. Backend currently marks the QR as redeemed immediately on scan.
9. Admin review chat is notified.
10. Optional merchant email is sent on first redemption.

The improvement is to make the scan page a restaurant review page first. The coupon should be redeemed only after the restaurant clicks a clear Redeem button.

## Desired Flow

1. Customer chooses coupon in Telegram.
2. Customer pays with the fixed PayBox link.
3. KuponFast validates the payment semi-automatically through the screenshot/admin review flow.
4. Bot creates a secure QR code.
5. Customer shows the QR at the restaurant.
6. Restaurant scans the QR.
7. Restaurant sees a Hebrew coupon detail page.
8. Restaurant checks the details and clicks `ממש קופון`.
9. Backend marks the coupon as redeemed exactly once.
10. Restaurant sees a Hebrew success page.
11. KuponFast admin review chat receives a Hebrew confirmation.
12. Restaurant Telegram chat/channel receives a Hebrew confirmation.

Customer Telegram confirmation after redemption remains optional and is not part of this feature.

## Restaurant Scan Page

`GET /api/redemptions/scan/{token}` should render a Hebrew HTML page and should not redeem the coupon by itself.

The page should show:

- status: valid, already redeemed, invalid, or expired if expiry is available
- order number
- merchant name
- coupon title
- amount paid
- payment status summary
- approval time if available
- customer display name if available
- redemption terms

Primary action for a valid unused coupon:

```text
ממש קופון
```

Recommended page copy:

```text
בדיקת קופון

נא לוודא שהפרטים מתאימים להזמנה לפני המימוש.

מספר הזמנה: PB-20260609180338
בית עסק: הרובע י״ב
קופון: פיצה משפחתית + תוספת
סכום ששולם: 59₪
סטטוס תשלום: אושר במערכת

[ממש קופון]
```

Already redeemed page:

```text
הקופון כבר מומש

הקופון הזה מומש בעבר ולא ניתן לממש אותו שוב.

מספר הזמנה: PB-20260609180338
מומש בתאריך: 11/06/2026 21:15
```

Invalid page:

```text
קוד לא תקין

לא ניתן לאמת את קוד הקופון. אין לכבד את הקופון בלי בדיקה מול KuponFast.
```

## Redeem Action

The Redeem button should submit a POST request:

```http
POST /api/redemptions/redeem/{token}
```

Rules:

- Only the first successful redeem changes state.
- Repeat clicks must return already-redeemed without changing `redeemed_at`.
- Invalid tokens must not reveal sensitive data.
- Notification failure must not roll back a successful redemption.
- The token must never include the customer Telegram chat id.
- Delivery and notifications must resolve all destinations from the database/config.

## Telegram Confirmations

### Restaurant Chat/Channel

The restaurant should receive the confirmation in a dedicated Telegram chat/channel configured for that merchant.

Recommended message:

```text
✅ קופון מומש בהצלחה

מספר הזמנה: PB-20260609180338
בית עסק: הרובע י״ב
קופון: פיצה משפחתית + תוספת
סכום ששולם: 59₪
סטטוס תשלום: אושר במערכת
שעת מימוש: 21:15 11/06/2026

המימוש נרשם במערכת KuponFast.
```

If the PayBox link belongs directly to the restaurant and money goes to the restaurant PayBox account, the payment line may be:

```text
סטטוס תשלום: אושר במערכת עבור קישור הפייבוקס של העסק
```

If KuponFast receives the money and later settles with the restaurant, do not say that the restaurant already received the money. Use:

```text
סטטוס תשלום: אושר במערכת KuponFast
```

### Admin Review Chat

Recommended message:

```text
✅ מימוש קופון במסעדה

מספר הזמנה: PB-20260609180338
בית עסק: הרובע י״ב
קופון: פיצה משפחתית + תוספת
סכום ששולם: 59₪
לקוח: ישראל ישראלי
Telegram ID: 123456789
שעת מימוש: 21:15 11/06/2026
נשלח אישור לצ׳אט העסק: -1001234567890
```

Repeat redeem attempt:

```text
⚠️ ניסיון מימוש חוזר

מספר הזמנה: PB-20260609180338
בית עסק: הרובע י״ב
קופון: פיצה משפחתית + תוספת
מומש לראשונה: 21:15 11/06/2026
```

## Configuration

Preferred long-term model: per-merchant Telegram destination in the database.

Add to `merchant_partners`:

- `redemption_notification_chat_id BIGINT NULL`
- `redemption_notification_chat_title TEXT NOT NULL DEFAULT ''`

Optional fallback env var for pilot:

```env
TELEGRAM_RESTAURANT_REDEMPTION_CHAT_ID=
```

Resolution order:

1. `merchant_partners.redemption_notification_chat_id`
2. `TELEGRAM_RESTAURANT_REDEMPTION_CHAT_ID`
3. No restaurant Telegram notification; still notify admin

The bot must be added to the restaurant chat/channel. For a Telegram channel, the bot must be an admin. For a group, membership is enough only if the group allows the bot to post.

## Migration And Table Review

### Tables Already Supporting This Flow

- `users`: stores customer Telegram identity.
- `merchant_partners`: stores restaurant/business details and currently stores contact info in `contact_reference`.
- `offers`: stores merchant name, coupon title, price, PayBox link, support contact, and redemption terms.
- `orders`: stores order number, price snapshot, status, PayBox link snapshot, and assigned predefined code.
- `manual_payment_claims`: stores payment screenshot evidence and review status.
- `predefined_codes`: stores coupon inventory and status.
- `coupon_deliveries`: stores QR delivery to the customer.
- `coupon_redemptions`: stores QR token and current redemption state.
- `admin_actions`: stores approval/rejection audit events.

### Current Schema Gap

`coupon_redemptions` currently has:

- `status` with only `issued` and `redeemed`
- `scanned_at`
- `merchant_reference`
- `scanner_reference`
- `scan_metadata_json`

Current implementation marks the coupon as `redeemed` during scan. For this feature, scan and redeem must be separate:

- scan means the restaurant opened the QR details page
- redeem means the restaurant clicked `ממש קופון`

### Proposed Migration

Add restaurant notification routing:

```sql
ALTER TABLE merchant_partners
    ADD COLUMN IF NOT EXISTS redemption_notification_chat_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS redemption_notification_chat_title TEXT NOT NULL DEFAULT '';
```

Add explicit redeem fields:

```sql
ALTER TABLE coupon_redemptions
    ADD COLUMN IF NOT EXISTS first_viewed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS redeemed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS redeemed_by_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS redeem_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS restaurant_notification_chat_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notification_message_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notified_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notification_error TEXT NOT NULL DEFAULT '';
```

Update the status constraint:

```sql
ALTER TABLE coupon_redemptions DROP CONSTRAINT IF EXISTS coupon_redemptions_status_check;
ALTER TABLE coupon_redemptions ADD CONSTRAINT coupon_redemptions_status_check
    CHECK (status IN ('issued', 'redeemed'));
```

Note: keep `status` simple for MVP. `first_viewed_at` records scan visibility without introducing a `previewed` lifecycle status.

### Optional Audit Table

If we want better auditability for repeat scans and repeated button clicks, add:

```sql
CREATE TABLE IF NOT EXISTS coupon_redemption_events (
    id BIGSERIAL PRIMARY KEY,
    coupon_redemption_id BIGINT NOT NULL REFERENCES coupon_redemptions(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('viewed', 'redeemed', 'repeat_redeem', 'invalid_token')),
    actor_reference TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coupon_redemption_events_redemption_created
    ON coupon_redemption_events (coupon_redemption_id, created_at DESC);
```

This is recommended if restaurants will have multiple staff members scanning coupons.

### Cleanup Considerations

The migration history still includes older tables and statuses from a previous checkout/listing model:

- `coupon_sources`
- `listings`
- `coupons`
- `payments`
- `fulfillment_jobs`
- old order statuses such as `pending_payment`, `paid`, `delivery_pending`, `delivered`

Do not drop them as part of this feature unless code usage is audited first. Some admin and compatibility paths may still reference them.

Recommended cleanup sequence:

1. Implement the restaurant redemption confirmation feature first.
2. Add tests proving the new flow uses only `offers`, `predefined_codes`, `orders`, `manual_payment_claims`, `coupon_deliveries`, and `coupon_redemptions`.
3. Search code usage for legacy tables.
4. Remove or archive legacy admin/import/payment paths.
5. Only then create a destructive cleanup migration.

## Backend Interface Changes

Add separate store operations:

```go
GetCouponRedemptionPreview(ctx, token) (CouponRedemptionPreview, error)
ConfirmCouponRedemption(ctx, params) (CouponRedemptionConfirmResult, error)
```

`GetCouponRedemptionPreview`:

- loads token, order, offer, merchant, buyer context
- updates `first_viewed_at` only if empty, or records a `viewed` event
- does not mark order/code/redemption as redeemed

`ConfirmCouponRedemption`:

- locks the redemption row
- if status is `issued`, marks redemption `redeemed`
- sets `redeemed_at`
- updates order to `coupon_redeemed`
- updates predefined code to `redeemed`
- returns whether this was the first redeem

Add notification interface:

```go
type RestaurantRedemptionNotifier interface {
    NotifyRestaurantCouponRedeemed(ctx context.Context, result CouponRedemptionConfirmResult) error
}
```

## HTTP Contract

### Preview

```http
GET /api/redemptions/scan/{token}
```

Returns Hebrew HTML.

### Confirm Redeem

```http
POST /api/redemptions/redeem/{token}
```

Returns Hebrew HTML for browser flow.

For API-style integrations, keep JSON support:

```http
POST /api/redemptions/redeem
Content-Type: application/json

{
  "redemption_token": "token",
  "merchant_reference": "restaurant-pos-1",
  "scanner_reference": "staff-name"
}
```

## Security Notes

- QR token is a bearer secret and should be long, random, and unguessable.
- The scan page must not expose internal database ids unless needed for support.
- The customer Telegram chat id must not be embedded in the QR or form payload.
- The restaurant notification chat id must be resolved from merchant config, not from request input.
- Repeat redemption must be idempotent.
- Invalid token page should be friendly but should not reveal whether an order exists.

## Test Plan

Store tests:

- valid token preview returns order, merchant, offer, amount, and status without redeeming
- first redeem changes redemption to `redeemed`
- first redeem sets `redeemed_at` only once
- repeat redeem returns already redeemed and does not rewrite `redeemed_at`
- order moves to `coupon_redeemed` only on first redeem
- predefined code moves to `redeemed` only on first redeem
- restaurant notification chat id resolves from merchant first, then fallback env

HTTP tests:

- `GET /api/redemptions/scan/{token}` returns Hebrew detail page with `ממש קופון`
- GET preview does not update order/code to redeemed
- `POST /api/redemptions/redeem/{token}` returns Hebrew success page
- repeat POST returns Hebrew already-redeemed page
- invalid token returns Hebrew invalid-code page

Telegram tests:

- first redeem sends Hebrew confirmation to restaurant chat/channel
- first redeem sends Hebrew confirmation to admin review chat
- repeat redeem sends Hebrew warning to admin review chat
- repeat redeem does not send a second restaurant success confirmation unless explicitly configured
- all restaurant/admin redemption messages are Hebrew

Regression tests:

- payment approval still sends QR to the correct customer from `orders.user_id -> users.telegram_user_id`
- approval callback still carries only `claim_id`
- QR token flow never trusts a customer chat id from request data

Run:

```powershell
$env:GOCACHE='c:\Projects\couponsTelBot\.gocache-local'
go test ./apps/api/...
```
