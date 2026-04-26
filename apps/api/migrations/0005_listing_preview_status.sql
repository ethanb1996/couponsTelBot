ALTER TABLE listings
    DROP CONSTRAINT IF EXISTS listings_status_check;

ALTER TABLE listings
    ADD CONSTRAINT listings_status_check
    CHECK (status IN ('preview', 'draft', 'active', 'paused', 'sold_out', 'expired', 'removed'));
