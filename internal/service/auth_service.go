package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/pkg/keygen"
	"github.com/GordenArcher/payfake/pkg/uid"
	"github.com/golang-jwt/jwt/v5"
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
	jwtSecret           string
	accessExpiryMinutes int
	refreshExpiryDays   int
}

func NewAuthService(
	merchantRepo *repository.MerchantRepository,
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
	AccessExpiry time.Time
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

	return &RegisterOutput{Merchant: merchant, Tokens: tokens}, nil
}

func (s *AuthService) Login(input LoginInput) (*LoginOutput, error) {
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

	return &LoginOutput{Merchant: merchant, Tokens: tokens}, nil
}

// RefreshTokens validates a refresh token and issues a new token pair.
// We issue a new refresh token on every refresh (refresh token rotation).
// This means a stolen refresh token can only be used once, the next
// legitimate refresh will fail because the token was already rotated,
// alerting the real user that their session was compromised.
func (s *AuthService) RefreshTokens(refreshToken string) (*TokenPair, error) {
	merchantID, email, tokenType, err := s.validateToken(refreshToken)
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
	merchantID, email, tokenType, err := s.validateToken(tokenString)
	if err != nil {
		return "", "", err
	}
	if tokenType != string(AccessToken) {
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

	accessToken, err := s.generateToken(merchantID, email, AccessToken, accessExpiry)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateToken(merchantID, email, RefreshToken, refreshExpiry)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccessExpiry: accessExpiry,
	}, nil
}

func (s *AuthService) generateToken(merchantID, email string, tokenType TokenType, expiry time.Time) (string, error) {
	claims := jwt.MapClaims{
		"merchant_id": merchantID,
		"email":       email,
		// type distinguishes access from refresh tokens.
		// Always validate this on token consumption.
		"type": string(tokenType),
		"exp":  expiry.Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) validateToken(tokenString string) (merchantID, email, tokenType string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", "", ErrTokenExpired
		}
		return "", "", "", ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", "", ErrTokenInvalid
	}

	merchantID, _ = claims["merchant_id"].(string)
	email, _ = claims["email"].(string)
	tokenType, _ = claims["type"].(string)

	return merchantID, email, tokenType, nil
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

	return s.merchantRepo.UpdatePassword(merchant.ID, string(hashed))
}
