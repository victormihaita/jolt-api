-- Rollback: Remove device_identifier column

-- Drop the new unique constraint
DROP INDEX IF EXISTS idx_user_device_identifier;

-- Restore the old unique constraint
ALTER TABLE devices ADD CONSTRAINT devices_user_id_push_token_key UNIQUE (user_id, push_token);

-- Drop the column
ALTER TABLE devices DROP COLUMN IF EXISTS device_identifier;
