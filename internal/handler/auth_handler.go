package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
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
			response.Error(c, http.StatusConflict,
				"Email already registered",
				response.AuthEmailTaken,
				field("email", "unique", "This email is already in use"))
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	middleware.SetAuthCookies(c, out.Tokens.AccessToken, out.Tokens.RefreshToken, out.Tokens.AccessExpiry, h.isProd)

	response.Success(c, http.StatusCreated, "Account created",
		response.AuthRegisterSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			"access_token":  out.Tokens.AccessToken,
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
		if errors.Is(err, service.ErrInvalidCredentials) || errors.Is(err, service.ErrMerchantInactive) {
			response.Error(c, http.StatusUnauthorized,
				"Wrong email or password",
				response.AuthInvalidCredentials, nil)
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	middleware.SetAuthCookies(c, out.Tokens.AccessToken, out.Tokens.RefreshToken, out.Tokens.AccessExpiry, h.isProd)

	response.Success(c, http.StatusOK, "Login successful",
		response.AuthLoginSuccess, gin.H{
			"merchant": gin.H{
				"id":            out.Merchant.ID,
				"business_name": out.Merchant.BusinessName,
				"email":         out.Merchant.Email,
				"public_key":    out.Merchant.PublicKey,
			},
			"access_token":  out.Tokens.AccessToken,
			"access_expiry": out.Tokens.AccessExpiry,
		})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("payfake_refresh")
	if err != nil || refreshToken == "" {
		response.UnauthorizedErr(c, "Refresh token is required")
		return
	}

	tokens, err := h.authSvc.RefreshTokens(refreshToken)
	if err != nil {
		middleware.ClearAuthCookies(c)
		if errors.Is(err, service.ErrTokenExpired) {
			response.Error(c, http.StatusUnauthorized,
				"Session expired, please login again",
				response.AuthTokenExpired, nil)
			return
		}
		response.UnauthorizedErr(c, "Invalid refresh token")
		return
	}

	middleware.SetAuthCookies(c, tokens.AccessToken, tokens.RefreshToken, tokens.AccessExpiry, h.isProd)

	response.Success(c, http.StatusOK, "Token refreshed",
		response.AuthRefreshSuccess, gin.H{
			"access_token":  tokens.AccessToken,
			"access_expiry": tokens.AccessExpiry,
		})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	middleware.ClearAuthCookies(c)
	response.Success(c, http.StatusOK, "Logged out successfully", response.AuthLogoutSuccess, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.authSvc.GetMerchant(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
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
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.authSvc.GetMerchant(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Keys retrieved",
		response.AuthKeysFetched, gin.H{
			"public_key": merchant.PublicKey,
			"secret_key": merchant.SecretKey,
		})
}

func (h *AuthHandler) RegenerateKeys(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	publicKey, secretKey, err := h.authSvc.RegenerateKeys(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Keys updated",
		response.AuthKeyRegenerated, gin.H{
			"public_key": publicKey,
			"secret_key": secretKey,
		})
}
