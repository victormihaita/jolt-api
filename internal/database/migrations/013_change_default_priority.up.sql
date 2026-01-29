-- Change default priority from 0 (None) to 2 (Normal)
ALTER TABLE reminders ALTER COLUMN priority SET DEFAULT 2;
