package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
)

// CreateReminderRequest is the request body for creating a reminder
type CreateReminderRequest struct {
	Title          string                  `json:"title" binding:"required,max=500"`
	Notes          *string                 `json:"notes,omitempty"`
	Priority       *int                    `json:"priority,omitempty"`
	DueAt          time.Time               `json:"due_at" binding:"required"`
	AllDay         bool                    `json:"all_day"`
	RecurrenceRule *models.RecurrenceRule  `json:"recurrence_rule,omitempty"`
	RecurrenceEnd  *time.Time              `json:"recurrence_end,omitempty"`
	LocalID        *string                 `json:"local_id,omitempty"`
}

// UpdateReminderRequest is the request body for updating a reminder
type UpdateReminderRequest struct {
	Title          *string                 `json:"title,omitempty" binding:"omitempty,max=500"`
	Notes          *string                 `json:"notes,omitempty"`
	Priority       *int                    `json:"priority,omitempty"`
	DueAt          *time.Time              `json:"due_at,omitempty"`
	AllDay         *bool                   `json:"all_day,omitempty"`
	RecurrenceRule *models.RecurrenceRule  `json:"recurrence_rule,omitempty"`
	RecurrenceEnd  *time.Time              `json:"recurrence_end,omitempty"`
	Status         *string                 `json:"status,omitempty"`
}

// SnoozeReminderRequest is the request body for snoozing a reminder
type SnoozeReminderRequest struct {
	Minutes int `json:"minutes" binding:"required,min=1,max=10080"` // Max 1 week
}

// ReminderDTO represents a reminder in responses
type ReminderDTO struct {
	ID             uuid.UUID               `json:"id"`
	Title          string                  `json:"title"`
	Notes          *string                 `json:"notes,omitempty"`
	Priority       int                     `json:"priority"`
	DueAt          time.Time               `json:"due_at"`
	AllDay         bool                    `json:"all_day"`
	RecurrenceRule *models.RecurrenceRule  `json:"recurrence_rule,omitempty"`
	RecurrenceEnd  *time.Time              `json:"recurrence_end,omitempty"`
	Status         string                  `json:"status"`
	CompletedAt    *time.Time              `json:"completed_at,omitempty"`
	SnoozedUntil   *time.Time              `json:"snoozed_until,omitempty"`
	SnoozeCount    int                     `json:"snooze_count"`
	LocalID        *string                 `json:"local_id,omitempty"`
	Version        int                     `json:"version"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// ReminderListResponse is the response for listing reminders
type ReminderListResponse struct {
	Reminders  []ReminderDTO `json:"reminders"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// ToDTO converts a Reminder model to ReminderDTO
func ReminderToDTO(r *models.Reminder) ReminderDTO {
	return ReminderDTO{
		ID:             r.ID,
		Title:          r.Title,
		Notes:          r.Notes,
		Priority:       int(r.Priority),
		DueAt:          r.DueAt,
		AllDay:         r.AllDay,
		RecurrenceRule: r.RecurrenceRule,
		RecurrenceEnd:  r.RecurrenceEnd,
		Status:         string(r.Status),
		CompletedAt:    r.CompletedAt,
		SnoozedUntil:   r.SnoozedUntil,
		SnoozeCount:    r.SnoozeCount,
		LocalID:        r.LocalID,
		Version:        r.Version,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// RemindersToDTO converts a slice of Reminder models to DTOs
func RemindersToDTO(reminders []models.Reminder) []ReminderDTO {
	dtos := make([]ReminderDTO, len(reminders))
	for i, r := range reminders {
		dtos[i] = ReminderToDTO(&r)
	}
	return dtos
}
