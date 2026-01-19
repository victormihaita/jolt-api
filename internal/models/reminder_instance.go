package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InstanceStatus string

const (
	InstancePending   InstanceStatus = "pending"
	InstanceNotified  InstanceStatus = "notified"
	InstanceCompleted InstanceStatus = "completed"
	InstanceSnoozed   InstanceStatus = "snoozed"
	InstanceDismissed InstanceStatus = "dismissed"
)

// ReminderInstance represents a single occurrence of a recurring reminder
type ReminderInstance struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ReminderID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"reminder_id"`
	ScheduledAt  time.Time      `gorm:"not null;index" json:"scheduled_at"`
	Status       InstanceStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	SnoozedUntil *time.Time     `json:"snoozed_until,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`

	// Relations
	Reminder *Reminder `gorm:"foreignKey:ReminderID" json:"-"`
}

func (ri *ReminderInstance) BeforeCreate(tx *gorm.DB) error {
	if ri.ID == uuid.Nil {
		ri.ID = uuid.New()
	}
	return nil
}

func (ri *ReminderInstance) Complete() {
	now := time.Now()
	ri.Status = InstanceCompleted
	ri.CompletedAt = &now
}

func (ri *ReminderInstance) Dismiss() {
	ri.Status = InstanceDismissed
}

func (ri *ReminderInstance) Snooze(duration time.Duration) {
	until := time.Now().Add(duration)
	ri.Status = InstanceSnoozed
	ri.SnoozedUntil = &until
}

func (ri *ReminderInstance) MarkNotified() {
	ri.Status = InstanceNotified
}
