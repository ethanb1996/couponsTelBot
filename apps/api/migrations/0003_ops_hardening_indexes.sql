CREATE INDEX IF NOT EXISTS idx_admin_actions_entity_created_at
    ON admin_actions (entity_type, entity_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_actions_created_at
    ON admin_actions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_cases_order_case_status
    ON support_cases (order_id, case_type, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_orders_pending_checkout_reference
    ON orders (provider_checkout_reference, status, updated_at DESC)
    WHERE provider_checkout_reference <> '';

CREATE INDEX IF NOT EXISTS idx_orders_ops_status
    ON orders (status, updated_at DESC)
    WHERE status IN ('pending_payment', 'paid', 'delivery_pending');
