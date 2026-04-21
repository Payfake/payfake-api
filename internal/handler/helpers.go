package handler

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/payfake/payfake-api/internal/response"
)

// parseBindingErrors converts Gin's validator errors into Paystack's
// field-keyed error map:
//
//	{ "email": [{ "rule": "required", "message": "Email is required" }] }
//
// This matches https://paystack.com/docs/api exactly so developers
// get the same error shape whether they're hitting Payfake or real Paystack.
func parseBindingErrors(err error) map[string][]response.ValidationDetail {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string][]response.ValidationDetail)
		for _, e := range ve {
			field := toSnakeCase(e.Field())
			fields[field] = append(fields[field], response.ValidationDetail{
				Rule:    e.Tag(),
				Message: ruleMessage(e),
			})
		}
		return fields
	}
	// Not a validation error, malformed JSON, wrong content type etc.
	return map[string][]response.ValidationDetail{
		"body": {{Rule: "invalid", Message: "Invalid request body"}},
	}
}

// field builds a single-field error map, shorthand for the common case
// of one field, one rule violation.
func field(name, rule, message string) map[string][]response.ValidationDetail {
	return map[string][]response.ValidationDetail{
		name: {{Rule: rule, Message: message}},
	}
}

// fields builds a multi-field error map from a flat list of (name, rule, message) triples.
func fields(pairs ...string) map[string][]response.ValidationDetail {
	result := make(map[string][]response.ValidationDetail)
	for i := 0; i+2 < len(pairs); i += 3 {
		name, rule, msg := pairs[i], pairs[i+1], pairs[i+2]
		result[name] = append(result[name], response.ValidationDetail{Rule: rule, Message: msg})
	}
	return result
}

func ruleMessage(e validator.FieldError) string {
	f := toSnakeCase(e.Field())
	switch e.Tag() {
	case "required":
		return strings.Title(f) + " is required"
	case "email":
		return "Must be a valid email address"
	case "min":
		return "Value too short or too small (min: " + e.Param() + ")"
	case "max":
		return "Value too long or too large (max: " + e.Param() + ")"
	case "oneof":
		return "Must be one of: " + e.Param()
	default:
		return "Invalid value"
	}
}

// toSnakeCase converts PascalCase/camelCase field names from validator
// to the snake_case names developers expect in the error response.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, []rune(strings.ToLower(string(r)))...)
	}
	return string(result)
}
