CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL UNIQUE,
    telegram_username TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    language_code TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked', 'deleted')),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS coupon_sources (
    id BIGSERIAL PRIMARY KEY,
    source_name TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('merchant_partner', 'reseller', 'licensed_distributor', 'manual_source')),
    contact_reference TEXT NOT NULL DEFAULT '',
    rights_status TEXT NOT NULL DEFAULT 'unknown' CHECK (rights_status IN ('unknown', 'review_pending', 'approved', 'restricted', 'rejected')),
    verification_notes TEXT NOT NULL DEFAULT '',
    risk_rating TEXT NOT NULL DEFAULT 'medium' CHECK (risk_rating IN ('low', 'medium', 'high')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS listings (
    id BIGSERIAL PRIMARY KEY,
    merchant_name TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    coupon_value_amount BIGINT NOT NULL CHECK (coupon_value_amount >= 0),
    sale_price_amount BIGINT NOT NULL CHECK (sale_price_amount >= 0),
    currency_code TEXT NOT NULL DEFAULT 'ILS',
    expiry_summary TEXT NOT NULL DEFAULT '',
    terms_summary TEXT NOT NULL DEFAULT '',
    redemption_instructions TEXT NOT NULL DEFAULT '',
    final_sale_disclosure_text TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'sold_out', 'expired', 'removed')),
    created_by_admin_id TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS coupons (
    id BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    source_id BIGINT NOT NULL REFERENCES coupon_sources(id) ON DELETE RESTRICT,
    merchant_name TEXT NOT NULL,
    coupon_title TEXT NOT NULL,
    coupon_value_amount BIGINT NOT NULL CHECK (coupon_value_amount >= 0),
    sale_price_amount BIGINT NOT NULL CHECK (sale_price_amount >= 0),
    currency_code TEXT NOT NULL DEFAULT 'ILS',
    coupon_code_ciphertext BYTEA NOT NULL,
    coupon_code_nonce BYTEA NOT NULL,
    coupon_masked_display TEXT NOT NULL,
    expiry_at TIMESTAMPTZ NOT NULL,
    transferability_status TEXT NOT NULL DEFAULT 'unknown' CHECK (transferability_status IN ('unknown', 'not_transferable', 'transferable_with_review', 'transferable')),
    inventory_status TEXT NOT NULL DEFAULT 'available' CHECK (inventory_status IN ('available', 'reserved', 'assigned', 'delivered', 'used', 'expired', 'voided', 'disputed')),
    rights_verified_at TIMESTAMPTZ NULL,
    rights_verification_note TEXT NOT NULL DEFAULT '',
    acquired_cost_amount BIGINT NOT NULL DEFAULT 0 CHECK (acquired_cost_amount >= 0),
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    listing_id BIGINT NOT NULL REFERENCES listings(id) ON DELETE RESTRICT,
    coupon_id BIGINT UNIQUE NULL REFERENCES coupons(id) ON DELETE RESTRICT,
    order_number TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending_payment', 'paid', 'delivery_pending', 'delivered', 'failed', 'cancelled', 'disputed')),
    currency_code TEXT NOT NULL DEFAULT 'ILS',
    sale_price_amount BIGINT NOT NULL CHECK (sale_price_amount >= 0),
    provider_checkout_reference TEXT NOT NULL DEFAULT '',
    final_sale_acknowledged_at TIMESTAMPTZ NULL,
    failure_reason TEXT NOT NULL DEFAULT '',
    placed_at TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    provider_name TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL DEFAULT '',
    provider_checkout_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'authorized', 'captured', 'failed', 'cancelled', 'chargeback', 'disputed')),
    amount BIGINT NOT NULL CHECK (amount >= 0),
    currency_code TEXT NOT NULL DEFAULT 'ILS',
    failure_code TEXT NOT NULL DEFAULT '',
    failure_message TEXT NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS coupon_deliveries (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    coupon_id BIGINT NOT NULL REFERENCES coupons(id) ON DELETE RESTRICT,
    delivery_channel TEXT NOT NULL DEFAULT 'telegram_bot',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'confirmed', 'failed')),
    telegram_message_id BIGINT NULL,
    delivery_payload_hash TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ NULL,
    confirmed_at TIMESTAMPTZ NULL,
    failure_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS support_cases (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    order_id BIGINT NULL REFERENCES orders(id) ON DELETE SET NULL,
    coupon_id BIGINT NULL REFERENCES coupons(id) ON DELETE SET NULL,
    case_type TEXT NOT NULL CHECK (case_type IN ('invalid_coupon', 'delivery_issue', 'payment_issue', 'chargeback_review', 'other')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'waiting_on_user', 'resolved', 'closed')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
    summary TEXT NOT NULL,
    resolution_note TEXT NOT NULL DEFAULT '',
    assigned_admin_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_actions (
    id BIGSERIAL PRIMARY KEY,
    admin_actor TEXT NOT NULL,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('coupon_source', 'listing', 'coupon', 'order', 'payment', 'support_case')),
    entity_id BIGINT NOT NULL,
    action_type TEXT NOT NULL,
    before_state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coupons_listing_status ON coupons (listing_id, inventory_status, expiry_at);
CREATE INDEX IF NOT EXISTS idx_listings_status ON listings (status, published_at);
CREATE INDEX IF NOT EXISTS idx_orders_user_status ON orders (user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_order_status ON payments (order_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_cases_status_priority ON support_cases (status, priority, created_at DESC);
