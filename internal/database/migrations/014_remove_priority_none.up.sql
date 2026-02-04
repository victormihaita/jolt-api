-- Migrate existing reminders with priority 0 (None) to 1 (Low)
UPDATE reminders SET priority = 1 WHERE priority = 0;
