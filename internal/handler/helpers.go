package handler

import (
	"errors"
	"strings"

	"github.com/GordenArcher/payfake/internal/response"
	"github.com/go-playground/validator/v10"
)

// parseBindingErrors converts Gin's binding validation errors into
// our ErrorField slice format. Gin uses the go-playground/validator
// library under the hood, when ShouldBindJSON fails it returns
// a ValidationErrors type we can range over to get field-level detail.
//
// If the error is not a ValidationErrors (e.g. malformed JSON) we
// return a generic "invalid request body" error field instead of
// panicking trying to type-assert something unexpected.
func parseBindingErrors(err error) []response.ErrorField {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fields := make([]response.ErrorField, 0, len(validationErrors))
		for _, ve := range validationErrors {
			fields = append(fields, response.ErrorField{
				// Field() returns the struct field name, we lowercase it
				// to match the JSON field name the client sent.
				Field:   strings.ToLower(ve.Field()),
				Message: validationMessage(ve),
			})
		}
		return fields
	}

	// Malformed JSON or wrong content type, not a validation error.
	return []response.ErrorField{
		{Field: "body", Message: "Invalid request body"},
	}
}

// validationMessage returns a human-readable message for each
// validator tag. We map the most common tags explicitly and fall
// back to the tag name for anything we haven't covered yet.
func validationMessage(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Must be a valid email address"
	case "min":
		return "Value is too short or too small (min: " + ve.Param() + ")"
	case "max":
		return "Value is too long or too large (max: " + ve.Param() + ")"
	case "oneof":
		return "Must be one of: " + ve.Param()
	default:
		return "Invalid value"
	}
}
