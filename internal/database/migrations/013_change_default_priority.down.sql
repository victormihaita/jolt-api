-- Revert default priority back to 0 (None)
ALTER TABLE reminders ALTER COLUMN priority SET DEFAULT 0;
