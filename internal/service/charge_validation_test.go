package service

import (
	"testing"
	"time"

	"github.com/payfake/payfake-api/internal/domain"
)

func TestOTPMatchesRequiresExactGeneratedValue(t *testing.T) {
	if !otpMatches("482931", "482931") {
		t.Fatal("expected the generated OTP to match itself")
	}
	if otpMatches("482931", "000000") {
		t.Fatal("expected a different six-digit OTP to be rejected")
	}
	if otpMatches("482931", "48293") {
		t.Fatal("expected malformed OTP to be rejected")
	}
}

func TestValidCardChecksLuhnExpiryAndCVV(t *testing.T) {
	now := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	if !validCard("4111111111111111", "12/26", "123", now) {
		t.Fatal("expected standard Visa test card to be valid")
	}
	if validCard("4111111111111112", "12/26", "123", now) {
		t.Fatal("expected invalid Luhn checksum to be rejected")
	}
	if validCard("4111111111111111", "07/26", "123", now) {
		t.Fatal("expected expired card to be rejected")
	}
}

func TestValidMomoProviderRejectsUnknownProvider(t *testing.T) {
	if !validMomoProvider(domain.ProviderMTN) {
		t.Fatal("expected MTN to be supported")
	}
	if validMomoProvider(domain.MomoProvider("unknown")) {
		t.Fatal("expected unknown provider to be rejected")
	}
}
