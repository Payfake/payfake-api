package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicVerifyRequiresAccessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/transaction/verify/TXN_123", nil)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "reference", Value: "TXN_123"}}

	handler := &TransactionHandler{}
	handler.PublicVerify(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got := body["message"]; got != "Access code is required" {
		t.Fatalf("expected missing access code message, got %v", got)
	}
}
