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

	// 4. Create index on list_id if it doesn't exist
	log.Println("Creating index on reminders.list_id...")
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_reminders_list_id ON reminders(list_id)").Error; err != nil {
		log.Printf("  Warning: Could not create index: %v", err)
	} else {
		log.Println("  ✓ Index created")
	}

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
	log.Println("Updated tables:")
	log.Println("  - reminders")
	log.Println("    - list_id (UUID, nullable)")
	log.Println("    - tags (TEXT[])")
}
