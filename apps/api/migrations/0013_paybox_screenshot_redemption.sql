ALTER TABLE manual_payment_claims
    ADD COLUMN IF NOT EXISTS payment_screenshot_file_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_screenshot_unique_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_screenshot_message_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS payment_screenshot_caption TEXT NOT NULL DEFAULT '';

ALTER TABLE predefined_codes DROP CONSTRAINT IF EXISTS predefined_codes_status_check;
ALTER TABLE predefined_codes ADD CONSTRAINT predefined_codes_status_check CHECK (
    status IN ('available', 'assigned', 'sent', 'redeemed', 'voided', 'expired')
);

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
        'coupon_redeemed',
        'payment_rejected',
        'support_required'
    )
);

CREATE TABLE IF NOT EXISTS coupon_redemptions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    predefined_code_id BIGINT NOT NULL REFERENCES predefined_codes(id) ON DELETE RESTRICT,
    redemption_token TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'issued' CHECK (status IN ('issued', 'redeemed')),
    merchant_reference TEXT NOT NULL DEFAULT '',
    scanner_reference TEXT NOT NULL DEFAULT '',
    scan_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    scanned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coupon_redemptions_order
    ON coupon_redemptions (order_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coupon_redemptions_token
    ON coupon_redemptions (redemption_token);

CREATE INDEX IF NOT EXISTS idx_coupon_redemptions_status
    ON coupon_redemptions (status, scanned_at DESC NULLS LAST);
