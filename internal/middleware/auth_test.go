package middleware

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type stubJWTValidator struct {
	merchantID string
	err        error
}

func (s stubJWTValidator) ValidateJWT(string) (string, string, error) {
	if s.err != nil {
		return "", "", s.err
	}
	return s.merchantID, "merchant@example.com", nil
}

func TestSetAuthCookiesSetsSecureCookiesInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	SetAuthCookies(ctx, "access-token", "refresh-token", time.Now().Add(time.Hour), true)

	cookies := recorder.Header().Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	var accessCookie, refreshCookie string
	for _, cookie := range cookies {
		switch {
		case strings.Contains(cookie, "payfake_access="):
			accessCookie = cookie
		case strings.Contains(cookie, "payfake_refresh="):
			refreshCookie = cookie
		}
	}

	if !strings.Contains(accessCookie, "Secure") || !strings.Contains(refreshCookie, "Secure") {
		t.Fatalf("expected secure cookies, got access=%q refresh=%q", accessCookie, refreshCookie)
	}
	if !strings.Contains(accessCookie, "HttpOnly") || !strings.Contains(refreshCookie, "HttpOnly") {
		t.Fatalf("expected httponly cookies, got access=%q refresh=%q", accessCookie, refreshCookie)
	}
	if !strings.Contains(accessCookie, "SameSite=Lax") || !strings.Contains(refreshCookie, "SameSite=Lax") {
		t.Fatalf("expected SameSite=Lax cookies, got access=%q refresh=%q", accessCookie, refreshCookie)
	}
}

func TestClearAuthCookiesMatchesProductionSecurityFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	ClearAuthCookies(ctx, true)

	cookies := recorder.Header().Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	for _, cookie := range cookies {
		if !strings.Contains(cookie, "Max-Age=0") {
			t.Fatalf("expected deletion cookie, got %q", cookie)
		}
		if !strings.Contains(cookie, "Secure") || !strings.Contains(cookie, "HttpOnly") {
			t.Fatalf("expected secure httponly deletion cookie, got %q", cookie)
		}
	}
}

func TestHydrateMerchantIDFromJWTStoresMerchantID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("jwt_token", "token")

	mw := HydrateMerchantIDFromJWT(stubJWTValidator{merchantID: "MRC_123"})
	mw(ctx)

	got, exists := ctx.Get("merchant_id_from_jwt")
	if !exists || got != "MRC_123" {
		t.Fatalf("expected merchant_id_from_jwt to be set, got %v exists=%v", got, exists)
	}
}

func TestHydrateMerchantIDFromJWTRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("jwt_token", "token")

	mw := HydrateMerchantIDFromJWT(stubJWTValidator{err: errors.New("invalid")})
	mw(ctx)

	if recorder.Code != 401 {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}
