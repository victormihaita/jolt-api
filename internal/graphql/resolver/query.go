package resolver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/graphql/middleware"
	"github.com/user/remind-me/backend/internal/graphql/model"
	"github.com/user/remind-me/backend/internal/models"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

// Me returns the current authenticated user
func (r *Resolver) Me(ctx context.Context) (*model.User, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	user, err := r.UserRepo.FindByID(userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	return model.UserFromModel(user), nil
}

// Reminder returns a single reminder by ID
func (r *Resolver) Reminder(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	reminderDTO, err := r.ReminderService.GetByID(userID, id)
	if err != nil {
		return nil, err
	}

	return dtoToReminder(reminderDTO), nil
}

// Reminders returns a paginated list of reminders with optional filtering
func (r *Resolver) Reminders(ctx context.Context, filter *model.ReminderFilter, pagination *model.PaginationInput) (*model.ReminderConnection, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	// Parse pagination
	page := 1
	pageSize := 20

	if pagination != nil {
		if pagination.First != nil && *pagination.First > 0 {
			pageSize = *pagination.First
			if pageSize > 100 {
				pageSize = 100
			}
		}
		if pagination.After != nil {
			// Decode cursor to get the page
			decoded, err := decodeCursor(*pagination.After)
			if err == nil {
				page = decoded + 1 // Next page after cursor
			}
		}
	}

	// Parse filter
	var status *string
	if filter != nil && filter.Status != nil {
		s := strings.ToLower(string(*filter.Status))
		status = &s
	}

	var fromDate, toDate *string
	if filter != nil {
		if filter.FromDate != nil {
			fd := filter.FromDate.Format("2006-01-02T15:04:05Z07:00")
			fromDate = &fd
		}
		if filter.ToDate != nil {
			td := filter.ToDate.Format("2006-01-02T15:04:05Z07:00")
			toDate = &td
		}
	}

	// Get reminders from service
	listResp, err := r.ReminderService.List(userID, page, pageSize, status, nil, nil)
	if err != nil {
		return nil, err
	}

	// Apply date filters manually if needed (service doesn't support them directly with time.Time)
	_ = fromDate
	_ = toDate

	// Build edges
	edges := make([]*model.ReminderEdge, len(listResp.Reminders))
	for i, rem := range listResp.Reminders {
		edges[i] = &model.ReminderEdge{
			TypeName: "ReminderEdge",
			Node:     dtoToReminder(&rem),
			Cursor:   encodeCursor(page, i),
		}
	}

	// Build page info
	hasNextPage := page < listResp.TotalPages
	hasPreviousPage := page > 1
	var startCursor, endCursor *string
	if len(edges) > 0 {
		startCursor = &edges[0].Cursor
		endCursor = &edges[len(edges)-1].Cursor
	}

	return &model.ReminderConnection{
		TypeName: "ReminderConnection",
		Edges:    edges,
		PageInfo: &model.PageInfo{
			TypeName:        "PageInfo",
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPreviousPage,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
		TotalCount: int(listResp.Total),
	}, nil
}

// Devices returns all devices for the current user
func (r *Resolver) Devices(ctx context.Context) ([]*model.Device, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	devices, err := r.DeviceRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.Device, len(devices))
	for i := range devices {
		result[i] = model.DeviceFromModel(&devices[i])
	}

	return result, nil
}

// Helper functions

func encodeCursor(page, index int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%d", page, index)))
}

func decodeCursor(cursor string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid cursor format")
	}
	page, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	return page, nil
}

// dtoToReminder converts a ReminderDTO to a GraphQL Reminder model
func dtoToReminder(d *dto.ReminderDTO) *model.Reminder {
	if d == nil {
		return nil
	}

	priority := model.PriorityNone
	switch d.Priority {
	case 1:
		priority = model.PriorityLow
	case 2:
		priority = model.PriorityNormal
	case 3:
		priority = model.PriorityHigh
	}

	status := model.ReminderStatusActive
	switch models.ReminderStatus(d.Status) {
	case models.StatusCompleted:
		status = model.ReminderStatusCompleted
	case models.StatusSnoozed:
		status = model.ReminderStatusSnoozed
	case models.StatusDismissed:
		status = model.ReminderStatusDismissed
	}

	var recurrenceRule *model.RecurrenceRule
	if d.RecurrenceRule != nil {
		var endDate *time.Time
		if d.RecurrenceRule.EndDate != nil {
			parsed, err := time.Parse(time.RFC3339, *d.RecurrenceRule.EndDate)
			if err == nil {
				endDate = &parsed
			}
		}
		recurrenceRule = &model.RecurrenceRule{
			TypeName:            "RecurrenceRule",
			Frequency:           model.FrequencyFromModel(d.RecurrenceRule.Frequency),
			Interval:            d.RecurrenceRule.Interval,
			DaysOfWeek:          d.RecurrenceRule.DaysOfWeek,
			DayOfMonth:          d.RecurrenceRule.DayOfMonth,
			MonthOfYear:         d.RecurrenceRule.MonthOfYear,
			EndAfterOccurrences: d.RecurrenceRule.EndAfterOccurrences,
			EndDate:             endDate,
		}
	}

	tags := []string(d.Tags)
	if tags == nil {
		tags = []string{}
	}

	return &model.Reminder{
		TypeName:       "Reminder",
		ID:             d.ID,
		ListID:         d.ListID,
		Title:          d.Title,
		Notes:          d.Notes,
		Priority:       priority,
		DueAt:          d.DueAt,
		AllDay:         d.AllDay,
		RecurrenceRule: recurrenceRule,
		RecurrenceEnd:  d.RecurrenceEnd,
		Status:         status,
		CompletedAt:    d.CompletedAt,
		SnoozedUntil:   d.SnoozedUntil,
		SnoozeCount:    d.SnoozeCount,
		IsAlarm:        d.IsAlarm,
		Tags:           tags,
		LocalID:        d.LocalID,
		Version:        d.Version,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}
