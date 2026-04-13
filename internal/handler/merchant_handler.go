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

type MerchantHandler struct {
	db          *gorm.DB
	merchantSvc *service.MerchantService
	authSvc     *service.AuthService
}

func NewMerchantHandler(db *gorm.DB, merchantSvc *service.MerchantService, authSvc *service.AuthService) *MerchantHandler {
	return &MerchantHandler{db: db, merchantSvc: merchantSvc, authSvc: authSvc}
}

// GetProfile handles GET /api/v1/merchant
// Returns the full merchant profile for the dashboard settings page.
func (h *MerchantHandler) GetProfile(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.merchantSvc.GetProfile(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch profile")
		return
	}

	response.Success(c, http.StatusOK, "Profile fetched",
		response.MerchantFetched, gin.H{
			"id":            merchant.ID,
			"business_name": merchant.BusinessName,
			"email":         merchant.Email,
			"public_key":    merchant.PublicKey,
			"webhook_url":   merchant.WebhookURL,
			"is_active":     merchant.IsActive,
			"created_at":    merchant.CreatedAt,
			"updated_at":    merchant.UpdatedAt,
		})
}

type updateProfileRequest struct {
	BusinessName string `json:"business_name"`
	WebhookURL   string `json:"webhook_url"`
}

// UpdateProfile handles PUT /api/v1/merchant
// Allows the merchant to update their business name and webhook URL
// from the dashboard settings page.
func (h *MerchantHandler) UpdateProfile(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.merchantSvc.UpdateProfile(merchantID, req.BusinessName, req.WebhookURL)
	if err != nil {
		response.InternalErr(c, "Failed to update profile")
		return
	}

	response.Success(c, http.StatusOK, "Profile updated",
		response.MerchantUpdated, gin.H{
			"id":            merchant.ID,
			"business_name": merchant.BusinessName,
			"email":         merchant.Email,
			"webhook_url":   merchant.WebhookURL,
			"updated_at":    merchant.UpdatedAt,
		})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword handles PUT /api/v1/merchant/password
func (h *MerchantHandler) ChangePassword(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	err := h.authSvc.ChangePassword(service.ChangePasswordInput{
		MerchantID:      merchantID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, "Current password is incorrect",
				response.AuthInvalidCredentials, []response.ErrorField{
					{Field: "current_password", Message: "Incorrect password"},
				})
			return
		}
		response.InternalErr(c, "Failed to change password")
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully",
		response.MerchantUpdated, nil)
}
