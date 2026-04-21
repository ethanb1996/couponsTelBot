CREATE UNIQUE INDEX IF NOT EXISTS idx_coupon_deliveries_coupon_id
    ON coupon_deliveries (coupon_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_payment_id
    ON payments (provider_name, provider_payment_id)
    WHERE provider_payment_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_checkout_id
    ON payments (provider_name, provider_checkout_id)
    WHERE provider_checkout_id <> '';

CREATE INDEX IF NOT EXISTS idx_coupons_assignable
    ON coupons (listing_id, expiry_at, id)
    WHERE inventory_status = 'available';
