package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// PrivateCORS restricts browser access to the configured dashboard origin.
// Cookie-backed dashboard routes must not be callable from arbitrary origins.
func PrivateCORS(frontendURL string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{frontendURL},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// PublicCORS allows browser checkout flows from any origin.
// These routes never rely on cookies, the access_code is the boundary.
func PublicCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
		},
		MaxAge: 12 * time.Hour,
	})
}
