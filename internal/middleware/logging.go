package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingMiddleware logs request details
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		// Get user ID if available
		userID := ""
		if id, exists := c.Get(UserIDKey); exists {
			userID = id.(string)
		}

		// Log the request
		if userID != "" {
			log.Printf("[%s] %d | %s | %s | %s | user=%s",
				method,
				status,
				latency,
				clientIP,
				path,
				userID,
			)
		} else {
			log.Printf("[%s] %d | %s | %s | %s",
				method,
				status,
				latency,
				clientIP,
				path,
			)
		}

		// Log errors
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				log.Printf("Error: %v", err.Err)
			}
		}
	}
}
