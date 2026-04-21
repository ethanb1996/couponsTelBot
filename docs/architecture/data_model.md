# Data Model

## Model Scope
This document defines the core data model for the simplified MVP. In this version, commerce is not deferred. It is the core product.

The MVP-active entities are:
- users
- coupon_sources
- coupons
- listings
- orders
- payments
- coupon_deliveries
- support_cases
- admin_actions

Refunds are intentionally not part of the normal product flow because the stated MVP policy is that all sales are final and no refunds are available. Even so, support and dispute logging must still exist.

## Modeling Principles
- Hold inventory before sale.
- Separate coupon inventory from Telegram-facing listings.
- Separate order state from payment state.
- Separate payment success from coupon delivery success.
- Preserve auditability for every inventory, payment, and support action.

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

## 2. coupon_sources
Represents the origin of pre-bought inventory.

Suggested fields:
- `id`
- `source_name`
- `source_type` (`merchant_partner`, `reseller`, `licensed_distributor`, `manual_source`)
- `contact_reference`
- `rights_status` (`unknown`, `review_pending`, `approved`, `restricted`, `rejected`)
- `verification_notes`
- `risk_rating` (`low`, `medium`, `high`)
- `is_active`
- `created_at`
- `updated_at`

## 3. coupons
Represents one unit of pre-bought coupon inventory.

Purpose:
- track owned inventory
- assign exactly one coupon to at most one order

Suggested fields:
- `id`
- `source_id`
- `merchant_name`
- `coupon_title`
- `coupon_value_amount`
- `sale_price_amount`
- `currency_code`
- `coupon_code_encrypted`
- `coupon_masked_display`
- `expiry_at`
- `transferability_status` (`unknown`, `not_transferable`, `transferable_with_review`, `transferable`)
- `inventory_status` (`available`, `reserved`, `assigned`, `delivered`, `used`, `expired`, `voided`, `disputed`)
- `rights_verified_at`
- `rights_verification_note`
- `acquired_cost_amount`
- `acquired_at`
- `assigned_order_id`
- `created_at`
- `updated_at`

## 4. listings
Represents the Telegram-facing sale listing tied to sellable inventory.

Purpose:
- allow one product listing to point to one or more coupons of the same sale shape

Suggested fields:
- `id`
- `merchant_name`
- `title`
- `description`
- `coupon_value_amount`
- `sale_price_amount`
- `currency_code`
- `expiry_summary`
- `terms_summary`
- `redemption_instructions`
- `final_sale_disclosure_text`
- `status` (`draft`, `active`, `paused`, `sold_out`, `expired`, `removed`)
- `source_id`
- `created_by_admin_id`
- `published_at`
- `created_at`
- `updated_at`

## 5. orders
Represents a user purchase attempt.

Suggested fields:
- `id`
- `user_id`
- `listing_id`
- `coupon_id`
- `order_number`
- `status` (`draft`, `pending_payment`, `paid`, `delivery_pending`, `delivered`, `failed`, `cancelled`, `disputed`)
- `currency_code`
- `sale_price_amount`
- `provider_checkout_reference`
- `final_sale_acknowledged_at`
- `failure_reason`
- `placed_at`
- `delivered_at`
- `created_at`
- `updated_at`

Notes:
- `coupon_id` may remain null until payment succeeds and inventory is assigned

## 6. payments
Represents payment attempts related to an order.

Suggested fields:
- `id`
- `order_id`
- `provider_name`
- `provider_payment_id`
- `provider_checkout_id`
- `status` (`pending`, `authorized`, `captured`, `failed`, `cancelled`, `chargeback`, `disputed`)
- `amount`
- `currency_code`
- `failure_code`
- `failure_message`
- `captured_at`
- `created_at`
- `updated_at`

Notes:
- raw card data should never be stored
- payment provider remains the source of truth for card handling

## 7. coupon_deliveries
Represents the act of releasing a coupon to the user after payment.

Purpose:
- separate payment success from delivery success
- provide evidence in disputes

Suggested fields:
- `id`
- `order_id`
- `coupon_id`
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
- `coupon_id`
- `case_type` (`invalid_coupon`, `delivery_issue`, `payment_issue`, `chargeback_review`, `other`)
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
- `entity_type` (`coupon`, `listing`, `order`, `payment`, `support_case`, `source`)
- `entity_id`
- `action_type`
- `before_state_json`
- `after_state_json`
- `reason_text`
- `created_at`

## Relationship Summary
- `coupon_sources` 1-to-many `coupons`
- `coupon_sources` 1-to-many `listings`
- `users` 1-to-many `orders`
- `listings` 1-to-many `orders`
- `orders` optionally belongs-to `coupons`
- `payments` belongs-to `orders`
- `coupon_deliveries` belongs-to `orders`
- `coupon_deliveries` belongs-to `coupons`
- `support_cases` may reference `orders` and `coupons`

## Recommended MVP Tables
- users
- coupon_sources
- coupons
- listings
- orders
- payments
- coupon_deliveries
- support_cases
- admin_actions

## Explicit Non-Table for MVP
No dedicated `refunds` table is recommended in the initial MVP because the product policy is that no refunds are available. If the business later changes policy or needs structured refund processing, that table can be added then.

## Key Design Decisions
- `coupons` represent real owned inventory.
- `listings` represent sellable Telegram products.
- `orders` represent purchase attempts.
- `payments` represent provider-confirmed money movement.
- `coupon_deliveries` represent actual release of coupon content to the user.
- final-sale acknowledgement should be stored with the order.
