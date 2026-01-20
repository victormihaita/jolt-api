package resolver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/graphql/middleware"
	"github.com/user/remind-me/backend/internal/graphql/model"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

// ReminderList returns a single reminder list by ID
func (r *Resolver) ReminderList(ctx context.Context, id uuid.UUID) (*model.ReminderList, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	listDTO, err := r.ReminderListService.GetByID(userID, id)
	if err != nil {
		return nil, err
	}

	return dtoToReminderList(listDTO), nil
}

// ReminderLists returns all reminder lists for the current user
func (r *Resolver) ReminderLists(ctx context.Context) ([]*model.ReminderList, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	lists, err := r.ReminderListService.List(userID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ReminderList, len(lists))
	for i := range lists {
		result[i] = dtoToReminderList(&lists[i])
	}

	return result, nil
}

// CreateReminderList creates a new reminder list
func (r *Resolver) CreateReminderList(ctx context.Context, input model.CreateReminderListInput) (*model.ReminderList, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	req := dto.CreateReminderListRequest{
		Name:     input.Name,
		ColorHex: input.ColorHex,
		IconName: input.IconName,
	}

	listDTO, err := r.ReminderListService.Create(userID, req)
	if err != nil {
		return nil, err
	}

	result := dtoToReminderList(listDTO)

	// Broadcast change event
	r.broadcastReminderListChange(userID, model.ChangeActionCreated, result)

	return result, nil
}

// UpdateReminderList updates an existing reminder list
func (r *Resolver) UpdateReminderList(ctx context.Context, id uuid.UUID, input model.UpdateReminderListInput) (*model.ReminderList, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	req := dto.UpdateReminderListRequest{
		Name:      input.Name,
		ColorHex:  input.ColorHex,
		IconName:  input.IconName,
		SortOrder: input.SortOrder,
	}

	listDTO, err := r.ReminderListService.Update(userID, id, req)
	if err != nil {
		return nil, err
	}

	result := dtoToReminderList(listDTO)

	// Broadcast change event
	r.broadcastReminderListChange(userID, model.ChangeActionUpdated, result)

	return result, nil
}

// DeleteReminderList deletes a reminder list
func (r *Resolver) DeleteReminderList(ctx context.Context, id uuid.UUID) (bool, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return false, apperrors.ErrUnauthorized
	}

	err := r.ReminderListService.Delete(userID, id)
	if err != nil {
		return false, err
	}

	// Broadcast delete event
	r.broadcastReminderListDelete(userID, id)

	return true, nil
}

// ReorderReminderLists reorders reminder lists
func (r *Resolver) ReorderReminderLists(ctx context.Context, ids []uuid.UUID) ([]*model.ReminderList, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	lists, err := r.ReminderListService.Reorder(userID, ids)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ReminderList, len(lists))
	for i := range lists {
		result[i] = dtoToReminderList(&lists[i])
	}

	return result, nil
}

// MoveReminderToList moves a reminder to a different list
func (r *Resolver) MoveReminderToList(ctx context.Context, reminderID uuid.UUID, listID uuid.UUID) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	deviceID, _ := middleware.GetDeviceID(ctx)

	req := dto.UpdateReminderRequest{
		ListID: &listID,
	}

	reminderDTO, err := r.ReminderService.Update(userID, reminderID, req, deviceID)
	if err != nil {
		return nil, err
	}

	result := dtoToReminder(reminderDTO)

	// Broadcast change event
	r.broadcastReminderChange(userID, model.ChangeActionUpdated, result)

	return result, nil
}

// Helper functions

func dtoToReminderList(d *dto.ReminderListDTO) *model.ReminderList {
	if d == nil {
		return nil
	}

	return &model.ReminderList{
		TypeName:      "ReminderList",
		ID:            d.ID,
		Name:          d.Name,
		ColorHex:      d.ColorHex,
		IconName:      d.IconName,
		SortOrder:     d.SortOrder,
		IsDefault:     d.IsDefault,
		ReminderCount: int(d.ReminderCount),
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func (r *Resolver) broadcastReminderListChange(userID uuid.UUID, action model.ChangeAction, list *model.ReminderList) {
	if r.Hub == nil {
		return
	}

	event := &model.ReminderListChangeEvent{
		TypeName:       "ReminderListChangeEvent",
		Action:         action,
		ReminderList:   list,
		ReminderListID: list.ID,
		Timestamp:      time.Now(),
	}

	r.Hub.BroadcastToUser(userID, event)
}

func (r *Resolver) broadcastReminderListDelete(userID uuid.UUID, listID uuid.UUID) {
	if r.Hub == nil {
		return
	}

	event := &model.ReminderListChangeEvent{
		TypeName:       "ReminderListChangeEvent",
		Action:         model.ChangeActionDeleted,
		ReminderList:   nil,
		ReminderListID: listID,
		Timestamp:      time.Now(),
	}

	r.Hub.BroadcastToUser(userID, event)
}
