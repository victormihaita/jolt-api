package dto

// GoogleAuthRequest is the request body for Google OAuth login
type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// AuthResponse is the response for successful authentication
type AuthResponse struct {
	AccessToken            string  `json:"access_token"`
	RefreshToken           string  `json:"refresh_token"`
	ExpiresIn              int64   `json:"expires_in"`
	User                   UserDTO `json:"user"`
	AccountPendingDeletion bool    `json:"account_pending_deletion"`
}

// RefreshTokenRequest is the request body for refreshing tokens
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserDTO represents user data in responses
type UserDTO struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	Timezone    string  `json:"timezone"`
	IsPremium   bool    `json:"is_premium"`
}

// UpdateUserRequest is the request body for updating user preferences
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
}
