# PROJECT_CONTEXT

## Product Vision
Build a Telegram-based coupon resale product for Israel with one very simple MVP loop:

1. Acquire pre-bought coupon inventory
2. List the coupon in Telegram
3. Accept payment in ILS through a payment provider
4. Deliver the coupon to the user in Telegram

The product should optimize for speed, operational simplicity, and controlled inventory rather than broad discovery. The goal is not to aggregate the whole market. The goal is to prove that users will buy a small number of clearly priced, immediately deliverable coupon deals inside a Telegram flow.

## Target Users
- Price-sensitive users in Israel who already use Telegram and want a fast purchase flow
- Users who prefer buying a ready coupon quickly instead of searching for deals themselves
- Users comfortable with a simple all-sales-final model if terms are clear before payment

## Core Problem
Users who want discounted fast-food or similar coupons often face a fragmented and inconvenient experience:
- offers are scattered across apps, websites, and social channels
- redemption value is hard to compare quickly
- finding a usable coupon can take more effort than the savings are worth

This MVP solves a narrower problem than general coupon discovery. It gives the user a direct purchase path to a pre-bought coupon that is already controlled by the seller and delivered immediately after payment.

## Value Proposition
The product offers:
- a simple Telegram-native buy flow
- pre-bought coupon inventory that is already in hand before sale
- immediate coupon delivery after successful payment
- pricing in ILS through a payment provider that supports ILS
- a narrow and understandable operating model

## Constraints

### Legal Constraints
- The product must not rely on captcha bypassing, anti-bot evasion, or restricted automation.
- The product must not misrepresent merchant affiliation, authorization, or coupon rights.
- The team must verify that any pre-bought coupon inventory is lawfully held and transferable before resale.
- The MVP must clearly disclose that all sales are final and that no refunds are available.
- The no-refund policy should be treated as a legal and trust-sensitive area that requires local review before launch.

### Operational Constraints
- Inventory must be pre-bought before listing so the system never sells stock it does not control.
- The team should start with a small set of coupon types and a small number of suppliers.
- Coupon freshness, validity, and transferability must be checked before listing.
- The team needs a manual process for voiding bad inventory and handling user complaints even if refunds are not offered.
- The MVP must remain small enough for a small team to operate manually.

### Technical Constraints
- Telegram is the primary user surface.
- The MVP should support a direct sale flow, not a large marketplace.
- The system must support ILS pricing and a payment provider that can accept ILS.
- The system must securely store coupon inventory and release it only after successful payment.
- The product must avoid risky automation dependencies on merchant or provider systems.

## Revenue Model Hypothesis
The MVP revenue model is straightforward:
- buy coupon inventory below resale price
- sell coupon inventory in Telegram at a markup

Gross margin comes from the spread between acquisition cost and resale price, minus payment provider fees and support loss from invalid inventory.

## Risks
- Coupon transferability risk if pre-bought coupons are not legally or contractually resellable
- Payment and chargeback risk because users may dispute all-sales-final transactions
- User trust risk if coupons fail after delivery and no refund is available
- Inventory risk if pre-bought stock expires, is revoked, or was already used
- Supplier risk if inventory quality depends on a small number of sources
- Regulatory or consumer-protection risk if the no-refund policy is not acceptable in practice
- Reputation risk if even a few failed transactions make the bot look unsafe

## Assumptions
- There is at least one category of coupons in Israel that can be pre-bought, stored, and resold with acceptable rights clarity.
- Users will accept a direct Telegram purchase flow if value and delivery are immediate.
- A payment provider that supports ILS can be integrated with acceptable friction.
- The business can start with controlled inventory rather than broad supply.
- A clearly disclosed no-refund policy may reduce operational complexity, but it also increases trust and legal risk.

## Open Questions
- Which coupon types are actually transferable and safe to resell?
- Which payment provider is the best fit for ILS payments in this product?
- Should the first launch use a Telegram bot, a channel, or both?
- What exact terms must be shown before purchase so the no-refund policy is explicit?
- What operational fallback is needed when a delivered coupon is invalid but the policy says no refunds?
- How much inventory should be pre-bought before proving demand?
- What evidence of coupon validity should be stored before listing inventory for sale?
