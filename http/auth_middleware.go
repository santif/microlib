package http

import (
	"context"
	"net/http"
	"time"

	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
)

// AuthConfig contains configuration for authentication middleware
type AuthConfig struct {
	// Enabled determines if authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// JWKS endpoint URL for fetching public keys
	JWKSEndpoint string `json:"jwks_endpoint" yaml:"jwks_endpoint" validate:"required_if=Enabled true"`

	// Issuer is the expected issuer of the token
	Issuer string `json:"issuer" yaml:"issuer"`

	// Audience is the expected audience of the token
	Audience []string `json:"audience" yaml:"audience"`

	// TokenLookup is a string in the form of "<source>:<n>" that is used
	// to extract token from the request.
	// Optional. Default value "header:Authorization".
	TokenLookup string `json:"token_lookup" yaml:"token_lookup"`

	// AuthScheme is a string that is used as the authentication scheme
	// Optional. Default value "Bearer".
	AuthScheme string `json:"auth_scheme" yaml:"auth_scheme"`

	// BypassPaths are paths that should bypass authentication
	BypassPaths []string `json:"bypass_paths" yaml:"bypass_paths"`

	// RefreshInterval is the interval for refreshing the JWKS cache
	RefreshInterval time.Duration `json:"refresh_interval" yaml:"refresh_interval"`

	// RequiredScopes are scopes that are required for all endpoints (unless overridden)
	RequiredScopes []string `json:"required_scopes" yaml:"required_scopes"`

	// ClaimsToContext is a list of claim names that should be extracted and added to the request context
	ClaimsToContext []string `json:"claims_to_context" yaml:"claims_to_context"`
}

// DefaultAuthConfig returns the default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled:         false,
		TokenLookup:     "header:Authorization",
		AuthScheme:      "Bearer",
		BypassPaths:     []string{"/health", "/metrics", "/openapi.json", "/swagger"},
		RefreshInterval: 1 * time.Hour,
		RequiredScopes:  []string{},
		ClaimsToContext: []string{},
	}
}

// AuthMiddleware creates middleware for JWT authentication
func AuthMiddleware(auth security.Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return auth.Middleware()(next)
	}
}

// AuthBypassMiddleware creates middleware that bypasses authentication for specified paths
func AuthBypassMiddleware(auth security.Authenticator, paths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return auth.WithBypass(paths)(next)
	}
}

// WithAuth configures authentication for the server
func WithAuth(enabled bool, jwksEndpoint string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.Enabled = enabled
		config.Auth.JWKSEndpoint = jwksEndpoint
	}
}

// WithAuthIssuer configures the expected issuer for authentication
func WithAuthIssuer(issuer string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.Issuer = issuer
	}
}

// WithAuthAudience configures the expected audience for authentication
func WithAuthAudience(audience []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.Audience = audience
	}
}

// WithAuthBypass configures paths that should bypass authentication
func WithAuthBypass(paths []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.BypassPaths = paths
	}
}

// WithAuthTokenLookup configures how to extract the token from the request
func WithAuthTokenLookup(tokenLookup string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.TokenLookup = tokenLookup
	}
}

// WithAuthScheme configures the authentication scheme
func WithAuthScheme(authScheme string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.AuthScheme = authScheme
	}
}

// WithAuthRefreshInterval configures the JWKS refresh interval
func WithAuthRefreshInterval(interval time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.RefreshInterval = interval
	}
}

// WithRequiredScopes configures the required scopes for all endpoints
func WithRequiredScopes(scopes []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.RequiredScopes = scopes
	}
}

// WithClaimsToContext configures which claims should be extracted to the context
func WithClaimsToContext(claims []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.ClaimsToContext = claims
	}
}

// NewAuthenticator creates a new authenticator from the server configuration
func NewAuthenticator(config AuthConfig, logger observability.Logger) (security.Authenticator, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Create security auth config from HTTP auth config
	securityConfig := security.AuthConfig{
		JWKSEndpoint:    config.JWKSEndpoint,
		Issuer:          config.Issuer,
		Audience:        config.Audience,
		TokenLookup:     config.TokenLookup,
		AuthScheme:      config.AuthScheme,
		BypassPaths:     config.BypassPaths,
		RefreshInterval: config.RefreshInterval,
	}

	// Create the authenticator
	return security.NewJWTAuthenticator(securityConfig, logger)
}

// RequireScopeMiddleware creates middleware that requires a specific scope
func RequireScopeMiddleware(scope string) Middleware {
	return func(next http.Handler) http.Handler {
		return security.RequireScope(scope)(next)
	}
}

// RequireScopesMiddleware creates middleware that requires all specified scopes
func RequireScopesMiddleware(scopes []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := security.GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if the user has all required scopes
			userScopes, ok := claims.Custom["scope"].(string)
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if all required scopes are present
			userScopeMap := security.ParseScopeString(userScopes)
			for _, requiredScope := range scopes {
				if !userScopeMap[requiredScope] {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsContextMiddleware creates middleware that extracts specified claims to the context
func ClaimsContextMiddleware(claimNames []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := security.GetClaimsFromContext(r.Context())
			if !ok {
				// No claims, just continue
				next.ServeHTTP(w, r)
				return
			}

			// Create a new context with the extracted claims
			ctx := r.Context()
			for _, claimName := range claimNames {
				if value, exists := claims.Custom[claimName]; exists {
					ctx = context.WithValue(ctx, security.ClaimContextKey(claimName), value)
				}
			}

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
