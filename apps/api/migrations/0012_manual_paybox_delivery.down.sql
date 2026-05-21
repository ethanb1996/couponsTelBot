DROP INDEX IF EXISTS idx_coupon_deliveries_predefined_code;

ALTER TABLE coupon_deliveries
    DROP CONSTRAINT IF EXISTS coupon_deliveries_fulfillment_unit_check;

ALTER TABLE coupon_deliveries
    ALTER COLUMN coupon_id SET NOT NULL;
