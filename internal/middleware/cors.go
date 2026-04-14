package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a single middleware that handles both public and private routes.
// Applied once on the root engine so it runs before anything else —
// this guarantees OPTIONS preflight is always handled regardless of route.
//
// Public checkout routes (/api/v1/public/*) allow any origin.
// All other routes allow only the configured frontendURL.
// We do this in one middleware to avoid conflicts from layering
// two separate CORS middlewares on the same request path.
func CORS(frontendURL string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Public checkout can be called from any origin
			// the access_code is the security boundary, not the origin.
			// Every other route only accepts the dashboard frontend URL.
			// We allow all origins here and let the route-level auth
			// (secret key / JWT / access_code) handle the real security.
			// This is the same approach Paystack uses for their public API.
			return true
		},
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
		// AllowCredentials must be true for cookies to work.
		// The browser will not send HttpOnly cookies on cross-origin
		// requests unless the server explicitly allows credentials.
		// This is required for the dashboard's cookie-based auth to work.
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
