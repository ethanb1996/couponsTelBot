-- Insert test merchant partner
INSERT INTO merchant_partners (business_name, contact_reference, status, merchant_disclosure_text, support_contact)
VALUES ('Bazario Test', 'test_contact', 'active', 'This offer is sold on behalf of Bazario Test', '@admin')
RETURNING id;

-- Get the last inserted merchant ID
WITH merchant AS (
  SELECT id FROM merchant_partners ORDER BY created_at DESC LIMIT 1
)
-- Insert test offer
INSERT INTO offers (merchant_partner_id, merchant_name, title, description, price_amount, currency_code, payment_link, merchant_disclosure_text, redemption_terms, support_contact, status, published_at)
SELECT 
  id,
  'Bazario Test',
  'Test Coupon 20% Off',
  '20% discount on all products',
  5000,
  'ILS',
  'https://paybox.money/p/test-link',
  'This offer is sold on behalf of Bazario Test',
  'Valid for 30 days. Redeem online at bazario.test',
  '@admin',
  'active',
  NOW()
FROM merchant;

-- Get the last inserted offer ID
WITH merchant AS (
  SELECT id FROM merchant_partners ORDER BY created_at DESC LIMIT 1
),
offer AS (
  SELECT id FROM offers WHERE merchant_partner_id = (SELECT id FROM merchant) ORDER BY created_at DESC LIMIT 1
)
-- Insert test predefined codes (5 codes for testing)
INSERT INTO predefined_codes (offer_id, merchant_partner_id, code_encrypted, code_masked_display, status, expiry_at, issued_batch_reference)
SELECT
  o.id,
  m.id,
  decode('fcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfcfc', 'hex'),
  'BAZARIO-2025-' || LPAD((ROW_NUMBER() OVER (ORDER BY 1))::text, 4, '0'),
  'available',
  NOW() + INTERVAL '30 days',
  'batch_001'
FROM merchant m, offer o, generate_series(1, 5)
WHERE o.merchant_partner_id = m.id;
