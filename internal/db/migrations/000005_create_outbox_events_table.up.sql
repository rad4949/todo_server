CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,

    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id TEXT NOT NULL,

    event_type VARCHAR(100) NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,

    payload JSONB NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,

    CONSTRAINT chk_outbox_events_status
        CHECK (status IN ('pending', 'processing', 'processed', 'failed')),

    CONSTRAINT chk_outbox_events_attempts
        CHECK (attempts >= 0),

    CONSTRAINT chk_outbox_events_version
        CHECK (event_version > 0)
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
ON outbox_events (next_attempt_at, created_at)
WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
ON outbox_events (aggregate_type, aggregate_id, created_at);