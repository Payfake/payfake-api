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

type MerchantHandler struct {
	db          *gorm.DB
	merchantSvc *service.MerchantService
	authSvc     *service.AuthService
}

func NewMerchantHandler(db *gorm.DB, merchantSvc *service.MerchantService, authSvc *service.AuthService) *MerchantHandler {
	return &MerchantHandler{db: db, merchantSvc: merchantSvc, authSvc: authSvc}
}

func (h *MerchantHandler) GetProfile(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.merchantSvc.GetProfile(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Profile retrieved",
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
		response.InternalErr(c, "An error occurred, please try again later")
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
			response.Error(c, http.StatusUnauthorized,
				"Current password is incorrect",
				response.AuthInvalidCredentials,
				field("current_password", "invalid", "Incorrect password"))
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Password updated", response.MerchantUpdated, nil)
}
