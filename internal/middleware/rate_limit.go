package middleware

import (
	"sync"
	"time"

	"github.com/GordenArcher/payfake/internal/response"
	"github.com/gin-gonic/gin"
)

// bucket represents a sliding window rate limit tracker for a single client.
// count is how many requests this client has made in the current window.
// windowStart is when the current window began.
type bucket struct {
	count       int
	windowStart time.Time
	mu          sync.Mutex
}

// limiter holds all active buckets, one per client IP.
// sync.Map is used instead of map + mutex because it's optimised
// for the case where keys are written once and read many times —
// exactly our pattern: first request creates the bucket, subsequent
// requests read and increment it.
var limiter sync.Map

// RateLimit returns a middleware that enforces a request rate limit.
// maxRequests is the number of requests allowed per window.
// windowDuration is the length of the time window.
//
// Example: RateLimit(100, time.Minute) = 100 requests per minute per IP.
//
// We implement a fixed window algorithm here, simple and predictable.
// A sliding window would be more accurate but requires storing timestamps
// for every request, which is expensive at scale. For a local simulator
// fixed window is more than sufficient.
func RateLimit(maxRequests int, windowDuration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// We key the rate limit by client IP.
		// In production behind a reverse proxy you'd use X-Forwarded-For
		// or X-Real-IP instead of the direct connection IP.
		// c.ClientIP() handles the X-Forwarded-For header automatically
		// when Gin is running in release mode behind a trusted proxy.
		clientIP := c.ClientIP()

		// Load or create the bucket for this client.
		// LoadOrStore is atomic, if two goroutines hit this simultaneously
		// for the same IP, only one bucket is created.
		raw, _ := limiter.LoadOrStore(clientIP, &bucket{
			windowStart: time.Now(),
		})
		b := raw.(*bucket)

		b.mu.Lock()
		defer b.mu.Unlock()

		now := time.Now()

		// Check if the current window has expired.
		// If it has, reset the counter and start a new window.
		// This is the "fixed window reset" at the start of each new
		// window every client gets their full quota back.
		if now.Sub(b.windowStart) >= windowDuration {
			b.count = 0
			b.windowStart = now
		}

		// Increment the request count for this window.
		b.count++

		// If the client has exceeded their quota, reject the request.
		// We return 429 Too Many Requests, the standard HTTP status
		// for rate limiting. Well-behaved clients should back off when
		// they receive this status.
		if b.count > maxRequests {
			response.Error(
				c,
				429,
				"Too many requests, please slow down",
				response.RateLimitExceeded,
				[]response.ErrorField{},
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
