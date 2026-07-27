CREATE TABLE notification_deliveries (
    event_id TEXT PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    recipient TEXT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'processing'
        CHECK (status IN ('processing', 'sent', 'failed')),

    attempts INTEGER NOT NULL DEFAULT 1
        CHECK (attempts >= 1),

    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX idx_notification_deliveries_status_updated_at
    ON notification_deliveries (status, updated_at);