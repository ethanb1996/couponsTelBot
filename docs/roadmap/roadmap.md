# Roadmap

## Roadmap Principles
- Trust before scale
- Merchant clarity before growth
- Manual verification before automation
- Delivery reliability before expansion
- Narrow offer focus before catalog breadth

## Phase 0: Validation

### Goals
- Validate that users will buy merchant-partner offers in Telegram
- Validate that small businesses will provide offers and predefined codes
- Validate that a PayBox plus manual-claim flow can complete cleanly
- Validate that manual approval can stay fast enough for user trust
- Measure support and payment-mismatch risk early

### Features
- Telegram bot with a very small set of offers
- merchant partner setup
- offer detail page with disclosure, price, and redemption terms
- PayBox link flow
- manual payment claim submission
- admin approval and predefined code delivery
- manual support logging

### Risks
- users may not trust the manual approval step
- merchants may not provide clear enough offer terms
- approval latency may be too slow
- payment mismatch rate may be higher than expected
- a few bad fulfillments may damage trust early

### Why It Comes In This Phase
This phase exists to answer the core question with the smallest possible surface area: can the team publish merchant offers, take payment through PayBox, verify manually, and deliver codes without immediate trust collapse?

## Phase 1: MVP

### Goals
- Turn the first validated purchase loop into a repeatable product
- Formalize merchant, offer, order, claim, and delivery operations
- Improve offer clarity and conversion
- Keep approval and support load manageable
- Prove the operating model works for a small team

### Features
- Telegram bot purchase flow
- controlled offer catalog
- admin workflow for merchant and offer management
- payment-claim queue and approval actions
- order, claim, and delivery tracking
- support-case logging
- analytics for conversion, approval speed, delivery success, and complaint rate

### Risks
- manual operations may become brittle
- payment mismatch handling may create support pressure
- invalid or missing codes may erase trust quickly
- merchant concentration may remain high
- growth may outpace review capacity

### Why It Comes In This Phase
Once the team proves someone will buy, the next step is to make the manual commerce loop reliable and measurable without prematurely adding automation.

## Phase 2: Growth

### Goals
- expand only the offer types with good conversion and low support load
- improve repeat purchase rate
- reduce operational effort per approved order
- broaden merchant supply without degrading quality
- decide where automation adds value safely

### Features
- better offer optimization and pricing experiments
- richer admin review tooling
- approval reminders and queue health tooling
- repeat-buyer nudges and basic retention features
- broader but still controlled merchant catalog
- merchant performance tracking

### Risks
- growth pressure may reduce partner quality
- more offers may increase support complexity
- repeat purchase may stall if approval still feels slow
- delivery quality may drop as volume grows
- manual review may become the main bottleneck

### Why It Comes In This Phase
Growth should happen only after approval, delivery, and support handling are stable. Otherwise the business would only scale confusion.

## Phase 3: Defensibility

### Goals
- secure stronger merchant relationships
- build user trust and repeat purchase habits
- reduce dependency on a few partners
- determine where payment and approval automation is justified
- turn the narrow Telegram sales loop into a defensible operating channel

### Features
- preferred merchant relationships
- stronger brand and trust systems
- better dispute evidence and operational controls
- deeper category and unit economics analysis
- retention mechanisms for repeat buyers
- selective automation for payment review or partner operations where justified

### Risks
- exclusivity may not be durable
- partner quality may still fluctuate
- trust may remain fragile if complaint handling is weak
- automation may add complexity before it adds leverage

### Why It Comes In This Phase
Defensibility matters only after the team proves it can reliably publish, sell, verify, and deliver. Before that, scale and moat conversations are premature.

## Phase Progression Logic
The roadmap intentionally moves from lowest-risk learning to higher-complexity execution:

1. Phase 0 proves the core manual sale loop.
2. Phase 1 makes the merchant-offer MVP repeatable.
3. Phase 2 scales only the offer and approval loops that work.
4. Phase 3 improves trust, partner depth, and operating leverage.
