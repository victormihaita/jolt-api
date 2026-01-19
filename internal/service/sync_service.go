package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/repository"
)

type SyncService struct {
	syncRepo     *repository.SyncRepository
	reminderRepo *repository.ReminderRepository
}

func NewSyncService(syncRepo *repository.SyncRepository, reminderRepo *repository.ReminderRepository) *SyncService {
	return &SyncService{
		syncRepo:     syncRepo,
		reminderRepo: reminderRepo,
	}
}

// GetChangesSince returns all sync events since the given timestamp.
func (s *SyncService) GetChangesSince(userID uuid.UUID, since time.Time, limit int) (*dto.SyncResponse, error) {
	events, hasMore, err := s.syncRepo.GetChangesSince(userID, since, limit)
	if err != nil {
		return nil, err
	}

	syncEvents := make([]dto.SyncEvent, len(events))
	for i, e := range events {
		syncEvents[i] = dto.SyncEvent{
			ID:         e.ID,
			EntityType: string(e.EntityType),
			EntityID:   e.EntityID,
			Action:     string(e.Action),
			Payload:    e.Payload,
			DeviceID:   e.DeviceID,
			CreatedAt:  e.CreatedAt,
		}
	}

	var nextCursor *string
	if hasMore && len(events) > 0 {
		cursor := events[len(events)-1].CreatedAt.Format(time.RFC3339Nano)
		nextCursor = &cursor
	}

	return &dto.SyncResponse{
		Changes:    syncEvents,
		LastSyncAt: time.Now(),
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}
