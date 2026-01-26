package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/notification/slack"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
	"github.com/user/remind-me/backend/pkg/jwt"
)

type AuthService struct {
	userRepo     *repository.UserRepository
	jwtManager   *jwt.Manager
	googleClient *http.Client
	slackClient  *slack.Client
}

func NewAuthService(userRepo *repository.UserRepository, jwtManager *jwt.Manager, slackClient *slack.Client) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		jwtManager:   jwtManager,
		googleClient: &http.Client{},
		slackClient:  slackClient,
	}
}

// GoogleUserInfo represents the response from Google's userinfo endpoint
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// AuthenticateWithGoogle verifies the Google ID token and returns auth tokens
func (s *AuthService) AuthenticateWithGoogle(ctx context.Context, idToken string) (*dto.AuthResponse, error) {
	// Verify the ID token with Google
	userInfo, err := s.verifyGoogleToken(ctx, idToken)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeUnauthorized, "Invalid Google token", http.StatusUnauthorized)
	}

	// Find or create user
	user, isNew, err := s.userRepo.FindOrCreate(
		userInfo.ID,
		userInfo.Email,
		userInfo.Name,
		userInfo.Picture,
	)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to create user", http.StatusInternalServerError)
	}

	// Generate JWT tokens
	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, nil)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to generate tokens", http.StatusInternalServerError)
	}

	// Send Slack notification for new user signups
	if isNew && s.slackClient != nil {
		s.slackClient.SendNewUserNotification(userInfo.Email, userInfo.Name)
	}

	return &dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    s.jwtManager.GetAccessDuration(),
		User:         s.userToDTO(user),
	}, nil
}

// RefreshToken generates new tokens from a valid refresh token
func (s *AuthService) RefreshToken(refreshToken string) (*dto.AuthResponse, error) {
	tokenPair, err := s.jwtManager.RefreshTokens(refreshToken)
	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, apperrors.ErrTokenExpired
		}
		return nil, apperrors.ErrInvalidToken
	}

	// Get the user from the token claims
	claims, err := s.jwtManager.ValidateToken(tokenPair.AccessToken)
	if err != nil {
		return nil, apperrors.ErrInvalidToken
	}

	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	return &dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    s.jwtManager.GetAccessDuration(),
		User:         s.userToDTO(user),
	}, nil
}

// verifyGoogleToken verifies the Google ID token using Google's tokeninfo endpoint
func (s *AuthService) verifyGoogleToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	// Use Google's tokeninfo endpoint to verify the ID token
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return nil, err
	}

	resp, err := s.googleClient.Do(req)
	if err != nil {
		fmt.Printf("Error calling Google tokeninfo: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Google token verification failed with status %d: %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("google token verification failed: %s", string(body))
	}

	// Parse the token info response
	var tokenInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, err
	}

	return &GoogleUserInfo{
		ID:            tokenInfo.Sub,
		Email:         tokenInfo.Email,
		VerifiedEmail: tokenInfo.EmailVerified == "true",
		Name:          tokenInfo.Name,
		Picture:       tokenInfo.Picture,
		GivenName:     tokenInfo.GivenName,
		FamilyName:    tokenInfo.FamilyName,
	}, nil
}

func (s *AuthService) userToDTO(user *models.User) dto.UserDTO {
	return dto.UserDTO{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Timezone:    user.Timezone,
		IsPremium:   user.HasActivePremium(),
	}
}
