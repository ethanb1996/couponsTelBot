# Go MVP Prompt Pack

This folder contains an ordered prompt set for building the Telegram bot MVP from `0` to a sellable version.

Use the files in this order:

1. `00_source_of_truth.md`
2. `01_scaffold_single_service.md`
3. `02_schema_inventory_orders.md`
4. `03_admin_inventory_ops.md`
5. `04_bot_sales_flow.md`
6. `05_payments_delivery.md`
7. `06_ops_hardening_launch.md`

## Why This Prompt Pack Exists
The repo contains product and architecture thinking from several iterations. This prompt pack intentionally removes the extra branches and keeps only the smallest viable Go implementation needed to start selling.

## Simplified MVP This Pack Assumes
- pre-bought coupon inventory only
- Telegram bot as the primary sales surface
- the bot sends `1` to `3` coupon options per message
- each coupon option has its own `Buy` button
- payment is taken in `ILS`
- one successful payment results in one coupon delivered in Telegram
- all sales are final
- no normal refund flow
- one Go backend service
- one small admin UI inside that same service

## Conflict Rule
If older repo docs conflict with this prompt pack, `00_source_of_truth.md` wins for implementation decisions.
