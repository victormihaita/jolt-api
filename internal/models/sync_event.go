package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SyncAction string

const (
	SyncActionCreate SyncAction = "create"
	SyncActionUpdate SyncAction = "update"
	SyncActionDelete SyncAction = "delete"
)

type EntityType string

const (
	EntityTypeReminder         EntityType = "reminder"
	EntityTypeReminderInstance EntityType = "reminder_instance"
)

// SyncPayload holds the JSON data for sync events
type SyncPayload map[string]interface{}

func (sp SyncPayload) Value() (driver.Value, error) {
	return json.Marshal(sp)
}

func (sp *SyncPayload) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal SyncPayload value")
	}
	return json.Unmarshal(bytes, sp)
}

// SyncEvent tracks changes for cross-device synchronization
type SyncEvent struct {
	ID         uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID   `gorm:"type:uuid;not null;index" json:"user_id"`
	EntityType EntityType  `gorm:"type:varchar(50);not null" json:"entity_type"`
	EntityID   uuid.UUID   `gorm:"type:uuid;not null" json:"entity_id"`
	Action     SyncAction  `gorm:"type:varchar(20);not null" json:"action"`
	Payload    SyncPayload `gorm:"type:jsonb" json:"payload,omitempty"`
	DeviceID   *uuid.UUID  `gorm:"type:uuid" json:"device_id,omitempty"`
	CreatedAt  time.Time   `gorm:"index" json:"created_at"`

	// Relations
	User   *User   `gorm:"foreignKey:UserID" json:"-"`
	Device *Device `gorm:"foreignKey:DeviceID" json:"-"`
}

func (se *SyncEvent) BeforeCreate(tx *gorm.DB) error {
	if se.ID == uuid.Nil {
		se.ID = uuid.New()
	}
	return nil
}

// CreateSyncEvent creates a new sync event for tracking changes
func CreateSyncEvent(userID uuid.UUID, entityType EntityType, entityID uuid.UUID, action SyncAction, payload interface{}, deviceID *uuid.UUID) *SyncEvent {
	var syncPayload SyncPayload
	if payload != nil {
		data, _ := json.Marshal(payload)
		json.Unmarshal(data, &syncPayload)
	}

	return &SyncEvent{
		UserID:     userID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Payload:    syncPayload,
		DeviceID:   deviceID,
	}
}
