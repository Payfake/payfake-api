package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Metadata is included in every response, success or error.
// timestamp tells the client exactly when the response was generated (UTC).
// request_id is injected by the request_id middleware and ties this response
// back to a specific request in the introspection logs.
type Metadata struct {
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id"`
}

// SuccessResponse is the envelope for all successful responses.
// data holds the actual payload, its shape varies per endpoint.
// We use any instead of interface{}, they are identical under the hood
// but any is the idiomatic choice since Go 1.18.
type SuccessResponse struct {
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Data     any      `json:"data"`
	Metadata Metadata `json:"metadata"`
	Code     Code     `json:"code"`
}

// ErrorField represents a single validation or domain error.
// field points to the exact request field that caused the problem.
// message is a human-readable explanation of what went wrong.
// Returning an array of these means we can surface ALL validation
// errors in one response instead of making the client fix one at a time.
type ErrorField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse is the envelope for all failed responses.
// errors is always an array, even for single errors, so the client
// can handle the shape consistently without type-checking.
// data is intentionally absent here. Success and error shapes are
// mutually exclusive, we never mix them in the same response.
type ErrorResponse struct {
	Status   string       `json:"status"`
	Message  string       `json:"message"`
	Errors   []ErrorField `json:"errors"`
	Metadata Metadata     `json:"metadata"`
	Code     Code         `json:"code"`
}

// getMetadata builds the metadata block for any response.
// It pulls request_id from the Gin context where the request_id
// middleware stored it. If somehow it's missing we degrade gracefully
// with an empty string rather than panicking.
func getMetadata(c *gin.Context) Metadata {
	requestID, _ := c.Get("request_id")
	rid, _ := requestID.(string)

	return Metadata{
		// RFC3339 is the standard for API timestamps, unambiguous and parseable
		// by every major language without extra configuration.
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: rid,
	}
}

// Success writes a successful JSON response with the agreed envelope.
// httpStatus is passed in explicitly because success isn't always 200,
// creates are 201, accepted async operations might be 202 etc.
func Success(c *gin.Context, httpStatus int, message string, code Code, data any) {
	c.JSON(httpStatus, SuccessResponse{
		Status:   "success",
		Message:  message,
		Data:     data,
		Metadata: getMetadata(c),
		Code:     code,
	})
}

// Error writes a failed JSON response with the agreed envelope.
// errors is always passed as a slice, callers build the slice before
// calling this so the logic stays clean here.
func Error(c *gin.Context, httpStatus int, message string, code Code, errors []ErrorField) {
	c.JSON(httpStatus, ErrorResponse{
		Status:   "error",
		Message:  message,
		Errors:   errors,
		Metadata: getMetadata(c),
		Code:     code,
	})
}

// ValidationErr is the shorthand for 422 validation failures.
// 422 is more semantically correct than 400 for validation,
// the request was well-formed but the content failed our rules.
func ValidationErr(c *gin.Context, errors []ErrorField) {
	Error(c, 422, "Validation failed", ValidationError, errors)
}

// UnauthorizedErr is the shorthand for 401 auth failures.
// message is kept flexible because the reason varies,
// missing key, expired token, invalid signature etc.
func UnauthorizedErr(c *gin.Context, message string) {
	Error(c, 401, message, AuthUnauthorized, []ErrorField{})
}

// NotFoundErr is the shorthand for 404 responses.
// We pass an empty errors slice here, there's no field to point to,
// the resource simply doesn't exist.
func NotFoundErr(c *gin.Context, message string) {
	Error(c, 404, message, NotFound, []ErrorField{})
}

// InternalErr is the shorthand for 500 responses.
// message should always be generic here, never expose internal
// error details like stack traces or DB errors to the client.
// The real error should be logged server-side before calling this.
func InternalErr(c *gin.Context, message string) {
	Error(c, 500, message, InternalError, []ErrorField{})
}

// BadRequestErr is the shorthand for 400 responses.
// Used when the request itself is malformed, wrong content type,
// unparseable JSON body, missing required headers etc.
func BadRequestErr(c *gin.Context, message string) {
	Error(c, 400, message, BadRequest, []ErrorField{})
}
