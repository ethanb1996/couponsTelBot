# Runbook

Legacy note:
This runbook still describes the older automated PayPal-oriented flow.
It is not the source of truth for the current manual PayBox v1 architecture.
Use `docs/architecture/` and `docs/prd/mvp_prd.md` for the target operating model.

## Operational priorities
- payment traceability
- fulfillment traceability
- admin override path
- incident logging

## Invalid coupon complaint
1. Open `/admin/support` and create or review the support case as `invalid_coupon`.
2. Open the linked order in `/admin/orders/{id}` and confirm the assigned masked coupon, payment status, delivery payload hash, and delivery timestamps.
3. Check the coupon row in `/admin/coupons` for status, expiry, and whether it was later disputed or voided.
4. Review the listing and source records if the complaint suggests bad inventory or rights issues.
5. Record the investigation result in the support resolution note before closing the case.

## Paid but not delivered
1. Check the dashboard card for `Paid But Not Delivered` alerts.
2. Open the order and confirm the latest payment event is `authorized` or `captured`.
3. Inspect the delivery block for a failed or missing Telegram delivery record.
4. If delivery failed, resolve the root cause and redeliver manually only after confirming the coupon is still valid.
5. Keep the support case open until delivery is confirmed or the coupon is marked disputed/voided.

## Duplicate charge claim
1. Open the order and inspect the payment history table for multiple provider payment IDs or repeated capture events.
2. Compare the provider checkout reference with PayPal history before taking any action.
3. If the customer received only one coupon but there are multiple successful captures, open a `payment_issue` support case immediately.
4. Add the exact payment IDs and timestamps to the support notes for later dispute handling.

## Chargeback review
1. Create or review a `chargeback_review` support case.
2. Gather the order number, final-sale acknowledgement timestamp, payment capture timestamp, assigned coupon mask, delivery payload hash, and Telegram message ID.
3. Review any admin audit trail tied to the listing, coupon, and support case.
4. Export or copy the evidence into the dispute response with the resolution note summarizing the timeline.
