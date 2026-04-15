# PROJECT_CONTEXT

## Product Vision
Build a Telegram-based coupon product for Israel that helps users discover and redeem real savings with minimal effort. The product should begin as a narrow MVP focused on trusted, easy-to-use offers delivered through Telegram, with a simple experience that does not depend on risky automation or platform-intrusive behavior.

The longer-term vision is to become a compliant savings layer for Israeli consumers, starting with curated coupon discovery and potentially expanding into compliant resale or facilitation models only where lawful and operationally reliable.

## Target Users
- Price-sensitive consumers in Israel looking for savings on fast food, food delivery, groceries, and other everyday purchases
- Telegram-native users who prefer a lightweight messaging experience over downloading a new app
- Deal seekers who value verified, usable offers over large volumes of low-quality promotions
- Early merchant or reseller partners who want a simple distribution channel for approved offers

## Core Problem
Consumers in Israel often need to search across multiple sites, apps, and social channels to find worthwhile discounts. Many available offers are expired, misleading, difficult to redeem, or not clearly relevant to local users. This creates wasted time, low trust, and inconsistent savings.

For a Telegram-first audience, there is no simple, trusted product that delivers relevant coupon opportunities in a fast, localized, low-friction format.

## Value Proposition
The product gives Israeli users a trusted stream of timely, relevant coupon offers in Telegram with:
- Fast access to current deals without app installation
- Clear redemption instructions in a mobile-friendly format
- Curated inventory that prioritizes validity and trust over volume
- A localized user experience suited to Israeli users and merchants
- A safer operating model that avoids captcha bypassing, restricted automation, and brittle sourcing methods

## Constraints

### Legal Constraints
- The product must minimize legal and platform risk at every stage.
- The MVP must not rely on captcha bypassing, anti-bot evasion, restricted automation, or behavior that violates third-party terms.
- The product must not misrepresent merchant affiliation, coupon ownership, availability, or endorsement.
- Any resale or facilitation model must be treated as conditional on legal review and lawful supply rights.
- Promotional disclosures, refund handling, and customer communications must be compatible with Israeli consumer expectations and applicable law.
- There must be a clear process for removing expired, invalid, disputed, or non-compliant offers.

### Operational Constraints
- The team should optimize for a small-team MVP with manageable support load.
- Offer freshness is critical; stale or invalid coupons will quickly erode trust.
- Manual verification and manual sourcing are acceptable in MVP if they reduce legal and operational risk.
- Supply should come from safer channels such as direct partnerships, licensed inventory, reseller agreements, affiliate relationships, user-submitted leads with review, or manually verified public offers where lawful.
- The product must remain focused on a narrow initial category set instead of broad marketplace coverage.

### Technical Constraints
- Telegram should be the primary user surface in MVP.
- The system should not depend on brittle scraping of protected sites.
- The MVP should avoid unnecessary complexity such as a full standalone consumer app.
- Core data must support offer source, expiration, redemption instructions, status, category, and audit history.
- Basic observability is required so the team can detect broken offers, poor content quality, and operational failures early.

## Revenue Model Hypothesis
The most plausible early revenue model is a mix of:
- Affiliate or referral commissions from compliant partner programs
- Sponsored placements from approved merchants or distributors
- Featured offer distribution for trusted partners
- Lead-generation fees for qualified partner traffic

If the business later proves demand and secures lawful supply rights, an additional revenue path may be a facilitation fee or managed-resale margin on approved inventory. That should not be assumed for MVP.

For MVP, the preferred monetization order is:
1. Affiliate or referral revenue
2. Sponsored merchant placements
3. Partner distribution fees
4. Managed inventory or resale economics only after validation and legal review

## Risks
- Legal or platform risk if sourcing drifts into restricted automation, scraping, or unclear resale rights
- Trust risk if users encounter expired, misleading, or hard-to-redeem offers
- Supply risk if the team cannot secure enough safe, repeatable inventory sources
- Margin risk if manual operations are too expensive relative to revenue
- Support risk if refunds, complaints, or failed redemptions create operational drag
- Distribution risk if Telegram retention is good but user acquisition is weak
- Reputation risk if a small number of bad coupon experiences damage credibility
- Localization risk if content, support, or merchant flows do not fit Israeli user expectations

## Assumptions
- Telegram is a viable primary channel for reaching at least one meaningful segment of Israeli deal seekers.
- Users will prefer a smaller number of verified offers over a large stream of noisy promotions.
- A manual, compliance-first MVP can validate demand before deeper automation or broader category expansion.
- Fast food and adjacent high-frequency consumer categories may be a strong starting wedge because users understand the value quickly.
- Merchant, reseller, or affiliate partners may see Telegram distribution as a useful incremental channel.
- Trust, freshness, and simplicity are stronger early differentiators than broad inventory coverage.

## Open Questions
- Should the MVP start strictly with fast food, or include adjacent categories such as food delivery and groceries?
- Will the initial supply model be affiliate, partner-based, reseller-based, manually curated, or some combination?
- Is the first launch surface a Telegram bot, a Telegram channel, or both?
- What proof is required before publishing an offer as valid and current?
- What commercial disclosures are needed when an offer is sponsored, affiliated, or partner-supplied?
- What refund or support policy is needed if a promoted offer fails at redemption time?
- What legal review is required before enabling any resale or managed inventory model in Israel?
- What are the best payment and payout options if the product eventually handles transactions directly?
- Which acquisition loop beyond Telegram can drive the first reliable cohort of users?
- What success threshold would justify moving from manual curation to more structured partner integrations?
