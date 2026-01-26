-- Migration 010: Fix device push_token uniqueness
-- This fixes duplicate push notifications by ensuring each push_token is unique per user

-- First, clean up any duplicate push tokens by keeping only the most recently seen device
-- This handles the case where the same physical device has multiple records
DELETE FROM devices d1
USING devices d2
WHERE d1.user_id = d2.user_id
  AND d1.push_token = d2.push_token
  AND d1.id != d2.id
  AND d1.last_seen_at < d2.last_seen_at;

-- Add unique constraint on (user_id, push_token) to prevent future duplicates
-- This was originally in migration 002 but was removed in 008
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_user_push_token
ON devices(user_id, push_token);

-- Also add a global unique constraint on device_identifier to ensure
-- a physical device can only be linked to one user at a time
-- First, clean up any devices with duplicate device_identifiers across users
-- Keep the most recently seen one
DELETE FROM devices d1
USING devices d2
WHERE d1.device_identifier = d2.device_identifier
  AND d1.id != d2.id
  AND d1.last_seen_at < d2.last_seen_at;

-- Now add the global unique constraint on device_identifier
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_device_identifier_unique
ON devices(device_identifier);
