DROP INDEX IF EXISTS idx_outbox_events_processing_lock;

ALTER TABLE outbox_events
DROP COLUMN IF EXISTS locked_by,
DROP COLUMN IF EXISTS locked_at;