DROP INDEX IF EXISTS idx_coupon_redemptions_status;
DROP INDEX IF EXISTS idx_coupon_redemptions_token;
DROP INDEX IF EXISTS idx_coupon_redemptions_order;
DROP TABLE IF EXISTS coupon_redemptions;

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

ALTER TABLE predefined_codes DROP CONSTRAINT IF EXISTS predefined_codes_status_check;
ALTER TABLE predefined_codes ADD CONSTRAINT predefined_codes_status_check CHECK (
    status IN ('available', 'assigned', 'sent', 'voided', 'expired')
);

ALTER TABLE manual_payment_claims
    DROP COLUMN IF EXISTS payment_screenshot_caption,
    DROP COLUMN IF EXISTS payment_screenshot_message_id,
    DROP COLUMN IF EXISTS payment_screenshot_unique_id,
    DROP COLUMN IF EXISTS payment_screenshot_file_id;
