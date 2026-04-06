package uid

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// prefix constants define the identifier format for every entity in Payfake.
// Prefixed IDs are more debuggable than raw UUIDs, when you see "TXN_xxx"
// in a log you immediately know what entity you're dealing with without
// having to query the DB or check the context.
const (
	PrefixMerchant        = "MRC"
	PrefixCustomer        = "CUS"
	PrefixTransaction     = "TXN"
	PrefixCharge          = "CHG"
	PrefixWebhookEvent    = "WHK"
	PrefixWebhookAttempt  = "WHA"
	PrefixWebhookEndpoint = "WHP"
	PrefixScenario        = "SCN"
	PrefixRequestLog      = "LOG"
	PrefixAccessCode      = "ACC"
)

// generate creates a prefixed unique identifier.
// We take the first 12 characters of a UUID (after stripping hyphens)
// and uppercase them. This gives us enough entropy for our use case
// while keeping IDs short and readable.
// Format: PREFIX_XXXXXXXXXXXX (e.g TXN_A1B2C3D4E5F6)
func generate(prefix string) string {
	// Generate a new UUID and strip the hyphens so we work with
	// a clean 32-character hex string
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")

	// Take only the first 12 characters, this is our entropy slice.
	// 12 hex chars = 48 bits of entropy, which is more than enough
	// for a simulator with no real financial risk.
	short := strings.ToUpper(raw[:12])

	return fmt.Sprintf("%s_%s", prefix, short)
}

// Each exported function below is the public API for generating
// IDs for a specific entity. Callers never pass a prefix manually —
// they call the typed function for the entity they're creating.
// This prevents accidentally using the wrong prefix for an entity.

func NewMerchantID() string        { return generate(PrefixMerchant) }
func NewCustomerID() string        { return generate(PrefixCustomer) }
func NewTransactionID() string     { return generate(PrefixTransaction) }
func NewChargeID() string          { return generate(PrefixCharge) }
func NewWebhookEventID() string    { return generate(PrefixWebhookEvent) }
func NewWebhookAttemptID() string  { return generate(PrefixWebhookAttempt) }
func NewWebhookEndpointID() string { return generate(PrefixWebhookEndpoint) }
func NewScenarioID() string        { return generate(PrefixScenario) }
func NewRequestLogID() string      { return generate(PrefixRequestLog) }

// NewAccessCode generates the access code returned on transaction initialize.
// This is what the frontend uses to open the payment popup, it is short-lived
// and single-use. Same format as other IDs but semantically different —
// it's a token, not a persistent entity identifier.
func NewAccessCode() string { return generate(PrefixAccessCode) }
