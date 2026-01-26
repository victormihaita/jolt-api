package resolver

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/graphql/middleware"
	"github.com/user/remind-me/backend/internal/graphql/model"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/notification"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

// AuthenticateWithGoogle authenticates a user with their Google ID token
func (r *Resolver) AuthenticateWithGoogle(ctx context.Context, idToken string) (*model.AuthPayload, error) {
	authResp, err := r.AuthService.AuthenticateWithGoogle(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := r.UserRepo.FindByID(uuid.MustParse(authResp.User.ID))
	if err != nil {
		return nil, err
	}

	return &model.AuthPayload{
		TypeName:     "AuthPayload",
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    int(authResp.ExpiresIn),
		User:         model.UserFromModel(user),
	}, nil
}

// RefreshToken generates new tokens from a valid refresh token
func (r *Resolver) RefreshToken(ctx context.Context, refreshToken string) (*model.AuthPayload, error) {
	authResp, err := r.AuthService.RefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := r.UserRepo.FindByID(uuid.MustParse(authResp.User.ID))
	if err != nil {
		return nil, err
	}

	return &model.AuthPayload{
		TypeName:     "AuthPayload",
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    int(authResp.ExpiresIn),
		User:         model.UserFromModel(user),
	}, nil
}

// Logout logs out the current user (currently just returns success)
func (r *Resolver) Logout(ctx context.Context) (bool, error) {
	// In a stateless JWT system, logout is handled client-side by deleting tokens
	// For a more robust implementation, you could blacklist the token
	return true, nil
}

// VerifySubscription checks the user's subscription status with RevenueCat
// and updates their premium status in the database
func (r *Resolver) VerifySubscription(ctx context.Context) (*model.User, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	// Verify subscription with RevenueCat
	_, _, err := r.SubscriptionService.VerifySubscription(userID)
	if err != nil {
		return nil, err
	}

	// Fetch updated user
	user, err := r.UserRepo.FindByID(userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	return model.UserFromModel(user), nil
}

// CreateReminder creates a new reminder
func (r *Resolver) CreateReminder(ctx context.Context, input model.CreateReminderInput) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	var priority *int
	if input.Priority != nil {
		p := int(model.PriorityToModel(*input.Priority))
		priority = &p
	}

	req := dto.CreateReminderRequest{
		ListID:         input.ListID,
		Title:          input.Title,
		Notes:          input.Notes,
		Priority:       priority,
		DueAt:          input.DueAt,
		AllDay:         input.AllDay,
		RecurrenceRule: model.RecurrenceRuleToModel(input.RecurrenceRule),
		RecurrenceEnd:  input.RecurrenceEnd,
		IsAlarm:        input.IsAlarm,
		SoundID:        input.SoundID,
		Tags:           input.Tags,
		LocalID:        input.LocalID,
	}

	reminderDTO, err := r.ReminderService.Create(userID, req, deviceID)
	if err != nil {
		return nil, err
	}

	result := dtoToReminder(reminderDTO)

	// Broadcast change event
	r.broadcastReminderChange(userID, model.ChangeActionCreated, result)

	return result, nil
}

// UpdateReminder updates an existing reminder
func (r *Resolver) UpdateReminder(ctx context.Context, id uuid.UUID, input model.UpdateReminderInput) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	var priority *int
	if input.Priority != nil {
		p := int(model.PriorityToModel(*input.Priority))
		priority = &p
	}

	var status *string
	if input.Status != nil {
		s := strings.ToLower(string(*input.Status))
		status = &s
	}

	req := dto.UpdateReminderRequest{
		ListID:         input.ListID,
		Title:          input.Title,
		Notes:          input.Notes,
		Priority:       priority,
		DueAt:          input.DueAt,
		AllDay:         input.AllDay,
		RecurrenceRule: model.RecurrenceRuleToModel(input.RecurrenceRule),
		RecurrenceEnd:  input.RecurrenceEnd,
		IsAlarm:        input.IsAlarm,
		SoundID:        input.SoundID,
		Status:         status,
		Tags:           input.Tags,
	}

	reminderDTO, err := r.ReminderService.Update(userID, id, req, deviceID)
	if err != nil {
		return nil, err
	}

	result := dtoToReminder(reminderDTO)

	// Broadcast change event
	r.broadcastReminderChange(userID, model.ChangeActionUpdated, result)

	return result, nil
}

// DeleteReminder deletes a reminder
func (r *Resolver) DeleteReminder(ctx context.Context, id uuid.UUID) (bool, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return false, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	err := r.ReminderService.Delete(userID, id, deviceID)
	if err != nil {
		return false, err
	}

	// Broadcast delete event
	r.broadcastReminderDelete(userID, id)

	// Send cross-device notification to dismiss notification on other devices
	if r.NotificationDispatcher != nil {
		go r.NotificationDispatcher.SendCrossDeviceAction(ctx, userID, deviceID, id, notification.ActionDelete)
	}

	return true, nil
}

// SnoozeReminder snoozes a reminder for the specified number of minutes
func (r *Resolver) SnoozeReminder(ctx context.Context, id uuid.UUID, minutes int) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	reminderDTO, err := r.ReminderService.Snooze(userID, id, minutes, deviceID)
	if err != nil {
		return nil, err
	}

	result := dtoToReminder(reminderDTO)

	// Broadcast change event
	r.broadcastReminderChange(userID, model.ChangeActionUpdated, result)

	// Send cross-device notification to dismiss notification on other devices
	if r.NotificationDispatcher != nil {
		go r.NotificationDispatcher.SendCrossDeviceAction(ctx, userID, deviceID, id, notification.ActionSnooze)
	}

	return result, nil
}

// CompleteReminder marks a reminder as complete
func (r *Resolver) CompleteReminder(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	reminderDTO, err := r.ReminderService.Complete(userID, id, deviceID)
	if err != nil {
		return nil, err
	}

	result := dtoToReminder(reminderDTO)

	// Broadcast change event
	r.broadcastReminderChange(userID, model.ChangeActionUpdated, result)

	// Send cross-device notification to dismiss notification on other devices
	if r.NotificationDispatcher != nil {
		go r.NotificationDispatcher.SendCrossDeviceAction(ctx, userID, deviceID, id, notification.ActionComplete)
	}

	return result, nil
}

// DismissReminder dismisses a reminder
func (r *Resolver) DismissReminder(ctx context.Context, id uuid.UUID) (bool, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return false, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	err := r.ReminderService.Dismiss(userID, id, deviceID)
	if err != nil {
		return false, err
	}

	// Get updated reminder for broadcast
	reminderDTO, _ := r.ReminderService.GetByID(userID, id)
	if reminderDTO != nil {
		r.broadcastReminderChange(userID, model.ChangeActionUpdated, dtoToReminder(reminderDTO))
	}

	// Send cross-device notification to dismiss notification on other devices
	if r.NotificationDispatcher != nil {
		go r.NotificationDispatcher.SendCrossDeviceAction(ctx, userID, deviceID, id, notification.ActionDismiss)
	}

	return true, nil
}

// RegisterDevice registers a new device for push notifications
func (r *Resolver) RegisterDevice(ctx context.Context, input model.RegisterDeviceInput) (*model.Device, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	platform := models.PlatformIOS
	if input.Platform == model.PlatformAndroid {
		platform = models.PlatformAndroid
	}

	var deviceName, appVersion, osVersion string
	if input.DeviceName != nil {
		deviceName = *input.DeviceName
	}
	if input.AppVersion != nil {
		appVersion = *input.AppVersion
	}
	if input.OsVersion != nil {
		osVersion = *input.OsVersion
	}

	device := &models.Device{
		UserID:           userID,
		DeviceIdentifier: input.DeviceIdentifier,
		Platform:         platform,
		PushToken:        input.PushToken,
		DeviceName:       deviceName,
		AppVersion:       appVersion,
		OSVersion:        osVersion,
		LastSeenAt:       time.Now(),
	}

	err := r.DeviceRepo.Upsert(device)
	if err != nil {
		return nil, err
	}

	return model.DeviceFromModel(device), nil
}

// UnregisterDevice removes a device from push notifications
func (r *Resolver) UnregisterDevice(ctx context.Context, id uuid.UUID) (bool, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return false, apperrors.ErrUnauthorized
	}

	// Verify device belongs to user
	device, err := r.DeviceRepo.FindByIDAndUser(id, userID)
	if err != nil {
		return false, apperrors.ErrDeviceNotFound
	}
	_ = device

	err = r.DeviceRepo.Delete(id)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Helper functions for broadcasting changes

func (r *Resolver) broadcastReminderChange(userID uuid.UUID, action model.ChangeAction, reminder *model.Reminder) {
	if r.Hub == nil {
		return
	}

	event := &model.ReminderChangeEvent{
		TypeName:   "ReminderChangeEvent",
		Action:     action,
		Reminder:   reminder,
		ReminderID: reminder.ID,
		Timestamp:  time.Now(),
	}

	r.Hub.BroadcastToUser(userID, event)
}

func (r *Resolver) broadcastReminderDelete(userID uuid.UUID, reminderID uuid.UUID) {
	if r.Hub == nil {
		return
	}

	event := &model.ReminderChangeEvent{
		TypeName:   "ReminderChangeEvent",
		Action:     model.ChangeActionDeleted,
		Reminder:   nil,
		ReminderID: reminderID,
		Timestamp:  time.Now(),
	}

	r.Hub.BroadcastToUser(userID, event)
}
