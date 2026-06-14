ALTER TABLE merchant_partners
    ADD COLUMN IF NOT EXISTS redemption_notification_chat_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS redemption_notification_chat_title TEXT NOT NULL DEFAULT '';

ALTER TABLE coupon_redemptions
    ADD COLUMN IF NOT EXISTS first_viewed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS redeemed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS redeemed_by_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS redeem_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS restaurant_notification_chat_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notification_message_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notified_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS restaurant_notification_error TEXT NOT NULL DEFAULT '';

ALTER TABLE coupon_redemptions DROP CONSTRAINT IF EXISTS coupon_redemptions_status_check;
ALTER TABLE coupon_redemptions ADD CONSTRAINT coupon_redemptions_status_check
    CHECK (status IN ('issued', 'redeemed'));

CREATE TABLE IF NOT EXISTS coupon_redemption_events (
    id BIGSERIAL PRIMARY KEY,
    coupon_redemption_id BIGINT NULL REFERENCES coupon_redemptions(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('viewed', 'redeemed', 'repeat_redeem', 'invalid_token')),
    actor_reference TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coupon_redemption_events_redemption_created
    ON coupon_redemption_events (coupon_redemption_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_coupon_redemptions_redeemed_at
    ON coupon_redemptions (redeemed_at DESC NULLS LAST);
