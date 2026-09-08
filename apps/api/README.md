# Coupon Menu Bot

## Configuration

```env
PORT=8080
TELEGRAM_BOT_TOKEN=
TELEGRAM_WEBHOOK_SECRET=
TELEGRAM_CHANNEL_ID=-1004448924956
TELEGRAM_ADMIN_USER_IDS=123456789
OFFERS_FILE=data/offers.json
```

The bot must be an administrator in the channel with permission to post, edit, and pin messages. It maintains one pinned menu message and updates its buttons after every new or edited offer post.

For a readable Telegram button, publish each manual offer with the restaurant on the first line and the essential value on the second line, for example:

```text
קופון חדש ל־Japan Japan
שובר בשווי 100 ₪ ב־79 ₪ בלבד
```

The menu renders that as `Japan Japan · 100 ₪ ב־79 ₪` while retaining the full post for the sales handoff.

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
