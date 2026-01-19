package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
)

// RegisterDeviceRequest is the request body for registering a device
type RegisterDeviceRequest struct {
	Platform   string `json:"platform" binding:"required,oneof=ios android"`
	PushToken  string `json:"push_token" binding:"required"`
	DeviceName string `json:"device_name,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
}

// DeviceDTO represents a device in responses
type DeviceDTO struct {
	ID         uuid.UUID `json:"id"`
	Platform   string    `json:"platform"`
	DeviceName string    `json:"device_name,omitempty"`
	AppVersion string    `json:"app_version,omitempty"`
	OSVersion  string    `json:"os_version,omitempty"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// DeviceListResponse is the response for listing devices
type DeviceListResponse struct {
	Devices []DeviceDTO `json:"devices"`
	Total   int         `json:"total"`
}

// ToDTO converts a Device model to DeviceDTO
func DeviceToDTO(d *models.Device) DeviceDTO {
	return DeviceDTO{
		ID:         d.ID,
		Platform:   string(d.Platform),
		DeviceName: d.DeviceName,
		AppVersion: d.AppVersion,
		OSVersion:  d.OSVersion,
		LastSeenAt: d.LastSeenAt,
		CreatedAt:  d.CreatedAt,
	}
}

// DevicesToDTO converts a slice of Device models to DTOs
func DevicesToDTO(devices []models.Device) []DeviceDTO {
	dtos := make([]DeviceDTO, len(devices))
	for i, d := range devices {
		dtos[i] = DeviceToDTO(&d)
	}
	return dtos
}
