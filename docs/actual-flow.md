# Actual Bot Flow

1. A coupon is posted manually in `TELEGRAM_CHANNEL_ID`.
2. Telegram delivers the post to `/webhooks/telegram` as a channel update.
3. The bot extracts text or image caption.
4. The bot computes a duplicate key:
   - canonical URL without query parameters or fragment, when present;
   - otherwise, normalized first non-empty line.
5. A duplicate refreshes the existing offer. A new offer is placed first.
6. Only the latest eight offers remain in `data/offers.json`.
7. The bot creates or edits one channel menu message, keeps its Telegram message id locally, and pins it.
8. Channel menu buttons open an offer-specific private chat with the bot.
9. `/start` and `/offers` also render one private-chat button per offer.
10. Selecting an offer sends a pre-written interest confirmation to the buyer.
11. Every configured Telegram admin receives the selected offer and a link to the buyer.

No sale, payment, coupon delivery, or redemption state is managed by the bot.
