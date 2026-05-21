# Data Model

## Model Scope
This document defines the target data model for the simplified manual PayBox MVP.

The MVP-active entities are:
- users
- merchant_partners
- predefined_codes
- offers
- orders
- manual_payment_claims
- coupon_deliveries
- support_cases
- admin_actions

Implementation note:
- current code and schema still use `coupon_sources`, `listings`, and `coupons`
- the target product model should treat these as temporary implementation primitives that will be migrated toward `merchant_partners`, `offers`, and `predefined_codes`

## Modeling Principles
- represent direct merchant authorization clearly
- separate the Telegram-facing offer from the code pool
- separate order state from payment-claim review state
- separate payment approval from code delivery
- preserve auditability for offer, claim, fulfillment, and support actions

## 1. users
Represents a Telegram user known to the system.

Suggested fields:
- `id`
- `telegram_user_id`
- `telegram_username`
- `display_name`
- `language_code`
- `status` (`active`, `blocked`, `deleted`)
- `first_seen_at`
- `last_seen_at`
- `created_at`
- `updated_at`

## 2. merchant_partners
Represents a small business that authorizes offers in the bot.

Suggested fields:
- `id`
- `business_name`
- `contact_reference`
- `status` (`lead`, `active`, `paused`, `inactive`)
- `approval_notes`
- `merchant_disclosure_text`
- `support_contact`
- `default_payment_link`
- `created_at`
- `updated_at`

## 3. predefined_codes
Represents one unique code that may be delivered after manual payment approval.

Purpose:
- track owned or merchant-issued fulfillment units
- assign exactly one code to at most one order

Suggested fields:
- `id`
- `offer_id`
- `merchant_partner_id`
- `code_encrypted`
- `code_masked_display`
- `status` (`available`, `assigned`, `sent`, `voided`, `expired`)
- `expiry_at`
- `issued_batch_reference`
- `created_at`
- `updated_at`

## 4. offers
Represents the Telegram-facing product.

Purpose:
- describe the fixed merchant deal shown to the buyer

Suggested fields:
- `id`
- `merchant_partner_id`
- `merchant_name`
- `title`
- `description`
- `price_amount`
- `currency_code`
- `payment_link`
- `merchant_disclosure_text`
- `redemption_terms`
- `support_contact`
- `status` (`draft`, `active`, `paused`, `sold_out`, `expired`, `removed`)
- `published_at`
- `created_at`
- `updated_at`

Notes:
- `merchant_disclosure_text` may be inherited from the partner by default but should be snapshotted on the offer if per-offer wording is needed

## 5. orders
Represents a user purchase attempt.

Suggested fields:
- `id`
- `user_id`
- `offer_id`
- `predefined_code_id`
- `order_number`
- `status` (`draft`, `awaiting_payment`, `payment_claim_submitted`, `payment_verified`, `coupon_sent`, `payment_rejected`, `cancelled`, `support_required`)
- `currency_code`
- `price_amount`
- `paybox_payment_link`
- `placed_at`
- `verified_at`
- `delivered_at`
- `failure_reason`
- `created_at`
- `updated_at`

Notes:
- `predefined_code_id` stays null until manual approval succeeds

## 6. manual_payment_claims
Represents a buyer-submitted payment claim.

Suggested fields:
- `id`
- `order_id`
- `payer_username`
- `claimed_amount`
- `submitted_at`
- `review_status` (`pending_review`, `verified`, `rejected`)
- `reviewed_by`
- `reviewed_at`
- `review_note`
- `created_at`
- `updated_at`

## 7. coupon_deliveries
Represents the act of sending a predefined code to the user after approval.

Purpose:
- separate payment verification from delivery success
- provide evidence in support disputes

Suggested fields:
- `id`
- `order_id`
- `predefined_code_id`
- `delivery_channel` (`telegram_bot`)
- `status` (`pending`, `sent`, `confirmed`, `failed`)
- `telegram_message_id`
- `delivery_payload_hash`
- `sent_at`
- `confirmed_at`
- `failure_reason`
- `created_at`
- `updated_at`

## 8. support_cases
Represents internal support handling.

Suggested fields:
- `id`
- `user_id`
- `order_id`
- `predefined_code_id`
- `payment_claim_id`
- `case_type` (`payment_mismatch`, `invalid_code`, `delivery_issue`, `merchant_issue`, `other`)
- `status` (`open`, `in_progress`, `waiting_on_user`, `resolved`, `closed`)
- `priority` (`low`, `medium`, `high`)
- `summary`
- `resolution_note`
- `assigned_admin_id`
- `created_at`
- `updated_at`

## 9. admin_actions
Captures sensitive internal actions for auditability.

Suggested fields:
- `id`
- `admin_user_id`
- `entity_type` (`merchant_partner`, `offer`, `predefined_code`, `order`, `manual_payment_claim`, `support_case`)
- `entity_id`
- `action_type`
- `before_state_json`
- `after_state_json`
- `reason_text`
- `created_at`

## Relationship Summary
- `merchant_partners` 1-to-many `offers`
- `merchant_partners` 1-to-many `predefined_codes`
- `users` 1-to-many `orders`
- `offers` 1-to-many `orders`
- `orders` optionally belongs-to `predefined_codes`
- `manual_payment_claims` belongs-to `orders`
- `coupon_deliveries` belongs-to `orders`
- `coupon_deliveries` belongs-to `predefined_codes`
- `support_cases` may reference `orders`, `predefined_codes`, and `manual_payment_claims`

## Recommended MVP Tables
- users
- merchant_partners
- offers
- predefined_codes
- orders
- manual_payment_claims
- coupon_deliveries
- support_cases
- admin_actions

## Key Design Decisions
- `merchant_partners` represent the business relationship.
- `offers` represent sellable Telegram products.
- `predefined_codes` represent individual fulfillment units.
- `orders` represent purchase attempts.
- `manual_payment_claims` represent buyer-submitted proof data awaiting review.
- `coupon_deliveries` represent actual release of the code.
