package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/config"
	"github.com/user/remind-me/backend/internal/repository"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

// RevenueCat API response structures
type RevenueCatSubscriber struct {
	Entitlements    map[string]RevenueCatEntitlement `json:"entitlements"`
	Subscriptions   map[string]RevenueCatSubscription `json:"subscriptions"`
	NonSubscriptions map[string][]RevenueCatPurchase `json:"non_subscriptions"`
}

type RevenueCatEntitlement struct {
	ExpiresDate        *string `json:"expires_date"`
	GracePeriodExpires *string `json:"grace_period_expires_date"`
	ProductIdentifier  string  `json:"product_identifier"`
	PurchaseDate       string  `json:"purchase_date"`
}

type RevenueCatSubscription struct {
	ExpiresDate       *string `json:"expires_date"`
	IsSandbox         bool    `json:"is_sandbox"`
	OriginalPurchaseDate string `json:"original_purchase_date"`
	PeriodType        string  `json:"period_type"`
	ProductIdentifier string  `json:"product_identifier"`
	PurchaseDate      string  `json:"purchase_date"`
	Store             string  `json:"store"`
	UnsubscribeDetected bool  `json:"unsubscribe_detected_at"`
}

type RevenueCatPurchase struct {
	ID           string `json:"id"`
	PurchaseDate string `json:"purchase_date"`
	Store        string `json:"store"`
}

type RevenueCatResponse struct {
	RequestDate string               `json:"request_date"`
	Subscriber  RevenueCatSubscriber `json:"subscriber"`
}

type SubscriptionService struct {
	config   *config.Config
	userRepo *repository.UserRepository
}

func NewSubscriptionService(cfg *config.Config, userRepo *repository.UserRepository) *SubscriptionService {
	return &SubscriptionService{
		config:   cfg,
		userRepo: userRepo,
	}
}

// VerifySubscription checks a user's subscription status with RevenueCat
// and updates their premium status in the database
func (s *SubscriptionService) VerifySubscription(userID uuid.UUID) (bool, *time.Time, error) {
	// Get subscriber info from RevenueCat
	subscriber, err := s.getRevenueCatSubscriber(userID.String())
	if err != nil {
		return false, nil, err
	}

	// Check for active "premium" entitlement - must match RevenueCat dashboard
	premiumEntitlement, hasPremium := subscriber.Entitlements["premium"]

	var isPremium bool
	var premiumUntil *time.Time

	if hasPremium {
		isPremium = true

		// Parse expiration date if exists (nil means lifetime)
		if premiumEntitlement.ExpiresDate != nil {
			expiresTime, err := time.Parse(time.RFC3339, *premiumEntitlement.ExpiresDate)
			if err == nil {
				// Check if still active
				if expiresTime.After(time.Now()) {
					premiumUntil = &expiresTime
				} else {
					isPremium = false
				}
			}
		}
		// If ExpiresDate is nil, it's a lifetime purchase - premiumUntil stays nil
	}

	// Update user's premium status in database
	if err := s.userRepo.UpdatePremiumStatus(userID, isPremium, premiumUntil); err != nil {
		return false, nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to update premium status", http.StatusInternalServerError)
	}

	return isPremium, premiumUntil, nil
}

// getRevenueCatSubscriber fetches subscriber info from RevenueCat API
func (s *SubscriptionService) getRevenueCatSubscriber(appUserID string) (*RevenueCatSubscriber, error) {
	if s.config.RevenueCatAPIKey == "" {
		return nil, apperrors.New(apperrors.CodeInternalError, "RevenueCat API key not configured", http.StatusInternalServerError)
	}

	url := fmt.Sprintf("https://api.revenuecat.com/v1/subscribers/%s", appUserID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to create request", http.StatusInternalServerError)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.RevenueCatAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to contact RevenueCat", http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to read response", http.StatusInternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		// RevenueCat returns 404 for users with no purchases - treat as no subscription
		if resp.StatusCode == http.StatusNotFound {
			return &RevenueCatSubscriber{
				Entitlements:     make(map[string]RevenueCatEntitlement),
				Subscriptions:    make(map[string]RevenueCatSubscription),
				NonSubscriptions: make(map[string][]RevenueCatPurchase),
			}, nil
		}
		return nil, apperrors.New(apperrors.CodeInternalError, fmt.Sprintf("RevenueCat API error: %s", string(body)), http.StatusInternalServerError)
	}

	var rcResponse RevenueCatResponse
	if err := json.Unmarshal(body, &rcResponse); err != nil {
		return nil, apperrors.Wrap(err, apperrors.CodeInternalError, "Failed to parse RevenueCat response", http.StatusInternalServerError)
	}

	return &rcResponse.Subscriber, nil
}
