# Live Test Setup - Complete ✓

## Configuration Completed

### 1. Environment (.env) ✓
- **Admin Telegram ID**: 8319213106
- **Database URL**: Configured (Supabase PostgreSQL)
- **Telegram Bot Token**: Configured
- **Telegram Webhook Secret**: Configured
- **Payment Provider**: PayPal (Sandbox)
- **API Base URL**: https://washbowl-decline-proclaim.ngrok-free.dev

### 2. Database Migrations ✓
All migrations applied:
- 0001: initial_schema
- 0002: store_invariants
- 0003: ops_hardening_indexes
- 0004: listing_import_metadata
- 0005: listing_preview_status
- 0006: listing_resell_price
- 0007: fulfillment_jobs
- 0011: manual_paybox_mvp_model
- 0012: manual_paybox_delivery
- 0013: paybox_screenshot_redemption

### 3. Test Data Created ✓
**Merchant Partner:**
- Name: Bazario Test
- Status: active
- ID: 1

**Offer:**
- Title: Test Coupon 20% Off
- Price: 50 ILS
- Payment Link: https://paybox.money/p-test-link
- Status: active
- ID: 1

**Predefined Codes:**
- 5 available codes ready for orders
- Format: BAZARIO-2025-####
- Expiry: 30 days from now

---

## Ready to Test - Next Steps

### Step 1: Start the API Server
```powershell
cd c:\Projects\couponsTelBot
go run ./apps/api/cmd/server/main.go
```

Expected output:
```
{"time":"2026-05-25T...","level":"INFO","msg":"server starting","addr":":8080","env":"development"}
```

### Step 2: Access Telegram Bot
1. Open Telegram
2. Start chat with your test bot
3. Send `/start` command
4. You should see the "Test Coupon 20% Off" offer

### Step 3: Complete a Test Purchase Flow
1. Tap "Buy" on the offer
2. Follow the payment instructions
3. Tap "Open PayBox" link
4. Complete payment in PayBox sandbox
5. Return to Telegram and tap "I paid"
6. Submit your PayBox username
7. Admin should receive a notification with your order

### Step 4: Admin Review
Access admin dashboard at: http://localhost:8080/admin/orders
- Username: ethanbo
- Password: ethanbo

Admin will see:
- New order from buyer
- Awaiting payment verification
- Can approve/reject the payment claim
- Coupon code auto-delivery on approval

---

## Architecture Overview

```
User (Telegram) 
  ↓
Bot (Telegram Bot API)
  ↓
API Server (:8080)
  ├─ Telegram Webhook Handler
  ├─ Payment Processing (PayPal)
  ├─ Admin Dashboard (Basic Auth)
  └─ Database (Supabase PostgreSQL)
```

## Manual PayBox MVP Flow

1. **User initiates purchase** → Order created in `awaiting_payment` state
2. **User pays with PayBox** → User submits PayBox username
3. **Manual verification** → Admin receives notification via Telegram
4. **Admin approves** → Coupon code automatically delivered to user
5. **Order fulfilled** → Marked as `delivered`

---

## Key Configuration Files

- `.env`: Environment variables with admin ID `8319213106`
- `apps/api/internal/config/config.go`: Configuration loader
- `apps/api/migrations/*`: Database schema
- `apps/api/internal/telegram/handlers.go`: Bot message handlers
- `docs/architecture/bot_flow.md`: Complete bot flow documentation

---

## Testing Checklist

- [ ] API server starts successfully
- [ ] Telegram /start command displays the test offer
- [ ] Admin receives notifications in Telegram
- [ ] Admin dashboard loads at localhost:8080/admin/orders
- [ ] Mock payment flow completes
- [ ] Coupon code is delivered after admin approval
- [ ] Order status transitions correctly

---

## Support

If you encounter any issues:
1. Check API server logs for errors
2. Verify .env configuration (especially TELEGRAM_ADMIN_USER_IDS=8319213106)
3. Confirm database migrations completed: `go run ./apps/api/cmd/migrate/main.go`
4. Check that Telegram webhook is configured for `/webhooks/telegram`

See `docs/ops/runbook.md` for operational procedures.
