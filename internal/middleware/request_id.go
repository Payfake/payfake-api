package middleware

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestID injects a unique request identifier into every request context.
// This ID travels through the entire request lifecycle, it gets stored in
// the response metadata, written to logs, and saved in the introspection DB.
// When a developer reports a bug with a request_id, we can trace exactly
// what happened at every layer without guessing.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First check if the client sent their own request ID.
		// Some HTTP clients (like our own SDKs) will generate and send
		// an X-Request-ID header so they can correlate their logs with ours.
		// We honour that instead of generating a new one, this is a common
		// pattern in distributed systems called "trace propagation".
		requestID := c.GetHeader("X-Request-ID")

		if requestID == "" {
			// No client-provided ID, generate our own.
			// Format: req_<timestamp_ms>_<4_random_digits>
			// The timestamp component makes IDs roughly sortable by time.
			// The random suffix prevents collisions if two requests arrive
			// at exactly the same millisecond.
			requestID = fmt.Sprintf("req_%d_%04d",
				time.Now().UnixMilli(),
				rand.Intn(10000),
			)
		}

		// Store in context so every subsequent handler and middleware
		// in this request's chain can access it via c.Get("request_id").
		c.Set("request_id", requestID)

		// Also write it to the response header so the client can read it.
		// This is critical for debugging, the client sees the same ID
		// that appears in our server logs.
		c.Header("X-Request-ID", requestID)

		// Pass control to the next handler in the chain.
		c.Next()
	}
}
