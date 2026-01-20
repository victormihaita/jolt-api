package service

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

type ReminderListService struct {
	listRepo     *repository.ReminderListRepository
	reminderRepo *repository.ReminderRepository
}

func NewReminderListService(
	listRepo *repository.ReminderListRepository,
	reminderRepo *repository.ReminderRepository,
) *ReminderListService {
	return &ReminderListService{
		listRepo:     listRepo,
		reminderRepo: reminderRepo,
	}
}

func (s *ReminderListService) Create(userID uuid.UUID, req dto.CreateReminderListRequest) (*dto.ReminderListDTO, error) {
	// Get the count to determine sort order
	count, _ := s.listRepo.CountByUser(userID)

	list := &models.ReminderList{
		UserID:    userID,
		Name:      req.Name,
		ColorHex:  defaultString(req.ColorHex, "#007AFF"),
		IconName:  defaultString(req.IconName, "list.bullet"),
		SortOrder: int(count),
		IsDefault: false,
	}

	if err := s.listRepo.Create(list); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to create list", http.StatusInternalServerError)
	}

	result := dto.ReminderListToDTO(list, 0)
	return &result, nil
}

func (s *ReminderListService) GetByID(userID, listID uuid.UUID) (*dto.ReminderListDTO, error) {
	list, err := s.listRepo.FindByIDAndUser(listID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderListNotFound
	}

	count, _ := s.listRepo.GetReminderCountForList(list.ID)
	result := dto.ReminderListToDTO(list, count)
	return &result, nil
}

func (s *ReminderListService) List(userID uuid.UUID) ([]dto.ReminderListDTO, error) {
	// Ensure default list exists
	_, _ = s.listRepo.EnsureDefaultListExists(userID)

	lists, err := s.listRepo.ListByUser(userID)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to list reminder lists", http.StatusInternalServerError)
	}

	result := make([]dto.ReminderListDTO, len(lists))
	for i, list := range lists {
		count, _ := s.listRepo.GetReminderCountForList(list.ID)
		result[i] = dto.ReminderListToDTO(&list, count)
	}

	return result, nil
}

func (s *ReminderListService) Update(userID, listID uuid.UUID, req dto.UpdateReminderListRequest) (*dto.ReminderListDTO, error) {
	list, err := s.listRepo.FindByIDAndUser(listID, userID)
	if err != nil {
		return nil, apperrors.ErrReminderListNotFound
	}

	// Apply updates
	if req.Name != nil {
		list.Name = *req.Name
	}
	if req.ColorHex != nil {
		list.ColorHex = *req.ColorHex
	}
	if req.IconName != nil {
		list.IconName = *req.IconName
	}
	if req.SortOrder != nil {
		list.SortOrder = *req.SortOrder
	}

	if err := s.listRepo.Update(list); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to update list", http.StatusInternalServerError)
	}

	count, _ := s.listRepo.GetReminderCountForList(list.ID)
	result := dto.ReminderListToDTO(list, count)
	return &result, nil
}

func (s *ReminderListService) Delete(userID, listID uuid.UUID) error {
	list, err := s.listRepo.FindByIDAndUser(listID, userID)
	if err != nil {
		return apperrors.ErrReminderListNotFound
	}

	// Cannot delete the default list
	if list.IsDefault {
		return apperrors.ErrCannotDeleteDefaultList
	}

	// Cascade delete all reminders in this list
	if err := s.listRepo.DeleteRemindersByListID(listID); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to delete reminders", http.StatusInternalServerError)
	}

	// Soft delete the list
	if err := s.listRepo.SoftDelete(listID); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to delete list", http.StatusInternalServerError)
	}

	return nil
}

func (s *ReminderListService) Reorder(userID uuid.UUID, listIDs []uuid.UUID) ([]dto.ReminderListDTO, error) {
	// Create a map of list ID to new sort order
	idOrders := make(map[uuid.UUID]int)
	for i, id := range listIDs {
		idOrders[id] = i
	}

	// Update all sort orders
	if err := s.listRepo.UpdateSortOrders(userID, idOrders); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to reorder lists", http.StatusInternalServerError)
	}

	// Return the updated list
	return s.List(userID)
}

func (s *ReminderListService) EnsureDefaultList(userID uuid.UUID) (*dto.ReminderListDTO, error) {
	list, err := s.listRepo.EnsureDefaultListExists(userID)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to ensure default list", http.StatusInternalServerError)
	}

	count, _ := s.listRepo.GetReminderCountForList(list.ID)
	result := dto.ReminderListToDTO(list, count)
	return &result, nil
}

func defaultString(ptr *string, defaultVal string) string {
	if ptr == nil || *ptr == "" {
		return defaultVal
	}
	return *ptr
}
