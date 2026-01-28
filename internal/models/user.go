package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GoogleID     string         `gorm:"uniqueIndex" json:"-"`
	AppleID      string         `gorm:"uniqueIndex" json:"-"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	DisplayName  string         `gorm:"size:255" json:"display_name"`
	AvatarURL    string         `json:"avatar_url,omitempty"`
	Timezone     string         `gorm:"default:'UTC'" json:"timezone"`
	IsPremium    bool           `gorm:"default:false" json:"is_premium"`
	PremiumUntil *time.Time     `json:"premium_until,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Devices   []Device   `gorm:"foreignKey:UserID" json:"-"`
	Reminders []Reminder `gorm:"foreignKey:UserID" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (u *User) HasActivePremium() bool {
	if !u.IsPremium {
		return false
	}
	if u.PremiumUntil == nil {
		return true // Lifetime premium
	}
	return u.PremiumUntil.After(time.Now())
}
