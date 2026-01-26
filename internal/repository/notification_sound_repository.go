package repository

import (
	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type NotificationSoundRepository struct {
	db *gorm.DB
}

func NewNotificationSoundRepository(db *gorm.DB) *NotificationSoundRepository {
	return &NotificationSoundRepository{db: db}
}

func (r *NotificationSoundRepository) ListAll() ([]models.NotificationSound, error) {
	var sounds []models.NotificationSound
	err := r.db.Order("name ASC").Find(&sounds).Error
	return sounds, err
}

func (r *NotificationSoundRepository) ListFree() ([]models.NotificationSound, error) {
	var sounds []models.NotificationSound
	err := r.db.Where("is_free = ?", true).Order("name ASC").Find(&sounds).Error
	return sounds, err
}

func (r *NotificationSoundRepository) FindByID(id uuid.UUID) (*models.NotificationSound, error) {
	var sound models.NotificationSound
	err := r.db.Where("id = ?", id).First(&sound).Error
	if err != nil {
		return nil, err
	}
	return &sound, nil
}

func (r *NotificationSoundRepository) FindByFilename(filename string) (*models.NotificationSound, error) {
	var sound models.NotificationSound
	err := r.db.Where("filename = ?", filename).First(&sound).Error
	if err != nil {
		return nil, err
	}
	return &sound, nil
}

func (r *NotificationSoundRepository) Create(sound *models.NotificationSound) error {
	return r.db.Create(sound).Error
}

func (r *NotificationSoundRepository) Update(sound *models.NotificationSound) error {
	return r.db.Save(sound).Error
}

func (r *NotificationSoundRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.NotificationSound{}, id).Error
}
