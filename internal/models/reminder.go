package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReminderStatus string

const (
	StatusActive    ReminderStatus = "active"
	StatusCompleted ReminderStatus = "completed"
	StatusSnoozed   ReminderStatus = "snoozed"
	StatusDismissed ReminderStatus = "dismissed"
)

type Priority int

const (
	PriorityNone   Priority = 0
	PriorityLow    Priority = 1
	PriorityMedium Priority = 2
	PriorityHigh   Priority = 3
)

type Frequency string

const (
	FrequencyHourly  Frequency = "hourly"
	FrequencyDaily   Frequency = "daily"
	FrequencyWeekly  Frequency = "weekly"
	FrequencyMonthly Frequency = "monthly"
	FrequencyYearly  Frequency = "yearly"
)

// RecurrenceRule defines how a reminder repeats
type RecurrenceRule struct {
	Frequency           Frequency `json:"frequency"`
	Interval            int       `json:"interval"`                        // Every N units (e.g., every 2 weeks)
	DaysOfWeek          []int     `json:"days_of_week,omitempty"`          // 0=Sunday, 1=Monday, etc.
	DayOfMonth          *int      `json:"day_of_month,omitempty"`          // 1-31
	MonthOfYear         *int      `json:"month_of_year,omitempty"`         // 1-12
	EndAfterOccurrences *int      `json:"end_after_occurrences,omitempty"` // End after N occurrences
	EndDate             *string   `json:"end_date,omitempty"`              // ISO date string
}

// Value implements driver.Valuer for JSONB storage
func (r RecurrenceRule) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements sql.Scanner for JSONB storage
func (r *RecurrenceRule) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal RecurrenceRule value")
	}
	return json.Unmarshal(bytes, r)
}

type Reminder struct {
	ID             uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	Title          string          `gorm:"size:500;not null" json:"title"`
	Notes          *string         `json:"notes,omitempty"`
	Priority       Priority        `gorm:"default:0" json:"priority"`
	DueAt          time.Time       `gorm:"not null;index" json:"due_at"`
	AllDay         bool            `gorm:"default:false" json:"all_day"`
	RecurrenceRule *RecurrenceRule `gorm:"type:jsonb" json:"recurrence_rule,omitempty"`
	RecurrenceEnd  *time.Time      `json:"recurrence_end,omitempty"`
	Status         ReminderStatus  `gorm:"type:varchar(20);default:'active';index" json:"status"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	SnoozedUntil   *time.Time      `json:"snoozed_until,omitempty"`
	SnoozeCount    int             `gorm:"default:0" json:"snooze_count"`
	LocalID        *string         `gorm:"size:255" json:"local_id,omitempty"` // Client-generated ID
	Version        int             `gorm:"default:1" json:"version"`
	LastModifiedBy *uuid.UUID      `gorm:"type:uuid" json:"last_modified_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	User      *User               `gorm:"foreignKey:UserID" json:"-"`
	Instances []ReminderInstance  `gorm:"foreignKey:ReminderID" json:"instances,omitempty"`
}

func (r *Reminder) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (r *Reminder) BeforeUpdate(tx *gorm.DB) error {
	r.Version++
	return nil
}

func (r *Reminder) IsRecurring() bool {
	return r.RecurrenceRule != nil
}

func (r *Reminder) IsActive() bool {
	return r.Status == StatusActive
}

func (r *Reminder) IsSnoozed() bool {
	return r.Status == StatusSnoozed && r.SnoozedUntil != nil && r.SnoozedUntil.After(time.Now())
}

func (r *Reminder) Complete() {
	now := time.Now()
	r.Status = StatusCompleted
	r.CompletedAt = &now
}

func (r *Reminder) Dismiss() {
	r.Status = StatusDismissed
}

func (r *Reminder) Snooze(duration time.Duration) {
	until := time.Now().Add(duration)
	r.Status = StatusSnoozed
	r.SnoozedUntil = &until
	r.SnoozeCount++
}

func (r *Reminder) Reactivate() {
	r.Status = StatusActive
	r.SnoozedUntil = nil
}
