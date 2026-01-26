-- Revert: Make due_at and all_day columns NOT NULL again
-- Note: This will fail if there are any NULL values in the database

-- First, set default values for any NULL entries
UPDATE reminders SET due_at = created_at WHERE due_at IS NULL;
UPDATE reminders SET all_day = FALSE WHERE all_day IS NULL;

-- Recreate original index
DROP INDEX IF EXISTS idx_reminders_user_due;
CREATE INDEX idx_reminders_user_due ON reminders(user_id, due_at) WHERE deleted_at IS NULL;

-- Make columns NOT NULL again
ALTER TABLE reminders ALTER COLUMN due_at SET NOT NULL;
ALTER TABLE reminders ALTER COLUMN all_day SET NOT NULL;
