package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitRejectsRequestsPastLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter = sync.Map{}

	router := gin.New()
	router.Use(RateLimit(1, time.Hour))
	router.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	firstReq := httptest.NewRequest(http.MethodGet, "/limited", nil)
	firstReq.RemoteAddr = "203.0.113.10:1234"
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, firstReq)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed, got %d", firstResp.Code)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/limited", nil)
	secondReq.RemoteAddr = "203.0.113.10:1234"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got %d", secondResp.Code)
	}
}

func TestRateLimitWebhookTestUsesMerchantIDContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	webhookTestLimiter = sync.Map{}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("merchant_id_from_jwt", "MRC_123")
		c.Next()
	})
	router.Use(RateLimitWebhookTest())
	router.POST("/webhook/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhook/test", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected request %d to pass, got %d", i+1, resp.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 6th request to be limited, got %d", resp.Code)
	}
}
