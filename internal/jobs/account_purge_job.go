package jobs

import (
	"context"
	"log"
	"time"

	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

// AccountPurgeJob handles permanently deleting soft-deleted user accounts
type AccountPurgeJob struct {
	db *gorm.DB
}

// NewAccountPurgeJob creates a new account purge job handler
func NewAccountPurgeJob(db *gorm.DB) *AccountPurgeJob {
	return &AccountPurgeJob{
		db: db,
	}
}

// PurgeDeletedAccounts permanently deletes user accounts that were soft-deleted
// more than the specified number of days ago. This includes all associated data:
// reminders, reminder lists, devices, and the user record itself.
func (j *AccountPurgeJob) PurgeDeletedAccounts(ctx context.Context, days int) (int64, error) {
	log.Printf("[AccountPurgeJob] Starting purge of accounts deleted more than %d days ago", days)

	threshold := time.Now().AddDate(0, 0, -days)

	// Find soft-deleted users older than threshold
	var users []models.User
	if err := j.db.Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", threshold).
		Find(&users).Error; err != nil {
		log.Printf("[AccountPurgeJob] Error finding deleted users: %v", err)
		return 0, err
	}

	if len(users) == 0 {
		log.Printf("[AccountPurgeJob] No accounts to purge")
		return 0, nil
	}

	var purged int64
	for _, user := range users {
		select {
		case <-ctx.Done():
			log.Printf("[AccountPurgeJob] Context cancelled, purged %d accounts so far", purged)
			return purged, ctx.Err()
		default:
		}

		if err := j.purgeUser(user.ID); err != nil {
			log.Printf("[AccountPurgeJob] Error purging user %s: %v", user.ID, err)
			continue
		}
		purged++
	}

	log.Printf("[AccountPurgeJob] Purged %d accounts", purged)
	return purged, nil
}

// purgeUser permanently deletes a user and all associated data
func (j *AccountPurgeJob) purgeUser(userID interface{}) error {
	return j.db.Transaction(func(tx *gorm.DB) error {
		// Permanently delete reminders
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&models.Reminder{}).Error; err != nil {
			return err
		}

		// Permanently delete reminder lists
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&models.ReminderList{}).Error; err != nil {
			return err
		}

		// Permanently delete devices
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&models.Device{}).Error; err != nil {
			return err
		}

		// Permanently delete the user
		if err := tx.Unscoped().Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
			return err
		}

		return nil
	})
}
