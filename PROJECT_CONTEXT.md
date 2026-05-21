# PROJECT_CONTEXT

## Product Vision
Build a Telegram-based coupon sales product for Israel with a very simple MVP loop:

1. Publish fixed offers from small businesses that work with us directly
2. Show the offer inside Telegram
3. Send the buyer to a PayBox payment link
4. Ask the buyer to submit their PayBox username after payment
5. Verify the payment manually
6. Deliver a predefined coupon code in Telegram

The product should optimize for speed, operational simplicity, and fast go-to-market validation. The goal is not to build a marketplace. The goal is to prove that users will buy a small number of clearly priced, partner-authorized offers inside a Telegram flow.

## Target Users
- Price-sensitive Telegram users in Israel who want a fast, simple offer purchase flow
- Users who care more about speed and clarity than broad browsing
- Small business operators who want a lightweight sales channel without a full ecommerce stack

## Core Problem
Users who want a local coupon or deal often face a fragmented experience:
- offers are scattered across channels
- terms are unclear
- redemption steps are inconsistent
- small businesses often lack a simple direct-sales funnel in Telegram

This MVP solves a narrower problem than general coupon discovery. It gives the user a direct purchase path to a merchant-authorized offer with a semi-manual payment verification loop.

## Value Proposition
The product offers:
- a Telegram-native purchase flow
- fixed merchant offers with clear pricing in ILS
- direct merchant-partner positioning instead of ambiguous resale
- a semi-manual payment verification path that is fast to launch
- predefined coupon code delivery after approval

## Constraints

### Legal And Trust Constraints
- The product must not rely on captcha bypassing, anti-bot evasion, or restricted automation.
- The product must not misrepresent merchant affiliation, authorization, or offer rights.
- Each active offer must come from a small business that has approved the offer shape and redemption terms.
- The bot must clearly disclose whether it is selling on behalf of the merchant or as the operating sales channel for that merchant.
- Purchase terms, support contact, and redemption instructions must be visible before the user pays.

### Operational Constraints
- The MVP must remain small enough for a small team to operate manually.
- Payment verification is manual and depends on the buyer submitting a PayBox username after payment.
- Each delivered code must be auditable to a specific order.
- Operators need a simple way to approve, reject, pause, or investigate orders and offers.
- The first launch should use a small number of merchants and a small number of active offers.

### Technical Constraints
- Telegram is the primary user surface.
- The MVP should support direct sales, not a broad marketplace.
- The system must support ILS pricing and PayBox payment links.
- The system must store predefined codes securely and release them only after manual approval.
- The product should avoid heavy automation dependencies in the first version.

## Revenue Model Hypothesis
The MVP revenue model is direct merchant-partner sales:
- agree on a fixed offer with a small business
- publish the offer in Telegram
- collect payment through the merchant-approved PayBox flow
- deliver a predefined code after verification

Revenue may come from margin, revenue share, or a fixed operator fee per sale depending on the merchant agreement, but the MVP architecture should remain neutral to that commercial detail.

## Risks
- Merchant authorization risk if the offer is published without clear approval
- Payment-claim fraud risk if users submit false PayBox usernames
- User trust risk if payment is confirmed but fulfillment is slow or inconsistent
- Offer validity risk if a merchant changes terms after launch
- Manual operations risk if approval volume grows faster than the team can review
- Reputation risk if the bot does not clearly explain who is selling the offer and how support works

## Assumptions
- Small businesses are willing to provide fixed offers and predefined codes for Telegram sales.
- Users will accept a semi-manual payment verification flow if it is clearly explained.
- PayBox is a good enough payment channel for fast validation.
- The business can launch with a small number of merchants and a small number of offers.
- Admin approval in Telegram is sufficient for the first operating loop.

## Open Questions
- What exact merchant disclosure wording should appear in the offer detail view?
- What turnaround time is acceptable between payment claim submission and approval?
- Should the first launch use only one payment link per merchant or one link per offer?
- What manual exception policy should exist when a payment cannot be matched cleanly?
- How much fulfillment volume can the team handle before admin approval must move into a richer web UI?
