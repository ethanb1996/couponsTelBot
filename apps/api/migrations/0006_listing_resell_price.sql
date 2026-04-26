ALTER TABLE listings
    ADD COLUMN IF NOT EXISTS resell_price_amount BIGINT NOT NULL DEFAULT 0 CHECK (resell_price_amount >= 0);

UPDATE listings
SET resell_price_amount = sale_price_amount
WHERE resell_price_amount = 0;
