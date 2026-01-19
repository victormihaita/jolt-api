package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type SyncRepository struct {
	db *gorm.DB
}

func NewSyncRepository(db *gorm.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) Create(event *models.SyncEvent) error {
	return r.db.Create(event).Error
}

func (r *SyncRepository) CreateBatch(events []*models.SyncEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.Create(events).Error
}

func (r *SyncRepository) FindByID(id uuid.UUID) (*models.SyncEvent, error) {
	var event models.SyncEvent
	err := r.db.Where("id = ?", id).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

type SyncEventsParams struct {
	UserID    uuid.UUID
	Since     time.Time
	Limit     int
	ExcludeDevice *uuid.UUID
}

func (r *SyncRepository) GetEventsSince(params SyncEventsParams) ([]models.SyncEvent, error) {
	var events []models.SyncEvent

	query := r.db.Where("user_id = ? AND created_at > ?", params.UserID, params.Since)

	// Optionally exclude events from a specific device
	if params.ExcludeDevice != nil {
		query = query.Where("device_id IS NULL OR device_id != ?", *params.ExcludeDevice)
	}

	err := query.
		Order("created_at ASC").
		Limit(params.Limit).
		Find(&events).Error

	return events, err
}

func (r *SyncRepository) GetLatestEventTime(userID uuid.UUID) (*time.Time, error) {
	var event models.SyncEvent
	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&event).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &event.CreatedAt, nil
}

func (r *SyncRepository) DeleteOldEvents(before time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", before).Delete(&models.SyncEvent{})
	return result.RowsAffected, result.Error
}

func (r *SyncRepository) GetEventsForEntity(entityType models.EntityType, entityID uuid.UUID) ([]models.SyncEvent, error) {
	var events []models.SyncEvent
	err := r.db.
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}

// RecordReminderChange creates a sync event for a reminder change
func (r *SyncRepository) RecordReminderChange(userID uuid.UUID, reminder *models.Reminder, action models.SyncAction, deviceID *uuid.UUID) error {
	event := models.CreateSyncEvent(
		userID,
		models.EntityTypeReminder,
		reminder.ID,
		action,
		reminder,
		deviceID,
	)
	return r.Create(event)
}

// GetChangesSince returns sync events since a given time with pagination support
func (r *SyncRepository) GetChangesSince(userID uuid.UUID, since time.Time, limit int) ([]models.SyncEvent, bool, error) {
	var events []models.SyncEvent

	// Fetch one extra to determine if there are more
	err := r.db.
		Where("user_id = ? AND created_at > ?", userID, since).
		Order("created_at ASC").
		Limit(limit + 1).
		Find(&events).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}

	return events, hasMore, nil
}
