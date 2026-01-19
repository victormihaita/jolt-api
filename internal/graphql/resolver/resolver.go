package resolver

import (
	"github.com/user/remind-me/backend/internal/pubsub"
	"github.com/user/remind-me/backend/internal/repository"
	"github.com/user/remind-me/backend/internal/service"
	"github.com/user/remind-me/backend/pkg/jwt"
)

// Resolver is the root resolver for all GraphQL operations
type Resolver struct {
	AuthService         *service.AuthService
	ReminderService     *service.ReminderService
	SubscriptionService *service.SubscriptionService
	UserRepo            *repository.UserRepository
	DeviceRepo          *repository.DeviceRepository
	JWTManager          *jwt.Manager
	Hub                 *pubsub.Hub
}

// NewResolver creates a new Resolver with all dependencies
func NewResolver(
	authService *service.AuthService,
	reminderService *service.ReminderService,
	subscriptionService *service.SubscriptionService,
	userRepo *repository.UserRepository,
	deviceRepo *repository.DeviceRepository,
	jwtManager *jwt.Manager,
	hub *pubsub.Hub,
) *Resolver {
	return &Resolver{
		AuthService:         authService,
		ReminderService:     reminderService,
		SubscriptionService: subscriptionService,
		UserRepo:            userRepo,
		DeviceRepo:          deviceRepo,
		JWTManager:          jwtManager,
		Hub:                 hub,
	}
}
