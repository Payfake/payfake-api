package response

// Code is a custom string type for all API response codes.
// Using a named type instead of raw strings means the compiler
// catches typos at build time, you can't accidentally pass
// an arbitrary string where a Code is expected.
type Code string

const (

	// Auth

	// Returned after a merchant successfully creates an account.
	AuthRegisterSuccess Code = "AUTH_REGISTER_SUCCESS"
	// Returned after a merchant successfully logs in and receives a JWT.
	AuthLoginSuccess Code = "AUTH_LOGIN_SUCCESS"
	// Returned after a merchant successfully logs out.
	AuthLogoutSuccess Code = "AUTH_LOGOUT_SUCCESS"
	// Returned when a merchant fetches their public/secret keys.
	AuthKeysFetched Code = "AUTH_KEYS_FETCHED"
	// Returned after a secret key is regenerated. Old key is immediately invalid.
	AuthKeyRegenerated Code = "AUTH_KEY_REGENERATED"

	// Auth errors
	// Wrong email or password on login.
	AuthInvalidCredentials Code = "AUTH_INVALID_CREDENTIALS"
	// Request made to a protected route without a valid token or key.
	AuthUnauthorized Code = "AUTH_UNAUTHORIZED"
	// JWT has passed its expiry time, merchant must log in again.
	AuthTokenExpired Code = "AUTH_TOKEN_EXPIRED"
	// JWT signature is invalid or the token is malformed.
	AuthTokenInvalid Code = "AUTH_TOKEN_INVALID"
	// Registration attempted with an email that already exists.
	AuthEmailTaken Code = "AUTH_EMAIL_TAKEN"
	// Secret key provided doesn't match any merchant in the system.
	AuthMerchantNotFound Code = "AUTH_MERCHANT_NOT_FOUND"

	// Transaction

	// Returned after a transaction is created and an access_code is issued.
	TransactionInitialized Code = "TRANSACTION_INITIALIZED"
	// Returned when a transaction reference is looked up and found.
	TransactionVerified Code = "TRANSACTION_VERIFIED"
	// Returned when a single transaction is fetched by ID.
	TransactionFetched Code = "TRANSACTION_FETCHED"
	// Returned when a paginated list of transactions is fetched.
	TransactionListFetched Code = "TRANSACTION_LIST_FETCHED"
	// Returned after a transaction has been successfully reversed/refunded.
	TransactionRefunded Code = "TRANSACTION_REFUNDED"

	// Transaction errors
	// No transaction found matching the given reference or ID.
	TransactionNotFound Code = "TRANSACTION_NOT_FOUND"
	// Verify called on a transaction that is already in a terminal state.
	TransactionAlreadyVerified Code = "TRANSACTION_ALREADY_VERIFIED"
	// Refund attempted on a transaction that was already refunded.
	TransactionAlreadyRefunded Code = "TRANSACTION_ALREADY_REFUNDED"
	// The reference provided on initialize is already used by another transaction.
	// References must be unique per merchant, this enforces idempotency.
	TransactionReferenceTaken Code = "TRANSACTION_REFERENCE_TAKEN"
	// Amount is zero or negative, we reject this early before hitting the simulator.
	TransactionInvalidAmount Code = "TRANSACTION_INVALID_AMOUNT"
	// Currency code provided is not in our supported list (GHS, NGN, KES, USD).
	TransactionInvalidCurrency Code = "TRANSACTION_INVALID_CURRENCY"
	// Transaction was initialized but never completed, customer left the flow.
	TransactionAbandoned Code = "TRANSACTION_ABANDONED"
	// Transaction exceeded the allowed window to be completed.
	TransactionExpired Code = "TRANSACTION_EXPIRED"

	// Charge

	// Card or bank charge completed successfully through the simulator.
	ChargeSuccessful Code = "CHARGE_SUCCESSFUL"
	// MoMo charge initiated, waiting for customer to approve on their phone.
	// This is an async flow: we return pending immediately, webhook fires later.
	ChargePending Code = "CHARGE_PENDING"
	// Charge request received and processing has started.
	ChargeInitiated Code = "CHARGE_INITIATED"

	// Charge errors
	// Generic charge failure, used when no specific error code applies.
	ChargeFailed Code = "CHARGE_FAILED"
	// Card number failed the Luhn check or is not a recognised format.
	ChargeInvalidCard Code = "CHARGE_INVALID_CARD"
	// Card expiry date is in the past.
	ChargeCardExpired Code = "CHARGE_CARD_EXPIRED"
	// CVV provided does not match the card.
	ChargeInvalidCVV Code = "CHARGE_INVALID_CVV"
	// PIN entered by customer is incorrect.
	ChargeInvalidPIN Code = "CHARGE_INVALID_PIN"
	// Account does not have enough funds to cover the amount + fees.
	ChargeInsufficientFunds Code = "CHARGE_INSUFFICIENT_FUNDS"
	// Issuing bank declined the transaction without a specific reason.
	// Very common in Ghana, banks block international or online transactions by default.
	ChargeDoNotHonor Code = "CHARGE_DO_NOT_HONOR"
	// Transaction type not allowed for this card (e.g. online payments disabled).
	ChargeNotPermitted Code = "CHARGE_NOT_PERMITTED"
	// Transaction exceeds the daily or single-transaction limit on the account.
	ChargeLimitExceeded Code = "CHARGE_LIMIT_EXCEEDED"
	// Network error between simulator and the fake card network. Simulates timeouts.
	ChargeNetworkError Code = "CHARGE_NETWORK_ERROR"
	// MoMo prompt was sent but customer did not respond within the timeout window.
	// Common in production, customers miss the prompt or have no signal.
	ChargeMomoTimeout Code = "CHARGE_MOMO_TIMEOUT"
	// Phone number is not registered on the specified MoMo network.
	ChargeMomoInvalidNumber Code = "CHARGE_MOMO_INVALID_NUMBER"
	// MoMo wallet has hit its daily transaction or balance limit.
	ChargeMomoLimitExceeded Code = "CHARGE_MOMO_LIMIT_EXCEEDED"
	// The MoMo provider (MTN, Vodafone, AirtelTigo) is temporarily down.
	// We simulate this to let developers handle provider outages gracefully.
	ChargeMomoProviderUnavailable Code = "CHARGE_MOMO_PROVIDER_UNAVAILABLE"
	// Bank account number provided does not exist or is invalid.
	ChargeBankInvalidAccount Code = "CHARGE_BANK_INVALID_ACCOUNT"
	// Bank transfer was initiated but failed at the simulated bank level.
	ChargeBankTransferFailed Code = "CHARGE_BANK_TRANSFER_FAILED"

	// Customer

	// New customer record created under the merchant's account.
	CustomerCreated Code = "CUSTOMER_CREATED"
	// Single customer fetched by code or ID.
	CustomerFetched Code = "CUSTOMER_FETCHED"
	// Paginated list of merchant's customers fetched.
	CustomerListFetched Code = "CUSTOMER_LIST_FETCHED"
	// Customer record updated successfully.
	CustomerUpdated Code = "CUSTOMER_UPDATED"

	// Customer errors
	// No customer found matching the given code or ID under this merchant.
	CustomerNotFound Code = "CUSTOMER_NOT_FOUND"
	// A customer with this email already exists under this merchant.
	CustomerEmailTaken Code = "CUSTOMER_EMAIL_TAKEN"
	// Phone number is not a valid Ghanaian or African format.
	CustomerInvalidPhone Code = "CUSTOMER_INVALID_PHONE"

	// Control (Payfake-specific)
	// These codes have no Paystack equivalent, they power the simulator layer.

	// Current scenario config fetched for the merchant.
	ScenarioFetched Code = "SCENARIO_FETCHED"
	// Scenario config updated, new behavior takes effect on next transaction.
	ScenarioUpdated Code = "SCENARIO_UPDATED"
	// Scenario reset to defaults, all transactions will succeed with no delay.
	ScenarioReset Code = "SCENARIO_RESET"
	// List of all webhook events and their delivery status fetched.
	WebhookListFetched Code = "WEBHOOK_LIST_FETCHED"
	// A failed webhook event has been manually re-triggered.
	WebhookRetried Code = "WEBHOOK_RETRIED"
	// Delivery attempt log for a specific webhook event fetched.
	WebhookAttemptsFetched Code = "WEBHOOK_ATTEMPTS_FETCHED"
	// A pending transaction's outcome has been manually forced.
	// This is the core testing tool, force any transaction to any terminal state.
	TransactionForced Code = "TRANSACTION_FORCED"
	// Full request/response introspection log fetched.
	LogsFetched Code = "LOGS_FETCHED"
	// Introspection log cleared.
	LogsCleared Code = "LOGS_CLEARED"

	// Control errors
	// Scenario config values are out of range (e.g. failure_rate > 1.0).
	ScenarioInvalidConfig Code = "SCENARIO_INVALID_CONFIG"
	// Webhook event not found by the given ID.
	WebhookNotFound Code = "WEBHOOK_NOT_FOUND"
	// Manual webhook retry attempted but delivery failed again.
	WebhookDeliveryFailed Code = "WEBHOOK_DELIVERY_FAILED"
	// Force endpoint called with a status that is not a valid terminal state.
	// Only "success", "failed", and "abandoned" are valid force targets.
	TransactionForceInvalidStatus Code = "TRANSACTION_FORCE_INVALID_STATUS"
	// Logs endpoint called but there are no entries yet.
	LogsEmpty Code = "LOGS_EMPTY"

	// Generic errors
	// Shared across all namespaces, not tied to any specific domain.

	// Request body or query params failed validation.
	// Always paired with an errors array pointing to specific fields.
	ValidationError Code = "VALIDATION_ERROR"
	// Unexpected server-side error. We log the real cause internally
	// but never expose internal error details to the client.
	InternalError Code = "INTERNAL_ERROR"
	// Too many requests from this client within the allowed window.
	RateLimitExceeded Code = "RATE_LIMIT_EXCEEDED"
	// Route exists but no resource was found at the given identifier.
	NotFound Code = "NOT_FOUND"
	// HTTP method is not supported on this route.
	MethodNotAllowed Code = "METHOD_NOT_ALLOWED"
	// Request is malformed, usually a missing required field or wrong content type.
	BadRequest Code = "BAD_REQUEST"
)
