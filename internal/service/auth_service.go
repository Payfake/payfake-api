package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/pkg/keygen"
	"github.com/payfake/payfake-api/pkg/uid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TokenType distinguishes access tokens from refresh tokens.
// We embed this in the JWT claims so we can reject a refresh token
// being used where an access token is expected and vice versa.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type AuthService struct {
	merchantRepo        *repository.MerchantRepository
	refreshSessionRepo  refreshSessionRepository
	jwtSecret           string
	accessExpiryMinutes int
	refreshExpiryDays   int
}

type refreshSessionRepository interface {
	Create(*domain.RefreshSession) error
	Rotate(string, *domain.RefreshSession) (bool, error)
	Revoke(string) error
	RevokeAll(string) error
}

func NewAuthService(
	merchantRepo *repository.MerchantRepository,
	refreshSessionRepo refreshSessionRepository,
	jwtSecret string,
	accessExpiryMinutes string,
	refreshExpiryDays string,
) *AuthService {
	accessMins, err := strconv.Atoi(accessExpiryMinutes)
	if err != nil {
		accessMins = 15
	}
	refreshDays, err := strconv.Atoi(refreshExpiryDays)
	if err != nil {
		refreshDays = 7
	}
	return &AuthService{
		merchantRepo:        merchantRepo,
		refreshSessionRepo:  refreshSessionRepo,
		jwtSecret:           jwtSecret,
		accessExpiryMinutes: accessMins,
		refreshExpiryDays:   refreshDays,
	}
}

type RegisterInput struct {
	BusinessName string
	Email        string
	Password     string
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	// AccessExpiry is returned so the dashboard knows when to refresh.
	// The dashboard stores this in memory (not localStorage) and
	// proactively refreshes before expiry.
	AccessExpiry  time.Time
	RefreshExpiry time.Time
	refreshID     string
}

type RegisterOutput struct {
	Merchant *domain.Merchant
	Tokens   TokenPair
}

type LoginOutput struct {
	Merchant *domain.Merchant
	Tokens   TokenPair
}

func (s *AuthService) Register(input RegisterInput) (*RegisterOutput, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.BusinessName = strings.TrimSpace(input.BusinessName)
	exists, err := s.merchantRepo.EmailExists(input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	publicKey, secretKey, err := keygen.NewKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	merchant := &domain.Merchant{
		Base:         domain.Base{ID: uid.NewMerchantID()},
		BusinessName: input.BusinessName,
		Email:        input.Email,
		Password:     string(hashedPassword),
		PublicKey:    publicKey,
		SecretKey:    secretKey,
		IsActive:     true,
	}

	if err := s.merchantRepo.Create(merchant); err != nil {
		return nil, fmt.Errorf("failed to create merchant: %w", err)
	}

	tokens, err := s.generateTokenPair(merchant.ID, merchant.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	if err := s.storeRefreshSession(merchant.ID, tokens); err != nil {
		return nil, fmt.Errorf("failed to create refresh session: %w", err)
	}

	return &RegisterOutput{Merchant: merchant, Tokens: tokens}, nil
}

func (s *AuthService) Login(input LoginInput) (*LoginOutput, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	merchant, err := s.merchantRepo.FindByEmail(input.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find merchant: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(merchant.Password), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if !merchant.IsActive {
		return nil, ErrMerchantInactive
	}

	tokens, err := s.generateTokenPair(merchant.ID, merchant.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	if err := s.storeRefreshSession(merchant.ID, tokens); err != nil {
		return nil, fmt.Errorf("failed to create refresh session: %w", err)
	}

	return &LoginOutput{Merchant: merchant, Tokens: tokens}, nil
}

// RefreshTokens validates a refresh token and issues a new token pair.
// We issue a new refresh token on every refresh (refresh token rotation).
// This means a stolen refresh token can only be used once, the next
// legitimate refresh will fail because the token was already rotated,
// alerting the real user that their session was compromised.
func (s *AuthService) RefreshTokens(refreshToken string) (*TokenPair, error) {
	merchantID, email, tokenType, tokenID, err := s.validateToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Reject access tokens used on the refresh endpoint.
	// Without this check a leaked access token could be used to
	// keep a session alive indefinitely.
	if tokenType != string(RefreshToken) {
		return nil, ErrTokenInvalid
	}

	// Verify the merchant still exists and is active.
	merchant, err := s.merchantRepo.FindByID(merchantID)
	if err != nil {
		return nil, ErrMerchantNotFound
	}
	if !merchant.IsActive {
		return nil, ErrMerchantInactive
	}

	_ = email
	tokens, err := s.generateTokenPair(merchantID, merchant.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	replacement := &domain.RefreshSession{
		Base:       domain.Base{ID: uuid.NewString()},
		MerchantID: merchantID,
		TokenID:    tokens.refreshID,
		ExpiresAt:  tokens.RefreshExpiry,
	}
	rotated, err := s.refreshSessionRepo.Rotate(tokenID, replacement)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate refresh session: %w", err)
	}
	if !rotated {
		return nil, ErrTokenInvalid
	}

	return &tokens, nil
}

func (s *AuthService) RegenerateKeys(merchantID string) (publicKey, secretKey string, err error) {
	publicKey, secretKey, err = keygen.NewKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate new key pair: %w", err)
	}
	if err := s.merchantRepo.UpdateKeys(merchantID, publicKey, secretKey); err != nil {
		return "", "", fmt.Errorf("failed to update keys: %w", err)
	}
	return publicKey, secretKey, nil
}

// ValidateAccessToken validates an access token specifically.
// Returns error if the token is a refresh token, prevents refresh
// tokens from being used to authenticate API requests.
func (s *AuthService) ValidateAccessToken(tokenString string) (merchantID, email string, err error) {
	merchantID, email, tokenType, _, err := s.validateToken(tokenString)
	if err != nil {
		return "", "", err
	}
	if tokenType != string(AccessToken) {
		return "", "", ErrTokenInvalid
	}
	merchant, lookupErr := s.merchantRepo.FindByID(merchantID)
	if lookupErr != nil || !merchant.IsActive {
		return "", "", ErrTokenInvalid
	}
	return merchantID, email, nil
}

// ValidateJWT satisfies the middleware.JWTValidator interface.
// It validates access tokens only, refresh tokens are rejected.
// This is the method the middleware calls on every protected route.
func (s *AuthService) ValidateJWT(tokenString string) (string, string, error) {
	return s.ValidateAccessToken(tokenString)
}

func (s *AuthService) GetMerchant(merchantID string) (*domain.Merchant, error) {
	merchant, err := s.merchantRepo.FindByID(merchantID)
	if err != nil {
		return nil, ErrMerchantNotFound
	}
	return merchant, nil
}

// generateTokenPair creates both access and refresh tokens atomically.
func (s *AuthService) generateTokenPair(merchantID, email string) (TokenPair, error) {
	accessExpiry := time.Now().Add(time.Duration(s.accessExpiryMinutes) * time.Minute)
	refreshExpiry := time.Now().Add(time.Duration(s.refreshExpiryDays) * 24 * time.Hour)

	accessID := uuid.NewString()
	refreshID := uuid.NewString()
	accessToken, err := s.generateToken(merchantID, email, AccessToken, accessID, accessExpiry)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateToken(merchantID, email, RefreshToken, refreshID, refreshExpiry)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		AccessExpiry:  accessExpiry,
		refreshID:     refreshID,
		RefreshExpiry: refreshExpiry,
	}, nil
}

func (s *AuthService) generateToken(merchantID, email string, tokenType TokenType, tokenID string, expiry time.Time) (string, error) {
	claims := jwt.MapClaims{
		"merchant_id": merchantID,
		"email":       email,
		// type distinguishes access from refresh tokens.
		// Always validate this on token consumption.
		"type": string(tokenType),
		"exp":  expiry.Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "payfake",
		"jti":  tokenID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) validateToken(tokenString string) (merchantID, email, tokenType, tokenID string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("payfake"), jwt.WithExpirationRequired())

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", "", "", ErrTokenExpired
		}
		return "", "", "", "", ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", "", "", ErrTokenInvalid
	}

	merchantID, _ = claims["merchant_id"].(string)
	email, _ = claims["email"].(string)
	tokenType, _ = claims["type"].(string)
	tokenID, _ = claims["jti"].(string)
	if merchantID == "" || email == "" || tokenType == "" || tokenID == "" {
		return "", "", "", "", ErrTokenInvalid
	}

	return merchantID, email, tokenType, tokenID, nil
}

// storeRefreshSession activates the refresh token from a newly issued pair.
// Login and registration call this only after credentials are accepted, so a
// refresh JWT is never returned unless its one-time server record also exists.
func (s *AuthService) storeRefreshSession(merchantID string, tokens TokenPair) error {
	return s.refreshSessionRepo.Create(&domain.RefreshSession{
		Base:       domain.Base{ID: uuid.NewString()},
		MerchantID: merchantID,
		TokenID:    tokens.refreshID,
		ExpiresAt:  tokens.RefreshExpiry,
	})
}

// RevokeRefreshToken invalidates the current browser refresh token on logout.
// Invalid or expired tokens are intentionally treated as already logged out.
func (s *AuthService) RevokeRefreshToken(tokenString string) error {
	_, _, tokenType, tokenID, err := s.validateToken(tokenString)
	if err != nil || tokenType != string(RefreshToken) {
		return nil
	}
	return s.refreshSessionRepo.Revoke(tokenID)
}

// ChangePasswordInput is the input for changing a merchant's password.
type ChangePasswordInput struct {
	MerchantID      string
	CurrentPassword string
	NewPassword     string
}

// ChangePassword verifies the current password before setting the new one.
// We never allow blind password overwrite, the merchant must prove they
// know the current password first. This prevents an attacker who gets
// temporary access to a dashboard session from locking the real owner out.
func (s *AuthService) ChangePassword(input ChangePasswordInput) error {
	merchant, err := s.merchantRepo.FindByID(input.MerchantID)
	if err != nil {
		return ErrMerchantNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(merchant.Password), []byte(input.CurrentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 12)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.merchantRepo.UpdatePassword(merchant.ID, string(hashed)); err != nil {
		return err
	}
	return s.refreshSessionRepo.RevokeAll(merchant.ID)
}
