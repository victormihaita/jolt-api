package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReminderList represents a user's reminder list/folder
type ReminderList struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	ColorHex  string         `gorm:"size:7;default:'#007AFF'" json:"color_hex"` // e.g., "#007AFF"
	IconName  string         `gorm:"size:50;default:'list.bullet'" json:"icon_name"` // SF Symbol name
	SortOrder int            `gorm:"default:0" json:"sort_order"`
	IsDefault bool           `gorm:"default:false" json:"is_default"` // Cannot be deleted
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	User      *User      `gorm:"foreignKey:UserID" json:"-"`
	Reminders []Reminder `gorm:"foreignKey:ListID" json:"reminders,omitempty"`
}

func (l *ReminderList) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// CreateDefaultList creates the default "Reminders" list for a user
func CreateDefaultList(userID uuid.UUID) *ReminderList {
	return &ReminderList{
		UserID:    userID,
		Name:      "Reminders",
		ColorHex:  "#007AFF",
		IconName:  "list.bullet",
		SortOrder: 0,
		IsDefault: true,
	}
}
