# Actual Bot Flow

1. A coupon photo with a non-empty caption is posted manually in `TELEGRAM_CHANNEL_ID`.
2. Telegram delivers the post to `/webhooks/telegram` as a channel update.
3. The bot ignores every post that is not a photo with a caption.
4. The bot computes a duplicate key:
   - canonical URL without tracking parameters or fragment, when present;
   - normalized restaurant and typed-offer lines, regardless of which appears first.
5. A duplicate refreshes the existing offer. A new offer is placed first.
6. Only the latest eight offers remain in `data/offers.json`.
7. Buttons use a compact `restaurant · typed offer` label capped at 40 characters, such as `Japan Japan · שובר 100 ₪ ב־79 ₪`.
8. The bot creates or edits one channel menu message, keeps its Telegram message id locally, and pins it.
9. Each button opens the public channel's Direct Messages chat and prepares `היי, אני מעוניין/ת בקופון: <restaurant · typed offer>` in the composer.
10. The user reviews and sends the message directly to the channel; the bot does not receive or relay buyer conversations.

No sale, payment, coupon delivery, or redemption state is managed by the bot.
