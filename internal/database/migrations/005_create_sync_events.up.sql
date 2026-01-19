-- Sync events for tracking changes across devices
CREATE TABLE IF NOT EXISTS sync_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       UUID NOT NULL,
    action          VARCHAR(20) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    payload         JSONB,
    device_id       UUID REFERENCES devices(id),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient sync queries
CREATE INDEX IF NOT EXISTS idx_sync_events_user_time ON sync_events(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sync_events_entity ON sync_events(entity_type, entity_id);
