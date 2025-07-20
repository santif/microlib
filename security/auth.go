package security

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/santif/microlib/observability"
)

// Common errors
var (
	ErrNoToken           = errors.New("no token provided")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrInvalidSignature  = errors.New("invalid token signature")
	ErrInvalidClaims     = errors.New("invalid token claims")
	ErrInvalidAudience   = errors.New("invalid token audience")
	ErrInvalidIssuer     = errors.New("invalid token issuer")
	ErrJWKSFetchFailed   = errors.New("failed to fetch JWKS")
	ErrKeyNotFound       = errors.New("signing key not found")
	ErrUnsupportedMethod = errors.New("unsupported signing method")
)

// contextKey is a type for context keys
type contextKey string

// Context keys
const (
	// ClaimsContextKey is the key used to store claims in the context
	ClaimsContextKey contextKey = "auth_claims"
)

// ClaimContextKey returns a context key for a specific claim
func ClaimContextKey(claim string) contextKey {
	return contextKey("auth_claim_" + claim)
}

// ParseScopeString parses a space-separated scope string into a map
func ParseScopeString(scope string) map[string]bool {
	scopes := make(map[string]bool)
	for _, s := range strings.Split(scope, " ") {
		if s != "" {
			scopes[s] = true
		}
	}
	return scopes
}

// Claims represents the standard JWT claims plus custom claims
type Claims struct {
	// Standard claims
	Subject   string    `json:"sub"`
	Issuer    string    `json:"iss,omitempty"`
	Audience  []string  `json:"aud,omitempty"`
	ExpiresAt time.Time `json:"exp,omitempty"`
	NotBefore time.Time `json:"nbf,omitempty"`
	IssuedAt  time.Time `json:"iat,omitempty"`
	ID        string    `json:"jti,omitempty"`

	// Custom claims as a map
	Custom map[string]interface{} `json:"-"`
}

// Valid validates the claims according to the JWT spec
func (c Claims) Valid() error {
	now := time.Now()

	// Check expiration
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return ErrTokenExpired
	}

	// Check not before
	if !c.NotBefore.IsZero() && now.Before(c.NotBefore) {
		return ErrInvalidToken
	}

	return nil
}

// Authenticator is the interface for JWT authentication
type Authenticator interface {
	// ValidateToken validates a JWT token and returns the claims
	ValidateToken(ctx context.Context, token string) (*Claims, error)

	// Middleware returns an HTTP middleware for JWT authentication
	Middleware() func(http.Handler) http.Handler

	// WithBypass returns a middleware that bypasses authentication for specified paths
	WithBypass(paths []string) func(http.Handler) http.Handler
}

// AuthConfig contains configuration for JWT authentication
type AuthConfig struct {
	// JWKS endpoint URL for fetching public keys
	JWKSEndpoint string `json:"jwks_endpoint" yaml:"jwks_endpoint" validate:"required"`

	// Issuer is the expected issuer of the token
	Issuer string `json:"issuer" yaml:"issuer"`

	// Audience is the expected audience of the token
	Audience []string `json:"audience" yaml:"audience"`

	// RefreshInterval is the interval for refreshing the JWKS cache
	RefreshInterval time.Duration `json:"refresh_interval" yaml:"refresh_interval"`

	// TokenLookup is a string in the form of "<source>:<n>" that is used
	// to extract token from the request.
	// Optional. Default value "header:Authorization".
	// Possible values:
	// - "header:<n>"
	// - "query:<n>"
	// - "cookie:<n>"
	TokenLookup string `json:"token_lookup" yaml:"token_lookup"`

	// AuthScheme is a string that is used as the authentication scheme
	// Optional. Default value "Bearer".
	AuthScheme string `json:"auth_scheme" yaml:"auth_scheme"`

	// BypassPaths are paths that should bypass authentication
	BypassPaths []string `json:"bypass_paths" yaml:"bypass_paths"`
}

// DefaultAuthConfig returns the default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		RefreshInterval: 1 * time.Hour,
		TokenLookup:     "header:Authorization",
		AuthScheme:      "Bearer",
		BypassPaths:     []string{"/health", "/metrics"},
	}
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
	Alg string   `json:"alg"`
	X5t string   `json:"x5t,omitempty"`
	X5u string   `json:"x5u,omitempty"`
}

// JWTAuthenticator implements the Authenticator interface for JWT tokens
type JWTAuthenticator struct {
	config      AuthConfig
	logger      observability.Logger
	keys        map[string]interface{}
	mu          sync.RWMutex
	httpClient  *http.Client
	lastRefresh time.Time
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config AuthConfig, logger observability.Logger) (*JWTAuthenticator, error) {
	if config.JWKSEndpoint == "" {
		return nil, errors.New("JWKS endpoint is required")
	}

	auth := &JWTAuthenticator{
		config: config,
		logger: logger,
		keys:   make(map[string]interface{}),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Fetch keys initially
	if err := auth.refreshKeys(); err != nil {
		return nil, fmt.Errorf("failed to fetch initial JWKS: %w", err)
	}

	// Start background refresh if interval is set
	if config.RefreshInterval > 0 {
		go auth.startKeyRefresher()
	}

	return auth, nil
}

// startKeyRefresher starts a background goroutine to refresh the JWKS keys
func (a *JWTAuthenticator) startKeyRefresher() {
	ticker := time.NewTicker(a.config.RefreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := a.refreshKeys(); err != nil {
			a.logger.Error("Failed to refresh JWKS keys", err)
		}
	}
}

// refreshKeys fetches the latest keys from the JWKS endpoint
func (a *JWTAuthenticator) refreshKeys() error {
	a.logger.Info("Refreshing JWKS keys",
		observability.NewField("endpoint", a.config.JWKSEndpoint))

	// Make HTTP request to JWKS endpoint
	resp, err := a.httpClient.Get(a.config.JWKSEndpoint)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status code %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: failed to read response body: %v", ErrJWKSFetchFailed, err)
	}

	// Parse JWKS
	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("%w: failed to parse JWKS: %v", ErrJWKSFetchFailed, err)
	}

	// Process keys
	newKeys := make(map[string]interface{})
	for _, key := range jwks.Keys {
		if key.Use == "sig" {
			var publicKey interface{}
			var err error

			switch key.Kty {
			case "RSA":
				if len(key.X5c) > 0 {
					// Convert X5c (X.509 certificate chain) to PEM format
					certData := "-----BEGIN CERTIFICATE-----\n" + key.X5c[0] + "\n-----END CERTIFICATE-----"
					publicKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(certData))
				} else if key.N != "" && key.E != "" {
					// Parse from modulus and exponent
					publicKey, err = parseRSAPublicKeyFromJWK(key.N, key.E)
				} else {
					a.logger.Error("RSA key missing required parameters", nil,
						observability.NewField("kid", key.Kid))
					continue
				}
			case "EC":
				// Support for EC keys could be added here
				a.logger.Error("EC keys not yet supported", nil,
					observability.NewField("kid", key.Kid))
				continue
			default:
				a.logger.Error("Unsupported key type", nil,
					observability.NewField("kid", key.Kid),
					observability.NewField("kty", key.Kty))
				continue
			}

			if err != nil {
				a.logger.Error("Failed to parse public key", err,
					observability.NewField("kid", key.Kid),
					observability.NewField("kty", key.Kty))
				continue
			}

			newKeys[key.Kid] = publicKey
		}
	}

	// Update keys
	a.mu.Lock()
	a.keys = newKeys
	a.lastRefresh = time.Now()
	a.mu.Unlock()

	a.logger.Info("JWKS keys refreshed successfully",
		observability.NewField("key_count", len(newKeys)))

	return nil
}

// getKey returns the key for the given key ID
func (a *JWTAuthenticator) getKey(kid string) (interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	key, ok := a.keys[kid]
	if !ok {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

// ValidateToken validates a JWT token and returns the claims
func (a *JWTAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, ErrUnsupportedMethod
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, ErrKeyNotFound
		}

		// Get the key for this key ID
		return a.getKey(kid)
	})

	// Handle parsing errors
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			// Try to refresh keys and retry
			if refreshErr := a.refreshKeys(); refreshErr != nil {
				return nil, fmt.Errorf("failed to refresh keys: %w", refreshErr)
			}

			// Retry parsing with new keys
			token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Verify the signing method
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, ErrUnsupportedMethod
				}

				// Get the key ID from the token header
				kid, ok := token.Header["kid"].(string)
				if !ok {
					return nil, ErrKeyNotFound
				}

				// Get the key for this key ID
				return a.getKey(kid)
			})

			if err != nil {
				return nil, fmt.Errorf("token validation failed after key refresh: %w", err)
			}
		} else {
			return nil, fmt.Errorf("token validation failed: %w", err)
		}
	}

	// Verify the token is valid
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Extract claims
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	// Create our custom claims
	claims := &Claims{
		Custom: make(map[string]interface{}),
	}

	// Extract standard claims
	if sub, ok := mapClaims["sub"].(string); ok {
		claims.Subject = sub
	}

	if iss, ok := mapClaims["iss"].(string); ok {
		claims.Issuer = iss
	}

	if aud, ok := mapClaims["aud"].([]interface{}); ok {
		claims.Audience = make([]string, len(aud))
		for i, a := range aud {
			if str, ok := a.(string); ok {
				claims.Audience[i] = str
			}
		}
	} else if aud, ok := mapClaims["aud"].(string); ok {
		claims.Audience = []string{aud}
	}

	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.ExpiresAt = time.Unix(int64(exp), 0)
	}

	if nbf, ok := mapClaims["nbf"].(float64); ok {
		claims.NotBefore = time.Unix(int64(nbf), 0)
	}

	if iat, ok := mapClaims["iat"].(float64); ok {
		claims.IssuedAt = time.Unix(int64(iat), 0)
	}

	if jti, ok := mapClaims["jti"].(string); ok {
		claims.ID = jti
	}

	// Extract custom claims
	for k, v := range mapClaims {
		switch k {
		case "sub", "iss", "aud", "exp", "nbf", "iat", "jti":
			// Skip standard claims
			continue
		default:
			claims.Custom[k] = v
		}
	}

	// Validate issuer if configured
	if a.config.Issuer != "" && claims.Issuer != a.config.Issuer {
		return nil, ErrInvalidIssuer
	}

	// Validate audience if configured
	if len(a.config.Audience) > 0 {
		valid := false
		for _, aud := range claims.Audience {
			for _, expectedAud := range a.config.Audience {
				if aud == expectedAud {
					valid = true
					break
				}
			}
			if valid {
				break
			}
		}
		if !valid {
			return nil, ErrInvalidAudience
		}
	}

	return claims, nil
}

// extractToken extracts the token from the request based on the TokenLookup configuration
func (a *JWTAuthenticator) extractToken(r *http.Request) (string, error) {
	parts := strings.Split(a.config.TokenLookup, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token lookup format: %s", a.config.TokenLookup)
	}

	source := parts[0]
	name := parts[1]

	switch source {
	case "header":
		// Get token from header
		authHeader := r.Header.Get(name)
		if authHeader == "" {
			return "", ErrNoToken
		}

		// If AuthScheme is specified, check that the header starts with it
		if a.config.AuthScheme != "" {
			if !strings.HasPrefix(authHeader, a.config.AuthScheme) {
				return "", ErrInvalidToken
			}
			// Remove auth scheme prefix
			return strings.TrimPrefix(authHeader, a.config.AuthScheme+" "), nil
		}

		return authHeader, nil

	case "query":
		// Get token from query parameter
		token := r.URL.Query().Get(name)
		if token == "" {
			return "", ErrNoToken
		}
		return token, nil

	case "cookie":
		// Get token from cookie
		cookie, err := r.Cookie(name)
		if err != nil {
			return "", ErrNoToken
		}
		return cookie.Value, nil

	default:
		return "", fmt.Errorf("unsupported token source: %s", source)
	}
}

// Middleware returns an HTTP middleware for JWT authentication
func (a *JWTAuthenticator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from request
			tokenString, err := a.extractToken(r)
			if err != nil {
				a.logger.ErrorContext(r.Context(), "Authentication failed", err,
					observability.NewField("path", r.URL.Path),
					observability.NewField("method", r.Method),
				)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := a.ValidateToken(r.Context(), tokenString)
			if err != nil {
				a.logger.ErrorContext(r.Context(), "Token validation failed", err,
					observability.NewField("path", r.URL.Path),
					observability.NewField("method", r.Method),
				)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithBypass returns a middleware that bypasses authentication for specified paths
func (a *JWTAuthenticator) WithBypass(paths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the path should bypass authentication
			for _, path := range paths {
				if strings.HasPrefix(r.URL.Path, path) {
					// Skip authentication for this path
					next.ServeHTTP(w, r)
					return
				}
			}

			// Apply the authentication middleware
			a.Middleware()(next).ServeHTTP(w, r)
		})
	}
}

// GetClaimsFromContext extracts the claims from the context
func GetClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	return claims, ok
}

// decodeBase64URLSegment decodes a base64url encoded string
func decodeBase64URLSegment(seg string) ([]byte, error) {
	// Add padding if necessary
	switch len(seg) % 4 {
	case 2:
		seg += "=="
	case 3:
		seg += "="
	}

	// Replace URL encoding with standard base64 encoding
	seg = strings.ReplaceAll(seg, "-", "+")
	seg = strings.ReplaceAll(seg, "_", "/")

	return base64.StdEncoding.DecodeString(seg)
}

// parseRSAPublicKeyFromJWK parses an RSA public key from JWK parameters
func parseRSAPublicKeyFromJWK(nStr, eStr string) (interface{}, error) {
	// Decode the base64 URL encoded modulus and exponent
	n, err := decodeBase64URLSegment(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	e, err := decodeBase64URLSegment(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert the exponent to an integer
	var exponent int
	for i := 0; i < len(e); i++ {
		exponent = exponent<<8 + int(e[i])
	}

	// Create the RSA public key
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(n),
		E: exponent,
	}, nil
}

// RequireScope returns a middleware that requires a specific scope
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if the user has the required scope
			scopes, ok := claims.Custom["scope"].(string)
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if the scope is in the list
			scopeList := strings.Split(scopes, " ")
			hasScope := false
			for _, s := range scopeList {
				if s == scope {
					hasScope = true
					break
				}
			}

			if !hasScope {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}
