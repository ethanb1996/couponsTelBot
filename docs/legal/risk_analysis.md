# Risk Analysis

## Purpose
This document defines the main legal, platform, payment, and operational risks of the simplified MVP business model described in `PROJECT_CONTEXT.md`:

1. pre-buy coupon inventory
2. list it in Telegram
3. take payment in ILS
4. deliver the coupon to the user
5. operate with an explicit all-sales-final, no-refund policy

This is a product and operational risk analysis, not legal advice. Because the MVP now includes direct resale and a no-refund policy, legal review is materially more important than in the earlier discovery-first version.

## Current Risk Posture
This MVP is simpler operationally, but riskier commercially and legally than a discovery-only model. The highest concentrations are:
- transferability and resale rights
- payment disputes and chargebacks
- user trust damage from invalid delivered coupons
- the enforceability and practical impact of a no-refund policy

## Risk Register

| Risk Area | Classification | Why It Matters | Mitigation Strategy |
| --- | --- | --- | --- |
| Telegram platform enforcement | Medium | Complaint-heavy commerce bots can face moderation or restrictions. | Keep flows clear and non-deceptive, publish support contact information, log complaints, and avoid spam-like distribution tactics. |
| Coupon transferability risk | High | A pre-bought coupon may still be non-transferable or restricted to the original buyer. | Only sell inventory with verified transferability or acceptable resale rights. Maintain source evidence per coupon batch. |
| Unauthorized resale risk | High | Even if a coupon technically works, resale may violate merchant or supplier terms. | Require documented source provenance and rights review before listing. Avoid ambiguous inventory. |
| Payment acceptance risk | High | Direct payments create fraud, dispute, reconciliation, and support obligations immediately. | Use a provider that accepts ILS and has dispute tooling. Keep strong order, payment, and delivery logs. |
| No-refund policy risk | High | Stating that no refunds are possible may increase legal exposure, chargebacks, and trust damage if something goes wrong. | Show final-sale terms clearly before payment, get legal review, and still maintain manual exception handling internally. |
| Chargeback risk | High | Users may dispute charges even if policy says no refunds. | Store payment evidence, final-sale acknowledgment, coupon assignment records, and delivery proof. Monitor dispute rates aggressively. |
| Invalid or already-used inventory | High | The business can lose money and trust if a delivered coupon fails. | Pre-verify inventory, keep inventory small, quarantine suspicious sources, and investigate complaints quickly. |
| Overselling inventory | Medium | Selling a coupon that is no longer actually available creates immediate customer harm. | Track coupon status explicitly and assign only after successful payment under transactional controls. |
| Supplier dependency | High | If inventory comes from too few sources, supply quality and margin become fragile. | Start with more than one trusted source where possible and track defect rate by source. |
| User trust collapse | High | Even a few bad purchases can make the bot look unsafe, especially with a no-refund policy. | Keep launch narrow, disclose clearly, verify inventory carefully, and log every failure. |
| Fraudulent buyers or stolen cards | Medium | Card fraud can trigger payment disputes and provider pressure. | Use provider risk controls, review unusual transactions, and keep low transaction amounts initially. |
| Data privacy and support handling | Medium | Payment, Telegram, and complaint data require careful handling. | Minimize stored personal data and keep payment details provider-owned. |

## Detailed Risk Notes

### 1. Platform / ToS Risk
Classification: Medium

Key concerns:
- Telegram complaints from unhappy buyers
- spam-like acquisition tactics
- misleading representation of merchant affiliation

Mitigations:
- state clearly that the bot is the seller unless a formal partnership exists
- provide support contact path
- keep product copy factual and specific

### 2. Coupon Transferability and Resale Rights
Classification: High

Key concerns:
- coupon may be tied to the original buyer
- merchant terms may prohibit resale
- supplier may not actually have the right to resell

Mitigations:
- only buy inventory from sources with acceptable rights clarity
- reject inventory with ambiguous resale status
- store provenance and verification notes per source or batch

### 3. Payment / Refund / Chargeback Risk
Classification: High

Key concerns:
- no-refund policy may not stop payment disputes
- users may claim invalid delivery or non-usable coupons
- processor may consider the business risky if complaint rates rise

Mitigations:
- show final-sale disclosure before payment
- store timestamped acceptance of no-refund terms
- store payment success, coupon assignment, and delivery evidence
- keep initial order values low while learning dispute behavior

### 4. Fraud / Abuse Risk
Classification: High

Key concerns:
- invalid supplier inventory
- duplicate sale of the same code
- stolen-card purchases

Mitigations:
- secure coupon storage
- single-assignment inventory model
- trusted-source-first launch
- provider fraud controls and manual review for suspicious payments

### 5. Supplier Dependency Risk
Classification: High

Key concerns:
- too much dependency on one source
- sudden quality drop in inventory
- rights assumptions change after launch

Mitigations:
- source diversification
- defect-rate monitoring
- immediate pause capability for listings tied to weak sources

### 6. User Trust Risk
Classification: High

Key concerns:
- failed coupon after purchase
- unclear terms before payment
- users perceive the no-refund policy as unfair or unsafe

Mitigations:
- present price, value, expiry, and final-sale terms clearly
- keep delivery immediate
- investigate all complaints even if refunds are not part of standard policy

## Mitigation Priorities for MVP
The highest-value mitigation priorities are:

1. Verify transferability and resale rights before inventory is listed.
2. Never sell inventory that is not already held.
3. Deliver coupons only after provider-confirmed payment success.
4. Show final-sale and no-refund terms clearly before payment.
5. Store strong evidence for payment, assignment, and delivery.
6. Start with low SKU count and low inventory volume.
7. Track defect rate, complaint rate, and chargeback rate by source and coupon type.

## Safer Alternative Business Models

### 1. Discovery and Click-Out
Risk level versus current MVP: Lower

Why safer:
- no direct payment risk
- no inventory holding
- lower refund and chargeback exposure

### 2. Partner Distribution Without Resale
Risk level versus current MVP: Lower

Why safer:
- fewer resale-rights issues
- lower direct inventory risk

### 3. Closed Beta With Very Small Inventory
Risk level versus current MVP: Lower than broad launch

Why safer:
- limits downside while still testing the chosen commerce model

## Recommended Launch Posture
If this MVP proceeds, the safest version of this direct-sale model is:

1. small number of SKUs
2. small number of verified suppliers
3. low transaction values
4. explicit final-sale disclosure before payment
5. strong order, delivery, and complaint logging

## Phase-Gate Before Scaling
Do not scale this resale model beyond a tightly controlled MVP unless all of the following are true:
- transferability issues are understood for the active coupon types
- dispute and chargeback rates are acceptable
- inventory defect rate is acceptable
- legal review of the no-refund posture has been completed
- payment provider relationship remains healthy
