# Risk Analysis

## Purpose
This document defines the main legal, platform, and operational risks of the current business model described in `PROJECT_CONTEXT.md`: a Telegram-based coupon product for Israeli users that begins with curated coupon discovery and may later expand into compliant resale or facilitation models.

This is a product and operational risk analysis, not legal advice. Any move into direct resale, managed inventory, or merchant-funded distribution should be reviewed by qualified Israeli counsel before launch.

## Current Risk Posture
The safest version of this business is a compliance-first coupon discovery and partner distribution product. Risk increases materially when the product:
- resells coupons without clear transfer rights
- accepts consumer payments before redemption rights are verified
- automates against third-party websites or merchant systems
- implies merchant authorization where none exists
- scales supply through unclear or fragile sources

For MVP, the business should assume that discovery, partner distribution, and compliant referral flows are lower-risk than direct resale.

## Risk Register

| Risk Area | Classification | Why It Matters | Mitigation Strategy |
| --- | --- | --- | --- |
| Telegram platform enforcement | Medium | Telegram may restrict bots, channels, payments, or promotional behavior if the product appears deceptive, spammy, infringing, or complaint-heavy. | Keep messaging opt-in, avoid spam growth tactics, publish clear support and contact details, maintain moderation logs, remove disputed offers quickly, and avoid misleading claims about merchant affiliation. |
| Third-party merchant ToS violations | High | Sourcing offers from merchant sites, apps, or protected flows may violate terms if done through restricted automation, scraping, or resale outside allowed use. | Do not use captcha bypassing, anti-bot evasion, or restricted automation. Prefer direct merchant agreements, affiliate programs, reseller rights, licensed inventory, or manual verification of publicly available offers. |
| Misrepresentation of affiliation | High | Users or merchants may believe the bot is officially connected to a brand when it is not, creating legal and reputational exposure. | Clearly label the product as an independent service unless a formal partnership exists. Add source labels, sponsorship disclosures, and merchant-authorization status to offer records and user-facing copy. |
| Coupon transferability restrictions | High | Many coupons, vouchers, promo codes, and gift-like instruments are non-transferable, single-account only, or limited to the original recipient. Resale can therefore fail at redemption or breach terms. | Treat transferability as a gating check before listing any resale offer. Require written supplier confirmation or explicit published terms allowing transfer/resale. Exclude any coupon class with ambiguous ownership or redemption rights. |
| Unauthorized resale of promotional instruments | High | Even if a code technically works, resale may be prohibited by merchant terms or local commercial rules, exposing the business to takedowns, refunds, chargebacks, and disputes. | Keep MVP focused on discovery, referral, or authorized partner distribution. Only enable resale for inventory with explicit contractual resale rights and a documented audit trail. |
| Consumer protection and disclosure risk | Medium | If users pay money or rely on an offer description that is incomplete, expired, or misleading, the business may face complaints, refund pressure, and regulatory scrutiny. | Show expiration windows, material restrictions, redemption steps, refund rules, merchant identity, and sponsorship/affiliate disclosures clearly before user action. Keep a correction and takedown workflow. |
| Payment acceptance risk | High | Once the product takes payment directly, it assumes additional obligations around failed delivery, duplicate payment, fraud screening, reconciliation, and customer support. | Delay direct payments in MVP where possible. Prefer referral, lead-gen, or partner-paid models first. If payments are later enabled, use a reputable processor with fraud tooling, hold-state order tracking, and explicit refund flows. |
| Refund and chargeback risk | High | Users may request refunds when codes fail, offers expire, merchants reject redemption, or value is unclear. Chargebacks can quickly erase margins. | Publish a narrow refund policy, store evidence of offer state at time of sale, verify inventory before sale, cap order values, and review disputes manually. Avoid selling ambiguous or user-submitted inventory without verification. |
| Fraudulent suppliers or fake inventory | High | Suppliers may provide invalid, already-used, stolen, or fabricated coupon inventory. This creates direct financial and trust losses. | Approve suppliers manually, require proof of source, start with low-volume trusted sources, track supplier defect rate, and suspend any source with failed-redemption patterns. |
| User abuse and arbitrage | Medium | Users may exploit introductory offers, repeatedly redeem the same code, collude on refunds, or resell within the platform in ways that increase support cost. | Rate-limit high-risk actions, track device/account history, enforce per-user limits where lawful, flag repeated refund behavior, and keep manual review for suspicious patterns. |
| Stolen payment instruments or account takeover | Medium | Fraudsters may use stolen cards or compromised accounts, leading to processor disputes and merchant complaints. | Use payment providers with strong risk controls, step-up verification for unusual orders, limit transaction size in MVP, and log device/account events for review. |
| Supplier dependency concentration | High | If most offers come from one merchant, one affiliate program, or one informal reseller, the business becomes fragile and margins are exposed. | Diversify supply early across multiple compliant sources, track supplier concentration, and avoid building the MVP around a single uncontracted channel. |
| Fragile manual operations | Medium | A manual sourcing and verification model is acceptable for MVP, but can fail if volume grows before tools and workflows mature. | Keep launch narrow, limit categories, define daily freshness checks, use simple admin tooling, and measure operational load before expanding inventory breadth. |
| Offer freshness and validity failure | High | Expired or already-used coupons directly undermine the product's core promise and quickly damage retention. | Require expiration metadata, verify inventory freshness before publishing, expire offers automatically when uncertain, and add a user feedback loop for "worked / didn't work." |
| User trust and brand damage | High | A few bad coupon experiences can make users view the bot as spammy, fake, or unsafe. Trust is the core product asset. | Prefer fewer verified offers over more risky ones, show transparent sourcing/disclosure info, resolve complaints quickly, and remove low-confidence offers fast. |
| Data privacy and user communication risk | Medium | Telegram usernames, phone-linked identities, payment details, and support interactions may create privacy and security obligations. | Collect minimal personal data, separate payment data from bot systems when possible, restrict admin access, and document retention and support-handling practices. |
| Regulatory ambiguity around reseller/facilitation model | Medium | A later move from discovery into managed resale may change the business from media/referral into commerce, with different legal and operational expectations. | Treat resale as a separate phase-gate. Do legal review before launch, define merchant/supplier contracts, and update terms, disclosures, and support operations before accepting consumer payments. |

## Detailed Risk Notes

### Platform / ToS Risks

#### 1. Telegram Platform Risk
Classification: Medium

Key concerns:
- complaint-driven moderation or bot restrictions
- spam-like distribution tactics
- unclear disclosures in sponsored or affiliate content
- fraud reports from users or merchants

Mitigations:
- use permission-based acquisition and opt-in messaging
- provide clear contact, support, and complaint channels
- keep offer content factual and non-deceptive
- log moderation, removals, and merchant complaints

#### 2. Merchant Website / App ToS Risk
Classification: High

Key concerns:
- scraping protected pages
- using automation against checkout, coupon, or account systems
- redistributing offers beyond allowed channels
- using merchant assets or branding in misleading ways

Mitigations:
- use manual curation or approved data feeds
- only use affiliate/referral or partner-based supply where terms allow it
- prohibit engineering shortcuts like captcha bypassing, anti-bot evasion, and account farming
- document source and rights for every offer type

### Coupon Transferability Risks

#### 3. Non-Transferable Coupon Structures
Classification: High

Key concerns:
- coupon tied to named user, account, phone number, or payment method
- redemption allowed only in original channel or original app account
- promotional code intended for one-time personal use only

Mitigations:
- define a transferability checklist before any resale listing
- require explicit rights confirmation from supplier or merchant terms
- exclude account-bound, invitation-only, or personal-use-only coupons from resale

#### 4. Ambiguous Ownership of Codes or Vouchers
Classification: High

Key concerns:
- supplier cannot prove lawful ownership
- code was obtained through promotions not meant for resale
- end-user is asked to trust unverifiable screenshots or messages

Mitigations:
- require source provenance for all inventory
- keep digital audit records for acquisition and sale
- start with partner-issued or licensed inventory rather than consumer-sourced resale

### Payment / Refund Risks

#### 5. Failed Redemption After Payment
Classification: High

Key concerns:
- user pays but merchant refuses redemption
- code already used or expired
- redemption terms were not fully disclosed

Mitigations:
- avoid direct payment in earliest MVP where possible
- if payment is accepted, verify inventory just before sale
- show full conditions pre-purchase
- define refund triggers and handling times clearly

#### 6. Chargebacks and Processor Friction
Classification: High

Key concerns:
- digital goods are often dispute-prone
- unclear product descriptions increase chargebacks
- processor may classify the business as high-risk if complaint volume rises

Mitigations:
- use strong order logs and delivery evidence
- keep item descriptions specific
- maintain responsive dispute handling
- start with low average order values and conservative risk thresholds

### Fraud / Abuse Risks

#### 7. Fake Supply, Reused Codes, or Stolen Inventory
Classification: High

Key concerns:
- seller provides unusable inventory
- same code sold multiple times
- coupons sourced through compromised accounts

Mitigations:
- only approve known suppliers initially
- test redemption patterns continuously
- create a defect-rate threshold that auto-pauses suppliers
- keep reserve funds for customer remediation

#### 8. User-Side Abuse
Classification: Medium

Key concerns:
- repeated refund claims
- abuse of introductory offers
- coordinated attacks to obtain value or discredit the service

Mitigations:
- rate limits, account history, and anomaly detection
- manual review for suspicious refund patterns
- limited introductory promotions and controlled experimentation

### Supplier Dependency Risks

#### 9. Single-Source Dependency
Classification: High

Key concerns:
- one merchant, one affiliate network, or one reseller drives most supply
- the business becomes vulnerable to policy or pricing changes

Mitigations:
- track supply concentration explicitly
- diversify categories and suppliers early
- prefer repeatable, contract-backed channels over informal one-off supply

#### 10. Informal Supplier Relationships
Classification: Medium

Key concerns:
- no clear SLA, no dispute process, no replacement inventory
- supplier behavior changes with no notice

Mitigations:
- use written commercial terms even for small pilots
- define replacement, cancellation, and defect obligations
- avoid scaling any source before repeatability is proven

### User Trust Risks

#### 11. Trust Collapse from Low-Quality Offers
Classification: High

Key concerns:
- a small number of failed offers can reduce repeat usage sharply
- Telegram users may quickly label the bot as spam or scam

Mitigations:
- bias heavily toward verified inventory
- display expiration, source type, and redemption method clearly
- resolve failures quickly and transparently
- use user feedback to suppress low-confidence sources

#### 12. Opaque Business Model
Classification: Medium

Key concerns:
- users may not understand whether the product is an affiliate, reseller, publisher, or merchant
- ambiguity increases complaints and suspicion

Mitigations:
- state the business role clearly in bot copy, FAQ, and checkout
- label sponsored, affiliate, partner, and direct-sale offers differently
- keep disclosures simple and consistent

## Mitigation Priorities for MVP
The highest-value mitigation priorities are:

1. Do not launch direct resale of coupons unless transferability and resale rights are explicit.
2. Do not rely on scraping, captcha bypassing, or restricted automation for supply.
3. Prefer discovery, referral, affiliate, and partner-distribution flows over payment-taking resale.
4. Build a verification workflow for freshness, rights, and redemption clarity before publishing any offer.
5. Keep launch narrow enough that every offer can be reviewed manually.
6. Add clear user disclosures for source type, merchant affiliation status, expiration, and refund handling.
7. Track supplier quality and remove weak sources quickly.

## Safer Alternative Business Models

### 1. Coupon Discovery and Alert Bot
Risk level versus direct resale: Lower

Model:
- The bot curates public or partner-approved offers.
- Users click out to redeem directly with the merchant or partner.
- Revenue comes from affiliate commissions, sponsorships, or lead generation.

Why safer:
- avoids direct handling of coupon ownership disputes
- reduces payment and refund burden
- fits MVP-first validation well

### 2. Partner Distribution Bot
Risk level versus direct resale: Lower

Model:
- The business distributes offers only from merchants, affiliate networks, or approved resellers with explicit permission.
- Offers are tagged as partner-supplied.

Why safer:
- clearer rights chain
- better merchant trust
- easier to defend operationally and legally

### 3. Concierge Deal Service
Risk level versus direct resale: Lower to Medium

Model:
- The team manually sources and verifies a small set of offers, then publishes only those that pass review.
- High-touch operations are used deliberately to learn what users actually want.

Why safer:
- reduces scale pressure
- improves trust and freshness
- surfaces transferability and support issues early

### 4. Merchant-Funded Promotions Channel
Risk level versus direct resale: Lower

Model:
- Merchants pay for placement or distribution of approved offers.
- The product acts as a distribution and engagement layer rather than a reseller.

Why safer:
- avoids unclear coupon ownership
- aligns incentives with merchants
- simpler payment and refund structure

### 5. Closed Beta with Approved Supply Only
Risk level versus direct resale: Lower

Model:
- Launch with a very small set of verified supply partners and a limited user cohort.
- Validate trust, redemption quality, and support burden before expanding.

Why safer:
- contains downside
- makes compliance monitoring manageable
- preserves brand trust during early learning

## Recommended Business Model Sequence
Recommended order of operations:

1. Start with coupon discovery, alerts, and partner-approved distribution.
2. Validate demand, engagement, and trust without taking payment for ambiguous inventory.
3. Add monetization through affiliate, sponsorship, or lead-generation models.
4. Only explore managed inventory or resale after legal review, explicit supplier rights, and operational controls are in place.

## Phase-Gate for Any Resale Model
Do not enable direct coupon resale unless all of the following are true:
- transferability is explicitly allowed
- supplier rights are documented
- refund policy is operationally feasible
- payment processor risk is understood
- customer support capacity exists
- merchant affiliation and disclosure language are approved
- fraud monitoring is in place
- legal review for Israel has been completed
