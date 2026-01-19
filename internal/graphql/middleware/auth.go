package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/pkg/jwt"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	DeviceIDKey contextKey = "deviceID"
	ClaimsKey   contextKey = "claims"
)

// GraphQLAuthMiddleware extracts JWT from Authorization header and adds user info to context
func GraphQLAuthMiddleware(jwtManager *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token != authHeader { // Had "Bearer " prefix
					claims, err := jwtManager.ValidateToken(token)
					if err == nil {
						ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
						ctx = context.WithValue(ctx, ClaimsKey, claims)
						if claims.DeviceID != nil {
							ctx = context.WithValue(ctx, DeviceIDKey, *claims.DeviceID)
						}
					}
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from the context
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// MustGetUserID extracts the user ID from the context or panics
func MustGetUserID(ctx context.Context) uuid.UUID {
	userID, ok := GetUserID(ctx)
	if !ok {
		panic("user ID not found in context")
	}
	return userID
}

// GetDeviceID extracts the device ID from the context
func GetDeviceID(ctx context.Context) (*uuid.UUID, bool) {
	deviceID, ok := ctx.Value(DeviceIDKey).(uuid.UUID)
	if !ok {
		return nil, false
	}
	return &deviceID, true
}

// GetClaims extracts the JWT claims from the context
func GetClaims(ctx context.Context) (*jwt.Claims, bool) {
	claims, ok := ctx.Value(ClaimsKey).(*jwt.Claims)
	return claims, ok
}

// IsAuthenticated checks if the request has a valid user ID in context
func IsAuthenticated(ctx context.Context) bool {
	_, ok := GetUserID(ctx)
	return ok
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithDeviceID adds a device ID to the context
func WithDeviceID(ctx context.Context, deviceID uuid.UUID) context.Context {
	return context.WithValue(ctx, DeviceIDKey, deviceID)
}

// WebSocketInitFunc is the function to call when a WebSocket connection is initialized
// It extracts the token from connection params and validates it
func WebSocketInitFunc(jwtManager *jwt.Manager) func(ctx context.Context, initPayload map[string]interface{}) (context.Context, error) {
	return func(ctx context.Context, initPayload map[string]interface{}) (context.Context, error) {
		authValue, ok := initPayload["Authorization"]
		if !ok {
			// Try lowercase
			authValue, ok = initPayload["authorization"]
		}
		if !ok {
			return ctx, nil // Allow unauthenticated connections, resolver will check auth
		}

		authStr, ok := authValue.(string)
		if !ok {
			return ctx, nil
		}

		token := strings.TrimPrefix(authStr, "Bearer ")
		if token == authStr {
			return ctx, nil
		}

		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			return ctx, nil // Invalid token, but don't reject connection
		}

		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, ClaimsKey, claims)
		if claims.DeviceID != nil {
			ctx = context.WithValue(ctx, DeviceIDKey, *claims.DeviceID)
		}

		return ctx, nil
	}
}
