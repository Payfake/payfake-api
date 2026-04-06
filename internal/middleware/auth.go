package middleware

import (
	"strings"

	"github.com/GordenArcher/payfake/internal/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/GordenArcher/payfake/internal/domain"
)

// RequireSecretKey is the authentication middleware for all Paystack-compatible
// routes, transaction, charge, customer namespaces.
// It expects the merchant's secret key in the Authorization header:
//
//	Authorization: Bearer sk_test_xxxxxxxxxxxxxxxxxxxxxxxx
//
// This mirrors exactly how Paystack authenticates server-side API calls.
// Developers who've used Paystack before will know this pattern already —
// zero learning curve, zero code changes when switching base URLs.
func RequireSecretKey(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Authorization header is completely missing from the request.
		if authHeader == "" {
			response.UnauthorizedErr(c, "Authorization header is required")
			// c.Abort() stops the middleware chain, the actual handler
			// never runs. Without Abort, Gin would continue to the next
			// handler even after we've written an error response, which
			// would cause a "headers already written" panic.
			c.Abort()
			return
		}

		// The header must follow the "Bearer <token>" format.
		// We split on space and expect exactly two parts.
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.UnauthorizedErr(c, "Authorization header format must be: Bearer <secret_key>")
			c.Abort()
			return
		}

		secretKey := parts[1]

		// Validate that the key looks like a Payfake secret key before
		// hitting the DB. This early rejection saves a DB round trip for
		// obviously invalid keys (empty strings, public keys used by mistake etc).
		if !strings.HasPrefix(secretKey, "sk_test_") {
			response.UnauthorizedErr(c, "Invalid secret key format")
			c.Abort()
			return
		}

		// Look up the merchant by their secret key.
		// If no merchant owns this key the request is rejected.
		var merchant domain.Merchant
		result := db.Where("secret_key = ? AND is_active = ?", secretKey, true).First(&merchant)

		if result.Error != nil {
			// We deliberately return the same generic message for both
			// "key not found" and "merchant inactive". Returning different
			// messages would let an attacker enumerate valid vs inactive keys.
			response.UnauthorizedErr(c, "Invalid or inactive secret key")
			c.Abort()
			return
		}

		// Store the full merchant struct in context so handlers downstream
		// don't need to re-query the DB to find out who's making the request.
		// They just call c.Get("merchant") and type-assert to domain.Merchant.
		c.Set("merchant", merchant)
		c.Set("merchant_id", merchant.ID)

		c.Next()
	}
}

// RequireJWT is the authentication middleware for dashboard routes.
// The dashboard uses JWT (issued on login) instead of API keys because:
// 1. Dashboard sessions should expire, JWT has built-in expiry
// 2. API keys are long-lived credentials, not suitable for UI sessions
// 3. Separating concerns means rotating an API key doesn't log you out
// JWT validation logic will be implemented in the auth service —
// we keep the middleware thin and delegate the actual verification.
func RequireJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			response.UnauthorizedErr(c, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.UnauthorizedErr(c, "Authorization header format must be: Bearer <token>")
			c.Abort()
			return
		}

		// Store the raw token string in context.
		// The actual JWT parsing and claims extraction happens in the
		// auth service, middleware stays thin, business logic stays
		// in the service layer where it belongs.
		c.Set("jwt_token", parts[1])
		c.Next()
	}
}

// GetMerchant is a convenience helper that handlers call to retrieve
// the authenticated merchant from the Gin context.
// It returns the merchant and a bool indicating whether it was found.
// Handlers should always check the bool, if false, the middleware
// chain was bypassed somehow and the handler should abort.
func GetMerchant(c *gin.Context) (domain.Merchant, bool) {
	val, exists := c.Get("merchant")
	if !exists {
		return domain.Merchant{}, false
	}
	merchant, ok := val.(domain.Merchant)
	return merchant, ok
}
