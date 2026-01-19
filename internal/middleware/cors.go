package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns a CORS middleware configuration
func CORSMiddleware() gin.HandlerFunc {
	corsHandler := cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"https://*.vercel.app",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept",
			"Accept-Encoding",
			"Authorization",
			"X-Requested-With",
			"X-Device-ID",
			"Upgrade",
			"Connection",
			"Sec-WebSocket-Key",
			"Sec-WebSocket-Version",
			"Sec-WebSocket-Protocol",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})

	return func(c *gin.Context) {
		// Skip CORS for WebSocket upgrade requests (mobile apps don't send Origin header)
		if c.GetHeader("Upgrade") == "websocket" {
			c.Next()
			return
		}

		// Allow requests without Origin header (mobile apps, curl, etc.)
		if c.GetHeader("Origin") == "" {
			c.Next()
			return
		}

		corsHandler(c)
	}
}
