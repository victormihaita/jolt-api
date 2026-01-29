package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StringArray is a custom type for PostgreSQL text[] arrays
type StringArray []string

// Value implements driver.Valuer for PostgreSQL text[] storage
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	if len(a) == 0 {
		return "{}", nil
	}
	// Format as PostgreSQL array literal: {elem1,elem2,elem3}
	escaped := make([]string, len(a))
	for i, s := range a {
		// Escape quotes and backslashes, wrap in quotes if contains special chars
		if strings.ContainsAny(s, ",{}\"\\") || strings.ContainsAny(s, " \t\n") {
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "\"", "\\\"")
			escaped[i] = "\"" + s + "\""
		} else {
			escaped[i] = s
		}
	}
	return "{" + strings.Join(escaped, ",") + "}", nil
}

// Scan implements sql.Scanner for PostgreSQL text[] storage
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return errors.New("failed to scan StringArray: unsupported type")
	}

	// Parse PostgreSQL array literal: {elem1,elem2,elem3}
	str = strings.TrimSpace(str)
	if str == "{}" || str == "" {
		*a = StringArray{}
		return nil
	}

	// Remove surrounding braces
	if len(str) < 2 || str[0] != '{' || str[len(str)-1] != '}' {
		return errors.New("invalid PostgreSQL array format")
	}
	str = str[1 : len(str)-1]

	// Parse elements (handling quoted strings)
	var result []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(str); i++ {
		c := str[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			continue
		}
		if c == ',' && !inQuotes {
			result = append(result, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	// Don't forget the last element
	if current.Len() > 0 || len(result) > 0 {
		result = append(result, current.String())
	}

	*a = result
	return nil
}

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
	ListID         *uuid.UUID      `gorm:"type:uuid;index" json:"list_id,omitempty"` // Optional: belongs to a list
	Title          string          `gorm:"size:500;not null" json:"title"`
	Notes          *string         `json:"notes,omitempty"`
	Priority       Priority        `gorm:"default:2" json:"priority"`
	DueAt          *time.Time      `gorm:"index" json:"due_at,omitempty"`  // Optional: reminders without dates don't trigger notifications
	AllDay         *bool           `json:"all_day,omitempty"`              // Optional: only relevant when DueAt is set
	RecurrenceRule *RecurrenceRule `gorm:"type:jsonb" json:"recurrence_rule,omitempty"`
	RecurrenceEnd  *time.Time      `json:"recurrence_end,omitempty"`
	Status         ReminderStatus  `gorm:"type:varchar(20);default:'active';index" json:"status"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	SnoozedUntil       *time.Time      `json:"snoozed_until,omitempty"`
	SnoozeCount        int             `gorm:"default:0" json:"snooze_count"`
	IsAlarm            bool            `gorm:"default:false" json:"is_alarm"`                     // Alarm-style notification (bypasses DND)
	SoundID            *string         `gorm:"size:50" json:"sound_id,omitempty"`                 // Sound to play for notification (e.g., "gentle_chime")
	NotificationSentAt *time.Time      `gorm:"index" json:"notification_sent_at,omitempty"`       // When notification was sent (prevents duplicates)
	Tags               StringArray     `gorm:"type:text[];default:'{}'" json:"tags,omitempty"`    // Tags for cross-list filtering
	LocalID        *string         `gorm:"size:255" json:"local_id,omitempty"` // Client-generated ID
	Version        int             `gorm:"default:1" json:"version"`
	LastModifiedBy *uuid.UUID      `gorm:"type:uuid" json:"last_modified_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	User      *User               `gorm:"foreignKey:UserID" json:"-"`
	List      *ReminderList       `gorm:"foreignKey:ListID" json:"-"`
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

// HasScheduledDate returns true if the reminder has a due date set
func (r *Reminder) HasScheduledDate() bool {
	return r.DueAt != nil
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
