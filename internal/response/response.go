package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PaystackSuccess is the exact envelope Paystack returns on success.
// status is always boolean true, matches https://paystack.com/docs/api exactly.
type PaystackSuccess struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// PaystackError is the exact envelope Paystack returns on failure.
// status is always boolean false.
// errors is a map of field name to array of rule violations —
// matches Paystack's validation error shape exactly.
type PaystackError struct {
	Status  bool                          `json:"status"`
	Message string                        `json:"message"`
	Errors  map[string][]ValidationDetail `json:"errors,omitempty"`
}

// ValidationDetail is a single validation rule violation.
// Matches Paystack's { "rule": "required", "message": "Email is required" } shape.
type ValidationDetail struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Success writes a boolean-true Paystack-compatible success response.
// The Payfake response code travels in the X-Payfake-Code header so the
// dashboard and control panel can read it without the body deviating from
// the Paystack envelope shape.
func Success(c *gin.Context, httpStatus int, message string, code Code, data any) {
	c.Header("X-Payfake-Code", string(code))
	c.Header("X-Request-ID", getRequestID(c))
	c.JSON(httpStatus, PaystackSuccess{
		Status:  true,
		Message: message,
		Data:    data,
	})
}

// Error writes a boolean-false Paystack-compatible error response.
// fieldErrors is optional, pass nil for errors with no field context.
func Error(c *gin.Context, httpStatus int, message string, code Code, fieldErrors map[string][]ValidationDetail) {
	c.Header("X-Payfake-Code", string(code))
	c.Header("X-Request-ID", getRequestID(c))
	resp := PaystackError{
		Status:  false,
		Message: message,
	}
	if len(fieldErrors) > 0 {
		resp.Errors = fieldErrors
	}
	c.JSON(httpStatus, resp)
}

// ValidationErr writes a 400 validation failure.
// fields maps each field name to its rule violations —
// matches Paystack's { "errors": { "email": [{ "rule": "required", "message": "..." }] } }
func ValidationErr(c *gin.Context, fields map[string][]ValidationDetail) {
	Error(c, http.StatusBadRequest, "Validation error has occurred", ValidationError, fields)
}

// UnauthorizedErr writes a 401.
func UnauthorizedErr(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message, AuthUnauthorized, nil)
}

// NotFoundErr writes a 404.
func NotFoundErr(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message, NotFound, nil)
}

// InternalErr writes a 500.
// message is always generic, never exposes internal error details to the client.
func InternalErr(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message, InternalError, nil)
}

// BadRequestErr writes a 400 without field-level errors.
func BadRequestErr(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message, BadRequest, nil)
}

// ConflictErr writes a 409.
func ConflictErr(c *gin.Context, message string, code Code) {
	Error(c, http.StatusConflict, message, code, nil)
}

// UnprocessableErr writes a 422.
func UnprocessableErr(c *gin.Context, message string, code Code, fields map[string][]ValidationDetail) {
	Error(c, http.StatusUnprocessableEntity, message, code, fields)
}

func getRequestID(c *gin.Context) string {
	id, _ := c.Get("request_id")
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}
