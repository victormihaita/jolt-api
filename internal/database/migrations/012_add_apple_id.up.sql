-- Add Apple ID column for Sign in with Apple authentication
ALTER TABLE users ADD COLUMN IF NOT EXISTS apple_id VARCHAR(255);

-- Make google_id nullable (for Apple-only users)
ALTER TABLE users ALTER COLUMN google_id DROP NOT NULL;

-- Add unique index on apple_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_apple_id ON users(apple_id) WHERE apple_id IS NOT NULL;
