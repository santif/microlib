CREATE TABLE IF NOT EXISTS outbox_messages (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    retry_count INTEGER NOT NULL DEFAULT 0,
    error TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_messages_processed_at ON outbox_messages (processed_at) 
WHERE processed_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS outbox_messages;