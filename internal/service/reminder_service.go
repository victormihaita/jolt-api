package service

import (
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

type ReminderService struct {
	reminderRepo *repository.ReminderRepository
	syncRepo     *repository.SyncRepository
	userRepo     *repository.UserRepository
}

func NewReminderService(
	reminderRepo *repository.ReminderRepository,
	syncRepo *repository.SyncRepository,
	userRepo *repository.UserRepository,
) *ReminderService {
	return &ReminderService{
		reminderRepo: reminderRepo,
		syncRepo:     syncRepo,
		userRepo:     userRepo,
	}
}

func (s *ReminderService) Create(userID uuid.UUID, req dto.CreateReminderRequest, deviceID *uuid.UUID) (*dto.ReminderDTO, error) {
	// Check for duplicate local ID
	if req.LocalID != nil {
		existing, err := s.reminderRepo.FindByLocalID(userID, *req.LocalID)
		if err == nil && existing != nil {
			// Return existing reminder (idempotent create)
			result := dto.ReminderToDTO(existing)
			return &result, nil
		}
	}

	// Check premium features
	if req.RecurrenceRule != nil {
		user, err := s.userRepo.FindByID(userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		if !user.HasActivePremium() && isAdvancedRecurrence(req.RecurrenceRule) {
			return nil, apperrors.ErrPremiumRequired
		}
	}

	reminder := &models.Reminder{
		UserID:         userID,
		Title:          req.Title,
		Notes:          req.Notes,
		DueAt:          req.DueAt,
		AllDay:         req.AllDay,
		RecurrenceRule: req.RecurrenceRule,
		RecurrenceEnd:  req.RecurrenceEnd,
		LocalID:        req.LocalID,
		Status:         models.StatusActive,
		LastModifiedBy: deviceID,
	}

	if req.Priority != nil {
		reminder.Priority = models.Priority(*req.Priority)
	}

	if err := s.reminderRepo.Create(reminder); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to create reminder", http.StatusInternalServerError)
	}

	// Record sync event
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionCreate, deviceID)

	result := dto.ReminderToDTO(reminder)
	return &result, nil
}

func (s *ReminderService) GetByID(userID, reminderID uuid.UUID) (*dto.ReminderDTO, error) {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderNotFound
	}

	result := dto.ReminderToDTO(reminder)
	return &result, nil
}

func (s *ReminderService) List(userID uuid.UUID, page, pageSize int, status *string, fromDate, toDate *time.Time) (*dto.ReminderListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	reminders, total, err := s.reminderRepo.List(repository.ReminderListParams{
		UserID:   userID,
		Status:   status,
		FromDate: fromDate,
		ToDate:   toDate,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to list reminders", http.StatusInternalServerError)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &dto.ReminderListResponse{
		Reminders:  dto.RemindersToDTO(reminders),
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *ReminderService) Update(userID, reminderID uuid.UUID, req dto.UpdateReminderRequest, deviceID *uuid.UUID) (*dto.ReminderDTO, error) {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderNotFound
	}

	// Check premium features for advanced recurrence
	if req.RecurrenceRule != nil {
		user, err := s.userRepo.FindByID(userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		if !user.HasActivePremium() && isAdvancedRecurrence(req.RecurrenceRule) {
			return nil, apperrors.ErrPremiumRequired
		}
	}

	// Apply updates
	if req.Title != nil {
		reminder.Title = *req.Title
	}
	if req.Notes != nil {
		reminder.Notes = req.Notes
	}
	if req.Priority != nil {
		reminder.Priority = models.Priority(*req.Priority)
	}
	if req.DueAt != nil {
		reminder.DueAt = *req.DueAt
	}
	if req.AllDay != nil {
		reminder.AllDay = *req.AllDay
	}
	if req.RecurrenceRule != nil {
		reminder.RecurrenceRule = req.RecurrenceRule
	}
	if req.RecurrenceEnd != nil {
		reminder.RecurrenceEnd = req.RecurrenceEnd
	}
	if req.Status != nil {
		reminder.Status = models.ReminderStatus(*req.Status)
	}

	reminder.LastModifiedBy = deviceID

	if err := s.reminderRepo.Update(reminder); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to update reminder", http.StatusInternalServerError)
	}

	// Record sync event
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionUpdate, deviceID)

	result := dto.ReminderToDTO(reminder)
	return &result, nil
}

func (s *ReminderService) Delete(userID, reminderID uuid.UUID, deviceID *uuid.UUID) error {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return apperrors.ErrReminderNotFound
	}

	if err := s.reminderRepo.SoftDelete(reminderID); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to delete reminder", http.StatusInternalServerError)
	}

	// Record sync event
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionDelete, deviceID)

	return nil
}

func (s *ReminderService) Snooze(userID, reminderID uuid.UUID, minutes int, deviceID *uuid.UUID) (*dto.ReminderDTO, error) {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderNotFound
	}

	// Check premium for custom snooze (not preset values)
	if !isPresetSnooze(minutes) {
		user, err := s.userRepo.FindByID(userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		if !user.HasActivePremium() {
			return nil, apperrors.ErrPremiumRequired
		}
	}

	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	if err := s.reminderRepo.Snooze(reminderID, until, deviceID); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to snooze reminder", http.StatusInternalServerError)
	}

	// Reload the reminder
	reminder, _ = s.reminderRepo.FindByID(reminderID)

	// Record sync event
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionUpdate, deviceID)

	result := dto.ReminderToDTO(reminder)
	return &result, nil
}

func (s *ReminderService) Complete(userID, reminderID uuid.UUID, deviceID *uuid.UUID) (*dto.ReminderDTO, error) {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderNotFound
	}

	if err := s.reminderRepo.Complete(reminderID, deviceID); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to complete reminder", http.StatusInternalServerError)
	}

	// Reload the reminder
	reminder, _ = s.reminderRepo.FindByID(reminderID)

	// Record sync event
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionUpdate, deviceID)

	result := dto.ReminderToDTO(reminder)
	return &result, nil
}

func (s *ReminderService) Dismiss(userID, reminderID uuid.UUID, deviceID *uuid.UUID) error {
	reminder, err := s.reminderRepo.FindByIDAndUser(reminderID, userID)
	if err != nil {
		return apperrors.ErrReminderNotFound
	}

	if err := s.reminderRepo.Dismiss(reminderID, deviceID); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to dismiss reminder", http.StatusInternalServerError)
	}

	// Record sync event (reload to get updated status)
	reminder, _ = s.reminderRepo.FindByID(reminderID)
	_ = s.syncRepo.RecordReminderChange(userID, reminder, models.SyncActionUpdate, deviceID)

	return nil
}

// isPresetSnooze checks if the snooze duration is a free preset
func isPresetSnooze(minutes int) bool {
	presets := []int{5, 15, 30, 60} // Free presets: 5 min, 15 min, 30 min, 1 hour
	for _, preset := range presets {
		if minutes == preset {
			return true
		}
	}
	return false
}

// isAdvancedRecurrence checks if the recurrence rule requires premium
func isAdvancedRecurrence(rule *models.RecurrenceRule) bool {
	if rule == nil {
		return false
	}

	// Basic recurrence (free): daily, weekly (all days)
	// Advanced recurrence (premium): specific days, monthly, yearly, custom intervals

	// Monthly, yearly, and hourly are premium
	if rule.Frequency == models.FrequencyMonthly ||
		rule.Frequency == models.FrequencyYearly ||
		rule.Frequency == models.FrequencyHourly {
		return true
	}

	// Custom intervals (not 1) are premium
	if rule.Interval > 1 {
		return true
	}

	// Specific days of week selection is premium
	if len(rule.DaysOfWeek) > 0 && len(rule.DaysOfWeek) < 7 {
		return true
	}

	return false
}
