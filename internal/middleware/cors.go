package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS configures cross-origin resource sharing for the public checkout
// endpoints. The React checkout app runs on a different origin than the
// Payfake server, without CORS the browser blocks the requests before
// they even reach the server.
//
// We apply two different CORS policies:
//   - Public routes (/api/v1/public/*) → permissive, any origin allowed.
//     The checkout page can be hosted anywhere, we can't know the origin
//     in advance. This is safe because public endpoints authenticate via
//     access_code, not cookies or credentials.
//   - All other routes → restrictive, only FRONTEND_URL allowed.
//     Dashboard and API routes should only be called from known origins.
func CORSPublic() gin.HandlerFunc {
	return cors.New(cors.Config{
		// Allow any origin for public checkout endpoints.
		// The access_code is the security boundary, not the origin.
		// Paystack's own public charge endpoints work the same way.
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

		// We don't allow credentials on public routes, no cookies,
		// no Authorization headers from the browser's credential store.
		// The access_code goes in the request body, not a header.
		AllowCredentials: false,

		MaxAge: 12 * time.Hour,
	})
}

func CORSPrivate(allowedOrigin string) gin.HandlerFunc {
	return cors.New(cors.Config{
		// Only allow requests from the configured frontend URL.
		// This covers the dashboard, which should only ever be
		// accessed from our own React dashboard app.
		AllowOrigins: []string{allowedOrigin},

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

		// Dashboard calls use the JWT in the Authorization header
		// credentials are allowed here because we know the origin.
		AllowCredentials: true,

		MaxAge: 12 * time.Hour,
	})
}
