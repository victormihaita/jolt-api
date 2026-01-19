-- Reminders table
CREATE TABLE IF NOT EXISTS reminders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title               VARCHAR(500) NOT NULL,
    notes               TEXT,
    priority            INTEGER DEFAULT 0 CHECK (priority BETWEEN 0 AND 3),

    -- Scheduling
    due_at              TIMESTAMP WITH TIME ZONE NOT NULL,
    all_day             BOOLEAN DEFAULT FALSE,

    -- Recurrence (JSONB for flexibility)
    recurrence_rule     JSONB,
    recurrence_end      TIMESTAMP WITH TIME ZONE,

    -- State
    status              VARCHAR(20) DEFAULT 'active'
                        CHECK (status IN ('active', 'completed', 'snoozed', 'dismissed')),
    completed_at        TIMESTAMP WITH TIME ZONE,
    snoozed_until       TIMESTAMP WITH TIME ZONE,
    snooze_count        INTEGER DEFAULT 0,

    -- Sync
    local_id            VARCHAR(255),
    version             INTEGER DEFAULT 1,
    last_modified_by    UUID REFERENCES devices(id),

    -- Metadata
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at          TIMESTAMP WITH TIME ZONE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_reminders_user_due ON reminders(user_id, due_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reminders_user_status ON reminders(user_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reminders_local_id ON reminders(user_id, local_id);
CREATE INDEX IF NOT EXISTS idx_reminders_deleted_at ON reminders(deleted_at);
