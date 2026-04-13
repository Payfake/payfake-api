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
	isProd  bool
}

func NewAuthHandler(db *gorm.DB, authSvc *service.AuthService, isProd bool) *AuthHandler {
	return &AuthHandler{db: db, authSvc: authSvc, isProd: isProd}
}

type registerRequest struct {
	BusinessName string `json:"business_name" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
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

	// Set HttpOnly cookies, the dashboard reads auth state from these,
	// never from localStorage. This is the more secure approach.
	middleware.SetAuthCookies(c,
		out.Tokens.AccessToken,
		out.Tokens.RefreshToken,
		out.Tokens.AccessExpiry,
		h.isProd,
	)

	response.Success(c, http.StatusCreated, "Account created successfully",
		response.AuthRegisterSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			// We still return the access token in the body so the Go/Python/JS/Rust
			// SDKs can use it without cookies. Dashboard uses the cookie.
			"token":         out.Tokens.AccessToken,
			"access_expiry": out.Tokens.AccessExpiry,
		})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

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
			response.Error(c, http.StatusUnauthorized, "Invalid email or password",
				response.AuthInvalidCredentials, []response.ErrorField{})
			return
		}
		response.InternalErr(c, "Failed to login")
		return
	}

	middleware.SetAuthCookies(c,
		out.Tokens.AccessToken,
		out.Tokens.RefreshToken,
		out.Tokens.AccessExpiry,
		h.isProd,
	)

	response.Success(c, http.StatusOK, "Login successful",
		response.AuthLoginSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			"token":         out.Tokens.AccessToken,
			"access_expiry": out.Tokens.AccessExpiry,
		})
}

// Refresh handles POST /api/v1/auth/refresh
// The browser sends the payfake_refresh cookie automatically.
// We validate it, issue a new token pair, and set fresh cookies.
// The old refresh token is invalidated by rotation, a rotated token
// cannot be used again because we've replaced it with a new one.
func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("payfake_refresh")
	if err != nil || refreshToken == "" {
		response.UnauthorizedErr(c, "Refresh token is required")
		return
	}

	tokens, err := h.authSvc.RefreshTokens(refreshToken)
	if err != nil {
		// Clear cookies on refresh failure, the session is invalid.
		middleware.ClearAuthCookies(c)
		if errors.Is(err, service.ErrTokenExpired) {
			response.Error(c, http.StatusUnauthorized, "Session expired, please login again",
				response.AuthTokenExpired, []response.ErrorField{})
			return
		}
		response.UnauthorizedErr(c, "Invalid refresh token")
		return
	}

	middleware.SetAuthCookies(c,
		tokens.AccessToken,
		tokens.RefreshToken,
		tokens.AccessExpiry,
		h.isProd,
	)

	response.Success(c, http.StatusOK, "Token refreshed",
		response.AuthLoginSuccess, gin.H{
			"access_token":  tokens.AccessToken,
			"access_expiry": tokens.AccessExpiry,
		})
}

// Logout handles POST /api/v1/auth/logout
// Clears both cookies, the browser discards them immediately.
func (h *AuthHandler) Logout(c *gin.Context) {
	middleware.ClearAuthCookies(c)
	response.Success(c, http.StatusOK, "Logged out successfully",
		response.AuthLogoutSuccess, nil)
}

// Me handles GET /api/v1/auth/me
// Called on dashboard mount to hydrate the current merchant's profile.
// If the access cookie is valid this returns the merchant, otherwise
// the dashboard knows to redirect to login.
func (h *AuthHandler) Me(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.authSvc.GetMerchant(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch profile")
		return
	}

	response.Success(c, http.StatusOK, "Profile fetched",
		response.AuthKeysFetched, gin.H{
			"id":            merchant.ID,
			"business_name": merchant.BusinessName,
			"email":         merchant.Email,
			"public_key":    merchant.PublicKey,
			"webhook_url":   merchant.WebhookURL,
			"is_active":     merchant.IsActive,
			"created_at":    merchant.CreatedAt,
		})
}

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
