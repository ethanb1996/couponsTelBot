# Decisions Log

## Template
- Date:
- Decision:
- Context:
- Reason:
- Consequences:
- Owner:

## Seed decisions

### 2026-04-15 - Legal-risk-aware supply strategy
- Decision: Do not design the MVP around captcha bypassing, anti-bot evasion, or restricted automation.
- Context: Supplier-side protections indicate elevated legal and operational risk.
- Reason: Reduces compliance exposure and platform dependency risk.
- Consequences: Favors manual, partner, or compliant supply alternatives for MVP.
- Owner: PRD + Architect

### 2026-04-15 - Agent workflow
- Decision: Use separate PRD, Architect, and Orchestrator agents with clear responsibilities.
- Context: Product and technical decisions need to remain explicit and traceable.
- Reason: Improves quality, speed, and consistency.
- Consequences: All major decisions should map to docs and implementation plans.
- Owner: Repo owner

### 2026-05-21 - Direct merchant manual PayBox MVP
- Decision: Reframe the MVP around direct merchant-partner offers, manual PayBox verification, and predefined code delivery.
- Context: The previous resale-oriented model created legal and product risk and no longer matches the intended go-to-market motion.
- Reason: This is the fastest compliant-enough validation path for selling predefined offers from small businesses directly.
- Consequences: Architecture, data model, bot flow, and admin workflows should center on manual payment claims and operator approval rather than automated provider webhooks.
- Owner: Repo owner + Architect
