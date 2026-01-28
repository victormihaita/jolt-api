-- Remove Apple ID column
DROP INDEX IF EXISTS idx_users_apple_id;

-- Restore google_id NOT NULL constraint (only if all rows have google_id)
-- Note: This may fail if there are Apple-only users
-- ALTER TABLE users ALTER COLUMN google_id SET NOT NULL;

ALTER TABLE users DROP COLUMN IF EXISTS apple_id;
