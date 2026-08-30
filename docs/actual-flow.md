# Actual Bot Flow

1. A coupon is posted manually in `TELEGRAM_CHANNEL_ID`.
2. Telegram delivers the post to `/webhooks/telegram` as a channel update.
3. The bot extracts text or image caption.
4. The bot computes a duplicate key:
   - canonical URL without query parameters or fragment, when present;
   - otherwise, normalized first non-empty line.
5. A duplicate refreshes the existing offer. A new offer is placed first.
6. Only the latest eight offers remain in `data/offers.json`.
7. `/start` and `/offers` render one button per offer.
8. Selecting an offer sends a pre-written interest confirmation to the buyer.
9. Every configured Telegram admin receives the selected offer and a link to the buyer.

No sale, payment, coupon delivery, or redemption state is managed by the bot.
