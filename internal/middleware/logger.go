package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const maxLoggedBodyLength = 2048
const maxLoggedRequestPreviewBytes = 64 << 10

var redactedLogKeys = map[string]struct{}{
	"access_code":        {},
	"access_token":       {},
	"authorization_code": {},
	"card_number":        {},
	"cookie":             {},
	"cvv":                {},
	"number":             {},
	"otp":                {},
	"otp_code":           {},
	"password":           {},
	"pin":                {},
	"refresh_token":      {},
	"secret_key":         {},
	"signature":          {},
	"token":              {},
}

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

		requestBodyLog := captureRequestBodyForLogging(c)

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
			Str("request_id", requestIDString(requestID)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", rw.statusCode).
			Dur("duration", duration).
			Str("ip", c.ClientIP()).
			Str("request_body", requestBodyLog).
			Str("response_body", sanitizeLogBody(rw.body.Bytes())).
			Msg("request completed")
	}
}

func captureRequestBodyForLogging(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}

	preview, consumed, truncated, err := readRequestBodyPreview(c.Request.Body, maxLoggedRequestPreviewBytes)
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(consumed), c.Request.Body))

	if err != nil {
		return fmt.Sprintf("[unavailable: failed to read request body preview: %v]", err)
	}
	if truncated {
		return fmt.Sprintf("[skipped: request body exceeds log preview limit of %d bytes]", maxLoggedRequestPreviewBytes)
	}

	return sanitizeLogBody(preview)
}

func readRequestBodyPreview(body io.Reader, limit int64) (preview []byte, consumed []byte, truncated bool, err error) {
	consumed, err = io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, consumed, false, err
	}

	if int64(len(consumed)) > limit {
		return consumed[:limit], consumed, true, nil
	}

	return consumed, consumed, false, nil
}

func sanitizeLogBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		redacted := redactLogValue(payload)
		if encoded, err := json.Marshal(redacted); err == nil {
			return truncateForLog(string(encoded))
		}
	}

	return truncateForLog(string(body))
}

func redactLogValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, child := range val {
			if _, ok := redactedLogKeys[strings.ToLower(key)]; ok {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactLogValue(child)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i := range val {
			out[i] = redactLogValue(val[i])
		}
		return out
	default:
		return v
	}
}

func truncateForLog(s string) string {
	if len(s) <= maxLoggedBodyLength {
		return s
	}
	return s[:maxLoggedBodyLength] + "...[truncated]"
}

func requestIDString(v any) string {
	id, _ := v.(string)
	return id
}
