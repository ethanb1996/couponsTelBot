DROP INDEX IF EXISTS idx_support_cases_payment_claim;
DROP INDEX IF EXISTS idx_orders_offer_status;
DROP INDEX IF EXISTS idx_manual_payment_claims_review_queue;
DROP INDEX IF EXISTS idx_manual_payment_claims_one_pending;
DROP INDEX IF EXISTS idx_predefined_codes_offer_status;
DROP INDEX IF EXISTS idx_offers_merchant_partner;
DROP INDEX IF EXISTS idx_offers_status_published;
DROP INDEX IF EXISTS idx_merchant_partners_status;

ALTER TABLE admin_actions DROP CONSTRAINT IF EXISTS admin_actions_entity_type_check;
ALTER TABLE admin_actions ADD CONSTRAINT admin_actions_entity_type_check CHECK (
    entity_type IN ('coupon_source', 'listing', 'coupon', 'order', 'payment', 'support_case')
);

ALTER TABLE support_cases DROP CONSTRAINT IF EXISTS support_cases_case_type_check;
ALTER TABLE support_cases ADD CONSTRAINT support_cases_case_type_check CHECK (
    case_type IN ('invalid_coupon', 'delivery_issue', 'payment_issue', 'chargeback_review', 'other')
);
ALTER TABLE support_cases
    DROP COLUMN IF EXISTS payment_claim_id,
    DROP COLUMN IF EXISTS predefined_code_id;

ALTER TABLE coupon_deliveries DROP COLUMN IF EXISTS predefined_code_id;

DROP TABLE IF EXISTS manual_payment_claims;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
    status IN ('draft', 'pending_payment', 'paid', 'delivery_pending', 'delivered', 'failed', 'cancelled', 'disputed')
);
ALTER TABLE orders
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS paybox_payment_link,
    DROP COLUMN IF EXISTS predefined_code_id,
    DROP COLUMN IF EXISTS offer_id;

DROP TABLE IF EXISTS predefined_codes;
DROP TABLE IF EXISTS offers;
DROP TABLE IF EXISTS merchant_partners;
