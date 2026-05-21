CREATE TABLE IF NOT EXISTS merchant_partners (
    id BIGSERIAL PRIMARY KEY,
    business_name TEXT NOT NULL,
    contact_reference TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'lead' CHECK (status IN ('lead', 'active', 'paused', 'inactive')),
    approval_notes TEXT NOT NULL DEFAULT '',
    merchant_disclosure_text TEXT NOT NULL DEFAULT '',
    support_contact TEXT NOT NULL DEFAULT '',
    default_payment_link TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS offers (
    id BIGSERIAL PRIMARY KEY,
    merchant_partner_id BIGINT NOT NULL REFERENCES merchant_partners(id) ON DELETE RESTRICT,
    merchant_name TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_amount BIGINT NOT NULL CHECK (price_amount >= 0),
    currency_code TEXT NOT NULL DEFAULT 'ILS',
    payment_link TEXT NOT NULL DEFAULT '',
    merchant_disclosure_text TEXT NOT NULL DEFAULT '',
    redemption_terms TEXT NOT NULL DEFAULT '',
    support_contact TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'sold_out', 'expired', 'removed')),
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS predefined_codes (
    id BIGSERIAL PRIMARY KEY,
    offer_id BIGINT NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    merchant_partner_id BIGINT NOT NULL REFERENCES merchant_partners(id) ON DELETE RESTRICT,
    code_encrypted BYTEA NOT NULL,
    code_masked_display TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'assigned', 'sent', 'voided', 'expired')),
    expiry_at TIMESTAMPTZ NULL,
    issued_batch_reference TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS offer_id BIGINT NULL REFERENCES offers(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS predefined_code_id BIGINT UNIQUE NULL REFERENCES predefined_codes(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS paybox_payment_link TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ NULL;

ALTER TABLE orders ALTER COLUMN listing_id DROP NOT NULL;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
    status IN (
        'draft',
        'pending_payment',
        'paid',
        'delivery_pending',
        'delivered',
        'failed',
        'cancelled',
        'disputed',
        'awaiting_payment',
        'payment_claim_submitted',
        'payment_verified',
        'coupon_sent',
        'payment_rejected',
        'support_required'
    )
);

CREATE TABLE IF NOT EXISTS manual_payment_claims (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    payer_username TEXT NOT NULL,
    claimed_amount BIGINT NOT NULL DEFAULT 0 CHECK (claimed_amount >= 0),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    review_status TEXT NOT NULL DEFAULT 'pending_review' CHECK (review_status IN ('pending_review', 'verified', 'rejected')),
    reviewed_by TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ NULL,
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE coupon_deliveries
    ADD COLUMN IF NOT EXISTS predefined_code_id BIGINT UNIQUE NULL REFERENCES predefined_codes(id) ON DELETE RESTRICT;

ALTER TABLE support_cases
    ADD COLUMN IF NOT EXISTS predefined_code_id BIGINT NULL REFERENCES predefined_codes(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS payment_claim_id BIGINT NULL REFERENCES manual_payment_claims(id) ON DELETE SET NULL;

ALTER TABLE support_cases DROP CONSTRAINT IF EXISTS support_cases_case_type_check;
ALTER TABLE support_cases ADD CONSTRAINT support_cases_case_type_check CHECK (
    case_type IN (
        'invalid_coupon',
        'delivery_issue',
        'payment_issue',
        'chargeback_review',
        'other',
        'payment_mismatch',
        'invalid_code',
        'merchant_issue'
    )
);

ALTER TABLE admin_actions DROP CONSTRAINT IF EXISTS admin_actions_entity_type_check;
ALTER TABLE admin_actions ADD CONSTRAINT admin_actions_entity_type_check CHECK (
    entity_type IN (
        'coupon_source',
        'listing',
        'coupon',
        'order',
        'payment',
        'support_case',
        'merchant_partner',
        'offer',
        'predefined_code',
        'manual_payment_claim'
    )
);

CREATE INDEX IF NOT EXISTS idx_merchant_partners_status ON merchant_partners (status, business_name);
CREATE INDEX IF NOT EXISTS idx_offers_status_published ON offers (status, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_offers_merchant_partner ON offers (merchant_partner_id, status);
CREATE INDEX IF NOT EXISTS idx_predefined_codes_offer_status ON predefined_codes (offer_id, status, expiry_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_manual_payment_claims_one_pending
    ON manual_payment_claims (order_id)
    WHERE review_status = 'pending_review';
CREATE INDEX IF NOT EXISTS idx_manual_payment_claims_review_queue
    ON manual_payment_claims (review_status, submitted_at ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_orders_offer_status ON orders (offer_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_cases_payment_claim ON support_cases (payment_claim_id);
