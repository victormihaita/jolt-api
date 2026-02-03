package notification

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/notification/apns"
	"github.com/user/remind-me/backend/internal/notification/fcm"
	"github.com/user/remind-me/backend/internal/repository"
)

// Payload represents a push notification payload
type Payload struct {
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Sound      string            `json:"sound,omitempty"`
	Badge      *int              `json:"badge,omitempty"`
	Category   string            `json:"category,omitempty"`
	ReminderID uuid.UUID         `json:"reminder_id"`
	DueAt      string            `json:"due_at,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
	IsAlarm    bool              `json:"is_alarm,omitempty"`
}

// CrossDeviceAction represents an action to propagate to other devices
type CrossDeviceAction string

const (
	ActionSnooze   CrossDeviceAction = "snooze"
	ActionComplete CrossDeviceAction = "complete"
	ActionDismiss  CrossDeviceAction = "dismiss"
	ActionDelete   CrossDeviceAction = "delete"
)

// Dispatcher handles sending notifications to multiple platforms
type Dispatcher struct {
	apnsClient *apns.Client
	fcmClient  *fcm.Client
	deviceRepo *repository.DeviceRepository
}

// NewDispatcher creates a new notification dispatcher
func NewDispatcher(
	apnsClient *apns.Client,
	fcmClient *fcm.Client,
	deviceRepo *repository.DeviceRepository,
) *Dispatcher {
	return &Dispatcher{
		apnsClient: apnsClient,
		fcmClient:  fcmClient,
		deviceRepo: deviceRepo,
	}
}

// SendToUser sends a notification to all devices of a user
func (d *Dispatcher) SendToUser(ctx context.Context, userID uuid.UUID, payload Payload) error {
	tokens, err := d.deviceRepo.GetAllPushTokens(userID)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(tokens))

	for _, token := range tokens {
		wg.Add(1)
		go func(platform models.Platform, pushToken string) {
			defer wg.Done()

			var sendErr error
			switch platform {
			case models.PlatformIOS:
				sendErr = d.sendToIOS(ctx, pushToken, payload)
			case models.PlatformAndroid:
				sendErr = d.sendToAndroid(ctx, pushToken, payload)
			}

			if sendErr != nil {
				errors <- sendErr
				log.Printf("Failed to send notification to %s device: %v", platform, sendErr)
			}
		}(token.Platform, token.PushToken)
	}

	wg.Wait()
	close(errors)

	// Collect errors (we don't fail the whole operation if some devices fail)
	var firstErr error
	for err := range errors {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// SendToDevice sends a notification to a specific device
func (d *Dispatcher) SendToDevice(ctx context.Context, deviceID uuid.UUID, payload Payload) error {
	device, err := d.deviceRepo.FindByID(deviceID)
	if err != nil {
		return err
	}

	switch device.Platform {
	case models.PlatformIOS:
		return d.sendToIOS(ctx, device.PushToken, payload)
	case models.PlatformAndroid:
		return d.sendToAndroid(ctx, device.PushToken, payload)
	}

	return nil
}

// SendToUserExcluding sends a notification to all devices of a user except the specified device
func (d *Dispatcher) SendToUserExcluding(ctx context.Context, userID uuid.UUID, excludeDeviceID *uuid.UUID, payload Payload) error {
	var tokens []struct {
		Platform  models.Platform
		PushToken string
	}
	var err error

	if excludeDeviceID != nil {
		tokens, err = d.deviceRepo.GetPushTokensExcluding(userID, *excludeDeviceID)
	} else {
		tokens, err = d.deviceRepo.GetAllPushTokens(userID)
	}
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(tokens))

	for _, token := range tokens {
		wg.Add(1)
		go func(platform models.Platform, pushToken string) {
			defer wg.Done()

			var sendErr error
			switch platform {
			case models.PlatformIOS:
				sendErr = d.sendToIOS(ctx, pushToken, payload)
			case models.PlatformAndroid:
				sendErr = d.sendToAndroid(ctx, pushToken, payload)
			}

			if sendErr != nil {
				errors <- sendErr
				log.Printf("Failed to send notification to %s device: %v", platform, sendErr)
			}
		}(token.Platform, token.PushToken)
	}

	wg.Wait()
	close(errors)

	var firstErr error
	for err := range errors {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// SendCrossDeviceAction sends a silent notification to propagate an action to other devices
func (d *Dispatcher) SendCrossDeviceAction(ctx context.Context, userID uuid.UUID, excludeDeviceID *uuid.UUID, reminderID uuid.UUID, action CrossDeviceAction) error {
	var tokens []struct {
		Platform  models.Platform
		PushToken string
	}
	var err error

	if excludeDeviceID != nil {
		tokens, err = d.deviceRepo.GetPushTokensExcluding(userID, *excludeDeviceID)
	} else {
		tokens, err = d.deviceRepo.GetAllPushTokens(userID)
	}
	if err != nil {
		return err
	}

	data := map[string]string{
		"type":        "cross_device_action",
		"action":      string(action),
		"reminder_id": reminderID.String(),
	}

	var wg sync.WaitGroup
	for _, token := range tokens {
		wg.Add(1)
		go func(platform models.Platform, pushToken string) {
			defer wg.Done()

			switch platform {
			case models.PlatformIOS:
				if d.apnsClient != nil {
					_ = d.apnsClient.SendData(ctx, pushToken, data)
				}
			case models.PlatformAndroid:
				if d.fcmClient != nil {
					_ = d.fcmClient.SendData(ctx, pushToken, data)
				}
			}
		}(token.Platform, token.PushToken)
	}

	wg.Wait()
	return nil
}

// SendSyncNotification sends a silent sync notification
func (d *Dispatcher) SendSyncNotification(ctx context.Context, userID uuid.UUID, excludeDevice *uuid.UUID) error {
	tokens, err := d.deviceRepo.GetAllPushTokens(userID)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, token := range tokens {
		wg.Add(1)
		go func(platform models.Platform, pushToken string) {
			defer wg.Done()

			switch platform {
			case models.PlatformIOS:
				_ = d.apnsClient.SendSilent(ctx, pushToken)
			case models.PlatformAndroid:
				_ = d.fcmClient.SendData(ctx, pushToken, map[string]string{
					"type": "sync",
				})
			}
		}(token.Platform, token.PushToken)
	}

	wg.Wait()
	return nil
}

func (d *Dispatcher) sendToIOS(ctx context.Context, token string, payload Payload) error {
	// Build base Data map
	data := map[string]interface{}{
		"reminder_id": payload.ReminderID.String(),
		"due_at":      payload.DueAt,
	}

	// Merge payload.Data (contains sound_id, is_alarm, type)
	for k, v := range payload.Data {
		data[k] = v
	}

	notification := apns.Notification{
		DeviceToken: token,
		Title:       payload.Title,
		Body:        payload.Body,
		Sound:       payload.Sound,
		Badge:       payload.Badge,
		Category:    payload.Category,
		Data:        data,
	}

	return d.apnsClient.Send(ctx, notification)
}

func (d *Dispatcher) sendToAndroid(ctx context.Context, token string, payload Payload) error {
	data := map[string]string{
		"reminder_id": payload.ReminderID.String(),
		"title":       payload.Title,
		"body":        payload.Body,
		"due_at":      payload.DueAt,
		"channel_id":  "reminders",
	}

	for k, v := range payload.Data {
		data[k] = v
	}

	return d.fcmClient.SendData(ctx, token, data)
}
