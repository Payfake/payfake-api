package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// responseWriter wraps gin's ResponseWriter so we can capture
// the response body and status code after the handler writes them.
// Gin writes the response directly to the underlying connection —
// there's no built-in way to read it back after the fact.
// We intercept the writes here so the logger can record them.
type responseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// Write intercepts every write to the response body.
// We write to both our buffer (for logging) and the real writer
// (so the client actually receives the response).
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// WriteHeader intercepts the status code write so we can record it.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Logger is structured request/response logging middleware.
// Every request gets a log line with method, path, status, duration,
// and the request/response bodies. In development this helps you see
// exactly what's flowing through the API. In production you'd ship
// these logs to a log aggregator (Loki, Datadog etc).
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Read the request body into a buffer so we can log it.
		// c.Request.Body is a ReadCloser, once you read it, it's gone.
		// We read it, log it, then put it back so the actual handler
		// can still parse it. Without this restore step the handler
		// would receive an empty body and fail to parse the request.
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			// io.NopCloser wraps our bytes back into a ReadCloser
			// so it satisfies the Body interface the handler expects.
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Wrap the response writer so we can capture the response body.
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     200,
		}
		c.Writer = rw

		// Let the request flow through to the actual handler.
		c.Next()

		// Everything below runs AFTER the handler has completed
		// and the response has been written.
		duration := time.Since(start)

		// Pull the request_id we injected earlier — this ties the
		// log line to the response metadata and introspection records.
		requestID, _ := c.Get("request_id")

		// zerolog outputs structured JSON logs. Structured logs are
		// machine-parseable, you can query them with tools like jq,
		// ship them to Loki, or filter them in Datadog without regex.
		log.Info().
			Str("request_id", requestID.(string)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", rw.statusCode).
			Dur("duration", duration).
			Str("ip", c.ClientIP()).
			Str("request_body", string(requestBody)).
			Str("response_body", rw.body.String()).
			Msg("request completed")
	}
}
