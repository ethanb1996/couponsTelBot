# Roadmap

## Roadmap Principles
- Trust before scale
- Compliance before automation
- Inventory control before catalog breadth
- Payment and delivery reliability before growth
- Narrow SKU focus before inventory expansion

## Phase 0: Validation

### Goals
- Validate that users will buy pre-bought coupons in Telegram
- Validate that at least one coupon type can be safely sourced and resold
- Validate that an ILS-capable payment flow can complete cleanly
- Validate that coupon delivery after payment can be reliable
- Measure complaint and dispute risk early

### Features
- Telegram bot with a very small set of listings
- manual inventory intake for pre-bought coupons
- listing detail page with value, price, expiry, and no-refund disclosure
- basic ILS payment flow
- coupon delivery after payment
- manual support logging

### Risks
- users may not trust a direct Telegram purchase flow
- coupon rights or transferability may be weaker than expected
- payment completion may be poor
- a few invalid coupons may damage trust early
- no-refund policy may drive early complaints or disputes

### Why It Comes in This Phase
This phase exists to answer the core direct-commerce questions with the smallest possible surface area: can the team hold inventory, sell it, get paid in ILS, and deliver coupons without immediate trust collapse.

## Phase 1: MVP

### Goals
- Turn the first validated purchase loop into a repeatable product
- Formalize inventory, order, payment, and delivery operations
- Improve listing clarity and conversion
- Keep chargebacks, complaints, and invalid inventory at manageable levels
- Prove basic unit economics

### Features
- Telegram bot purchase flow
- controlled inventory catalog
- admin workflow for coupon intake and listing management
- ILS payment provider integration
- order, payment, and delivery tracking
- explicit final-sale / no-refund acknowledgment before payment
- support-case logging
- analytics for conversion, delivery success, complaint rate, and chargebacks

### Risks
- invalid inventory may erase margins
- no-refund policy may increase dispute pressure
- manual operations may become brittle
- supplier concentration may remain high
- payment-provider tolerance may be lower than expected if complaints rise

### Why It Comes in This Phase
Once the team proves someone will buy, the next step is to make the sale flow reliable and measurable. This phase operationalizes the actual business model rather than broadening scope.

## Phase 2: Growth

### Goals
- expand only the coupon types with good margins and acceptable complaint rates
- improve conversion and repeat purchase rate
- reduce operational effort per order
- broaden inventory supply without degrading quality
- protect payment-provider health while growing volume

### Features
- better listing optimization and pricing experiments
- improved inventory management tooling
- stronger payment reconciliation and support tooling
- repeat-buyer nudges and basic retention features
- broader but still controlled inventory categories
- supplier performance tracking and source diversification

### Risks
- growth pressure may lower inventory quality
- more SKUs may increase stale or invalid stock
- repeat purchase may stall if trust is weak
- chargeback rates may rise with volume
- supplier quality may become harder to control

### Why It Comes in This Phase
Growth comes only after payment, delivery, and complaint handling are stable. Otherwise the business would only scale failure.

## Phase 3: Defensibility

### Goals
- secure stronger inventory access and better margins
- build user trust and repeat purchase habits
- reduce source dependency
- determine whether the no-refund posture remains viable at scale
- turn the narrow Telegram sales loop into a defensible business

### Features
- preferred supplier relationships
- stronger brand and trust systems
- better dispute evidence and operational controls
- deeper category and unit economics analysis
- retention mechanisms for repeat buyers
- legal and policy review of whether refund or exception handling needs to change over time

### Risks
- exclusivity may not be durable
- deeper supplier relationships may still rely on weak resale rights
- trust may remain fragile if complaint handling is poor
- the no-refund model may become a growth constraint even if it is simple operationally

### Why It Comes in This Phase
Defensibility matters only after the team proves it can reliably buy, sell, get paid, and deliver. Before that, scale and moat conversations are premature.

## Phase Progression Logic
The roadmap intentionally moves from lowest-risk learning to higher-complexity execution:

1. Phase 0 proves the core sale loop.
2. Phase 1 makes the direct-sale MVP repeatable.
3. Phase 2 scales only the inventory and payment loops that work.
4. Phase 3 improves margin, trust, and supply defensibility.
