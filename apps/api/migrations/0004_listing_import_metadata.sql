ALTER TABLE listings
    ADD COLUMN IF NOT EXISTS photo_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS external_import_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_listings_external_import_key
    ON listings (external_import_key)
    WHERE external_import_key <> '';
