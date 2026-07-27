ALTER TABLE outbox_events
ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS locked_by VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_outbox_events_processing_lock
ON outbox_events (locked_at)
WHERE status = 'processing';