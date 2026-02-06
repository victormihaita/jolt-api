-- Fix google_id unique constraint to allow multiple NULL/empty values
-- (matching the partial index pattern used for apple_id in migration 012)

-- Drop the original UNIQUE constraint from CREATE TABLE
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_google_id_key;

-- Drop any GORM-created unique index
DROP INDEX IF EXISTS idx_users_google_id;

-- Set any empty string google_id values to NULL for consistency
UPDATE users SET google_id = NULL WHERE google_id = '';

-- Create a partial unique index (only enforces uniqueness on non-NULL values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
