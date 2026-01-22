-- Add device_identifier column to devices table
-- This allows tracking physical devices across push token changes

-- Add the column (nullable initially for existing rows)
ALTER TABLE devices ADD COLUMN device_identifier VARCHAR(255);

-- For existing devices, use the device ID as the identifier (one-time migration)
UPDATE devices SET device_identifier = id::text WHERE device_identifier IS NULL;

-- Make the column non-null after populating existing rows
ALTER TABLE devices ALTER COLUMN device_identifier SET NOT NULL;

-- Drop the old unique constraint on (user_id, push_token)
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_user_id_push_token_key;

-- Add new unique constraint on (user_id, device_identifier)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_device_identifier ON devices(user_id, device_identifier);
