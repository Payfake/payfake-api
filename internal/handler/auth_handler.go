package handler

import (
	"errors"
	"net/http"

	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db      *gorm.DB
	authSvc *service.AuthService
}

func NewAuthHandler(db *gorm.DB, authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{db: db, authSvc: authSvc}
}

// registerRequest is the expected shape of the registration request body.
// We define this inline in the handler because it is purely an HTTP concern
// the shape of what comes over the wire. The service has its own input
// struct (RegisterInput) that is the business concern. Keeping them
// separate means changing the API shape doesn't force changes in the service.
type registerRequest struct {
	BusinessName string `json:"business_name" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest

	// ShouldBindJSON parses the request body into req and validates
	// the binding tags (required, email, min=8 etc).
	// If validation fails it returns an error with details about
	// which fields failed, we surface that in the errors array.
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.authSvc.Register(service.RegisterInput{
		BusinessName: req.BusinessName,
		Email:        req.Email,
		Password:     req.Password,
	})

	if err != nil {
		// Map sentinel errors to specific response codes.
		// This is the key pattern, service returns a known error,
		// handler maps it to the correct HTTP status and code.
		// Any unknown error falls through to the generic 500.
		if errors.Is(err, service.ErrEmailTaken) {
			response.Error(c, http.StatusConflict, "Email is already registered",
				response.AuthEmailTaken, []response.ErrorField{
					{Field: "email", Message: "This email is already in use"},
				})
			return
		}
		response.InternalErr(c, "Failed to create account")
		return
	}

	response.Success(c, http.StatusCreated, "Account created successfully",
		response.AuthRegisterSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			"token": out.Token,
		})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.authSvc.Login(service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) ||
			errors.Is(err, service.ErrMerchantInactive) {
			// Same message for both cases, never tell the client
			// whether the email exists or the password is wrong.
			response.Error(c, http.StatusUnauthorized, "Invalid email or password",
				response.AuthInvalidCredentials, []response.ErrorField{})
			return
		}
		response.InternalErr(c, "Failed to login")
		return
	}

	response.Success(c, http.StatusOK, "Login successful",
		response.AuthLoginSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			"token": out.Token,
		})
}

// Logout handles POST /api/v1/auth/logout
// JWT is stateless, there's nothing to invalidate server-side.
// We return a success response and the client discards the token.
// In a production system you'd maintain a token blocklist in Redis
// to support true logout before expiry, we keep it simple for now.
func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logged out successfully",
		response.AuthLogoutSuccess, nil)
}

// GetKeys handles GET /api/v1/auth/keys
func (h *AuthHandler) GetKeys(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	merchant, err := h.authSvc.GetMerchant(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch keys")
		return
	}

	response.Success(c, http.StatusOK, "Keys fetched successfully",
		response.AuthKeysFetched, gin.H{
			"public_key": merchant.PublicKey,
			"secret_key": merchant.SecretKey,
		})
}

// RegenerateKeys handles POST /api/v1/auth/keys/regenerate
func (h *AuthHandler) RegenerateKeys(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	publicKey, secretKey, err := h.authSvc.RegenerateKeys(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to regenerate keys")
		return
	}

	response.Success(c, http.StatusOK, "Keys regenerated successfully",
		response.AuthKeyRegenerated, gin.H{
			"public_key": publicKey,
			"secret_key": secretKey,
		})
}
