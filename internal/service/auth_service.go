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

type AuthService struct {
	merchantRepo *repository.MerchantRepository
	jwtSecret    string
	jwtExpiry    int
}

func NewAuthService(merchantRepo *repository.MerchantRepository, jwtSecret string, jwtExpiry string) *AuthService {
	// Parse expiry hours from string, default to 24 if parsing fails.
	// We store it as a string in config (from env) and convert here
	// so the service works with a clean int internally.
	expiry, err := strconv.Atoi(jwtExpiry)
	if err != nil {
		expiry = 24
	}
	return &AuthService{
		merchantRepo: merchantRepo,
		jwtSecret:    jwtSecret,
		jwtExpiry:    expiry,
	}
}

// RegisterInput is the validated input shape for merchant registration.
// Keeping input structs in the service layer means handlers just parse
// the raw request and pass it here, no business logic in handlers.
type RegisterInput struct {
	BusinessName string
	Email        string
	Password     string
}

// RegisterOutput is what the service returns on successful registration.
// We return both the merchant and a JWT so the developer is immediately
// authenticated after registering, no need to call login separately.
type RegisterOutput struct {
	Merchant *domain.Merchant
	Token    string
}

// Register creates a new merchant account with a fresh key pair.
// Steps:
//  1. Check email is not already taken
//  2. Hash the password with bcrypt
//  3. Generate pk/sk key pair
//  4. Create the merchant record
//  5. Create a default scenario config for this merchant
//  6. Issue a JWT
func (s *AuthService) Register(input RegisterInput) (*RegisterOutput, error) {
	// Check for duplicate email before doing any work.
	exists, err := s.merchantRepo.EmailExists(input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		// Return a sentinel error the handler can check for to return
		// the correct response code (AUTH_EMAIL_TAKEN vs INTERNAL_ERROR).
		return nil, ErrEmailTaken
	}

	// bcrypt is the correct algorithm for password hashing.
	// It is intentionally slow (cost factor 12 = ~250ms per hash)
	// which makes brute force attacks computationally expensive.
	// Never use MD5, SHA1, or SHA256 for passwords, they are too fast.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate the key pair atomically, both keys or neither.
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

	// Issue JWT immediately after registration.
	token, err := s.generateJWT(merchant.ID, merchant.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &RegisterOutput{Merchant: merchant, Token: token}, nil
}

// LoginInput is the validated input shape for merchant login.
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput is what the service returns on successful login.
type LoginOutput struct {
	Merchant *domain.Merchant
	Token    string
}

// Login verifies credentials and issues a JWT on success.
// We use the same error message for "email not found" and "wrong password"
// deliberately, returning different messages would let attackers enumerate
// which emails are registered in Payfake.
func (s *AuthService) Login(input LoginInput) (*LoginOutput, error) {
	merchant, err := s.merchantRepo.FindByEmail(input.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find merchant: %w", err)
	}

	// bcrypt.CompareHashAndPassword does a constant-time comparison,
	// it takes the same amount of time whether the password is right or
	// wrong. This prevents timing attacks where an attacker measures
	// response time to guess how many characters they got right.
	if err := bcrypt.CompareHashAndPassword([]byte(merchant.Password), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if !merchant.IsActive {
		return nil, ErrMerchantInactive
	}

	token, err := s.generateJWT(merchant.ID, merchant.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginOutput{Merchant: merchant, Token: token}, nil
}

// RegenerateKeys creates a new key pair for a merchant.
// Only the secret key rotation actually matters for security
// if a secret key is leaked the merchant needs to rotate it immediately.
// We regenerate both for simplicity and consistency.
// After this call all requests using the old secret key will fail.
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

// ValidateJWT parses and validates a JWT string.
// Returns the merchant ID and email embedded in the claims if valid.
// Called by handlers that need to know WHO is making the request
// after RequireJWT middleware has confirmed the token is present.
func (s *AuthService) ValidateJWT(tokenString string) (merchantID, email string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Verify the signing method is what we expect.
		// If we don't check this an attacker could send a token signed
		// with "alg: none" or a different algorithm and bypass verification.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", ErrTokenExpired
		}
		return "", "", ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", ErrTokenInvalid
	}

	merchantID, _ = claims["merchant_id"].(string)
	email, _ = claims["email"].(string)

	return merchantID, email, nil
}

// generateJWT creates a signed JWT embedding the merchant's ID and email.
// We embed both so downstream handlers can identify the merchant without
// a DB lookup, the token itself carries the identity.
func (s *AuthService) generateJWT(merchantID, email string) (string, error) {
	claims := jwt.MapClaims{
		"merchant_id": merchantID,
		"email":       email,
		// exp is the standard JWT expiry claim, libraries check this
		// automatically during Parse so we don't need to check it manually.
		"exp": time.Now().Add(time.Duration(s.jwtExpiry) * time.Hour).Unix(),
		// iat (issued at) lets you detect if a token was issued before
		// a security event (like a password change) and invalidate it.
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
