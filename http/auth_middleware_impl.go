package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
)

// AuthMiddlewareOptions contains options for the authentication middleware
type AuthMiddlewareOptions struct {
	// BypassPaths are paths that should bypass authentication
	BypassPaths []string

	// RequiredScopes are scopes that are required for all endpoints
	RequiredScopes []string

	// ClaimsToContext is a list of claim names that should be extracted and added to the request context
	ClaimsToContext []string
}

// NewAuthMiddleware creates a new authentication middleware with options
func NewAuthMiddleware(auth security.Authenticator, opts AuthMiddlewareOptions) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the path should bypass authentication
			for _, path := range opts.BypassPaths {
				if r.URL.Path == path || (path != "/" && strings.HasPrefix(r.URL.Path, path)) {
					// Skip authentication for this path
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract token from request
			tokenString, err := extractToken(r, "header:Authorization", "Bearer")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := auth.ValidateToken(r.Context(), tokenString)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), security.ClaimsContextKey, claims)

			// Extract specified claims to context
			for _, claimName := range opts.ClaimsToContext {
				if value, exists := claims.Custom[claimName]; exists {
					ctx = context.WithValue(ctx, security.ClaimContextKey(claimName), value)
				}
			}

			// Check required scopes if any
			if len(opts.RequiredScopes) > 0 {
				// Get scopes from claims
				scopeStr, ok := claims.Custom["scope"].(string)
				if !ok {
					http.Error(w, "Forbidden - Missing required scopes", http.StatusForbidden)
					return
				}

				// Parse scopes
				scopes := security.ParseScopeString(scopeStr)

				// Check if all required scopes are present
				for _, requiredScope := range opts.RequiredScopes {
					if !scopes[requiredScope] {
						http.Error(w, "Forbidden - Missing required scope", http.StatusForbidden)
						return
					}
				}
			}

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken extracts the token from the request based on the tokenLookup configuration
func extractToken(r *http.Request, tokenLookup string, authScheme string) (string, error) {
	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return "", security.ErrNoToken
	}

	source := parts[0]
	name := parts[1]

	switch source {
	case "header":
		// Get token from header
		authHeader := r.Header.Get(name)
		if authHeader == "" {
			return "", security.ErrNoToken
		}

		// If AuthScheme is specified, check that the header starts with it
		if authScheme != "" {
			if !strings.HasPrefix(authHeader, authScheme) {
				return "", security.ErrInvalidToken
			}
			// Remove auth scheme prefix
			return strings.TrimPrefix(authHeader, authScheme+" "), nil
		}

		return authHeader, nil

	case "query":
		// Get token from query parameter
		token := r.URL.Query().Get(name)
		if token == "" {
			return "", security.ErrNoToken
		}
		return token, nil

	case "cookie":
		// Get token from cookie
		cookie, err := r.Cookie(name)
		if err != nil {
			return "", security.ErrNoToken
		}
		return cookie.Value, nil

	default:
		return "", security.ErrNoToken
	}
}

// RegisterAuthMiddleware registers the authentication middleware with the server
func RegisterAuthMiddleware(server Server, logger observability.Logger) error {
	config := server.Config()
	if config.Auth == nil || !config.Auth.Enabled {
		return nil
	}

	// Create the authenticator
	auth, err := NewAuthenticator(*config.Auth, logger)
	if err != nil {
		return err
	}

	// Create middleware options
	opts := AuthMiddlewareOptions{
		BypassPaths:     config.Auth.BypassPaths,
		RequiredScopes:  config.Auth.RequiredScopes,
		ClaimsToContext: config.Auth.ClaimsToContext,
	}

	// Register the middleware
	server.RegisterMiddleware(NewAuthMiddleware(auth, opts))

	// Log registration
	if logger != nil {
		logger.Info("Authentication middleware registered",
			observability.NewField("jwks_endpoint", config.Auth.JWKSEndpoint),
			observability.NewField("bypass_paths", config.Auth.BypassPaths),
			observability.NewField("required_scopes", config.Auth.RequiredScopes),
		)
	}

	return nil
}

// RequireScope returns a middleware that requires a specific scope
func RequireScope(scope string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := security.GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if the user has the required scope
			scopeStr, ok := claims.Custom["scope"].(string)
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Parse scopes
			scopes := security.ParseScopeString(scopeStr)

			// Check if the required scope is present
			if !scopes[scope] {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequireScopes returns a middleware that requires all specified scopes
func RequireScopes(requiredScopes []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := security.GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if the user has all required scopes
			scopeStr, ok := claims.Custom["scope"].(string)
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Parse scopes
			scopes := security.ParseScopeString(scopeStr)

			// Check if all required scopes are present
			for _, requiredScope := range requiredScopes {
				if !scopes[requiredScope] {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ExtractClaimsToContext returns a middleware that extracts specified claims to the context
func ExtractClaimsToContext(claimNames []string) Middleware {
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
