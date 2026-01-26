package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/user/remind-me/backend/internal/config"
	"github.com/user/remind-me/backend/internal/database"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Running database migrations for Lists feature...")

	// 1. Create the reminder_lists table using raw SQL
	log.Println("Creating reminder_lists table...")
	createListsTableSQL := `
		CREATE TABLE IF NOT EXISTS reminder_lists (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			name VARCHAR(100) NOT NULL,
			color_hex VARCHAR(7) DEFAULT '#007AFF',
			icon_name VARCHAR(50) DEFAULT 'list.bullet',
			sort_order INT DEFAULT 0,
			is_default BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT fk_reminder_lists_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`
	if err := db.Exec(createListsTableSQL).Error; err != nil {
		log.Fatalf("Failed to create reminder_lists table: %v", err)
	}
	log.Println("  ✓ reminder_lists table created")

	// Create indexes for reminder_lists
	log.Println("Creating indexes for reminder_lists...")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_reminder_lists_user_id ON reminder_lists(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_reminder_lists_deleted_at ON reminder_lists(deleted_at)")
	log.Println("  ✓ Indexes created")

	// 2. Add list_id column to reminders table if it doesn't exist
	log.Println("Adding list_id column to reminders table...")
	addListIdSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reminders' AND column_name = 'list_id'
			) THEN
				ALTER TABLE reminders ADD COLUMN list_id UUID;
			END IF;
		END $$;
	`
	if err := db.Exec(addListIdSQL).Error; err != nil {
		log.Fatalf("Failed to add list_id column: %v", err)
	}
	log.Println("  ✓ list_id column added (or already exists)")

	// 3. Add tags column to reminders table if it doesn't exist
	log.Println("Adding tags column to reminders table...")
	addTagsSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reminders' AND column_name = 'tags'
			) THEN
				ALTER TABLE reminders ADD COLUMN tags TEXT[] DEFAULT '{}';
			END IF;
		END $$;
	`
	if err := db.Exec(addTagsSQL).Error; err != nil {
		log.Fatalf("Failed to add tags column: %v", err)
	}
	log.Println("  ✓ tags column added (or already exists)")

	// 4. Add is_alarm column to reminders table if it doesn't exist
	log.Println("Adding is_alarm column to reminders table...")
	addIsAlarmSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reminders' AND column_name = 'is_alarm'
			) THEN
				ALTER TABLE reminders ADD COLUMN is_alarm BOOLEAN DEFAULT false;
			END IF;
		END $$;
	`
	if err := db.Exec(addIsAlarmSQL).Error; err != nil {
		log.Fatalf("Failed to add is_alarm column: %v", err)
	}
	log.Println("  ✓ is_alarm column added (or already exists)")

	// 5. Add notification_sent_at column to reminders table if it doesn't exist
	log.Println("Adding notification_sent_at column to reminders table...")
	addNotificationSentAtSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reminders' AND column_name = 'notification_sent_at'
			) THEN
				ALTER TABLE reminders ADD COLUMN notification_sent_at TIMESTAMP WITH TIME ZONE;
			END IF;
		END $$;
	`
	if err := db.Exec(addNotificationSentAtSQL).Error; err != nil {
		log.Fatalf("Failed to add notification_sent_at column: %v", err)
	}
	log.Println("  ✓ notification_sent_at column added (or already exists)")

	// 6. Create indexes
	log.Println("Creating indexes...")
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_reminders_list_id ON reminders(list_id)").Error; err != nil {
		log.Printf("  Warning: Could not create list_id index: %v", err)
	} else {
		log.Println("  ✓ list_id index created")
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_reminders_notification_sent ON reminders(notification_sent_at) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("  Warning: Could not create notification_sent_at index: %v", err)
	} else {
		log.Println("  ✓ notification_sent_at index created")
	}

	// 7. Add device_identifier column to devices table
	log.Println("Adding device_identifier column to devices table...")
	addDeviceIdentifierSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'devices' AND column_name = 'device_identifier'
			) THEN
				-- Add the column (nullable initially)
				ALTER TABLE devices ADD COLUMN device_identifier VARCHAR(255);

				-- Populate existing rows with their ID as the identifier
				UPDATE devices SET device_identifier = id::text WHERE device_identifier IS NULL;

				-- Make the column non-null
				ALTER TABLE devices ALTER COLUMN device_identifier SET NOT NULL;
			END IF;
		END $$;
	`
	if err := db.Exec(addDeviceIdentifierSQL).Error; err != nil {
		log.Fatalf("Failed to add device_identifier column: %v", err)
	}
	log.Println("  ✓ device_identifier column added (or already exists)")

	// 8. Update unique constraint on devices table
	log.Println("Updating unique constraint on devices table...")
	// Drop old constraint if it exists
	db.Exec("ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_user_id_push_token_key")
	// Create new unique index on (user_id, device_identifier)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_device_identifier ON devices(user_id, device_identifier)").Error; err != nil {
		log.Printf("  Warning: Could not create device_identifier unique index: %v", err)
	} else {
		log.Println("  ✓ device_identifier unique index created")
	}

	// 9. Fix device push_token uniqueness (migration 010)
	// This fixes duplicate push notifications by ensuring each push_token is unique per user
	log.Println("Fixing device push_token uniqueness...")

	// Clean up any duplicate push tokens by keeping only the most recently seen device
	cleanupDuplicatePushTokensSQL := `
		DELETE FROM devices d1
		USING devices d2
		WHERE d1.user_id = d2.user_id
		  AND d1.push_token = d2.push_token
		  AND d1.id != d2.id
		  AND d1.last_seen_at < d2.last_seen_at;
	`
	if result := db.Exec(cleanupDuplicatePushTokensSQL); result.Error != nil {
		log.Printf("  Warning: Could not clean up duplicate push tokens: %v", result.Error)
	} else {
		log.Printf("  ✓ Cleaned up %d duplicate push token entries", result.RowsAffected)
	}

	// Add unique constraint on (user_id, push_token)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_user_push_token ON devices(user_id, push_token)").Error; err != nil {
		log.Printf("  Warning: Could not create push_token unique index: %v", err)
	} else {
		log.Println("  ✓ push_token unique index created")
	}

	// Clean up any devices with duplicate device_identifiers across users (keep most recent)
	cleanupDuplicateDeviceIdentifiersSQL := `
		DELETE FROM devices d1
		USING devices d2
		WHERE d1.device_identifier = d2.device_identifier
		  AND d1.id != d2.id
		  AND d1.last_seen_at < d2.last_seen_at;
	`
	if result := db.Exec(cleanupDuplicateDeviceIdentifiersSQL); result.Error != nil {
		log.Printf("  Warning: Could not clean up duplicate device identifiers: %v", result.Error)
	} else {
		log.Printf("  ✓ Cleaned up %d duplicate device identifier entries", result.RowsAffected)
	}

	// Add global unique constraint on device_identifier (a device can only belong to one user)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_device_identifier_unique ON devices(device_identifier)").Error; err != nil {
		log.Printf("  Warning: Could not create global device_identifier unique index: %v", err)
	} else {
		log.Println("  ✓ Global device_identifier unique index created")
	}

	// 10. Create notification_sounds table (migration 011)
	log.Println("Creating notification_sounds table...")
	createNotificationSoundsSQL := `
		CREATE TABLE IF NOT EXISTS notification_sounds (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name       VARCHAR(100) NOT NULL,
			filename   VARCHAR(255) UNIQUE NOT NULL,
			is_free    BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);
	`
	if err := db.Exec(createNotificationSoundsSQL).Error; err != nil {
		log.Fatalf("Failed to create notification_sounds table: %v", err)
	}
	log.Println("  ✓ notification_sounds table created")

	// Create indexes for notification_sounds
	log.Println("Creating indexes for notification_sounds...")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_notification_sounds_is_free ON notification_sounds(is_free) WHERE deleted_at IS NULL")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_notification_sounds_deleted_at ON notification_sounds(deleted_at)")
	log.Println("  ✓ Indexes created")

	// Seed notification sounds (3 free, 5 premium)
	log.Println("Seeding notification sounds...")
	seedSoundsSQL := `
		INSERT INTO notification_sounds (name, filename, is_free)
		VALUES
			('Ambient', 'ambient.wav', TRUE),
			('Ambient Soft', 'ambient2.wav', FALSE),
			('Hop', 'hop.wav', TRUE),
			('Progressive', 'progressive.wav', FALSE),
			('Reverb', 'reverb.wav', FALSE),
			('Rock', 'rock.wav', TRUE),
			('Synth Pop', 'syntpop.wav', FALSE),
			('Techno', 'techno.wav', FALSE)
		ON CONFLICT (filename) DO NOTHING;
	`
	if result := db.Exec(seedSoundsSQL); result.Error != nil {
		log.Printf("  Warning: Could not seed notification sounds: %v", result.Error)
	} else {
		log.Printf("  ✓ Seeded %d notification sounds", result.RowsAffected)
	}

	// 11. Add sound_id column to reminders table
	log.Println("Adding sound_id column to reminders table...")
	addSoundIdSQL := `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reminders' AND column_name = 'sound_id'
			) THEN
				ALTER TABLE reminders ADD COLUMN sound_id VARCHAR(50);
			END IF;
		END $$;
	`
	if err := db.Exec(addSoundIdSQL).Error; err != nil {
		log.Fatalf("Failed to add sound_id column: %v", err)
	}
	log.Println("  ✓ sound_id column added (or already exists)")

	log.Println("")
	log.Println("========================================")
	log.Println("Migrations completed successfully!")
	log.Println("========================================")
	log.Println("")
	log.Println("New tables:")
	log.Println("  - reminder_lists")
	log.Println("    - id (UUID, primary key)")
	log.Println("    - user_id (UUID, foreign key)")
	log.Println("    - name (VARCHAR 100)")
	log.Println("    - color_hex (VARCHAR 7)")
	log.Println("    - icon_name (VARCHAR 50)")
	log.Println("    - sort_order (INT)")
	log.Println("    - is_default (BOOL)")
	log.Println("    - created_at, updated_at, deleted_at")
	log.Println("")
	log.Println("  - notification_sounds")
	log.Println("    - id (UUID, primary key)")
	log.Println("    - name (VARCHAR 100)")
	log.Println("    - filename (VARCHAR 255, unique)")
	log.Println("    - is_free (BOOLEAN)")
	log.Println("    - created_at, updated_at, deleted_at")
	log.Println("")
	log.Println("Updated tables:")
	log.Println("  - reminders")
	log.Println("    - list_id (UUID, nullable)")
	log.Println("    - tags (TEXT[])")
	log.Println("    - is_alarm (BOOLEAN)")
	log.Println("    - sound_id (VARCHAR 50)")
	log.Println("    - notification_sent_at (TIMESTAMP)")
}
