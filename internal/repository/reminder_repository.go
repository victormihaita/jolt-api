package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type ReminderRepository struct {
	db *gorm.DB
}

func NewReminderRepository(db *gorm.DB) *ReminderRepository {
	return &ReminderRepository{db: db}
}

func (r *ReminderRepository) Create(reminder *models.Reminder) error {
	return r.db.Create(reminder).Error
}

func (r *ReminderRepository) FindByID(id uuid.UUID) (*models.Reminder, error) {
	var reminder models.Reminder
	err := r.db.Where("id = ?", id).First(&reminder).Error
	if err != nil {
		return nil, err
	}
	return &reminder, nil
}

func (r *ReminderRepository) FindByIDAndUser(id, userID uuid.UUID) (*models.Reminder, error) {
	var reminder models.Reminder
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&reminder).Error
	if err != nil {
		return nil, err
	}
	return &reminder, nil
}

func (r *ReminderRepository) FindByLocalID(userID uuid.UUID, localID string) (*models.Reminder, error) {
	var reminder models.Reminder
	err := r.db.Where("user_id = ? AND local_id = ?", userID, localID).First(&reminder).Error
	if err != nil {
		return nil, err
	}
	return &reminder, nil
}

type ReminderListParams struct {
	UserID   uuid.UUID
	Status   *string
	FromDate *time.Time
	ToDate   *time.Time
	Page     int
	PageSize int
}

func (r *ReminderRepository) List(params ReminderListParams) ([]models.Reminder, int64, error) {
	var reminders []models.Reminder
	var total int64

	query := r.db.Model(&models.Reminder{}).Where("user_id = ?", params.UserID)

	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}
	if params.FromDate != nil {
		// Only filter by date if FromDate is set - reminders without dates won't match
		query = query.Where("due_at >= ?", *params.FromDate)
	}
	if params.ToDate != nil {
		// Only filter by date if ToDate is set - reminders without dates won't match
		query = query.Where("due_at <= ?", *params.ToDate)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Paginate - NULLS LAST puts reminders without dates at the end
	offset := (params.Page - 1) * params.PageSize
	err := query.
		Order("due_at ASC NULLS LAST").
		Offset(offset).
		Limit(params.PageSize).
		Find(&reminders).Error

	if err != nil {
		return nil, 0, err
	}

	return reminders, total, nil
}

func (r *ReminderRepository) ListActive(userID uuid.UUID) ([]models.Reminder, error) {
	var reminders []models.Reminder
	// Simplified: snoozed reminders now have status="active" with updated dueAt
	err := r.db.
		Where("user_id = ? AND status = ?", userID, models.StatusActive).
		Order("due_at ASC NULLS LAST"). // Reminders without dates go to the end
		Find(&reminders).Error
	return reminders, err
}

func (r *ReminderRepository) ListUpcoming(userID uuid.UUID, from, to time.Time) ([]models.Reminder, error) {
	var reminders []models.Reminder
	err := r.db.
		Where("user_id = ? AND due_at >= ? AND due_at <= ? AND status = ?", userID, from, to, "active").
		Order("due_at ASC").
		Find(&reminders).Error
	return reminders, err
}

func (r *ReminderRepository) Update(reminder *models.Reminder) error {
	return r.db.Save(reminder).Error
}

func (r *ReminderRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Reminder{}, id).Error
}

func (r *ReminderRepository) DeleteByUserID(userID uuid.UUID) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.Reminder{}).Error
}

func (r *ReminderRepository) SoftDelete(id uuid.UUID) error {
	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

func (r *ReminderRepository) Snooze(id uuid.UUID, until time.Time, deviceID *uuid.UUID) error {
	updates := map[string]interface{}{
		// Keep status as "active" - simplified snooze model
		"status": models.StatusActive,
		// Update dueAt directly instead of snoozedUntil
		"due_at": until,
		// Clear snoozedUntil for backward compat
		"snoozed_until":        nil,
		"snooze_count":         gorm.Expr("snooze_count + 1"),
		"notification_sent_at": nil, // Clear so a new notification will be sent after snooze
	}
	if deviceID != nil {
		updates["last_modified_by"] = deviceID
	}

	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *ReminderRepository) Complete(id uuid.UUID, deviceID *uuid.UUID) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.StatusCompleted,
		"completed_at": now,
	}
	if deviceID != nil {
		updates["last_modified_by"] = deviceID
	}

	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *ReminderRepository) Dismiss(id uuid.UUID, deviceID *uuid.UUID) error {
	updates := map[string]interface{}{
		"status": models.StatusDismissed,
	}
	if deviceID != nil {
		updates["last_modified_by"] = deviceID
	}

	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *ReminderRepository) Reactivate(id uuid.UUID) error {
	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.StatusActive,
			"snoozed_until": nil,
		}).Error
}

func (r *ReminderRepository) GetDueReminders(before time.Time) ([]models.Reminder, error) {
	var reminders []models.Reminder
	// Only include active reminders with scheduled dates (due_at IS NOT NULL)
	// Simplified: snoozed reminders now have updated dueAt with status="active"
	err := r.db.
		Where("due_at IS NOT NULL AND status = ? AND due_at <= ?", models.StatusActive, before).
		Preload("User").
		Find(&reminders).Error
	return reminders, err
}

func (r *ReminderRepository) CountByUser(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Reminder{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// FindDueForNotification finds reminders that are due and haven't been notified yet
// It looks for active reminders where due_at is now or in the past AND notification_sent_at IS NULL
// Note: Reminders without scheduled dates (due_at IS NULL) are excluded from notifications
// Simplified: snoozed reminders now have updated dueAt with status="active"
func (r *ReminderRepository) FindDueForNotification(now time.Time) ([]models.Reminder, error) {
	var reminders []models.Reminder
	err := r.db.
		Where("due_at IS NOT NULL AND status = ? AND due_at <= ? AND notification_sent_at IS NULL", models.StatusActive, now).
		Preload("User").
		Find(&reminders).Error
	return reminders, err
}

// MarkNotificationSent marks a reminder as having its notification sent
func (r *ReminderRepository) MarkNotificationSent(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Update("notification_sent_at", now).Error
}

// ClearNotificationSent clears the notification_sent_at field (used when rescheduling after snooze)
func (r *ReminderRepository) ClearNotificationSent(id uuid.UUID) error {
	return r.db.Model(&models.Reminder{}).
		Where("id = ?", id).
		Update("notification_sent_at", nil).Error
}

// RestoreByUserID restores all soft-deleted reminders for a user
func (r *ReminderRepository) RestoreByUserID(userID uuid.UUID) error {
	return r.db.Unscoped().Model(&models.Reminder{}).
		Where("user_id = ? AND deleted_at IS NOT NULL", userID).
		Update("deleted_at", nil).Error
}
