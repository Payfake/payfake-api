package service

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/payfake/payfake-api/internal/domain"
)

type fakeRefreshSessionRepo struct {
	created []*domain.RefreshSession
	revoked string
}

func (f *fakeRefreshSessionRepo) Create(session *domain.RefreshSession) error {
	f.created = append(f.created, session)
	return nil
}
func (f *fakeRefreshSessionRepo) Rotate(string, *domain.RefreshSession) (bool, error) {
	return true, nil
}
func (f *fakeRefreshSessionRepo) Revoke(tokenID string) error {
	f.revoked = tokenID
	return nil
}
func (f *fakeRefreshSessionRepo) RevokeAll(string) error { return nil }

func TestGeneratedRefreshTokensHaveUniqueServerIDs(t *testing.T) {
	repo := &fakeRefreshSessionRepo{}
	svc := NewAuthService(nil, repo, "test-secret", "15", "7")

	first, err := svc.generateTokenPair("MRC_123", "merchant@example.com")
	if err != nil {
		t.Fatalf("failed to generate first token pair: %v", err)
	}
	second, err := svc.generateTokenPair("MRC_123", "merchant@example.com")
	if err != nil {
		t.Fatalf("failed to generate second token pair: %v", err)
	}
	if first.RefreshToken == second.RefreshToken || first.refreshID == second.refreshID {
		t.Fatal("expected independently generated refresh token IDs")
	}
}

func TestRevokeRefreshTokenUsesSignedJTI(t *testing.T) {
	repo := &fakeRefreshSessionRepo{}
	svc := NewAuthService(nil, repo, "test-secret", "15", "7")
	tokens, err := svc.generateTokenPair("MRC_123", "merchant@example.com")
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	if err := svc.RevokeRefreshToken(tokens.RefreshToken); err != nil {
		t.Fatalf("failed to revoke refresh token: %v", err)
	}
	if repo.revoked != tokens.refreshID {
		t.Fatalf("expected token ID %q to be revoked, got %q", tokens.refreshID, repo.revoked)
	}
}

func TestValidateTokenRejectsMissingJTI(t *testing.T) {
	svc := NewAuthService(nil, &fakeRefreshSessionRepo{}, "test-secret", "15", "7")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"merchant_id": "MRC_123",
		"email":       "merchant@example.com",
		"type":        string(RefreshToken),
		"iss":         "payfake",
		"exp":         time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	if _, _, _, _, err := svc.validateToken(signed); err == nil {
		t.Fatal("expected token without jti to be rejected")
	}
}
