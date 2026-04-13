package middleware

import (
	"strings"
	"time"

	"github.com/GordenArcher/payfake/internal/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/GordenArcher/payfake/internal/domain"
)

// RequireSecretKey is the authentication middleware for all Paystack-compatible
// routes, transaction, charge, customer namespaces.
// It expects the merchant's secret key in the Authorization header:
//
//	Authorization: Bearer <SK TEST KEY HERE>
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

// RequireJWT checks for a valid access token in either:
// 1. The payfake_access cookie (dashboard, HttpOnly, set by login)
// 2. The Authorization header (SDK/API calls Bearer token)
// Cookie takes priority since it's the more secure option for browsers.
// The Authorization header fallback keeps the existing SDK behaviour working.
func RequireJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Try cookie first, HttpOnly cookies can't be read by JavaScript
		// which protects against XSS attacks stealing the token.
		cookieToken, err := c.Cookie("payfake_access")
		if err == nil && cookieToken != "" {
			token = cookieToken
		} else {
			// Fall back to Authorization header for SDK/programmatic access.
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				response.UnauthorizedErr(c, "Authorization required")
				c.Abort()
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.UnauthorizedErr(c, "Authorization header format must be: Bearer <token>")
				c.Abort()
				return
			}
			token = parts[1]
		}

		c.Set("jwt_token", token)
		c.Next()
	}
}

// SetAuthCookies writes the access and refresh tokens as HttpOnly cookies.
// HttpOnly = JavaScript cannot read these cookies, only the browser sends them.
// SameSite=Strict = cookies are only sent on same-site requests, blocking CSRF.
// Secure = cookies only sent over HTTPS in production.
// We set both tokens here so login and refresh both call one function.
func SetAuthCookies(c *gin.Context, accessToken, refreshToken string, accessExpiry time.Time, isProd bool) {
	accessMaxAge := int(time.Until(accessExpiry).Seconds())
	refreshMaxAge := 7 * 24 * 60 * 60 // 7 days in seconds

	// Access token cookie, short lived, used on every authenticated request.
	c.SetCookie(
		"payfake_access",
		accessToken,
		accessMaxAge,
		"/",
		"",
		isProd, // Secure flag, HTTPS only in production
		true,   // HttpOnly, not accessible via JavaScript
	)

	// Refresh token cookie, long lived, only sent to the refresh endpoint.
	// Path="/api/v1/auth/refresh" means the browser only sends this cookie
	// to that specific endpoint, not to every API call. This limits the
	// window where a stolen refresh token could be used.
	c.SetCookie(
		"payfake_refresh",
		refreshToken,
		refreshMaxAge,
		"/api/v1/auth/refresh",
		"",
		isProd,
		true,
	)
}

// ClearAuthCookies removes both auth cookies on logout.
// Setting MaxAge=-1 tells the browser to delete the cookie immediately.
func ClearAuthCookies(c *gin.Context) {
	c.SetCookie("payfake_access", "", -1, "/", "", false, true)
	c.SetCookie("payfake_refresh", "", -1, "/api/v1/auth/refresh", "", false, true)
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

// GetMerchantIDFromJWT extracts the merchant ID from a validated JWT token.
// Called by handlers sitting behind RequireJWT middleware, the token
// is already in context, we just need to parse the claims here.
func GetMerchantIDFromJWT(c *gin.Context, authSvc interface {
	ValidateJWT(string) (string, string, error)
}) (string, bool) {
	tokenVal, exists := c.Get("jwt_token")
	if !exists {
		return "", false
	}

	token, _ := tokenVal.(string)
	merchantID, _, err := authSvc.ValidateJWT(token)
	if err != nil {
		return "", false
	}

	return merchantID, true
}
