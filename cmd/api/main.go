package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/user/remind-me/backend/internal/config"
	"github.com/user/remind-me/backend/internal/database"
	gqlhandler "github.com/user/remind-me/backend/internal/graphql/handler"
	"github.com/user/remind-me/backend/internal/graphql/resolver"
	"github.com/user/remind-me/backend/internal/jobs"
	"github.com/user/remind-me/backend/internal/middleware"
	"github.com/user/remind-me/backend/internal/notification"
	"github.com/user/remind-me/backend/internal/notification/apns"
	"github.com/user/remind-me/backend/internal/notification/fcm"
	"github.com/user/remind-me/backend/internal/notification/slack"
	"github.com/user/remind-me/backend/internal/pubsub"
	"github.com/user/remind-me/backend/internal/repository"
	"github.com/user/remind-me/backend/internal/service"
	"github.com/user/remind-me/backend/pkg/jwt"
)

func main() {
	// Load .env file if it exists (for local development)
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize JWT manager
	jwtManager := jwt.NewManager(cfg.JWTSecret)

	// Initialize pub/sub hub for GraphQL subscriptions
	hub := pubsub.NewHub()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)
	reminderRepo := repository.NewReminderRepository(db)
	reminderListRepo := repository.NewReminderListRepository(db)
	notificationSoundRepo := repository.NewNotificationSoundRepository(db)
	syncRepo := repository.NewSyncRepository(db)

	// Initialize Slack client for signup notifications
	var slackClient *slack.Client
	if cfg.SlackWebhookURL != "" {
		slackClient = slack.NewClient(cfg.SlackWebhookURL)
		log.Printf("Slack notification client initialized")
	}

	// Initialize services
	authService := service.NewAuthService(userRepo, jwtManager, slackClient)
	reminderService := service.NewReminderService(reminderRepo, syncRepo, userRepo)
	reminderListService := service.NewReminderListService(reminderListRepo, reminderRepo)
	subscriptionService := service.NewSubscriptionService(cfg, userRepo)

	// Initialize notification clients (may be nil if not configured)
	var notificationDispatcher *notification.Dispatcher
	var notificationJob *jobs.NotificationJob

	var apnsClient *apns.Client
	if cfg.APNsKeyID != "" && cfg.APNsTeamID != "" && cfg.APNsPrivateKey != "" {
		apnsClient, err = apns.NewClient(apns.Config{
			KeyID:        cfg.APNsKeyID,
			TeamID:       cfg.APNsTeamID,
			PrivateKey:   cfg.APNsPrivateKey,
			BundleID:     cfg.APNsBundleID,
			IsProduction: cfg.IsProduction(),
		})
		if err != nil {
			log.Printf("Warning: APNs client not initialized: %v", err)
			apnsClient = nil
		}
	}

	var fcmClient *fcm.Client
	if cfg.FCMProjectID != "" && cfg.FCMPrivateKey != "" {
		fcmClient, err = fcm.NewClient(fcm.Config{
			ProjectID:       cfg.FCMProjectID,
			CredentialsJSON: cfg.FCMPrivateKey,
		})
		if err != nil {
			log.Printf("Warning: FCM client not initialized: %v", err)
			fcmClient = nil
		}
	}

	if apnsClient != nil || fcmClient != nil {
		notificationDispatcher = notification.NewDispatcher(apnsClient, fcmClient, deviceRepo)
		notificationJob = jobs.NewNotificationJob(reminderRepo, notificationDispatcher)
		log.Printf("Notification dispatcher initialized")
	}

	// Initialize GraphQL resolver
	gqlResolver := resolver.NewResolver(
		authService,
		reminderService,
		reminderListService,
		subscriptionService,
		userRepo,
		deviceRepo,
		reminderRepo,
		reminderListRepo,
		notificationSoundRepo,
		jwtManager,
		hub,
		notificationDispatcher,
	)

	// Initialize GraphQL handler
	graphqlHandler := gqlhandler.NewHandler(gqlResolver, jwtManager)

	// Set up Gin
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Rate limiter: 100 requests per minute
	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
	r.Use(middleware.RateLimitMiddleware(rateLimiter))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "app": "zalt"})
	})

	// Cron endpoint for processing due notifications
	// Called by GCP Cloud Scheduler every minute
	r.POST("/api/cron/notifications", func(c *gin.Context) {
		// Verify cron secret
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+cfg.CronSecret {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}

		if notificationJob == nil {
			c.JSON(503, gin.H{"error": "notification service not configured"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		defer cancel()

		count, err := notificationJob.ProcessDueReminders(ctx)
		if err != nil {
			log.Printf("Error processing due reminders: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"processed": count})
	})

	// Cron endpoint for cleaning up stale devices
	// Called by GCP Cloud Scheduler daily
	deviceCleanupJob := jobs.NewDeviceCleanupJob(deviceRepo)
	r.POST("/api/cron/device-cleanup", func(c *gin.Context) {
		// Verify cron secret
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+cfg.CronSecret {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		defer cancel()

		// Clean up devices not seen in 14 days
		count, err := deviceCleanupJob.CleanupStaleDevices(ctx, 14)
		if err != nil {
			log.Printf("Error cleaning up stale devices: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"deleted": count})
	})

	// Cron endpoint for purging soft-deleted accounts
	// Called by GCP Cloud Scheduler daily
	accountPurgeJob := jobs.NewAccountPurgeJob(db)
	r.POST("/api/cron/account-purge", func(c *gin.Context) {
		// Verify cron secret
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+cfg.CronSecret {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		defer cancel()

		// Purge accounts deleted more than 30 days ago
		count, err := accountPurgeJob.PurgeDeletedAccounts(ctx, 30)
		if err != nil {
			log.Printf("Error purging deleted accounts: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"purged": count})
	})

	// GraphQL endpoints
	// Single endpoint that handles both HTTP and WebSocket (for subscriptions)
	r.POST("/graphql", graphqlHandler.GraphQL)
	r.GET("/graphql", func(c *gin.Context) {
		// Check if this is a WebSocket upgrade request
		if c.GetHeader("Upgrade") == "websocket" {
			graphqlHandler.WebSocketHandler(c)
			return
		}
		// Check if this is a GraphQL query via GET (some clients use this)
		if c.Query("query") != "" {
			graphqlHandler.GraphQLGet(c)
			return
		}
		// Otherwise serve the playground
		graphqlHandler.Playground(c)
	})

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Zalt GraphQL server on port %s", port)
	log.Printf("GraphQL Playground available at http://localhost:%s/graphql", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
