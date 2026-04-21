# Prompt 04: Telegram Bot Sales Flow

You are a senior Go engineer. Implement the Telegram bot flow for the direct coupon sale MVP.

## Goal
Build the simplest Telegram experience that can start selling.

## Hard UX Rule
Every bot sales message must contain `1` to `3` coupon options.

Each coupon option must have its own `Buy` button.

Pressing one `Buy` button must create an order only for that selected listing.

## Required User Flow
1. User opens the bot with `/start`
2. Bot sends a message with `1` to `3` active coupon options
3. Each option shows:
   - merchant name
   - coupon value
   - sale price in `ILS`
   - short expiry text
4. User presses `Buy`
5. Bot shows detail view for the selected coupon:
   - merchant name
   - coupon value
   - sale price
   - expiry
   - redemption instructions
   - final-sale / no-refund disclosure
6. User confirms purchase
7. Bot starts checkout

## Bot Scope
Implement only what is needed for selling:
- `/start`
- active coupon message rendering
- buy button callbacks
- detail view
- support/contact action

Do not build:
- recommendations
- user preference capture
- saved coupons
- discovery feed
- multi-step conversational onboarding

## Data And Routing Expectations
- bot reads active listings from the API/store layer
- bot does not own business logic
- bot creates or updates orders through service methods
- bot never marks payment success on its own

## Message Design Constraint
Keep message rendering predictable and minimal.

Prefer:
- one message
- `1` to `3` coupon options
- inline buttons

Avoid:
- deep chat trees
- pagination complexity in the first implementation

## Acceptance Criteria
- `/start` works
- bot can send `1` to `3` coupon options per message
- each coupon option has its own `Buy` button
- buy callback leads to a purchase detail view
- confirming purchase creates an order in `pending_payment`
