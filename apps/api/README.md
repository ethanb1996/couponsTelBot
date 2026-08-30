# Coupon Menu Bot

## Configuration

```env
PORT=8080
TELEGRAM_BOT_TOKEN=
TELEGRAM_WEBHOOK_SECRET=
TELEGRAM_CHANNEL_ID=-4448924956
TELEGRAM_ADMIN_USER_IDS=123456789
OFFERS_FILE=data/offers.json
```

The bot must be an administrator in the channel so Telegram sends it `channel_post` and `edited_channel_post` updates.

## Run

```powershell
go run ./apps/api/cmd/server
```

Expose port 8080:

```powershell
ngrok http 8080
```

Register the HTTPS ngrok URL as the Telegram webhook and include these update types:

- `message`
- `callback_query`
- `channel_post`
- `edited_channel_post`

## Important Telegram limitation

The Bot API cannot retrieve channel history. Only posts delivered after the webhook is active can enter the menu. Repost or forward existing offers once to seed the first eight entries.
