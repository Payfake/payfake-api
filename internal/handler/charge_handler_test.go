package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicSubmitOTPRequiresAccessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/charge/submit_otp", strings.NewReader(`{"reference":"TXN_123","otp":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	handler := &ChargeHandler{}
	handler.PublicSubmitOTP(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}

	var body struct {
		Errors map[string][]map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body.Errors["access_code"]; !ok {
		t.Fatalf("expected access_code validation error, got %v", body.Errors)
	}
}

func TestSimulate3DSRejectsMismatchedReferenceBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/simulate/3ds/TXN_123", strings.NewReader(`{"access_code":"ACC_123","reference":"TXN_999"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "reference", Value: "TXN_123"}}

	handler := &ChargeHandler{}
	handler.Simulate3DS(ctx)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got := body["message"]; got != "Transaction not found" {
		t.Fatalf("expected transaction not found message, got %v", got)
	}
}
