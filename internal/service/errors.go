package service

import "errors"

// Sentinel errors are package-level error values that handlers
// can check with errors.Is() to return the correct response code.
// Using sentinel errors instead of error strings means the compiler
// catches typos and refactors stay clean, you change the error
// in one place and every check updates automatically.
var (
	// Auth errors
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMerchantInactive   = errors.New("merchant account is inactive")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenInvalid       = errors.New("token is invalid")
	ErrMerchantNotFound   = errors.New("merchant not found")
	ErrUnauthorized       = errors.New("unauthorized")

	// Transaction errors
	ErrTransactionNotFound        = errors.New("transaction not found")
	ErrReferenceTaken             = errors.New("transaction reference already exists")
	ErrTransactionAlreadyVerified = errors.New("transaction already verified")
	ErrTransactionAlreadyRefunded = errors.New("transaction already refunded")
	ErrTransactionNotPending      = errors.New("transaction is not in pending state")
	ErrTransactionExpired         = errors.New("transaction has expired")
	ErrInvalidAmount              = errors.New("amount must be greater than zero")
	ErrInvalidCurrency            = errors.New("unsupported currency")

	// Customer errors
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrCustomerEmailTaken = errors.New("customer email already exists")

	// Charge errors
	ErrChargeFailed   = errors.New("charge failed")
	ErrChargeNotFound = errors.New("charge not found")

	// Webhook errors
	ErrWebhookNotFound = errors.New("webhook event not found")

	// Scenario errors
	ErrInvalidScenarioConfig = errors.New("invalid scenario configuration")

	// Control errors
	ErrLogsEmpty          = errors.New("no logs found")
	ErrInvalidForceStatus = errors.New("invalid force status")

	ErrInvalidOTP            = errors.New("invalid OTP")
	ErrChargeFlowInvalidStep = errors.New("invalid step in charge flow")
	ErrChargeFlowExpired     = errors.New("charge flow has expired")
)
