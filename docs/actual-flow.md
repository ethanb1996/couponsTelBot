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
9. Channel menu buttons open an offer-specific private chat with the bot.
10. `/start` and `/offers` also render one private-chat button per offer.
11. Selecting an offer sends a pre-written interest confirmation to the buyer.
12. Every configured Telegram admin receives the selected offer and a link to the buyer.

No sale, payment, coupon delivery, or redemption state is managed by the bot.
