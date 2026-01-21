-- Add missing columns to reminders table

-- List ID for organizing reminders into lists
ALTER TABLE reminders ADD COLUMN IF NOT EXISTS list_id UUID REFERENCES reminder_lists(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_reminders_list_id ON reminders(list_id) WHERE deleted_at IS NULL;

-- Alarm flag for critical notifications that bypass DND
ALTER TABLE reminders ADD COLUMN IF NOT EXISTS is_alarm BOOLEAN DEFAULT FALSE;

-- Track when notification was sent to prevent duplicates
ALTER TABLE reminders ADD COLUMN IF NOT EXISTS notification_sent_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_reminders_notification_sent ON reminders(notification_sent_at) WHERE deleted_at IS NULL;

-- Tags for cross-list filtering
ALTER TABLE reminders ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';
