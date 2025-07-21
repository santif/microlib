package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
)

func TestJWTAuthenticator_Integration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create a test JWKS server
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksServer.Close()

	// Create auth config
	config := AuthConfig{
		JWKSEndpoint:    jwksServer.URL,
		Issuer:          "https://example.com",
		Audience:        []string{"api"},
		RefreshInterval: 1 * time.Hour,
		TokenLookup:     "header:Authorization",
		AuthScheme:      "Bearer",
		BypassPaths:     []string{"/health", "/metrics"},
	}

	// Create authenticator
	auth, err := NewJWTAuthenticator(config, logger)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get claims from context
		claims, ok := GetClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "No claims in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(claims.Subject))
	})

	// Apply the middleware
	handler := auth.Middleware()(testHandler)

	// Create a test request with a bypass path
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	// Create a simple handler that doesn't check for claims
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("bypassed"))
	})

	// Apply the bypass middleware
	bypassHandler := auth.WithBypass(config.BypassPaths)(simpleHandler)
	bypassHandler.ServeHTTP(rec, req)

	// Verify the response (should bypass auth)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for bypass path, got %d", http.StatusOK, rec.Code)
	}

	// Create a test request with a non-bypass path and no token
	req = httptest.NewRequest("GET", "/api/users", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should require auth)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d for missing token, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestJWTAuthenticator_WithScope(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// We don't need the mock authenticator for this test as we're directly testing the RequireScope middleware
	_ = logger

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware with scope requirement
	handler := RequireScope("read:users")(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/api/users", nil)
	rec := httptest.NewRecorder()

	// Add claims to context
	claims := &Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "read:users write:users",
		},
	}
	ctx := context.WithValue(req.Context(), ClaimsContextKey, claims)
	req = req.WithContext(ctx)

	// Call the handler
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for valid scope, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with missing required scope
	req = httptest.NewRequest("GET", "/api/users", nil)
	rec = httptest.NewRecorder()
	claims = &Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "write:users",
		},
	}
	ctx = context.WithValue(req.Context(), ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d for missing scope, got %d", http.StatusForbidden, rec.Code)
	}
}
