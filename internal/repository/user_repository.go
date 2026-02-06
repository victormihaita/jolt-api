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

func (r *UserRepository) FindOrCreate(googleID, email, displayName, avatarURL string) (*models.User, bool, bool, error) {
	var user models.User
	err := r.db.Where("google_id = ?", googleID).First(&user).Error
	if err == nil {
		// User exists, update their info
		user.Email = email
		user.DisplayName = displayName
		user.AvatarURL = avatarURL
		if err := r.db.Save(&user).Error; err != nil {
			return nil, false, false, err
		}
		return &user, false, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, false, err
	}

	// Check for soft-deleted user with same Google ID
	err = r.db.Unscoped().Where("google_id = ? AND deleted_at IS NOT NULL", googleID).First(&user).Error
	if err == nil {
		// Found soft-deleted user, restore and update info
		user.Email = email
		user.DisplayName = displayName
		user.AvatarURL = avatarURL
		if err := r.db.Unscoped().Model(&user).Updates(map[string]interface{}{
			"deleted_at":    nil,
			"email":         email,
			"display_name":  displayName,
			"avatar_url":    avatarURL,
		}).Error; err != nil {
			return nil, false, false, err
		}
		return &user, false, true, nil
	}

	// Check for soft-deleted user with same email
	if email != "" {
		err = r.db.Unscoped().Where("email = ? AND deleted_at IS NOT NULL", email).First(&user).Error
		if err == nil {
			// Found soft-deleted user by email, restore and link Google ID
			if err := r.db.Unscoped().Model(&user).Updates(map[string]interface{}{
				"deleted_at":   nil,
				"google_id":    googleID,
				"email":        email,
				"display_name": displayName,
				"avatar_url":   avatarURL,
			}).Error; err != nil {
				return nil, false, false, err
			}
			return &user, false, true, nil
		}
	}

	// Create new user
	user = models.User{
		GoogleID:    &googleID,
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, false, false, err
	}

	return &user, true, false, nil
}

func (r *UserRepository) UpdatePremiumStatus(id uuid.UUID, isPremium bool, premiumUntil *time.Time) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_premium":    isPremium,
			"premium_until": premiumUntil,
		}).Error
}

func (r *UserRepository) FindOrCreateByAppleID(appleID, email, displayName string) (*models.User, bool, bool, error) {
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
				return nil, false, false, err
			}
		}
		return &user, false, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, false, err
	}

	// Check for soft-deleted user with same Apple ID
	err = r.db.Unscoped().Where("apple_id = ? AND deleted_at IS NOT NULL", appleID).First(&user).Error
	if err == nil {
		// Found soft-deleted user, restore and update info
		updates := map[string]interface{}{
			"deleted_at": nil,
		}
		if email != "" {
			updates["email"] = email
		}
		if displayName != "" {
			updates["display_name"] = displayName
		}
		if err := r.db.Unscoped().Model(&user).Updates(updates).Error; err != nil {
			return nil, false, false, err
		}
		return &user, false, true, nil
	}

	// Check if user exists with same email (account linking)
	// Use Unscoped to also find soft-deleted users by email
	if email != "" {
		err = r.db.Where("email = ?", email).First(&user).Error
		if err == nil {
			// Link Apple ID to existing active account
			user.AppleID = &appleID
			if displayName != "" && user.DisplayName == "" {
				user.DisplayName = displayName
			}
			if err := r.db.Save(&user).Error; err != nil {
				return nil, false, false, err
			}
			return &user, false, false, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, false, false, err
		}

		// Check for soft-deleted user with same email
		err = r.db.Unscoped().Where("email = ? AND deleted_at IS NOT NULL", email).First(&user).Error
		if err == nil {
			// Found soft-deleted user by email, restore, link Apple ID
			updates := map[string]interface{}{
				"deleted_at": nil,
				"apple_id":   appleID,
			}
			if displayName != "" {
				updates["display_name"] = displayName
			}
			if err := r.db.Unscoped().Model(&user).Updates(updates).Error; err != nil {
				return nil, false, false, err
			}
			return &user, false, true, nil
		}
	}

	// Create new user
	// If no email provided, use a placeholder (Apple may hide the real email)
	userEmail := email
	if userEmail == "" {
		userEmail = appleID + "@privaterelay.appleid.com"
	}

	// Check for soft-deleted user with the resolved email (covers placeholder emails too)
	err = r.db.Unscoped().Where("email = ? AND deleted_at IS NOT NULL", userEmail).First(&user).Error
	if err == nil {
		// Found soft-deleted user by email, restore and link Apple ID
		updates := map[string]interface{}{
			"deleted_at": nil,
			"apple_id":   appleID,
		}
		if displayName != "" {
			updates["display_name"] = displayName
		}
		if err := r.db.Unscoped().Model(&user).Updates(updates).Error; err != nil {
			return nil, false, false, err
		}
		return &user, false, true, nil
	}

	// If no display name, use a default
	userName := displayName
	if userName == "" {
		userName = "Apple User"
	}

	user = models.User{
		AppleID:     &appleID,
		Email:       userEmail,
		DisplayName: userName,
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, false, false, err
	}

	return &user, true, false, nil
}
