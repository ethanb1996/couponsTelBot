DROP INDEX IF EXISTS idx_coupon_redemptions_redeemed_at;
DROP INDEX IF EXISTS idx_coupon_redemption_events_redemption_created;
DROP TABLE IF EXISTS coupon_redemption_events;

ALTER TABLE coupon_redemptions
    DROP COLUMN IF EXISTS restaurant_notification_error,
    DROP COLUMN IF EXISTS restaurant_notified_at,
    DROP COLUMN IF EXISTS restaurant_notification_message_id,
    DROP COLUMN IF EXISTS restaurant_notification_chat_id,
    DROP COLUMN IF EXISTS redeem_metadata_json,
    DROP COLUMN IF EXISTS redeemed_by_reference,
    DROP COLUMN IF EXISTS redeemed_at,
    DROP COLUMN IF EXISTS first_viewed_at;

ALTER TABLE merchant_partners
    DROP COLUMN IF EXISTS redemption_notification_chat_title,
    DROP COLUMN IF EXISTS redemption_notification_chat_id;
