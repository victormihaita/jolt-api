package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
)

// CreateReminderListRequest is the request body for creating a reminder list
type CreateReminderListRequest struct {
	Name     string  `json:"name" binding:"required,max=100"`
	ColorHex *string `json:"color_hex,omitempty"`
	IconName *string `json:"icon_name,omitempty"`
}

// UpdateReminderListRequest is the request body for updating a reminder list
type UpdateReminderListRequest struct {
	Name      *string `json:"name,omitempty" binding:"omitempty,max=100"`
	ColorHex  *string `json:"color_hex,omitempty"`
	IconName  *string `json:"icon_name,omitempty"`
	SortOrder *int    `json:"sort_order,omitempty"`
}

// ReminderListDTO represents a reminder list in responses
type ReminderListDTO struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	ColorHex      string    `json:"color_hex"`
	IconName      string    `json:"icon_name"`
	SortOrder     int       `json:"sort_order"`
	IsDefault     bool      `json:"is_default"`
	ReminderCount int64     `json:"reminder_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ReminderListToDTO converts a ReminderList model to ReminderListDTO
func ReminderListToDTO(l *models.ReminderList, reminderCount int64) ReminderListDTO {
	return ReminderListDTO{
		ID:            l.ID,
		Name:          l.Name,
		ColorHex:      l.ColorHex,
		IconName:      l.IconName,
		SortOrder:     l.SortOrder,
		IsDefault:     l.IsDefault,
		ReminderCount: reminderCount,
		CreatedAt:     l.CreatedAt,
		UpdatedAt:     l.UpdatedAt,
	}
}

// ReminderListsToDTO converts a slice of ReminderList models to DTOs
func ReminderListsToDTO(lists []models.ReminderList, getCounts func(uuid.UUID) int64) []ReminderListDTO {
	dtos := make([]ReminderListDTO, len(lists))
	for i, l := range lists {
		count := int64(0)
		if getCounts != nil {
			count = getCounts(l.ID)
		}
		dtos[i] = ReminderListToDTO(&l, count)
	}
	return dtos
}
