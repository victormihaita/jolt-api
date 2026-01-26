-- Make due_at and all_day columns nullable for reminders without scheduled dates
-- Reminders without dates won't trigger notifications

-- Make due_at nullable
ALTER TABLE reminders ALTER COLUMN due_at DROP NOT NULL;

-- Make all_day nullable (defaults to NULL when no date is set)
-- Note: all_day already has a default of FALSE, but we need to allow NULL
-- when there's no due_at set
ALTER TABLE reminders ALTER COLUMN all_day DROP NOT NULL;

-- Update the index to handle NULL due_at values
-- Reminders without dates will be excluded from time-based queries automatically
DROP INDEX IF EXISTS idx_reminders_user_due;
CREATE INDEX idx_reminders_user_due ON reminders(user_id, due_at)
WHERE deleted_at IS NULL AND due_at IS NOT NULL;
