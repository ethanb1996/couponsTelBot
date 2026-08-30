# Telegram Coupon Menu Bot

A small Telegram bot for validating coupon demand without checkout automation.

## What it does

1. An admin manually publishes a coupon post in the configured Telegram channel.
2. The bot receives the channel post through its webhook.
3. It stores the latest eight distinct offers in `data/offers.json`.
4. `/start` or `/offers` shows those offers as menu buttons.
5. A click sends a pre-written interest message and notifies the configured admins.

Duplicate posts are refreshed instead of added twice. The bot identifies duplicates by canonical URL when the post contains one, otherwise by its normalized first line.

There is no database, payment flow, inventory, QR delivery, redemption system, email integration, or web admin.

See [apps/api/README.md](apps/api/README.md) for setup and operation.
