ALTER TABLE coupon_deliveries
    ALTER COLUMN coupon_id DROP NOT NULL;

ALTER TABLE coupon_deliveries
    DROP CONSTRAINT IF EXISTS coupon_deliveries_fulfillment_unit_check;

ALTER TABLE coupon_deliveries
    ADD CONSTRAINT coupon_deliveries_fulfillment_unit_check CHECK (
        coupon_id IS NOT NULL OR predefined_code_id IS NOT NULL
    );

CREATE INDEX IF NOT EXISTS idx_coupon_deliveries_predefined_code
    ON coupon_deliveries (predefined_code_id)
    WHERE predefined_code_id IS NOT NULL;
