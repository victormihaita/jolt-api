package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByGoogleID(googleID string) (*models.User, error) {
	var user models.User
	err := r.db.Where("google_id = ?", googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByAppleID(appleID string) (*models.User, error) {
	var user models.User
	err := r.db.Where("apple_id = ?", appleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.User{}, id).Error
}

func (r *UserRepository) FindOrCreate(googleID, email, displayName, avatarURL string) (*models.User, bool, error) {
	var user models.User
	err := r.db.Where("google_id = ?", googleID).First(&user).Error
	if err == nil {
		// User exists, update their info
		user.Email = email
		user.DisplayName = displayName
		user.AvatarURL = avatarURL
		if err := r.db.Save(&user).Error; err != nil {
			return nil, false, err
		}
		return &user, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	// Create new user
	user = models.User{
		GoogleID:    googleID,
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, false, err
	}

	return &user, true, nil
}

func (r *UserRepository) UpdatePremiumStatus(id uuid.UUID, isPremium bool, premiumUntil *time.Time) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_premium":    isPremium,
			"premium_until": premiumUntil,
		}).Error
}

func (r *UserRepository) FindOrCreateByAppleID(appleID, email, displayName string) (*models.User, bool, error) {
	var user models.User
	err := r.db.Where("apple_id = ?", appleID).First(&user).Error
	if err == nil {
		// User exists, update their info if provided
		updated := false
		if email != "" && user.Email == "" {
			user.Email = email
			updated = true
		}
		if displayName != "" && user.DisplayName == "" {
			user.DisplayName = displayName
			updated = true
		}
		if updated {
			if err := r.db.Save(&user).Error; err != nil {
				return nil, false, err
			}
		}
		return &user, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	// Check if user exists with same email (account linking)
	if email != "" {
		err = r.db.Where("email = ?", email).First(&user).Error
		if err == nil {
			// Link Apple ID to existing account
			user.AppleID = appleID
			if displayName != "" && user.DisplayName == "" {
				user.DisplayName = displayName
			}
			if err := r.db.Save(&user).Error; err != nil {
				return nil, false, err
			}
			return &user, false, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, false, err
		}
	}

	// Create new user
	// If no email provided, use a placeholder (Apple may hide the real email)
	userEmail := email
	if userEmail == "" {
		userEmail = appleID + "@privaterelay.appleid.com"
	}

	// If no display name, use a default
	userName := displayName
	if userName == "" {
		userName = "Apple User"
	}

	user = models.User{
		AppleID:     appleID,
		Email:       userEmail,
		DisplayName: userName,
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, false, err
	}

	return &user, true, nil
}
