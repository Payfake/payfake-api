package crypto

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
)

// Payfake signs outgoing webhook payloads using HMAC-SHA512.
// This mirrors Paystack's webhook signature scheme exactly —
// developers verify our webhooks the same way they verify Paystack's.
//
// The verification flow on the developer's server:
//   1. Receive webhook POST request
//   2. Read the raw request body (before JSON parsing)
//   3. Compute HMAC-SHA512 of the body using their secret key
//   4. Compare with the X-Paystack-Signature header we sent
//   5. If they match → request is genuinely from Payfake, process it
//   6. If they don't → reject the request, someone is spoofing webhooks
//
// We use the header name X-Paystack-Signature intentionally —
// developers don't need to change their webhook verification code
// when pointing at Payfake instead of Paystack.

const SignatureHeader = "X-Paystack-Signature"

// Sign computes the HMAC-SHA512 signature for a webhook payload.
// secretKey is the merchant's secret key, the same one they use
// to authenticate API calls. This ties the webhook signature to
// the merchant's identity without needing a separate signing secret.
// payload is the raw JSON bytes of the webhook body, we sign the
// raw bytes, not a parsed structure, to guarantee byte-for-byte
// consistency between what we sign and what the developer receives.
func Sign(secretKey string, payload []byte) string {
	// hmac.New creates a new HMAC hash using SHA-512 as the underlying
	// hash function. SHA-512 is stronger than SHA-256 and is what
	// Paystack uses, we match it for full compatibility.
	mac := hmac.New(sha512.New, []byte(secretKey))

	// Write the payload bytes into the HMAC. This cannot fail for
	// an in-memory hash writer so we intentionally ignore the error.
	mac.Write(payload)

	// Sum(nil) finalises the HMAC computation and returns the raw bytes.
	// hex.EncodeToString converts those bytes to a lowercase hex string
	// which is what gets sent in the X-Paystack-Signature header.
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether a received signature matches the expected
// signature for a given payload and secret key.
// This is used internally when Payfake needs to verify its own
// signatures, for example in integration tests or the control panel.
// We use hmac.Equal instead of == for the comparison. hmac.Equal does
// a constant-time comparison which prevents timing attacks. an attacker
// cannot determine how many characters they got right by measuring
// how long the comparison takes.
func Verify(secretKey string, payload []byte, signature string) bool {
	expected := Sign(secretKey, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// BuildSignatureHeader returns the full header value string.
// Some webhook verification guides expect just the hex string,
// others expect a "sha512=xxx" prefix. Paystack uses the raw hex,
// so we match that, but this helper exists so we can change the
// format in one place if needed.
func BuildSignatureHeader(secretKey string, payload []byte) string {
	return fmt.Sprintf("%s", Sign(secretKey, payload))
}
