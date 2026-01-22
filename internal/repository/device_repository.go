package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Create(device *models.Device) error {
	return r.db.Create(device).Error
}

func (r *DeviceRepository) FindByID(id uuid.UUID) (*models.Device, error) {
	var device models.Device
	err := r.db.Where("id = ?", id).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) FindByIDAndUser(id, userID uuid.UUID) (*models.Device, error) {
	var device models.Device
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) FindByPushToken(userID uuid.UUID, pushToken string) (*models.Device, error) {
	var device models.Device
	err := r.db.Where("user_id = ? AND push_token = ?", userID, pushToken).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) ListByUser(userID uuid.UUID) ([]models.Device, error) {
	var devices []models.Device
	err := r.db.Where("user_id = ?", userID).Order("last_seen_at DESC").Find(&devices).Error
	return devices, err
}

func (r *DeviceRepository) ListByPlatform(userID uuid.UUID, platform models.Platform) ([]models.Device, error) {
	var devices []models.Device
	err := r.db.
		Where("user_id = ? AND platform = ?", userID, platform).
		Find(&devices).Error
	return devices, err
}

func (r *DeviceRepository) Update(device *models.Device) error {
	return r.db.Save(device).Error
}

func (r *DeviceRepository) UpdateLastSeen(id uuid.UUID) error {
	return r.db.Model(&models.Device{}).
		Where("id = ?", id).
		Update("last_seen_at", time.Now()).Error
}

func (r *DeviceRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Device{}, id).Error
}

func (r *DeviceRepository) DeleteByUser(userID uuid.UUID) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.Device{}).Error
}

func (r *DeviceRepository) Upsert(device *models.Device) error {
	// Try to find existing device with same device identifier
	// This ensures we update the push token when it changes, instead of creating duplicates
	var existing models.Device
	err := r.db.Where("user_id = ? AND device_identifier = ?", device.UserID, device.DeviceIdentifier).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new device
		return r.db.Create(device).Error
	}

	if err != nil {
		return err
	}

	// Update existing device, including the push token which may have changed
	existing.PushToken = device.PushToken
	existing.DeviceName = device.DeviceName
	existing.AppVersion = device.AppVersion
	existing.OSVersion = device.OSVersion
	existing.LastSeenAt = time.Now()
	device.ID = existing.ID

	return r.db.Save(&existing).Error
}

func (r *DeviceRepository) CountByUser(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Device{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

func (r *DeviceRepository) GetAllPushTokens(userID uuid.UUID) ([]struct {
	Platform  models.Platform
	PushToken string
}, error) {
	var result []struct {
		Platform  models.Platform
		PushToken string
	}
	err := r.db.Model(&models.Device{}).
		Select("platform", "push_token").
		Where("user_id = ?", userID).
		Find(&result).Error
	return result, err
}

// GetPushTokensExcluding returns all push tokens for a user except the specified device
func (r *DeviceRepository) GetPushTokensExcluding(userID uuid.UUID, excludeDeviceID uuid.UUID) ([]struct {
	Platform  models.Platform
	PushToken string
}, error) {
	var result []struct {
		Platform  models.Platform
		PushToken string
	}
	err := r.db.Model(&models.Device{}).
		Select("platform", "push_token").
		Where("user_id = ? AND id != ?", userID, excludeDeviceID).
		Find(&result).Error
	return result, err
}
