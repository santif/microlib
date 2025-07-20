package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
)

// mockAuthenticator is a mock implementation of security.Authenticator for testing
type mockAuthenticator struct {
	validateFunc func(ctx context.Context, token string) (*security.Claims, error)
	bypassPaths  []string
}

func (m *mockAuthenticator) ValidateToken(ctx context.Context, token string) (*security.Claims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return &security.Claims{Subject: "test-user"}, nil
}

func (m *mockAuthenticator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token := authHeader[7:]
			claims, err := m.ValidateToken(r.Context(), token)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), security.ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *mockAuthenticator) WithBypass(paths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the path should bypass authentication
			for _, path := range paths {
				if r.URL.Path == path || (path != "/" && len(r.URL.Path) >= len(path) && r.URL.Path[:len(path)] == path) {
					// Skip authentication for this path
					next.ServeHTTP(w, r)
					return
				}
			}

			// Apply the authentication middleware
			m.Middleware()(next).ServeHTTP(w, r)
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Create a mock authenticator
	auth := &mockAuthenticator{}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := AuthMiddleware(auth)(testHandler)

	// Test with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "test-user" {
		t.Errorf("Expected body %q, got %q", "test-user", rec.Body.String())
	}

	// Test with invalid token
	auth.validateFunc = func(ctx context.Context, token string) (*security.Claims, error) {
		return nil, security.ErrInvalidToken
	}
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Test with missing token
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthBypassMiddleware(t *testing.T) {
	// Create a mock authenticator
	auth := &mockAuthenticator{}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware with bypass paths
	bypassPaths := []string{"/health", "/metrics", "/openapi.json", "/swagger"}
	handler := AuthBypassMiddleware(auth, bypassPaths)(testHandler)

	// Test with bypass path
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should bypass auth)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with another bypass path
	req = httptest.NewRequest("GET", "/openapi.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should bypass auth)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with non-bypass path and no token
	req = httptest.NewRequest("GET", "/api/users", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response (should require auth)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Test with non-bypass path and valid token
	auth.validateFunc = nil // Reset to return valid claims
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestNewAuthenticator(t *testing.T) {
	// Create a logger
	logger := observability.NewNoOpLogger()

	// Test with auth disabled
	config := AuthConfig{
		Enabled: false,
	}
	auth, err := NewAuthenticator(config, logger)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if auth != nil {
		t.Errorf("Expected nil authenticator, got %v", auth)
	}

	// Test with auth enabled but missing JWKS endpoint
	config = AuthConfig{
		Enabled: true,
	}
	auth, err = NewAuthenticator(config, logger)
	if err == nil {
		t.Errorf("Expected error for missing JWKS endpoint, got nil")
	}
	// We don't need to check auth here as we're only concerned with the error

	// Test with auth enabled and valid config
	// Note: This will fail in real usage because we're not mocking the JWKS endpoint
	// In a real test, you would mock the HTTP client or use a test server
	// For this test, we'll just verify that the function attempts to create an authenticator
	config = AuthConfig{
		Enabled:      true,
		JWKSEndpoint: "https://example.com/.well-known/jwks.json",
		Issuer:       "https://example.com",
		Audience:     []string{"api"},
	}
	// We expect this to fail because the JWKS endpoint doesn't exist
	_, err = NewAuthenticator(config, logger)
	if err == nil {
		t.Errorf("Expected error for invalid JWKS endpoint, got nil")
	}
}

func TestWithAuthOptions(t *testing.T) {
	// Test WithAuth
	config := DefaultServerConfig()
	WithAuth(true, "https://example.com/.well-known/jwks.json")(&config)
	if config.Auth == nil {
		t.Errorf("Expected Auth to be non-nil")
	}
	if !config.Auth.Enabled {
		t.Errorf("Expected Auth.Enabled to be true")
	}
	if config.Auth.JWKSEndpoint != "https://example.com/.well-known/jwks.json" {
		t.Errorf("Expected Auth.JWKSEndpoint to be %q, got %q", "https://example.com/.well-known/jwks.json", config.Auth.JWKSEndpoint)
	}

	// Test WithAuthIssuer
	WithAuthIssuer("https://example.com")(&config)
	if config.Auth.Issuer != "https://example.com" {
		t.Errorf("Expected Auth.Issuer to be %q, got %q", "https://example.com", config.Auth.Issuer)
	}

	// Test WithAuthAudience
	audience := []string{"api", "web"}
	WithAuthAudience(audience)(&config)
	if len(config.Auth.Audience) != 2 || config.Auth.Audience[0] != "api" || config.Auth.Audience[1] != "web" {
		t.Errorf("Expected Auth.Audience to be %v, got %v", audience, config.Auth.Audience)
	}

	// Test WithAuthBypass
	paths := []string{"/health", "/metrics", "/docs"}
	WithAuthBypass(paths)(&config)
	if len(config.Auth.BypassPaths) != 3 || config.Auth.BypassPaths[0] != "/health" || config.Auth.BypassPaths[1] != "/metrics" || config.Auth.BypassPaths[2] != "/docs" {
		t.Errorf("Expected Auth.BypassPaths to be %v, got %v", paths, config.Auth.BypassPaths)
	}
}

func TestRequireScopeMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RequireScopeMiddleware("read:users")(testHandler)

	// Test with valid scope
	req := httptest.NewRequest("GET", "/test", nil)
	claims := &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "read:users write:users",
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
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with invalid scope
	req = httptest.NewRequest("GET", "/test", nil)
	claims = &security.Claims{
		Subject: "test-user",
		Custom: map[string]interface{}{
			"scope": "write:users",
		},
	}
	ctx = context.WithValue(req.Context(), security.ClaimsContextKey, claims)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, rec.Code)
	}

	// Test with no claims
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
