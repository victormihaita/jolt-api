-- Revert to original full unique index on google_id
DROP INDEX IF EXISTS idx_users_google_id;
CREATE UNIQUE INDEX idx_users_google_id ON users(google_id);
