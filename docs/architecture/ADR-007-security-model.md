# ADR-007: Security Model and Authentication

## Status

Accepted

## Context

Microservices require robust authentication and authorization mechanisms. The security model must support modern standards like JWT/OIDC while being flexible enough to integrate with different identity providers and authorization systems.

Key requirements:
- JWT token validation with OIDC support
- JWKS endpoint integration for key rotation
- Pluggable authorization system
- Context-based security information
- Support for service-to-service authentication
- Integration with popular identity providers

## Decision

We will implement a layered security model with the following components:

### 1. Authentication Layer
JWT-based authentication with OIDC support:

```go
type Authenticator interface {
    ValidateToken(ctx context.Context, token string) (*Claims, error)
    Middleware() Middleware
}

type Claims struct {
    Subject   string                 `json:"sub"`
    Audience  []string              `json:"aud"`
    ExpiresAt time.Time             `json:"exp"`
    IssuedAt  time.Time             `json:"iat"`
    Issuer    string                `json:"iss"`
    Custom    map[string]interface{} `json:"-"`
}
```

### 2. JWKS Integration
Automatic public key retrieval and caching:

```go
type JWKSClient interface {
    GetKey(ctx context.Context, keyID string) (*rsa.PublicKey, error)
    RefreshKeys(ctx context.Context) error
}
```

### 3. Authorization Layer
Pluggable authorization system with OPA support:

```go
type Authorizer interface {
    Authorize(ctx context.Context, subject, action, resource string) error
}

type AuthorizationContext struct {
    Subject  string
    Action   string
    Resource string
    Claims   *Claims
    Request  *http.Request
}
```

### 4. Context Integration
Security information injected into request context:

```go
type SecurityContext struct {
    Claims      *Claims
    Permissions []string
    Roles       []string
}

func FromContext(ctx context.Context) *SecurityContext
func WithContext(ctx context.Context, sec *SecurityContext) context.Context
```

### 5. Middleware Integration
Automatic authentication and authorization for HTTP/gRPC:

```go
// HTTP Middleware
func (a *JWTAuthenticator) Middleware() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            claims, err := a.ValidateToken(r.Context(), token)
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            ctx := WithContext(r.Context(), &SecurityContext{Claims: claims})
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// gRPC Interceptor
func (a *JWTAuthenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        token := extractTokenFromMetadata(ctx)
        claims, err := a.ValidateToken(ctx, token)
        if err != nil {
            return nil, status.Errorf(codes.Unauthenticated, "invalid token")
        }
        
        ctx = WithContext(ctx, &SecurityContext{Claims: claims})
        return handler(ctx, req)
    }
}
```

### 6. Configuration
Flexible security configuration:

```go
type SecurityConfig struct {
    JWT JWTConfig `yaml:"jwt"`
    OPA OPAConfig `yaml:"opa"`
}

type JWTConfig struct {
    Enabled       bool     `yaml:"enabled"`
    JWKSEndpoint  string   `yaml:"jwks_endpoint" validate:"required_if=Enabled true,url"`
    Audience      string   `yaml:"audience" validate:"required_if=Enabled true"`
    Issuer        string   `yaml:"issuer"`
    SkipPaths     []string `yaml:"skip_paths"`
}
```

## Consequences

### Positive
- Industry-standard JWT/OIDC authentication
- Automatic key rotation through JWKS
- Pluggable authorization system
- Context-based security information
- Consistent security across HTTP and gRPC
- Easy integration with identity providers
- Configurable security policies

### Negative
- Dependency on external identity providers
- Potential performance impact from token validation
- Complexity in multi-tenant scenarios
- Key rotation requires network calls
- Learning curve for authorization policies

## Alternatives Considered

### 1. API Key Authentication
Use simple API keys instead of JWT tokens.
- **Rejected**: Less secure, no standard claims, difficult key rotation

### 2. Session-Based Authentication
Use traditional session cookies.
- **Rejected**: Not suitable for stateless microservices, scaling issues

### 3. mTLS Only
Use mutual TLS for all authentication.
- **Rejected**: Complex certificate management, doesn't provide user context

### 4. Custom Token Format
Create a custom token format instead of JWT.
- **Rejected**: Reinventing standards, no ecosystem support

### 5. Built-in Authorization
Implement authorization logic within the framework.
- **Rejected**: Too opinionated, doesn't support complex authorization requirements

### 6. OAuth 2.0 Client Credentials
Use OAuth 2.0 client credentials flow for service-to-service.
- **Considered for future**: Good for service-to-service, but JWT covers most use cases