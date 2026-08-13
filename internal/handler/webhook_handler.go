package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	db          *gorm.DB
	merchantSvc *service.MerchantService
	authSvc     *service.AuthService
	webhookSvc  *service.WebhookService
}

func NewWebhookHandler(db *gorm.DB, merchantSvc *service.MerchantService, authSvc *service.AuthService, webhookSvc *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{db: db, merchantSvc: merchantSvc, authSvc: authSvc, webhookSvc: webhookSvc}
}

func (h *WebhookHandler) GetWebhookURL(c *gin.Context) {
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

	response.Success(c, http.StatusOK, "Webhook config retrieved",
		response.MerchantFetched, gin.H{
			"webhook_url": merchant.WebhookURL,
			"is_set":      merchant.WebhookURL != "",
		})
}

type updateWebhookURLRequest struct {
	WebhookURL string `json:"webhook_url" binding:"required,url"`
}

func (h *WebhookHandler) UpdateWebhookURL(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	var req updateWebhookURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.merchantSvc.UpdateProfile(merchantID, nil, &req.WebhookURL)
	if err != nil {
		if errors.Is(err, service.ErrInvalidWebhookURL) {
			response.UnprocessableErr(c, "Invalid webhook URL", response.MerchantUpdated,
				field("webhook_url", "url", "Webhook URL is not allowed"))
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Webhook URL updated",
		response.MerchantUpdated, gin.H{
			"webhook_url": merchant.WebhookURL,
		})
}

func (h *WebhookHandler) TestWebhook(c *gin.Context) {
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

	if merchant.WebhookURL == "" {
		response.Error(c, http.StatusUnprocessableEntity,
			"No webhook URL configured",
			response.WebhookNotFound,
			field("webhook_url", "required", "Configure a webhook URL first"))
		return
	}

	payload := map[string]any{
		"event": "charge.success",
		"data": map[string]any{
			"id":               "TXN_TEST_WEBHOOK",
			"domain":           "test",
			"reference":        fmt.Sprintf("test-webhook-%d", time.Now().Unix()),
			"amount":           10000,
			"currency":         "GHS",
			"status":           "success",
			"channel":          "card",
			"gateway_response": "Approved",
			"paid_at":          time.Now().Format(time.RFC3339),
			"created_at":       time.Now().Format(time.RFC3339),
			"customer": map[string]any{
				"email": "test@payfake.dev",
			},
		},
	}

	statusCode, err := h.webhookSvc.DeliverTest(merchant, payload)

	result := gin.H{
		"webhook_url": merchant.WebhookURL,
		"payload":     payload,
	}

	if err != nil {
		result["success"] = false
		result["error"] = err.Error()
		result["status_code"] = 0
		response.Success(c, http.StatusOK, "Webhook delivery failed",
			response.WebhookDeliveryFailed, result)
		return
	}
	result["success"] = statusCode >= 200 && statusCode < 300
	result["status_code"] = statusCode

	response.Success(c, http.StatusOK, "Test webhook sent",
		response.WebhookRetried, result)
}
