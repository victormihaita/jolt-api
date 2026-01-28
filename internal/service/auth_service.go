package service

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/user/remind-me/backend/internal/dto"
	"github.com/user/remind-me/backend/internal/models"
	"github.com/user/remind-me/backend/internal/notification/slack"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
	"github.com/user/remind-me/backend/pkg/jwt"
)

type AuthService struct {
	userRepo      *repository.UserRepository
	jwtManager    *jwt.Manager
	googleClient  *http.Client
	slackClient   *slack.Client
	appleKeys     map[string]*rsa.PublicKey
	appleKeysMu   sync.RWMutex
	appleKeysTime time.Time
}

func NewAuthService(userRepo *repository.UserRepository, jwtManager *jwt.Manager, slackClient *slack.Client) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		jwtManager:   jwtManager,
		googleClient: &http.Client{},
		slackClient:  slackClient,
		appleKeys:    make(map[string]*rsa.PublicKey),
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

// AppleUserInfo represents extracted info from Apple's identity token
type AppleUserInfo struct {
	Sub   string // Apple's unique user identifier
	Email string
}

// AuthenticateWithApple verifies the Apple identity token and returns auth tokens
func (s *AuthService) AuthenticateWithApple(ctx context.Context, identityToken, userIdentifier, email, displayName string) (*dto.AuthResponse, error) {
	// Verify the identity token with Apple
	appleUserInfo, err := s.verifyAppleToken(ctx, identityToken)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeUnauthorized, "Invalid Apple token", http.StatusUnauthorized)
	}

	// Verify that the user identifier matches the token's subject
	if appleUserInfo.Sub != userIdentifier {
		return nil, apperrors.New(apperrors.CodeUnauthorized, "User identifier mismatch", http.StatusUnauthorized)
	}

	// Use email from token if available, otherwise use provided email
	userEmail := appleUserInfo.Email
	if userEmail == "" {
		userEmail = email
	}

	// Find or create user
	user, isNew, err := s.userRepo.FindOrCreateByAppleID(
		userIdentifier,
		userEmail,
		displayName,
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
		s.slackClient.SendNewUserNotification(user.Email, user.DisplayName+" (Apple)")
	}

	return &dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    s.jwtManager.GetAccessDuration(),
		User:         s.userToDTO(user),
	}, nil
}

// verifyAppleToken verifies the Apple identity token using Apple's public keys
func (s *AuthService) verifyAppleToken(ctx context.Context, identityToken string) (*AppleUserInfo, error) {
	// Fetch Apple's public keys if needed
	if err := s.refreshAppleKeysIfNeeded(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch Apple public keys: %w", err)
	}

	// Parse the token without verification first to get the key ID
	token, _, err := new(gojwt.Parser).ParseUnverified(identityToken, gojwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing key ID in token header")
	}

	// Get the public key
	s.appleKeysMu.RLock()
	publicKey, ok := s.appleKeys[kid]
	s.appleKeysMu.RUnlock()
	if !ok {
		// Try refreshing keys in case Apple rotated them
		if err := s.forceRefreshAppleKeys(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh Apple public keys: %w", err)
		}
		s.appleKeysMu.RLock()
		publicKey, ok = s.appleKeys[kid]
		s.appleKeysMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unknown key ID: %s", kid)
		}
	}

	// Parse and verify the token
	token, err = gojwt.Parse(identityToken, func(token *gojwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}

	claims, ok := token.Claims.(gojwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	iss, _ := claims["iss"].(string)
	if iss != "https://appleid.apple.com" {
		return nil, fmt.Errorf("invalid issuer: %s", iss)
	}

	// Extract user info
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	if sub == "" {
		return nil, fmt.Errorf("missing subject in token")
	}

	return &AppleUserInfo{
		Sub:   sub,
		Email: email,
	}, nil
}

// Apple JWKS response structure
type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (s *AuthService) refreshAppleKeysIfNeeded(ctx context.Context) error {
	s.appleKeysMu.RLock()
	needsRefresh := len(s.appleKeys) == 0 || time.Since(s.appleKeysTime) > 24*time.Hour
	s.appleKeysMu.RUnlock()

	if !needsRefresh {
		return nil
	}

	return s.forceRefreshAppleKeys(ctx)
}

func (s *AuthService) forceRefreshAppleKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://appleid.apple.com/auth/keys", nil)
	if err != nil {
		return err
	}

	resp, err := s.googleClient.Do(req) // Reuse the HTTP client
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch Apple keys: %s", string(body))
	}

	var jwks appleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		// Decode the modulus
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)

		// Decode the exponent
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		var e int
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}

		keys[jwk.Kid] = &rsa.PublicKey{
			N: n,
			E: e,
		}
	}

	s.appleKeysMu.Lock()
	s.appleKeys = keys
	s.appleKeysTime = time.Now()
	s.appleKeysMu.Unlock()

	return nil
}
