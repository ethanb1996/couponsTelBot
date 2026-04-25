DROP INDEX IF EXISTS idx_listings_external_import_key;

ALTER TABLE listings
    DROP COLUMN IF EXISTS external_import_key,
    DROP COLUMN IF EXISTS photo_key;
