# Coupon Messaging Display SPEC v1

Project: Telegram Restaurant Coupon Bot — Ashdod MVP  
Audience: Codex / implementation agent  
Primary goal: Adapt coupon display in the Telegram bot to maximize purchase conversion while keeping the system simple, measurable, and reusable across restaurants.

---

## 1. Product Goal

Build a reusable coupon presentation engine for Telegram that shows restaurant coupons using a high-conversion structure:

- Clear food hook
- Business identity and area
- Simple offer description
- Strong price anchor
- Real urgency
- Real scarcity
- Clear purchase CTA
- PayBox payment link button

The bot should support a general template strategy with light customization per business.

Rule:

```txt
80% shared template logic
20% business-specific customization
```

Do not hardcode one-off messages per restaurant unless explicitly needed for a manual campaign.

---

## 2. Important Telegram Implementation Notes

Telegram supports styled message text and inline keyboard buttons. Use Telegram Bot API `sendMessage` with `reply_markup.inline_keyboard` for CTA buttons.

Preferred implementation:

- Use HTML parse mode instead of MarkdownV2 to avoid escaping complexity.
- Use inline keyboard URL buttons for external PayBox links.
- Keep the PayBox URL out of the message body when possible and place it in a button.

Example button label:

```txt
👇 רכישה בפייבוקס
```

Expected inline keyboard structure:

```json
{
  "inline_keyboard": [
    [
      {
        "text": "👇 רכישה בפייבוקס",
        "url": "https://paybox-link.example"
      }
    ]
  ]
}
```

Notes:

- If tracking clicks is required, do not use the raw PayBox URL directly. Use an internal redirect URL first, then redirect to PayBox.
- Example: `https://your-domain.com/r/couponId?variant=A` → PayBox link.
- If no backend redirect exists yet, use PayBox links directly for MVP.

---

## 3. Coupon Display Formula

Every coupon message should be generated from these components:

```txt
[HOOK]
[BUSINESS]
[OFFER]
[PRICE_ANCHOR]
[URGENCY]
[SCARCITY]
[REDEMPTION_INFO]
[CTA_BUTTON]
```

### 3.1 HOOK

Purpose: Catch attention in the first 1–2 seconds.

Examples for pizza:

```txt
🍕 רעבים ואין כוח לבשל?
🍕 ערב פיצה משפחתי?
🍕 היום לא מבשלים
🍕 פיצה חמה ליד הבית
```

Examples by category:

```txt
Pizza: 🍕 רעבים ואין כוח לבשל?
Burger: 🍔 בא לכם להתפנק הערב?
Sushi: 🍣 ערב זוגי בלי לקרוע את הכיס?
Cafe: ☕ קפה ומאפה לפתוח את היום
Shawarma: 🥙 ארוחת צהריים מהירה ומשתלמת
```

### 3.2 BUSINESS

Format:

```txt
<businessName> | <areaLabel>
```

Example:

```txt
לקפריזה | רובע י״ב אשדוד
```

### 3.3 OFFER

Offer must be short and concrete.

Good:

```txt
פיצה משפחתית + תוספת
2 מגשי פיצה + שתייה
מגש משפחתי + 2 תוספות
```

Bad:

```txt
מבצע שווה במיוחד על אוכל טעים
```

### 3.4 PRICE_ANCHOR

Always show a clear comparison when there is a real discount.

Format:

```txt
במקום <originalPrice>₪ → <couponPrice>₪ בלבד
```

Example:

```txt
במקום 74₪ → 59₪ בלבד
```

If original price is unknown or not verified, do not fake it. Use:

```txt
מחיר קופון: 59₪ בלבד
```

### 3.5 URGENCY

Use time-based urgency only when true.

Examples:

```txt
🔥 היום בלבד
⏰ תקף עד 22:00
⚡ תקף ל־24 שעות
```

### 3.6 SCARCITY

Use quantity limits only when real.

Examples:

```txt
20 קופונים בלבד
נשארו 8 קופונים
מוגבל להיום
```

Rules:

- Never fake remaining coupon quantity.
- If the bot does not track inventory, use generic wording like `כמות מוגבלת` only if the business actually agreed to a limit.
- If inventory tracking exists, show real remaining quantity.

### 3.7 REDEMPTION_INFO

Show how the customer can use the coupon.

Examples:

```txt
📍 מימוש באיסוף עצמי / ישיבה במקום
📍 מימוש בסניף בלבד
📍 הציגו את אישור התשלום בבית העסק
```

For MVP, use:

```txt
📍 הציגו את אישור התשלום בבית העסק
```

### 3.8 CTA_BUTTON

Use one primary action.

Button text options:

```txt
👇 רכישה בפייבוקס
👇 קנה עכשיו
👇 שמור את ההטבה
```

Preferred for PayBox:

```txt
👇 רכישה בפייבוקס
```

---

## 4. Message Templates

### 4.1 Template A — Direct Conversion

Use when the offer has a clear discount.

```html
🍕 <b>רעבים ואין כוח לבשל?</b>

<b>לקפריזה | רובע י״ב אשדוד</b>

פיצה משפחתית + תוספת

במקום <s>74₪</s> → <b>59₪ בלבד</b>

🔥 היום בלבד
20 קופונים בלבד

📍 הציגו את אישור התשלום בבית העסק
```

Button:

```txt
👇 רכישה בפייבוקס
```

---

### 4.2 Template B — Scarcity Focus

Use when only a small number of coupons remain.

```html
🍕 <b>ערב פיצה?</b>

<b>לקפריזה | רובע י״ב</b>

פיצה משפחתית + תוספת

במקום <s>74₪</s> → <b>59₪ בלבד</b>

⚡ נשארו 8 קופונים

📍 הציגו את אישור התשלום בבית העסק
```

Button:

```txt
👇 קנה עכשיו
```

---

### 4.3 Template C — Family Angle

Use for family dinner positioning.

```html
👨‍👩‍👧 <b>ערב משפחתי בלי בישולים</b>

<b>לקפריזה | אשדוד</b>

פיצה משפחתית + תוספת

<b>59₪ בלבד</b>

⏰ תקף עד 22:00

📍 הציגו את אישור התשלום בבית העסק
```

Button:

```txt
👇 להזמנה בפייבוקס
```

---

## 5. Data Model

Implement or adapt the coupon model to support message generation.

```ts
type CouponCategory = 'pizza' | 'burger' | 'sushi' | 'cafe' | 'shawarma' | 'other';

type CouponStatus = 'draft' | 'active' | 'paused' | 'sold_out' | 'expired';

type CouponDisplayVariant = 'direct' | 'scarcity' | 'family';

interface Coupon {
  id: string;
  businessId: string;
  businessName: string;
  areaLabel: string;
  category: CouponCategory;

  title: string;
  offerDescription: string;

  originalPriceIls?: number;
  couponPriceIls: number;

  payboxUrl: string;
  internalTrackingUrl?: string;

  totalQuantity?: number;
  soldQuantity?: number;

  validUntil?: string; // ISO datetime
  validTodayOnly?: boolean;

  redemptionInfo?: string;
  kosherLabel?: string;

  status: CouponStatus;
  displayVariant?: CouponDisplayVariant;

  createdAt: string;
  updatedAt: string;
}
```

Computed fields:

```ts
remainingQuantity = totalQuantity - soldQuantity
hasDiscount = originalPriceIls && originalPriceIls > couponPriceIls
isSoldOut = remainingQuantity <= 0
isExpired = validUntil < now
```

---

## 6. Rendering Rules

### 6.1 Select Template

```ts
function selectTemplate(coupon: Coupon): CouponDisplayVariant {
  if (coupon.displayVariant) return coupon.displayVariant;

  const remaining = getRemainingQuantity(coupon);

  if (remaining !== null && remaining <= 8) return 'scarcity';

  if (coupon.category === 'pizza') return 'family';

  return 'direct';
}
```

### 6.2 Select Hook

```ts
const hooksByCategory = {
  pizza: [
    '🍕 רעבים ואין כוח לבשל?',
    '🍕 ערב פיצה משפחתי?',
    '🍕 היום לא מבשלים',
    '🍕 פיצה חמה ליד הבית'
  ],
  burger: [
    '🍔 בא לכם להתפנק הערב?',
    '🍔 המבורגר טוב לסגור את היום'
  ],
  sushi: [
    '🍣 ערב זוגי בלי לקרוע את הכיס?',
    '🍣 בא לכם סושי?'
  ],
  cafe: [
    '☕ קפה ומאפה לפתוח את היום',
    '☕ עצירה קטנה לקפה טוב'
  ],
  shawarma: [
    '🥙 ארוחת צהריים מהירה ומשתלמת',
    '🥙 רעבים למשהו מהיר?'
  ],
  other: [
    '🔥 הטבה מקומית באשדוד',
    '✨ קופון חדש ליד הבית'
  ]
};
```

For MVP:

- Pick the first hook by category.
- Later: rotate hooks for A/B testing.

### 6.3 Price Rendering

```ts
function renderPrice(coupon: Coupon): string {
  if (coupon.originalPriceIls && coupon.originalPriceIls > coupon.couponPriceIls) {
    return `במקום <s>${coupon.originalPriceIls}₪</s> → <b>${coupon.couponPriceIls}₪ בלבד</b>`;
  }

  return `<b>${coupon.couponPriceIls}₪ בלבד</b>`;
}
```

### 6.4 Urgency Rendering

Priority:

1. If `validTodayOnly = true`, show `🔥 היום בלבד`.
2. Else if `validUntil` is today and time exists, show `⏰ תקף עד HH:mm`.
3. Else if valid within 24h, show `⚡ תקף ל־24 שעות`.
4. Else omit urgency.

### 6.5 Scarcity Rendering

Priority:

1. If remaining quantity exists and remaining <= 10: `⚡ נשארו X קופונים`
2. Else if total quantity exists: `X קופונים בלבד`
3. Else omit scarcity.

Never show fake scarcity.

### 6.6 CTA URL

```ts
function getCtaUrl(coupon: Coupon): string {
  return coupon.internalTrackingUrl ?? coupon.payboxUrl;
}
```

---

## 7. Telegram Output Contract

The renderer should return this structure:

```ts
interface TelegramCouponMessage {
  text: string;
  parse_mode: 'HTML';
  reply_markup: {
    inline_keyboard: Array<Array<{
      text: string;
      url: string;
    }>>;
  };
}
```

Example output:

```ts
{
  text: `🍕 <b>רעבים ואין כוח לבשל?</b>\n\n<b>לקפריזה | רובע י״ב אשדוד</b>\n\nפיצה משפחתית + תוספת\n\nבמקום <s>74₪</s> → <b>59₪ בלבד</b>\n\n🔥 היום בלבד\n20 קופונים בלבד\n\n📍 הציגו את אישור התשלום בבית העסק`,
  parse_mode: 'HTML',
  reply_markup: {
    inline_keyboard: [[
      {
        text: '👇 רכישה בפייבוקס',
        url: 'https://paybox.example/link'
      }
    ]]
  }
}
```

---

## 8. A/B Testing SPEC

### 8.1 MVP Experiments

Start with simple copy tests only.

Experiment 1 — Price framing:

```txt
A: במקום 74₪ → 59₪ בלבד
B: חיסכון של 15₪ — משלמים רק 59₪
```

Experiment 2 — Hook:

```txt
A: 🍕 רעבים ואין כוח לבשל?
B: 👨‍👩‍👧 ערב משפחתי בלי בישולים
```

Experiment 3 — Scarcity:

```txt
A: 20 קופונים בלבד
B: נשארו 8 קופונים
```

Only run B if remaining quantity is real.

### 8.2 Tracking Events

Track these events:

```ts
type CouponEventType =
  | 'coupon_viewed'
  | 'cta_clicked'
  | 'payment_started'
  | 'purchase_confirmed'
  | 'coupon_redeemed';
```

Minimum event payload:

```ts
interface CouponEvent {
  id: string;
  couponId: string;
  businessId: string;
  userId?: string;
  variant: string;
  eventType: CouponEventType;
  timestamp: string;
}
```

Important:

- Telegram URL button clicks cannot always be tracked directly if linking straight to PayBox.
- For click tracking, use internal redirect URLs.
- For confirmed purchase tracking, MVP may require manual confirmation or PayBox reconciliation.

### 8.3 Metrics

Track per coupon and per business:

```txt
Coupon Views
CTA Clicks
Click-Through Rate
Confirmed Purchases
Purchase Conversion Rate
Coupon Redemptions
Revenue Per Coupon
Revenue Per Business
```

Formulas:

```txt
CTR = CTA Clicks / Coupon Views
Purchase Conversion = Confirmed Purchases / Coupon Views
Click-to-Purchase = Confirmed Purchases / CTA Clicks
```

---

## 9. Admin / Business Setup Requirements

The admin should be able to create a coupon with:

```txt
Business name
Area
Category
Offer description
Original price
Coupon price
PayBox URL
Quantity limit
Validity time
Redemption instructions
Display variant
```

Recommended validation:

- `couponPriceIls` is required and must be positive.
- `payboxUrl` is required and must be a valid URL.
- `originalPriceIls`, if provided, must be higher than `couponPriceIls`.
- `totalQuantity`, if provided, must be positive.
- `soldQuantity` cannot exceed `totalQuantity`.
- Expired coupons should not be shown.
- Sold-out coupons should show a sold-out state or be hidden.

---

## 10. Sold Out / Expired States

### Sold Out Message

```html
🍕 <b>הקופון אזל</b>

<b>לקפריזה | רובע י״ב אשדוד</b>

הקופון הזה נגמר, אבל נעדכן כשיעלה מבצע חדש.
```

Button:

```txt
🔔 עדכנו אותי על קופונים חדשים
```

### Expired Message

```html
⏰ <b>הקופון הסתיים</b>

<b>לקפריזה | רובע י״ב אשדוד</b>

ההטבה כבר לא בתוקף.
```

Button:

```txt
🔎 הצג קופונים פעילים
```

---

## 11. MVP Coupon Example — La Capriza

Seed data:

```json
{
  "id": "coupon_lacapriza_family_pizza_001",
  "businessId": "business_lacapriza_ashdod_yudbet",
  "businessName": "לקפריזה",
  "areaLabel": "רובע י״ב אשדוד",
  "category": "pizza",
  "title": "פיצה משפחתית + תוספת",
  "offerDescription": "פיצה משפחתית + תוספת",
  "originalPriceIls": 74,
  "couponPriceIls": 59,
  "payboxUrl": "https://paybox.example/lacapriza",
  "totalQuantity": 20,
  "soldQuantity": 0,
  "validTodayOnly": true,
  "redemptionInfo": "📍 הציגו את אישור התשלום בבית העסק",
  "kosherLabel": "כשר",
  "status": "active",
  "displayVariant": "direct"
}
```

Expected rendered message:

```html
🍕 <b>רעבים ואין כוח לבשל?</b>

<b>לקפריזה | רובע י״ב אשדוד</b>

פיצה משפחתית + תוספת

במקום <s>74₪</s> → <b>59₪ בלבד</b>

🔥 היום בלבד
20 קופונים בלבד

📍 הציגו את אישור התשלום בבית העסק
```

Button:

```txt
👇 רכישה בפייבוקס
```

---

## 12. Implementation Tasks for Codex

### Task 1 — Create coupon message renderer

Create a pure function:

```ts
renderCouponTelegramMessage(coupon: Coupon): TelegramCouponMessage
```

Requirements:

- Uses HTML parse mode.
- Uses selected template.
- Adds inline PayBox CTA button.
- Handles direct, scarcity, and family variants.
- Handles missing original price.
- Handles sold out and expired states.

### Task 2 — Add coupon display config

Create a config file for:

- Hooks by category
- CTA labels
- Default redemption text
- Template selection thresholds

Example:

```ts
export const couponMessagingConfig = {
  lowInventoryThreshold: 10,
  defaultCtaLabel: '👇 רכישה בפייבוקס',
  defaultRedemptionInfo: '📍 הציגו את אישור התשלום בבית העסק'
};
```

### Task 3 — Add tracking URL support

If the backend has routing support, create redirect endpoint:

```txt
GET /r/:couponId?variant=A
```

Behavior:

1. Log `cta_clicked` event.
2. Redirect to coupon PayBox URL.

If backend routing does not exist yet, skip this task and use direct PayBox links.

### Task 4 — Add tests

Test cases:

- Renders direct pizza coupon.
- Renders scarcity coupon when remaining quantity <= 10.
- Does not render fake scarcity when quantity is missing.
- Renders price without old price when original price missing.
- Renders sold-out message.
- Renders expired message.
- Produces valid Telegram message object.

---

## 13. Non-Goals for MVP

Do not build yet:

- Complex personalization engine
- AI-generated copy per user
- Full payment integration
- Full restaurant dashboard
- Automated PayBox reconciliation
- Multi-city marketplace logic

Focus only on:

```txt
Better coupon display → more PayBox clicks → real purchase validation
```

---

## 14. Success Criteria

The implementation is successful when:

- Each active coupon is rendered in a consistent conversion-focused format.
- A restaurant-specific coupon can be created without changing code.
- PayBox purchase CTA is obvious and easy to click.
- Inventory and urgency are never faked.
- At least basic view/click/purchase metrics can be tracked or prepared for tracking.
- The same renderer can support La Capriza and future Ashdod restaurants.
