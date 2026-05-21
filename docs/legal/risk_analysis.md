# Risk Analysis

## Purpose
This document defines the main legal, payment, operational, and trust risks of the manual PayBox MVP described in `PROJECT_CONTEXT.md`:

1. publish merchant-authorized offers
2. accept payment through PayBox
3. collect buyer payment claims in Telegram
4. verify payments manually
5. deliver predefined codes after approval

This is a product and operational risk analysis, not legal advice.

## Current Risk Posture
This MVP is simpler than a full automation-heavy commerce flow, but it still has meaningful risk concentrations:
- merchant authorization and disclosure
- payment-claim fraud and mismatch handling
- support load caused by manual review
- trust damage from invalid or delayed fulfillment

## Risk Register

| Risk Area | Classification | Why It Matters | Mitigation Strategy |
| --- | --- | --- | --- |
| Merchant authorization risk | High | Publishing an offer without clear approval can create legal and trust problems immediately. | Tie every offer to a merchant partner record, keep approval notes, and require disclosure text before publish. |
| Misleading representation risk | High | Users may think the merchant is directly operating the bot if the relationship is unclear. | Show clear merchant disclosure and support contact in the offer detail view. |
| Payment-claim fraud | High | A buyer can claim to have paid without matching payment evidence. | Require PayBox username submission, keep claims pending until manual review, and log all approvals and rejections. |
| Manual review delay | Medium | Slow approval harms trust even if payment was made correctly. | Keep launch volume low, send Telegram admin alerts, and monitor approval latency. |
| Invalid or missing predefined codes | High | Approved orders without deliverable codes create immediate support problems. | Require code availability before publish, move broken approvals to `support_required`, and keep pause controls. |
| Offer drift risk | Medium | Merchant terms may change after the offer is published. | Keep offers narrow, maintain partner communication, and pause offers quickly when terms change. |
| Telegram platform enforcement | Medium | Complaint-heavy commerce bots can face restrictions. | Keep messaging clear, support accessible, and avoid spam-like acquisition tactics. |
| User trust collapse | High | A few bad redemptions or slow approvals can make the product feel unsafe. | Launch with few offers, clear instructions, fast manual review, and strong support logging. |
| Data handling risk | Medium | Telegram and payment-claim data must be handled carefully. | Minimize stored personal data and keep only the fields needed for review and audit. |
| Merchant dependency risk | Medium | Too much volume from one merchant makes quality and supply fragile. | Start with a few merchants and track issue rate by partner. |

## Detailed Risk Notes

### 1. Merchant Authorization And Disclosure
Classification: High

Key concerns:
- no documented merchant approval
- unclear rights to market the offer
- users do not understand the merchant relationship

Mitigations:
- merchant partner records
- explicit approval notes
- required disclosure text before publish

### 2. Payment Claim Fraud And Mismatch
Classification: High

Key concerns:
- buyer enters the wrong PayBox username
- claim cannot be matched to a payment
- admin approves the wrong claim

Mitigations:
- structured payment-claim submission
- manual review queue
- approval and rejection audit trail
- retry and support path for mismatches

### 3. Manual Operations Risk
Classification: Medium

Key concerns:
- admin review bottlenecks
- delayed approvals
- inconsistent support responses

Mitigations:
- low launch volume
- narrow offer catalog
- simple Telegram-first review flow
- measure approval latency and support backlog

### 4. Fulfillment Reliability Risk
Classification: High

Key concerns:
- code pool runs out
- wrong code is sent
- delivery fails after approval

Mitigations:
- code availability checks before publish
- explicit assignment and delivery state
- manual retry path
- support case creation for broken approvals

## Mitigation Priorities For MVP
The highest-value mitigation priorities are:

1. Require merchant authorization and disclosure before publish.
2. Keep payment verification manual until the team understands the failure modes.
3. Never assign or send a predefined code before approval.
4. Track approval latency, rejection rate, and invalid code rate from day one.
5. Launch with a very small number of offers and merchants.

## Recommended Launch Posture
The safest first launch posture for this model is:

1. a small number of merchants
2. a small number of active offers
3. manual Telegram admin approval
4. predefined code pools that are checked before publish
5. clear support contact on every offer

## Phase Gate Before Scaling
Do not scale this model broadly until all of the following are true:
- merchant approval and disclosure practices are stable
- payment mismatch rate is acceptable
- average approval time is acceptable
- invalid code rate is acceptable
- support volume is manageable for the team
