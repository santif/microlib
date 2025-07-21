package security

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/santif/microlib/observability"
)

// Common authorization errors
var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrPolicyNotFound    = errors.New("policy not found")
	ErrInvalidPolicy     = errors.New("invalid policy")
	ErrInvalidRole       = errors.New("invalid role")
	ErrInvalidPermission = errors.New("invalid permission")
)

// Authorization context keys
const (
	// RolesContextKey is the key used to store roles in the context
	RolesContextKey contextKey = "auth_roles"

	// PermissionsContextKey is the key used to store permissions in the context
	PermissionsContextKey contextKey = "auth_permissions"
)

// AuthorizationConfig contains configuration for authorization
type AuthorizationConfig struct {
	// Enabled determines if authorization is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// DefaultPolicy is the default policy to use if no policy is specified
	DefaultPolicy string `json:"default_policy" yaml:"default_policy"`

	// OPAEndpoint is the URL of the OPA server for external policy evaluation
	OPAEndpoint string `json:"opa_endpoint" yaml:"opa_endpoint"`

	// PolicyPath is the path to the policy files
	PolicyPath string `json:"policy_path" yaml:"policy_path"`

	// RoleMappingClaim is the JWT claim that contains role information
	RoleMappingClaim string `json:"role_mapping_claim" yaml:"role_mapping_claim"`

	// PermissionMappingClaim is the JWT claim that contains permission information
	PermissionMappingClaim string `json:"permission_mapping_claim" yaml:"permission_mapping_claim"`
}

// DefaultAuthorizationConfig returns the default authorization configuration
func DefaultAuthorizationConfig() AuthorizationConfig {
	return AuthorizationConfig{
		Enabled:                false,
		DefaultPolicy:          "deny",
		RoleMappingClaim:       "roles",
		PermissionMappingClaim: "permissions",
	}
}

// Permission represents a permission in the system
type Permission string

// Role represents a role in the system
type Role string

// Subject represents an entity that can be authorized
type Subject struct {
	// ID is the unique identifier of the subject
	ID string

	// Roles are the roles assigned to the subject
	Roles []Role

	// Permissions are the permissions assigned to the subject
	Permissions []Permission

	// Attributes are additional attributes of the subject
	Attributes map[string]interface{}
}

// Resource represents a resource that can be accessed
type Resource struct {
	// Type is the type of the resource
	Type string

	// ID is the unique identifier of the resource
	ID string

	// Attributes are additional attributes of the resource
	Attributes map[string]interface{}
}

// Action represents an action that can be performed on a resource
type Action string

// AuthorizationRequest represents a request for authorization
type AuthorizationRequest struct {
	// Subject is the entity requesting access
	Subject Subject

	// Resource is the resource being accessed
	Resource Resource

	// Action is the action being performed
	Action Action

	// Context contains additional context for the authorization decision
	Context map[string]interface{}
}

// AuthorizationResponse represents the response to an authorization request
type AuthorizationResponse struct {
	// Allowed indicates if the request is allowed
	Allowed bool

	// Reason is the reason for the decision
	Reason string

	// Obligations are additional requirements for the authorization
	Obligations map[string]interface{}
}

// Authorizer is the interface for authorization
type Authorizer interface {
	// Authorize determines if a subject can perform an action on a resource
	Authorize(ctx context.Context, req AuthorizationRequest) (*AuthorizationResponse, error)

	// HasRole determines if a subject has a specific role
	HasRole(ctx context.Context, role Role) (bool, error)

	// HasPermission determines if a subject has a specific permission
	HasPermission(ctx context.Context, permission Permission) (bool, error)

	// GetRoles returns the roles of the subject from the context
	GetRoles(ctx context.Context) ([]Role, bool)

	// GetPermissions returns the permissions of the subject from the context
	GetPermissions(ctx context.Context) ([]Permission, bool)
}

// PolicyEvaluator is the interface for policy evaluation
type PolicyEvaluator interface {
	// Evaluate evaluates a policy against an authorization request
	Evaluate(ctx context.Context, req AuthorizationRequest) (*AuthorizationResponse, error)
}

// RBACAuthorizer implements the Authorizer interface using RBAC
type RBACAuthorizer struct {
	config    AuthorizationConfig
	logger    observability.Logger
	evaluator PolicyEvaluator
	roleMap   map[Role][]Permission
	roleMutex sync.RWMutex
}

// NewRBACAuthorizer creates a new RBAC authorizer
func NewRBACAuthorizer(config AuthorizationConfig, logger observability.Logger) (*RBACAuthorizer, error) {
	var evaluator PolicyEvaluator

	// Create policy evaluator based on configuration
	if config.OPAEndpoint != "" {
		evaluator = NewOPAEvaluator(config.OPAEndpoint, logger)
	} else {
		evaluator = NewLocalPolicyEvaluator(config.PolicyPath, logger)
	}

	return &RBACAuthorizer{
		config:    config,
		logger:    logger,
		evaluator: evaluator,
		roleMap:   make(map[Role][]Permission),
	}, nil
}

// Authorize determines if a subject can perform an action on a resource
func (a *RBACAuthorizer) Authorize(ctx context.Context, req AuthorizationRequest) (*AuthorizationResponse, error) {
	// Log the authorization request
	a.logger.InfoContext(ctx, "Authorization request",
		observability.NewField("subject", req.Subject.ID),
		observability.NewField("resource", fmt.Sprintf("%s:%s", req.Resource.Type, req.Resource.ID)),
		observability.NewField("action", string(req.Action)),
	)

	// Evaluate the policy
	resp, err := a.evaluator.Evaluate(ctx, req)
	if err != nil {
		a.logger.ErrorContext(ctx, "Policy evaluation failed", err,
			observability.NewField("subject", req.Subject.ID),
			observability.NewField("resource", fmt.Sprintf("%s:%s", req.Resource.Type, req.Resource.ID)),
			observability.NewField("action", string(req.Action)),
		)
		return nil, err
	}

	// Log the authorization response
	a.logger.InfoContext(ctx, "Authorization response",
		observability.NewField("allowed", resp.Allowed),
		observability.NewField("reason", resp.Reason),
	)

	return resp, nil
}

// HasRole determines if a subject has a specific role
func (a *RBACAuthorizer) HasRole(ctx context.Context, role Role) (bool, error) {
	// Get roles from context
	roles, ok := a.GetRoles(ctx)
	if !ok {
		return false, ErrUnauthorized
	}

	// Check if the role is in the list
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}

// HasPermission determines if a subject has a specific permission
func (a *RBACAuthorizer) HasPermission(ctx context.Context, permission Permission) (bool, error) {
	// First check direct permissions
	permissions, ok := a.GetPermissions(ctx)
	if ok {
		for _, p := range permissions {
			if p == permission {
				return true, nil
			}
		}
	}

	// Then check role-based permissions
	roles, ok := a.GetRoles(ctx)
	if !ok {
		return false, ErrUnauthorized
	}

	a.roleMutex.RLock()
	defer a.roleMutex.RUnlock()

	for _, role := range roles {
		perms, exists := a.roleMap[role]
		if !exists {
			continue
		}

		for _, p := range perms {
			if p == permission {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetRoles returns the roles of the subject from the context
func (a *RBACAuthorizer) GetRoles(ctx context.Context) ([]Role, bool) {
	// Get roles from context
	rolesVal := ctx.Value(RolesContextKey)
	if rolesVal == nil {
		return nil, false
	}

	roles, ok := rolesVal.([]Role)
	return roles, ok
}

// GetPermissions returns the permissions of the subject from the context
func (a *RBACAuthorizer) GetPermissions(ctx context.Context) ([]Permission, bool) {
	// Get permissions from context
	permsVal := ctx.Value(PermissionsContextKey)
	if permsVal == nil {
		return nil, false
	}

	perms, ok := permsVal.([]Permission)
	return perms, ok
}

// AddRolePermissions adds permissions to a role
func (a *RBACAuthorizer) AddRolePermissions(role Role, permissions []Permission) {
	a.roleMutex.Lock()
	defer a.roleMutex.Unlock()

	// Get existing permissions or create new slice
	existingPerms, exists := a.roleMap[role]
	if !exists {
		existingPerms = []Permission{}
	}

	// Add new permissions
	for _, p := range permissions {
		// Check if permission already exists
		exists := false
		for _, ep := range existingPerms {
			if ep == p {
				exists = true
				break
			}
		}

		// Add if not exists
		if !exists {
			existingPerms = append(existingPerms, p)
		}
	}

	// Update role map
	a.roleMap[role] = existingPerms
}

// RemoveRolePermissions removes permissions from a role
func (a *RBACAuthorizer) RemoveRolePermissions(role Role, permissions []Permission) {
	a.roleMutex.Lock()
	defer a.roleMutex.Unlock()

	// Get existing permissions
	existingPerms, exists := a.roleMap[role]
	if !exists {
		return
	}

	// Create a map for quick lookup
	removeMap := make(map[Permission]bool)
	for _, p := range permissions {
		removeMap[p] = true
	}

	// Filter permissions
	newPerms := []Permission{}
	for _, p := range existingPerms {
		if !removeMap[p] {
			newPerms = append(newPerms, p)
		}
	}

	// Update role map
	a.roleMap[role] = newPerms
}

// GetRolePermissions returns the permissions for a role
func (a *RBACAuthorizer) GetRolePermissions(role Role) []Permission {
	a.roleMutex.RLock()
	defer a.roleMutex.RUnlock()

	perms, exists := a.roleMap[role]
	if !exists {
		return []Permission{}
	}

	// Return a copy to prevent modification
	result := make([]Permission, len(perms))
	copy(result, perms)
	return result
}

// ExtractRolesAndPermissions extracts roles and permissions from JWT claims
func ExtractRolesAndPermissions(ctx context.Context, roleClaim, permissionClaim string) ([]Role, []Permission) {
	// Get claims from context
	claims, ok := GetClaimsFromContext(ctx)
	if !ok {
		return nil, nil
	}

	// Extract roles
	var roles []Role
	if roleClaim != "" {
		if rolesVal, exists := claims.Custom[roleClaim]; exists {
			switch v := rolesVal.(type) {
			case []interface{}:
				// Handle array of roles
				for _, r := range v {
					if rStr, ok := r.(string); ok {
						roles = append(roles, Role(rStr))
					}
				}
			case []string:
				// Handle array of string roles
				for _, r := range v {
					roles = append(roles, Role(r))
				}
			case string:
				// Handle space-separated roles
				for _, r := range strings.Split(v, " ") {
					if r != "" {
						roles = append(roles, Role(r))
					}
				}
			}
		}
	}

	// Extract permissions
	var permissions []Permission
	if permissionClaim != "" {
		if permsVal, exists := claims.Custom[permissionClaim]; exists {
			switch v := permsVal.(type) {
			case []interface{}:
				// Handle array of permissions
				for _, p := range v {
					if pStr, ok := p.(string); ok {
						permissions = append(permissions, Permission(pStr))
					}
				}
			case []string:
				// Handle array of string permissions
				for _, p := range v {
					permissions = append(permissions, Permission(p))
				}
			case string:
				// Handle space-separated permissions
				for _, p := range strings.Split(v, " ") {
					if p != "" {
						permissions = append(permissions, Permission(p))
					}
				}
			}
		}
	}

	return roles, permissions
}

// AuthorizationMiddleware creates middleware for authorization
func AuthorizationMiddleware(authorizer Authorizer, resourceType string, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Create subject
			subject := Subject{
				ID:         claims.Subject,
				Attributes: claims.Custom,
			}

			// Get roles and permissions from context
			if roles, ok := authorizer.GetRoles(r.Context()); ok {
				subject.Roles = roles
			}
			if permissions, ok := authorizer.GetPermissions(r.Context()); ok {
				subject.Permissions = permissions
			}

			// Create resource
			resource := Resource{
				Type: resourceType,
				ID:   r.URL.Path,
				Attributes: map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
				},
			}

			// Create authorization request
			req := AuthorizationRequest{
				Subject:  subject,
				Resource: resource,
				Action:   action,
				Context: map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
				},
			}

			// Authorize the request
			resp, err := authorizer.Authorize(r.Context(), req)
			if err != nil {
				http.Error(w, "Authorization error", http.StatusInternalServerError)
				return
			}

			if !resp.Allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoleMiddleware creates middleware that requires a specific role
func RequireRoleMiddleware(authorizer Authorizer, role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the user has the required role
			hasRole, err := authorizer.HasRole(r.Context(), role)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !hasRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermissionMiddleware creates middleware that requires a specific permission
func RequirePermissionMiddleware(authorizer Authorizer, permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the user has the required permission
			hasPermission, err := authorizer.HasPermission(r.Context(), permission)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !hasPermission {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RolesAndPermissionsMiddleware creates middleware that extracts roles and permissions from JWT claims
func RolesAndPermissionsMiddleware(roleClaim, permissionClaim string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract roles and permissions
			roles, permissions := ExtractRolesAndPermissions(r.Context(), roleClaim, permissionClaim)

			// Add to context
			ctx := r.Context()
			if len(roles) > 0 {
				ctx = context.WithValue(ctx, RolesContextKey, roles)
			}
			if len(permissions) > 0 {
				ctx = context.WithValue(ctx, PermissionsContextKey, permissions)
			}

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
