-- Reminder Lists table for organizing reminders
CREATE TABLE IF NOT EXISTS reminder_lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    color_hex   VARCHAR(7) DEFAULT '#007AFF',
    icon_name   VARCHAR(50) DEFAULT 'list.bullet',
    sort_order  INTEGER DEFAULT 0,
    is_default  BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at  TIMESTAMP WITH TIME ZONE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_reminder_lists_user_id ON reminder_lists(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reminder_lists_deleted_at ON reminder_lists(deleted_at);

-- Ensure only one default list per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_reminder_lists_user_default ON reminder_lists(user_id) WHERE is_default = TRUE AND deleted_at IS NULL;
