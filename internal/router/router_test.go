package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateRoutesDoNotInheritPublicWildcardCORS(t *testing.T) {
	result := Setup(nil, "test-secret", "15", "7", "http://localhost:5173", "development")
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	result.Engine.ServeHTTP(recorder, req)
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("expected private route to reject unknown origin, got %q", origin)
	}
}

func TestPublicRoutesAllowWildcardCORS(t *testing.T) {
	result := Setup(nil, "test-secret", "15", "7", "http://localhost:5173", "development")
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/public/charge", nil)
	req.Header.Set("Origin", "https://checkout.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	result.Engine.ServeHTTP(recorder, req)
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("expected public wildcard origin, got %q", origin)
	}
}

func TestPrivateRoutesAllowConfiguredOriginPreflight(t *testing.T) {
	result := Setup(nil, "test-secret", "15", "7", "http://localhost:5173", "development")
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	result.Engine.ServeHTTP(recorder, req)
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
		t.Fatalf("expected configured private origin, got %q", origin)
	}
}
