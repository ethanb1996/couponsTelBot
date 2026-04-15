# MVP PRD

Status: Draft
Owner: PRD Agent

## Product Summary
The MVP is a Telegram-first coupon discovery and partner distribution product for users in Israel. It helps users find verified fast-food and adjacent everyday savings offers through a lightweight bot or channel experience, with clear redemption instructions and minimal friction.

The MVP is intentionally designed to maximize learning while minimizing legal, platform, and operational risk. It will not begin as a direct coupon resale business. Instead, it will focus on compliant discovery, manual curation, partner-approved distribution, and referral-style monetization.

## Problem Statement
Israeli consumers looking for fast-food and similar everyday savings face a fragmented, low-trust experience. Deals are scattered across merchant apps, websites, promo channels, and social feeds. Many offers are expired, confusing, or difficult to redeem.

For Telegram-native users, there is no simple, trusted, localized way to receive a stream of verified offers that are easy to understand and quick to act on. The current market failure is not just lack of discovery, but lack of confidence that an offer is valid, relevant, and worth the effort.

## Product Goals
1. Validate that Israeli users want a Telegram-native coupon discovery experience.
2. Prove that users prefer a smaller number of verified offers over a large volume of noisy promotions.
3. Learn which categories, formats, and delivery patterns drive engagement and redemption intent.
4. Establish a compliant, low-risk supply model that does not depend on restricted automation or unclear resale rights.
5. Test early monetization through affiliate, sponsorship, or partner distribution without taking on direct commerce risk.

## Non-Goals
- Direct resale of coupons in MVP
- Managed inventory of ambiguous or non-transferable coupon assets
- Automated scraping of protected merchant sites or apps
- Captcha bypassing, anti-bot evasion, or restricted automation
- A full standalone consumer mobile app
- Broad category coverage before the core loop is proven
- Complex merchant self-serve tooling

## Target Users

### Primary Users
- Price-sensitive consumers in Israel who regularly buy fast food, food delivery, and other high-frequency consumer items
- Telegram-native users who prefer a fast messaging experience over browsing multiple merchant websites or installing new apps
- Deal seekers who value trust, simplicity, and relevance more than sheer offer volume

### Secondary Users
- Approved merchants, affiliate partners, and licensed distributors seeking a lightweight promotion channel
- Small internal operators curating and verifying deals manually during MVP

## User Pains
- Finding worthwhile offers takes too much time
- Many offers appear untrustworthy or expire before use
- Redemption rules are often unclear
- Users do not know which deals are relevant for Israel
- Existing discovery channels are cluttered, inconsistent, or spammy

## Core Value Proposition
Users receive a trusted stream of relevant, verified savings opportunities in Telegram, with:
- no app install required
- clear redemption instructions
- localized offer context
- fewer but higher-confidence offers
- a safer operating model that prioritizes compliance and trust over risky scale

## Assumptions
- Telegram is a viable first distribution channel for at least one meaningful cohort of Israeli deal seekers.
- Fast food is a strong entry category because value is easy to understand and purchase frequency is high.
- Manual verification is acceptable in MVP if it meaningfully improves trust and reduces platform/legal risk.
- Partner-approved and affiliate-compatible offers are sufficient to test demand before direct commerce is needed.
- Users will return if the product is consistently useful and trustworthy, even if inventory breadth is limited.

## User Journeys

### Journey 1: User Joins and Receives First Value
1. User discovers the Telegram bot or channel from a friend, community post, or partner promotion.
2. User joins and sees a short explanation of what the product does and how offers are verified.
3. User chooses categories of interest such as fast food, food delivery, or groceries.
4. User immediately sees a small set of currently active offers with merchant name, value summary, expiry, and redemption instructions.
5. User clicks through or saves an offer for later.

Success condition:
The user understands the product quickly and finds at least one relevant offer within the first session.

### Journey 2: User Views an Offer and Decides Whether to Act
1. User opens an offer in Telegram.
2. User sees:
   merchant name
   offer summary
   expiration date
   key restrictions
   source type such as partner, affiliate, or curated public offer
   redemption path
3. User either clicks through to the merchant or marks the offer as not relevant.
4. User can send simple feedback such as "worked" or "didn't work."

Success condition:
The user can evaluate the offer without confusion and understands exactly what action to take.

### Journey 3: Operator Curates and Publishes a Verified Offer
1. Internal operator reviews a potential offer from a compliant source.
2. Operator verifies source, availability window, restrictions, and publishing rights.
3. Operator enters standardized metadata into the admin workflow.
4. Offer is published to the relevant Telegram audience.
5. Operator monitors feedback and removes or updates the offer if quality drops.

Success condition:
The offer is published with consistent quality and can be removed quickly if it becomes stale or disputed.

### Journey 4: User Reports a Bad Offer
1. User marks an offer as invalid or misleading.
2. The report enters a lightweight review queue.
3. Operator checks the claim and either updates, pauses, or removes the offer.
4. The user receives a brief acknowledgment if support follow-up is needed.

Success condition:
Bad offers are handled quickly enough to preserve trust.

## MVP Features

### 1. Telegram Delivery Surface
Must have

Description:
- Telegram bot, channel, or a hybrid of both as the primary user surface
- simple onboarding message
- category selection or lightweight preference capture

Why included:
- fastest path to testing demand in the intended channel
- avoids the cost and complexity of a standalone app

### 2. Curated Offer Feed
Must have

Description:
- manually reviewed offers with consistent formatting
- merchant name, offer summary, validity window, restrictions, and redemption instructions
- support for a narrow initial category set, likely fast food first

Why included:
- this is the core product experience
- supports learning around relevance, quality, and engagement

### 3. Source and Disclosure Labels
Must have

Description:
- every offer is labeled by source type, such as affiliate, partner, sponsored, or curated public offer
- copy makes clear when the product is not the merchant

Why included:
- reduces legal, trust, and support risk
- helps users understand the business model

### 4. Manual Verification Workflow
Must have

Description:
- internal checklist before publication
- verify source, expiry, key restrictions, and rights to distribute
- manual review and removal process for stale or disputed offers

Why included:
- high leverage for trust
- critical for risk control in MVP

### 5. Feedback Loop
Must have

Description:
- users can mark an offer as "worked" or "didn't work"
- internal team can review reports and pause weak sources

Why included:
- creates fast learning on offer quality
- helps manage freshness and trust

### 6. Basic Offer Analytics
Must have

Description:
- track joins, views, clicks, saves if supported, feedback rates, and invalid-offer reports
- measure engagement by category and source type

Why included:
- needed to decide whether the MVP is working
- enables monetization and roadmap decisions

### 7. Lightweight Admin Operations
Must have

Description:
- ability to add, edit, pause, and expire offers
- ability to log offer source and review status
- simple review queue for reported issues

Why included:
- supports manual-first operations without overbuilding

### 8. Support and Takedown Flow
Must have

Description:
- simple support contact path
- process for removing invalid or disputed offers quickly

Why included:
- protects user trust and reduces platform risk

## Features Explicitly Excluded from MVP
- Direct sale or resale of coupons to end users
- Checkout or payment-taking flow for ambiguous inventory
- Wallet balances, stored value, or internal credits
- User-to-user coupon marketplace
- User-uploaded coupon listings published without review
- Automated collection from protected merchant websites or apps
- Browser automation against checkout or account systems
- Merchant dashboard with self-serve campaign creation
- Multi-country expansion
- Broad inventory across many categories before fast-food and adjacent categories are validated

## Legal / Platform Risk Review
The MVP is shaped specifically to remove or defer the highest-risk elements identified in `docs/legal/risk_analysis.md`.

High-risk ideas removed from MVP:
- direct resale of coupons without explicit transferability rights
- accepting user payments for coupon inventory with uncertain redemption rights
- scraping or automation against merchant systems
- presenting the product as merchant-authorized when it is not

Risk controls included in MVP:
- manual verification before publishing
- explicit source and disclosure labels
- narrow category scope
- lightweight support and takedown process
- partner, affiliate, or manually curated public-offer sourcing only

Phase-gated for later consideration only:
- managed inventory
- resale margins
- direct payments
- deeper supplier integrations

## Safer Alternatives Considered

### Risky Idea: Direct Coupon Resale
Why risky:
- transferability may be prohibited
- refund and chargeback exposure is high
- supplier rights may be unclear

Safer replacement:
- discovery and click-out model using partner-approved, affiliate, or public verified offers

### Risky Idea: Automated Offer Extraction from Merchant Systems
Why risky:
- likely ToS and platform risk
- brittle and operationally fragile

Safer replacement:
- manual curation
- approved feeds
- direct partner submissions

### Risky Idea: Open Marketplace for User-Submitted Coupons
Why risky:
- high fraud exposure
- unclear rights and provenance
- heavy support burden

Safer replacement:
- curated intake only, with manual review and no public listing by default

### Risky Idea: Direct Consumer Payments in MVP
Why risky:
- adds payment, refund, and chargeback risk before product trust is proven

Safer replacement:
- monetization through affiliate, sponsorship, and partner distribution first

## Monetization Approach
The MVP monetization strategy should favor lower-risk, low-ops models:

1. Affiliate or referral revenue
Description:
- users click through to partner or merchant flows
- revenue is generated from tracked referrals where permitted

Why first:
- lowest operational burden
- no need to own redemption or payment flow

2. Sponsored placements from approved merchants or distributors
Description:
- merchants pay for featured placement or campaign visibility

Why second:
- aligned with the distribution model
- easier to operate than resale

3. Partner distribution fees or lead-generation fees
Description:
- approved partners pay for qualified traffic or campaign delivery

Why third:
- still lower risk than direct commerce
- creates a bridge toward stronger merchant relationships

Explicitly not in MVP monetization:
- resale margin on ambiguous coupon inventory
- payment-taking consumer checkout for unverified digital goods

## Success Metrics

### User Value Metrics
- percentage of new users who view at least one offer in their first session
- click-through rate from offer view to merchant destination
- repeat engagement within 7 days
- percentage of offers receiving positive "worked" feedback

### Trust Metrics
- invalid-offer report rate
- average time to remove or update a disputed offer
- support contacts per 100 active users
- share of published offers with complete source and expiry metadata

### Supply Metrics
- number of active compliant supply sources
- supplier concentration ratio
- percentage of offers published through approved channels
- offer freshness rate

### Monetization Metrics
- affiliate revenue per active user
- sponsor revenue per campaign
- conversion by source type
- share of revenue from compliant, repeatable channels

## MVP Success Criteria
The MVP should be considered successful if it demonstrates the following:

1. Users consistently engage with Telegram-delivered offers and a meaningful share return within 7 days.
2. Offer quality remains high enough that invalid-offer reports stay low and trust does not erode.
3. The team can operate the system manually without excessive support burden.
4. At least one lower-risk monetization path shows early revenue potential.
5. The product learns which categories, source types, and message formats drive the strongest engagement.

## Launch Plan

### Phase 0: Setup
- define sourcing checklist
- define publishing template
- define support and takedown procedure
- identify first compliant supply sources

### Phase 1: Closed Beta
- launch to a small group of users
- publish a limited set of verified offers
- collect engagement and quality feedback

### Phase 2: Narrow Expansion
- expand the strongest categories
- add more partner-approved sources
- refine onboarding and offer presentation based on performance

## Open Questions
- should the first launch be bot-only, channel-only, or hybrid
- which fast-food or adjacent categories have the cleanest compliant supply
- what minimum proof should an operator require before publishing a curated public offer
- which affiliate or partner models are most workable in Israel for the initial category set
- what exact support response time is necessary to preserve trust in a Telegram-native experience
