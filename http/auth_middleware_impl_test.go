package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
)

func TestNewAuthMiddleware(t *testing.T) {
	// Create a mock authenticator
	auth := &mockAuthenticator{}

	// Create middleware options
	opts := AuthMiddlewareOptions{
		BypassPaths:     []string{"/health", "/metrics"},
		RequiredScopes:  []string{"api:read"},
		ClaimsToContext: []string{"sub", "email"},
	}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For bypass paths, we don't expect claims
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("bypassed"))
			return
		}

		// Get claims from context
		claims, ok := security.GetClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "No claims in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(claims.Subject))
	})

	// Apply the middleware
	handler := NewAuthMiddleware(auth, opts)(testHandler)

	// Test with bypass path
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should bypass auth)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for bypass path, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "bypassed" {
		t.Errorf("Expected body %q for bypass path, got %q", "bypassed", rec.Body.String())
	}

	// Test with valid token and scopes
	auth.validateFunc = func(ctx context.Context, token string) (*security.Claims, error) {
		return &security.Claims{
			Subject: "test-user",
			Custom: map[string]interface{}{
				"scope": "api:read api:write",
				"email": "test@example.com",
			},
		}, nil
	}
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for valid token, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "test-user" {
		t.Errorf("Expected body %q, got %q", "test-user", rec.Body.String())
	}

	// Test with valid token but missing required scope
	auth.validateFunc = func(ctx context.Context, token string) (*security.Claims, error) {
		return &security.Claims{
			Subject: "test-user",
			Custom: map[string]interface{}{
				"scope": "api:write",
			},
		}, nil
	}
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d for missing scope, got %d", http.StatusForbidden, rec.Code)
	}

	// Test with invalid token
	auth.validateFunc = func(ctx context.Context, token string) (*security.Claims, error) {
		return nil, security.ErrInvalidToken
	}
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d for invalid token, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireScope(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RequireScope("api:read")(testHandler)

	// Test with valid scope
	req := httptest.NewRequest("GET", "/test", nil)
	claims := &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "api:read api:write",
		},
	}
	ctx := context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for valid scope, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with missing scope
	req = httptest.NewRequest("GET", "/test", nil)
	claims = &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "api:write",
		},
	}
	ctx = context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d for missing scope, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestRequireScopes(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RequireScopes([]string{"api:read", "api:write"})(testHandler)

	// Test with all required scopes
	req := httptest.NewRequest("GET", "/test", nil)
	claims := &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "api:read api:write api:delete",
		},
	}
	ctx := context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d for valid scopes, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with missing scope
	req = httptest.NewRequest("GET", "/test", nil)
	claims = &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "api:read",
		},
	}
	ctx = context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d for missing scope, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestExtractClaimsToContext(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get extracted claim from context
		email := r.Context().Value(security.ClaimContextKey("email"))
		if email == nil {
			http.Error(w, "No email in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(email.(string)))
	})

	// Apply the middleware
	handler := ExtractClaimsToContext([]string{"email", "name"})(testHandler)

	// Test with claims in context
	req := httptest.NewRequest("GET", "/test", nil)
	claims := &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"email": "test@example.com",
			"name":  "Test User",
		},
	}
	ctx := context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "test@example.com" {
		t.Errorf("Expected body %q, got %q", "test@example.com", rec.Body.String())
	}

	// Test with no claims in context
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should fail because the test handler expects the claim)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d for no claims, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestRegisterAuthMiddleware(t *testing.T) {
	// Create a mock server
	logger := observability.NewNoOpLogger()
	server := NewServer(ServerDependencies{Logger: logger})

	// Test with auth disabled
	config := server.Config()
	config.Auth.Enabled = false
	err := RegisterAuthMiddleware(server, logger)
	if err != nil {
		t.Errorf("Expected no error for disabled auth, got %v", err)
	}

	// Skip test with auth enabled since it requires a real JWKS endpoint
	// In a real test, you would mock the HTTP client or use a test server
}
