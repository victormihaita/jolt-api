package dto

import (
	"time"

	"github.com/google/uuid"
)

// SyncRequest is the request body for pushing local changes
type SyncRequest struct {
	Changes []SyncChange `json:"changes" binding:"required"`
}

// SyncChange represents a single change to sync
type SyncChange struct {
	EntityType string                 `json:"entity_type" binding:"required,oneof=reminder reminder_instance"`
	EntityID   uuid.UUID              `json:"entity_id" binding:"required"`
	Action     string                 `json:"action" binding:"required,oneof=create update delete"`
	Version    int                    `json:"version"`
	Data       map[string]interface{} `json:"data,omitempty"`
	ModifiedAt time.Time              `json:"modified_at"`
}

// SyncResponse is the response for sync operations
type SyncResponse struct {
	Changes     []SyncEvent `json:"changes"`
	LastSyncAt  time.Time   `json:"last_sync_at"`
	HasMore     bool        `json:"has_more"`
	NextCursor  *string     `json:"next_cursor,omitempty"`
}

// SyncEvent represents a change event from the server
type SyncEvent struct {
	ID         uuid.UUID              `json:"id"`
	EntityType string                 `json:"entity_type"`
	EntityID   uuid.UUID              `json:"entity_id"`
	Action     string                 `json:"action"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	DeviceID   *uuid.UUID             `json:"device_id,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// SyncConflict represents a conflict during sync
type SyncConflict struct {
	EntityType    string                 `json:"entity_type"`
	EntityID      uuid.UUID              `json:"entity_id"`
	ClientVersion int                    `json:"client_version"`
	ServerVersion int                    `json:"server_version"`
	ServerData    map[string]interface{} `json:"server_data"`
	Resolution    string                 `json:"resolution"` // "server_wins", "client_wins", "merge"
}

// SyncPushResponse is the response for pushing changes
type SyncPushResponse struct {
	Accepted  []uuid.UUID    `json:"accepted"`
	Rejected  []uuid.UUID    `json:"rejected"`
	Conflicts []SyncConflict `json:"conflicts,omitempty"`
}

// WebSocketMessage is the structure for WebSocket messages
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	EventID   *uuid.UUID  `json:"event_id,omitempty"`
}

// WebSocket message types
const (
	WSTypeReminderCreated   = "reminder.created"
	WSTypeReminderUpdated   = "reminder.updated"
	WSTypeReminderDeleted   = "reminder.deleted"
	WSTypeReminderSnoozed   = "reminder.snoozed"
	WSTypeReminderCompleted = "reminder.completed"
	WSTypeReminderDismissed = "reminder.dismissed"
	WSTypeAck               = "ack"
	WSTypePing              = "ping"
	WSTypePong              = "pong"
)
