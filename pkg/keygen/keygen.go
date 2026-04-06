package keygen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Key format mirrors Paystack exactly:
//
//	pk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx  (public key)
//	sk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx  (secret key)
//
// The "test" segment is permanent in Payfake, there is no live mode.
// This means any developer who has built against Paystack's test keys
// will instantly recognise the format and feel at home.
const (
	publicKeyPrefix = "pk_test_"
	secretKeyPrefix = "sk_test_"

	// keyByteLength controls how many random bytes we generate.
	// 32 bytes = 256 bits of entropy = 64 hex characters after encoding.
	// This matches the length of real Paystack keys.
	keyByteLength = 32
)

// generateRandom produces a cryptographically secure random hex string.
// We use crypto/rand here, NOT math/rand. math/rand is seeded and
// therefore predictable. API keys must be unpredictable. crypto/rand
// reads from the OS entropy pool (/dev/urandom on Linux) which is
// suitable for generating secrets.
func generateRandom() (string, error) {
	bytes := make([]byte, keyByteLength)

	// rand.Read fills the slice with random bytes from the OS.
	// It only returns an error in extraordinary circumstances
	// (e.g. OS entropy pool is exhausted) but we always handle it.
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// hex.EncodeToString converts each byte to two hex characters,
	// giving us a 64-character string that is URL-safe and printable.
	return hex.EncodeToString(bytes), nil
}

// NewPublicKey generates a new public key for a merchant.
// The public key is safe to expose on the frontend, it can only
// be used to initialize transactions, not to verify or charge.
func NewPublicKey() (string, error) {
	random, err := generateRandom()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", publicKeyPrefix, random), nil
}

// NewSecretKey generates a new secret key for a merchant.
// The secret key must NEVER be exposed on the frontend or committed
// to version control. It authorises all server-side API calls.
// We regenerate this (not the public key) when a merchant requests
// a key rotation, their frontend integration stays unaffected.
func NewSecretKey() (string, error) {
	random, err := generateRandom()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", secretKeyPrefix, random), nil
}

// NewKeyPair generates both keys atomically.
// We always generate them together on merchant registration so
// there's no window where a merchant has one key but not the other.
func NewKeyPair() (publicKey, secretKey string, err error) {
	publicKey, err = NewPublicKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	secretKey, err = NewSecretKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate secret key: %w", err)
	}

	return publicKey, secretKey, nil
}
