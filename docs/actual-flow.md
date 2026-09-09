# Actual Bot Flow

1. A coupon is posted manually in `TELEGRAM_CHANNEL_ID`.
2. Telegram delivers the post to `/webhooks/telegram` as a channel update.
3. The bot extracts text or image caption.
4. The bot computes a duplicate key:
   - canonical URL without tracking parameters or fragment, when present;
   - normalized restaurant and essential-offer lines.
5. A duplicate refreshes the existing offer. A new offer is placed first.
6. Only the latest eight offers remain in `data/offers.json`.
7. Buttons use a compact `restaurant · essential offer` label capped at 40 characters so the value remains visible on narrow screens.
8. The bot creates or edits one channel menu message, keeps its Telegram message id locally, and pins it.
9. Each button opens the public channel's Direct Messages chat and prepares `היי, אני מעוניין/ת בקופון: <restaurant · essential offer>` in the composer.
10. The user reviews and sends the message directly to the channel; the bot does not receive or relay buyer conversations.

No sale, payment, coupon delivery, or redemption state is managed by the bot.
