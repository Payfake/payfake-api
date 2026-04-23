package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSanitizeLogBodyRedactsSensitiveFields(t *testing.T) {
	input := []byte(`{"email":"dev@example.com","password":"secret123","secret_key":"sk_test_123","card":{"number":"4111111111111111","cvv":"123"},"nested":{"access_token":"abc","authorization_code":"AUTH_1"}}`)

	sanitized := sanitizeLogBody(input)

	for _, forbidden := range []string{"secret123", "sk_test_123", "4111111111111111", `"cvv":"123"`, `"access_token":"abc"`, `"authorization_code":"AUTH_1"`} {
		if strings.Contains(sanitized, forbidden) {
			t.Fatalf("expected %q to be redacted from %q", forbidden, sanitized)
		}
	}
	for _, expected := range []string{`"password":"[REDACTED]"`, `"secret_key":"[REDACTED]"`, `"number":"[REDACTED]"`, `"cvv":"[REDACTED]"`, `"access_token":"[REDACTED]"`, `"authorization_code":"[REDACTED]"`} {
		if !strings.Contains(sanitized, expected) {
			t.Fatalf("expected %q in %q", expected, sanitized)
		}
	}
}

func TestSanitizeLogBodyTruncatesLargePayloads(t *testing.T) {
	input := []byte(strings.Repeat("a", maxLoggedBodyLength+100))

	sanitized := sanitizeLogBody(input)

	if !strings.HasSuffix(sanitized, "...[truncated]") {
		t.Fatalf("expected truncated suffix, got %q", sanitized)
	}
	if len(sanitized) <= maxLoggedBodyLength {
		t.Fatalf("expected truncated body to exceed prefix length with marker, got %d", len(sanitized))
	}
}

func TestCaptureRequestBodyForLoggingSkipsLargeBodiesAndPreservesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	largeBody := strings.Repeat("a", maxLoggedRequestPreviewBytes+100)
	req := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader(largeBody))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	logged := captureRequestBodyForLogging(ctx)
	if !strings.Contains(logged, "request body exceeds log preview limit") {
		t.Fatalf("expected large body marker, got %q", logged)
	}

	bodyAfter, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		t.Fatalf("failed to read restored request body: %v", err)
	}
	if string(bodyAfter) != largeBody {
		t.Fatalf("expected original body to be preserved")
	}
}
