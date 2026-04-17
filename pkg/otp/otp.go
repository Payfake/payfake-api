package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Generate produces a cryptographically random numeric OTP.
// length controls how many digits, we use 6 for OTPs.
// We use crypto/rand not math/rand, math/rand is seeded and
// predictable which would let an attacker guess OTPs.
func Generate(length int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate OTP: %w", err)
	}
	// Zero-pad so a number like 42 becomes "000042" for length=6
	return fmt.Sprintf("%0*d", length, n), nil
}

// GenerateOTP returns a 6-digit OTP string.
func GenerateOTP() (string, error) {
	return Generate(6)
}
