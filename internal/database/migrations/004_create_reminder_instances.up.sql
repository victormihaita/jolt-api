-- Reminder instances for recurring reminders
CREATE TABLE IF NOT EXISTS reminder_instances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id         UUID NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    scheduled_at        TIMESTAMP WITH TIME ZONE NOT NULL,
    status              VARCHAR(20) DEFAULT 'pending'
                        CHECK (status IN ('pending', 'notified', 'completed', 'snoozed', 'dismissed')),
    snoozed_until       TIMESTAMP WITH TIME ZONE,
    completed_at        TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_reminder_instances_reminder_id ON reminder_instances(reminder_id);
CREATE INDEX IF NOT EXISTS idx_reminder_instances_scheduled ON reminder_instances(scheduled_at, status);
