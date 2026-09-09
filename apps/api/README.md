# Coupon Menu Bot

## Configuration

```env
PORT=8080
TELEGRAM_BOT_TOKEN=
TELEGRAM_WEBHOOK_SECRET=
TELEGRAM_CHANNEL_ID=-1004448924956
OFFERS_FILE=data/offers.json
```

The bot must be an administrator in a public channel with permission to post, edit, and pin messages. The public username is read directly from Telegram and is required for channel Direct Messages links. The bot maintains one pinned menu message and updates its buttons after every new or edited offer post.

Because the channel is public, every channel post and the pinned menu are public content. Keep credentials only in the ignored local `.env`; never place tokens, webhook secrets, customer data, or internal notes in an offer post. The webhook rejects requests without Telegram's secret-token header and accepts only channel post updates.

An offer is captured only when the channel post contains a Telegram photo and a non-empty caption. Text-only posts, photos without captions, videos, files, and other media are ignored. If an existing offer post is edited so that it no longer meets those rules, it is removed from the menu.

For a readable Telegram button, put the restaurant and the typed offer (`שובר`, `1+1`, `% הנחה`, a specific meal, and so on) on separate caption lines. Either order is accepted, for example:

```text
קופון חדש ל־Japan Japan
שובר בשווי 100 ₪ ב־79 ₪ בלבד
```

The menu renders that as `Japan Japan · שובר 100 ₪ ב־79 ₪`. Clicking it opens the channel's Direct Messages chat with `היי, אני מעוניין/ת בקופון: Japan Japan · שובר 100 ₪ ב־79 ₪` prepared in the composer. The user reviews and sends it; the menu bot does not conduct the sale.

## Run

```powershell
go run ./apps/api/cmd/server
```

Expose port 8080:

```powershell
ngrok http 8080
```

Register the HTTPS ngrok URL as the Telegram webhook and include these update types:

- `channel_post`
- `edited_channel_post`

## Important Telegram limitation

The Bot API cannot retrieve channel history. Only posts delivered after the webhook is active can enter the menu. Repost or forward existing offers once to seed the first eight entries.
