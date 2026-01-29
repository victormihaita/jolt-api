package repository

import (
	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type ReminderListRepository struct {
	db *gorm.DB
}

func NewReminderListRepository(db *gorm.DB) *ReminderListRepository {
	return &ReminderListRepository{db: db}
}

func (r *ReminderListRepository) Create(list *models.ReminderList) error {
	return r.db.Create(list).Error
}

func (r *ReminderListRepository) FindByID(id uuid.UUID) (*models.ReminderList, error) {
	var list models.ReminderList
	err := r.db.Where("id = ?", id).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *ReminderListRepository) FindByIDAndUser(id, userID uuid.UUID) (*models.ReminderList, error) {
	var list models.ReminderList
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *ReminderListRepository) FindDefaultByUser(userID uuid.UUID) (*models.ReminderList, error) {
	var list models.ReminderList
	err := r.db.Where("user_id = ? AND is_default = ?", userID, true).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *ReminderListRepository) ListByUser(userID uuid.UUID) ([]models.ReminderList, error) {
	var lists []models.ReminderList
	err := r.db.
		Where("user_id = ?", userID).
		Order("sort_order ASC, created_at ASC").
		Find(&lists).Error
	return lists, err
}

func (r *ReminderListRepository) Update(list *models.ReminderList) error {
	return r.db.Save(list).Error
}

func (r *ReminderListRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.ReminderList{}, id).Error
}

func (r *ReminderListRepository) DeleteByUserID(userID uuid.UUID) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.ReminderList{}).Error
}

func (r *ReminderListRepository) SoftDelete(id uuid.UUID) error {
	return r.db.Model(&models.ReminderList{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

func (r *ReminderListRepository) CountByUser(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.ReminderList{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// UpdateSortOrders updates the sort order of multiple lists in a single transaction
func (r *ReminderListRepository) UpdateSortOrders(userID uuid.UUID, idOrders map[uuid.UUID]int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for id, order := range idOrders {
			if err := tx.Model(&models.ReminderList{}).
				Where("id = ? AND user_id = ?", id, userID).
				Update("sort_order", order).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetReminderCountForList returns the count of active reminders in a list
func (r *ReminderListRepository) GetReminderCountForList(listID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Reminder{}).
		Where("list_id = ? AND status IN ?", listID, []string{"active", "snoozed"}).
		Count(&count).Error
	return count, err
}

// EnsureDefaultListExists creates the default list for a user if it doesn't exist
func (r *ReminderListRepository) EnsureDefaultListExists(userID uuid.UUID) (*models.ReminderList, error) {
	// Try to find existing default list
	existing, err := r.FindDefaultByUser(userID)
	if err == nil {
		return existing, nil
	}

	// Create default list
	defaultList := models.CreateDefaultList(userID)
	if err := r.Create(defaultList); err != nil {
		return nil, err
	}

	return defaultList, nil
}

// MoveRemindersToList moves all reminders from one list to another
func (r *ReminderListRepository) MoveRemindersToList(fromListID, toListID uuid.UUID) error {
	return r.db.Model(&models.Reminder{}).
		Where("list_id = ?", fromListID).
		Update("list_id", toListID).Error
}

// DeleteRemindersByListID deletes all reminders belonging to a list
func (r *ReminderListRepository) DeleteRemindersByListID(listID uuid.UUID) error {
	return r.db.Where("list_id = ?", listID).Delete(&models.Reminder{}).Error
}

// RestoreByUserID restores all soft-deleted reminder lists for a user
func (r *ReminderListRepository) RestoreByUserID(userID uuid.UUID) error {
	return r.db.Unscoped().Model(&models.ReminderList{}).
		Where("user_id = ? AND deleted_at IS NOT NULL", userID).
		Update("deleted_at", nil).Error
}
