package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/user/remind-me/backend/internal/config"
	"github.com/user/remind-me/backend/internal/database"
	gqlhandler "github.com/user/remind-me/backend/internal/graphql/handler"
	"github.com/user/remind-me/backend/internal/graphql/resolver"
	"github.com/user/remind-me/backend/internal/middleware"
	"github.com/user/remind-me/backend/internal/repository"
	"github.com/user/remind-me/backend/internal/service"
	"github.com/user/remind-me/backend/internal/pubsub"
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
	syncRepo := repository.NewSyncRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, jwtManager)
	reminderService := service.NewReminderService(reminderRepo, syncRepo, userRepo)
	subscriptionService := service.NewSubscriptionService(cfg, userRepo)

	// Initialize GraphQL resolver
	gqlResolver := resolver.NewResolver(
		authService,
		reminderService,
		subscriptionService,
		userRepo,
		deviceRepo,
		jwtManager,
		hub,
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
		c.JSON(200, gin.H{"status": "ok", "app": "jolt"})
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

	log.Printf("Starting Jolt GraphQL server on port %s", port)
	log.Printf("GraphQL Playground available at http://localhost:%s/graphql", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
