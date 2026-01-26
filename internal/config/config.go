package config

import (
	"os"
)

type Config struct {
	// Database
	DatabaseURL string

	// Auth
	GoogleClientID string
	JWTSecret      string

	// Push Notifications
	APNsKeyID      string
	APNsTeamID     string
	APNsPrivateKey string
	APNsBundleID   string
	FCMProjectID   string
	FCMPrivateKey  string

	// RevenueCat
	RevenueCatAPIKey string

	// Slack
	SlackWebhookURL string

	// Cron
	CronSecret string

	// Server
	Port        string
	Environment string
}

func Load() *Config {
	return &Config{
		// Database
		DatabaseURL: getEnv("DATABASE_URL", ""),

		// Auth
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),

		// Push Notifications
		APNsKeyID:      getEnv("APNS_KEY_ID", ""),
		APNsTeamID:     getEnv("APNS_TEAM_ID", ""),
		APNsPrivateKey: getEnv("APNS_PRIVATE_KEY", ""),
		APNsBundleID:   getEnv("APNS_BUNDLE_ID", "com.remindme.app"),
		FCMProjectID:   getEnv("FCM_PROJECT_ID", ""),
		FCMPrivateKey:  getEnv("FCM_PRIVATE_KEY", ""),

		// RevenueCat
		RevenueCatAPIKey: getEnv("REVENUECAT_API_KEY", ""),

		// Slack
		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),

		// Cron
		CronSecret: getEnv("CRON_SECRET", ""),

		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}
