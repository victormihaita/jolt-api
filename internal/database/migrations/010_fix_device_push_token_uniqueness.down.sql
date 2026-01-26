-- Rollback migration 010
DROP INDEX IF EXISTS idx_devices_user_push_token;
DROP INDEX IF EXISTS idx_devices_device_identifier_unique;
