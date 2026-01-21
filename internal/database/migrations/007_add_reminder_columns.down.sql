-- Remove columns added in 006_add_reminder_columns.up.sql

DROP INDEX IF EXISTS idx_reminders_list_id;
DROP INDEX IF EXISTS idx_reminders_notification_sent;

ALTER TABLE reminders DROP COLUMN IF EXISTS list_id;
ALTER TABLE reminders DROP COLUMN IF EXISTS is_alarm;
ALTER TABLE reminders DROP COLUMN IF EXISTS notification_sent_at;
ALTER TABLE reminders DROP COLUMN IF EXISTS tags;
