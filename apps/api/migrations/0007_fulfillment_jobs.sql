CREATE TABLE IF NOT EXISTS fulfillment_jobs (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    provider_checkout_reference TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'retry_scheduled', 'succeeded', 'failed_terminal')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_step TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    locked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fulfillment_jobs_due
    ON fulfillment_jobs (status, next_attempt_at, id)
    WHERE status IN ('pending', 'processing', 'retry_scheduled');
