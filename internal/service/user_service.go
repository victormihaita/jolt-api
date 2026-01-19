package service

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// GetByID returns a user by their ID.
func (s *UserService) GetByID(id uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return user, nil
}

// GetByEmail returns a user by their email.
func (s *UserService) GetByEmail(email string) (*models.User, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return user, nil
}

// Update updates a user's profile.
func (s *UserService) Update(id uuid.UUID, displayName, timezone *string) (*models.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	if displayName != nil {
		user.DisplayName = *displayName
	}
	if timezone != nil {
		user.Timezone = *timezone
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to update user", http.StatusInternalServerError)
	}

	return user, nil
}

// UpdatePremiumStatus updates a user's premium subscription status.
func (s *UserService) UpdatePremiumStatus(id uuid.UUID, isPremium bool, premiumUntil *time.Time) error {
	if err := s.userRepo.UpdatePremiumStatus(id, isPremium, premiumUntil); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to update premium status", http.StatusInternalServerError)
	}
	return nil
}

// Delete deletes a user account.
func (s *UserService) Delete(id uuid.UUID) error {
	if err := s.userRepo.Delete(id); err != nil {
		return apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to delete user", http.StatusInternalServerError)
	}
	return nil
}
