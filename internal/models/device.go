package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

type Device struct {
	ID         uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Platform   Platform       `gorm:"type:varchar(10);not null" json:"platform"`
	PushToken  string         `gorm:"not null" json:"-"`
	DeviceName string         `gorm:"size:255" json:"device_name"`
	AppVersion string         `gorm:"size:20" json:"app_version"`
	OSVersion  string         `gorm:"size:20" json:"os_version"`
	LastSeenAt time.Time      `gorm:"default:now()" json:"last_seen_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (d *Device) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (d *Device) IsIOS() bool {
	return d.Platform == PlatformIOS
}

func (d *Device) IsAndroid() bool {
	return d.Platform == PlatformAndroid
}
