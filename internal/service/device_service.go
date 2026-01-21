package service

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

type DeviceService struct {
	deviceRepo *repository.DeviceRepository
	userRepo   *repository.UserRepository
}

func NewDeviceService(deviceRepo *repository.DeviceRepository, userRepo *repository.UserRepository) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
	}
}

// Register registers a new device or updates an existing one.
func (s *DeviceService) Register(userID uuid.UUID, req dto.RegisterDeviceRequest) (*dto.DeviceDTO, error) {
	// Validate push token is not empty
	if req.PushToken == "" {
		return nil, apperrors.ValidationError("Push token is required")
	}

	// Check device limit for non-premium users
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	count, err := s.deviceRepo.CountByUser(userID)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to count devices", http.StatusInternalServerError)
	}

	// Free users are limited to 2 devices
	if !user.HasActivePremium() && count >= 2 {
		// Check if this token already exists (update case)
		_, existingErr := s.deviceRepo.FindByPushToken(userID, req.PushToken)
		if existingErr != nil {
			return nil, apperrors.New(apperrors.CodePremiumRequired, "Device limit reached. Upgrade to premium for unlimited devices.", http.StatusForbidden)
		}
	}

	device := &models.Device{
		UserID:     userID,
		Platform:   models.Platform(req.Platform),
		PushToken:  req.PushToken,
		DeviceName: req.DeviceName,
		AppVersion: req.AppVersion,
		OSVersion:  req.OSVersion,
		LastSeenAt: time.Now(),
	}

	if err := s.deviceRepo.Upsert(device); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to register device", http.StatusInternalServerError)
	}

	return deviceToDTO(device), nil
}

// Unregister removes a device.
func (s *DeviceService) Unregister(userID, deviceID uuid.UUID) error {
	// Verify the device belongs to the user
	_, err := s.deviceRepo.FindByIDAndUser(deviceID, userID)
	if err != nil {
		return apperrors.New(apperrors.CodeNotFound, "Device not found", http.StatusNotFound)
	}

	if err := s.deviceRepo.Delete(deviceID); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to unregister device", http.StatusInternalServerError)
	}

	return nil
}

// ListByUser returns all devices for a user.
func (s *DeviceService) ListByUser(userID uuid.UUID) ([]dto.DeviceDTO, error) {
	devices, err := s.deviceRepo.ListByUser(userID)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to list devices", http.StatusInternalServerError)
	}

	result := make([]dto.DeviceDTO, len(devices))
	for i, d := range devices {
		result[i] = *deviceToDTO(&d)
	}
	return result, nil
}

// GetByID returns a single device by ID.
func (s *DeviceService) GetByID(userID, deviceID uuid.UUID) (*dto.DeviceDTO, error) {
	device, err := s.deviceRepo.FindByIDAndUser(deviceID, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.CodeNotFound, "Device not found", http.StatusNotFound)
	}
	return deviceToDTO(device), nil
}

// UpdateLastSeen updates the last seen timestamp for a device.
func (s *DeviceService) UpdateLastSeen(deviceID uuid.UUID) error {
	return s.deviceRepo.UpdateLastSeen(deviceID)
}

// GetPushTokens returns all push tokens for a user.
func (s *DeviceService) GetPushTokens(userID uuid.UUID) (map[models.Platform][]string, error) {
	tokens, err := s.deviceRepo.GetAllPushTokens(userID)
	if err != nil {
		return nil, err
	}

	result := make(map[models.Platform][]string)
	for _, t := range tokens {
		result[t.Platform] = append(result[t.Platform], t.PushToken)
	}
	return result, nil
}

func deviceToDTO(d *models.Device) *dto.DeviceDTO {
	return &dto.DeviceDTO{
		ID:         d.ID,
		Platform:   string(d.Platform),
		DeviceName: d.DeviceName,
		AppVersion: d.AppVersion,
		OSVersion:  d.OSVersion,
		LastSeenAt: d.LastSeenAt,
		CreatedAt:  d.CreatedAt,
	}
}
