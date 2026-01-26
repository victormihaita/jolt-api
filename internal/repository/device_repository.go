package repository

import (
	"log"
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

// FindByDeviceIdentifier finds a device by its identifier regardless of user
func (r *DeviceRepository) FindByDeviceIdentifier(deviceIdentifier string) (*models.Device, error) {
	var device models.Device
	err := r.db.Where("device_identifier = ?", deviceIdentifier).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// DeleteByDeviceIdentifier removes a device by its identifier (used for unlinking)
func (r *DeviceRepository) DeleteByDeviceIdentifier(deviceIdentifier string) error {
	return r.db.Where("device_identifier = ?", deviceIdentifier).Delete(&models.Device{}).Error
}

// DeleteByPushToken removes a device by its push token (used when APNs returns invalid token)
func (r *DeviceRepository) DeleteByPushToken(pushToken string) error {
	return r.db.Where("push_token = ?", pushToken).Delete(&models.Device{}).Error
}

// DeleteStaleDevices removes devices not seen in the specified number of days
func (r *DeviceRepository) DeleteStaleDevices(days int) (int64, error) {
	staleThreshold := time.Now().AddDate(0, 0, -days)
	result := r.db.Where("last_seen_at < ?", staleThreshold).Delete(&models.Device{})
	return result.RowsAffected, result.Error
}

func (r *DeviceRepository) Upsert(device *models.Device) error {
	// Step 1: Check if this device_identifier exists for a DIFFERENT user
	// If so, unlink it from that user (device can only belong to one account)
	var existingForOtherUser models.Device
	err := r.db.Where("device_identifier = ? AND user_id != ?", device.DeviceIdentifier, device.UserID).First(&existingForOtherUser).Error
	if err == nil {
		log.Printf("[DeviceRepository] Device %s was linked to user %s, unlinking before linking to user %s",
			device.DeviceIdentifier, existingForOtherUser.UserID, device.UserID)
		r.db.Delete(&existingForOtherUser)
	}

	// Step 2: Check if this push_token exists for current user with a DIFFERENT device_identifier
	// This handles the case where device_identifier changed but push_token stayed the same
	var existingWithSameToken models.Device
	err = r.db.Where("user_id = ? AND push_token = ? AND device_identifier != ?",
		device.UserID, device.PushToken, device.DeviceIdentifier).First(&existingWithSameToken).Error
	if err == nil {
		log.Printf("[DeviceRepository] Removing stale device entry with same push_token but different identifier: %s",
			existingWithSameToken.DeviceIdentifier)
		r.db.Delete(&existingWithSameToken)
	}

	// Step 3: Try to find existing device with same device identifier for current user
	var existing models.Device
	err = r.db.Where("user_id = ? AND device_identifier = ?", device.UserID, device.DeviceIdentifier).First(&existing).Error

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
	// Use DISTINCT to prevent duplicate notifications when same push_token
	// exists multiple times (e.g., after device_identifier changes)
	err := r.db.Model(&models.Device{}).
		Select("DISTINCT push_token, platform").
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
	// Use DISTINCT to prevent duplicate notifications when same push_token
	// exists multiple times (e.g., after device_identifier changes)
	err := r.db.Model(&models.Device{}).
		Select("DISTINCT push_token, platform").
		Where("user_id = ? AND id != ?", userID, excludeDeviceID).
		Find(&result).Error
	return result, err
}
