package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/santif/microlib/observability"
)

func TestRBACAuthorizer_HasRole(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create authorizer
	config := DefaultAuthorizationConfig()
	authorizer, err := NewRBACAuthorizer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create authorizer: %v", err)
	}

	// Create context with roles
	roles := []Role{"admin", "user"}
	ctx := context.WithValue(context.Background(), RolesContextKey, roles)

	// Test with existing role
	hasRole, err := authorizer.HasRole(ctx, "admin")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !hasRole {
		t.Errorf("Expected hasRole to be true for 'admin'")
	}

	// Test with non-existing role
	hasRole, err = authorizer.HasRole(ctx, "editor")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if hasRole {
		t.Errorf("Expected hasRole to be false for 'editor'")
	}

	// Test with no roles in context
	hasRole, err = authorizer.HasRole(context.Background(), "admin")
	if err == nil {
		t.Errorf("Expected error for missing roles in context")
	}
	if hasRole {
		t.Errorf("Expected hasRole to be false for missing roles in context")
	}
}

func TestRBACAuthorizer_HasPermission(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create authorizer
	config := DefaultAuthorizationConfig()
	authorizer, err := NewRBACAuthorizer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create authorizer: %v", err)
	}

	// Add role permissions
	authorizer.AddRolePermissions("admin", []Permission{"read:users", "write:users"})
	authorizer.AddRolePermissions("user", []Permission{"read:users"})

	// Create context with roles
	roles := []Role{"admin"}
	ctx := context.WithValue(context.Background(), RolesContextKey, roles)

	// Test with permission from role
	hasPermission, err := authorizer.HasPermission(ctx, "read:users")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !hasPermission {
		t.Errorf("Expected hasPermission to be true for 'read:users'")
	}

	// Test with non-existing permission
	hasPermission, err = authorizer.HasPermission(ctx, "delete:users")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if hasPermission {
		t.Errorf("Expected hasPermission to be false for 'delete:users'")
	}

	// Test with direct permissions
	permissions := []Permission{"delete:users"}
	ctx = context.WithValue(ctx, PermissionsContextKey, permissions)

	hasPermission, err = authorizer.HasPermission(ctx, "delete:users")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !hasPermission {
		t.Errorf("Expected hasPermission to be true for direct permission 'delete:users'")
	}

	// Test with no roles or permissions in context
	hasPermission, err = authorizer.HasPermission(context.Background(), "read:users")
	if err == nil {
		t.Errorf("Expected error for missing roles in context")
	}
	if hasPermission {
		t.Errorf("Expected hasPermission to be false for missing roles in context")
	}
}

func TestExtractRolesAndPermissions(t *testing.T) {
	// Create claims
	claims := &Claims{
		Subject: "user123",
		Custom: map[string]interface{}{
			"roles":       "admin user",
			"permissions": []interface{}{"read:users", "write:users"},
		},
	}

	// Create context with claims
	ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

	// Extract roles and permissions
	roles, permissions := ExtractRolesAndPermissions(ctx, "roles", "permissions")

	// Check roles
	if len(roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(roles))
	}
	if string(roles[0]) != "admin" || string(roles[1]) != "user" {
		t.Errorf("Expected roles [admin user], got %v", roles)
	}

	// Check permissions
	if len(permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(permissions))
	}
	if string(permissions[0]) != "read:users" || string(permissions[1]) != "write:users" {
		t.Errorf("Expected permissions [read:users write:users], got %v", permissions)
	}

	// Test with no claims in context
	roles, permissions = ExtractRolesAndPermissions(context.Background(), "roles", "permissions")
	if len(roles) != 0 || len(permissions) != 0 {
		t.Errorf("Expected empty roles and permissions for missing claims")
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create authorizer
	config := DefaultAuthorizationConfig()
	authorizer, err := NewRBACAuthorizer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create authorizer: %v", err)
	}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RequireRoleMiddleware(authorizer, "admin")(testHandler)

	// Test with valid role
	req := httptest.NewRequest("GET", "/test", nil)
	roles := []Role{"admin", "user"}
	ctx := context.WithValue(req.Context(), RolesContextKey, roles)
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

	// Test with invalid role
	req = httptest.NewRequest("GET", "/test", nil)
	roles = []Role{"user"}
	ctx = context.WithValue(req.Context(), RolesContextKey, roles)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, rec.Code)
	}

	// Test with no roles in context
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequirePermissionMiddleware(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create authorizer
	config := DefaultAuthorizationConfig()
	authorizer, err := NewRBACAuthorizer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create authorizer: %v", err)
	}

	// Add role permissions
	authorizer.AddRolePermissions("admin", []Permission{"read:users", "write:users"})
	authorizer.AddRolePermissions("user", []Permission{"read:users"})

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RequirePermissionMiddleware(authorizer, "write:users")(testHandler)

	// Test with valid permission from role
	req := httptest.NewRequest("GET", "/test", nil)
	roles := []Role{"admin"}
	ctx := context.WithValue(req.Context(), RolesContextKey, roles)
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

	// Test with invalid permission
	req = httptest.NewRequest("GET", "/test", nil)
	roles = []Role{"user"}
	ctx = context.WithValue(req.Context(), RolesContextKey, roles)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, rec.Code)
	}

	// Test with direct permission
	req = httptest.NewRequest("GET", "/test", nil)
	permissions := []Permission{"write:users"}
	ctx = context.WithValue(req.Context(), PermissionsContextKey, permissions)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", rec.Body.String())
	}

	// Test with no roles or permissions in context
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRolesAndPermissionsMiddleware(t *testing.T) {
	// Create claims
	claims := &Claims{
		Subject: "user123",
		Custom: map[string]interface{}{
			"roles":       "admin user",
			"permissions": []interface{}{"read:users", "write:users"},
		},
	}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get roles from context
		roles, ok := r.Context().Value(RolesContextKey).([]Role)
		if !ok {
			t.Errorf("Expected roles in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get permissions from context
		permissions, ok := r.Context().Value(PermissionsContextKey).([]Permission)
		if !ok {
			t.Errorf("Expected permissions in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Check roles
		if len(roles) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(roles))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Check permissions
		if len(permissions) != 2 {
			t.Errorf("Expected 2 permissions, got %d", len(permissions))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply the middleware
	handler := RolesAndPermissionsMiddleware("roles", "permissions")(testHandler)

	// Test with valid claims
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), ClaimsContextKey, claims)
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

	// Test with no claims in context
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()

	// Create a simpler handler for this case that doesn't expect roles/permissions
	noClaimsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("no claims"))
	})

	// Apply middleware to the simpler handler
	noClaimsMiddleware := RolesAndPermissionsMiddleware("roles", "permissions")(noClaimsHandler)
	noClaimsMiddleware.ServeHTTP(rec, req)

	// Should still work, just without roles and permissions
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestLocalPolicyEvaluator_Evaluate(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create evaluator without loading policies
	evaluator := NewLocalPolicyEvaluator("", logger)

	// Add a test policy
	evaluator.policies["test"] = Policy{
		Name:          "test",
		Description:   "Test policy",
		DefaultAction: "deny",
		Rules: []PolicyRule{
			{
				Name:      "allow-admin",
				Effect:    "allow",
				Subjects:  []string{"role:admin"},
				Resources: []string{"test:*"},
				Actions:   []string{"read", "write"},
			},
			{
				Name:      "allow-user-read",
				Effect:    "allow",
				Subjects:  []string{"role:user"},
				Resources: []string{"test:*"},
				Actions:   []string{"read"},
			},
		},
	}

	// Test with admin role and read action
	req := AuthorizationRequest{
		Subject: Subject{
			ID:    "user123",
			Roles: []Role{"admin"},
		},
		Resource: Resource{
			Type: "test",
			ID:   "resource1",
		},
		Action: "read",
	}

	resp, err := evaluator.Evaluate(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !resp.Allowed {
		t.Errorf("Expected allowed to be true for admin role and read action")
	}

	// Test with user role and read action
	req.Subject.Roles = []Role{"user"}
	resp, err = evaluator.Evaluate(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !resp.Allowed {
		t.Errorf("Expected allowed to be true for user role and read action")
	}

	// Test with user role and write action
	req.Action = "write"
	resp, err = evaluator.Evaluate(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp.Allowed {
		t.Errorf("Expected allowed to be false for user role and write action")
	}

	// Test with unknown role
	req.Subject.Roles = []Role{"guest"}
	resp, err = evaluator.Evaluate(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp.Allowed {
		t.Errorf("Expected allowed to be false for unknown role")
	}

	// Test with unknown resource type
	req.Resource.Type = "unknown"
	resp, err = evaluator.Evaluate(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp.Allowed {
		t.Errorf("Expected allowed to be false for unknown resource type")
	}
}
