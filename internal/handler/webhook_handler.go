package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	pfcrypto "github.com/GordenArcher/payfake/pkg/crypto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	db          *gorm.DB
	merchantSvc *service.MerchantService
	authSvc     *service.AuthService
}

func NewWebhookHandler(db *gorm.DB, merchantSvc *service.MerchantService, authSvc *service.AuthService) *WebhookHandler {
	return &WebhookHandler{db: db, merchantSvc: merchantSvc, authSvc: authSvc}
}

type updateWebhookURLRequest struct {
	WebhookURL string `json:"webhook_url" binding:"required,url"`
}

var (
	webhookTestMu      sync.Mutex
	webhookTestBuckets = make(map[string]*webhookTestBucket)
)

type webhookTestBucket struct {
	count       int
	windowStart time.Time
}

// webhookTestRateLimit returns true if the merchant is within the
// 5-per-minute limit, false if they've exceeded it.
func webhookTestRateLimit(merchantID string) bool {
	webhookTestMu.Lock()
	defer webhookTestMu.Unlock()

	b, ok := webhookTestBuckets[merchantID]
	if !ok {
		webhookTestBuckets[merchantID] = &webhookTestBucket{
			count:       1,
			windowStart: time.Now(),
		}
		return true
	}

	if time.Since(b.windowStart) >= time.Minute {
		b.count = 1
		b.windowStart = time.Now()
		return true
	}

	b.count++
	return b.count <= 5
}

// UpdateWebhookURL handles POST /api/v1/merchant/webhook
// Sets the merchant's webhook URL from the dashboard.
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

	merchant, err := h.merchantSvc.UpdateProfile(merchantID, "", req.WebhookURL)
	if err != nil {
		response.InternalErr(c, "Failed to update webhook URL")
		return
	}

	response.Success(c, http.StatusOK, "Webhook URL updated",
		response.MerchantUpdated, gin.H{
			"webhook_url": merchant.WebhookURL,
		})
}

// GetWebhookURL handles GET /api/v1/merchant/webhook
func (h *WebhookHandler) GetWebhookURL(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	merchant, err := h.merchantSvc.GetProfile(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch webhook config")
		return
	}

	response.Success(c, http.StatusOK, "Webhook config fetched",
		response.MerchantFetched, gin.H{
			"webhook_url": merchant.WebhookURL,
			"is_set":      merchant.WebhookURL != "",
		})
}

// TestWebhook handles POST /api/v1/merchant/webhook/test
// Fires a test webhook to the merchant's configured URL so they can
// verify their endpoint is reachable and their signature verification works.
func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	// Rate limit — 5 test webhooks per minute per merchant.
	// Stored in a package-level sync.Map keyed by merchant ID.
	if !webhookTestRateLimit(merchantID) {
		response.Error(c, http.StatusTooManyRequests,
			"Too many test webhooks — wait a minute before trying again",
			response.RateLimitExceeded, []response.ErrorField{})
		return
	}

	merchant, err := h.merchantSvc.GetProfile(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch merchant")
		return
	}

	if merchant.WebhookURL == "" {
		response.Error(c, http.StatusUnprocessableEntity,
			"No webhook URL configured",
			response.WebhookNotFound, []response.ErrorField{
				{Field: "webhook_url", Message: "Configure a webhook URL first"},
			})
		return
	}

	payload := map[string]any{
		"event": "charge.success",
		"data": map[string]any{
			"id":         "TXN_TEST_WEBHOOK",
			"reference":  fmt.Sprintf("test-webhook-%d", time.Now().Unix()),
			"amount":     10000,
			"currency":   "GHS",
			"status":     "success",
			"channel":    "card",
			"paid_at":    time.Now().Format(time.RFC3339),
			"created_at": time.Now().Format(time.RFC3339),
			"customer": map[string]any{
				"email": "test@payfake.dev",
			},
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	signature := pfcrypto.Sign(merchant.SecretKey, payloadBytes)

	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, _ := http.NewRequest("POST", merchant.WebhookURL, bytes.NewBuffer(payloadBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Paystack-Signature", signature)

	httpResp, err := client.Do(httpReq)

	result := gin.H{
		"webhook_url": merchant.WebhookURL,
		"payload":     payload,
	}

	if err != nil {
		result["success"] = false
		result["error"] = err.Error()
		result["status_code"] = 0
		response.Success(c, http.StatusOK, "Test webhook delivery failed",
			response.WebhookDeliveryFailed, result)
		return
	}
	defer httpResp.Body.Close()

	result["success"] = httpResp.StatusCode >= 200 && httpResp.StatusCode < 300
	result["status_code"] = httpResp.StatusCode

	response.Success(c, http.StatusOK, "Test webhook sent",
		response.WebhookRetried, result)
}
