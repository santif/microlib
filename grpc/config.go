package grpc

import (
	"time"

	"github.com/santif/microlib/security"
)

// ServerConfig contains configuration for the gRPC server
type ServerConfig struct {
	// Address is the address to listen on
	Address string `json:"address" yaml:"address" validate:"required"`

	// Host is an alias for Address for backward compatibility
	Host string `json:"host" yaml:"host"`

	// Port is the port to listen on
	Port int `json:"port" yaml:"port" validate:"required"`

	// TLS configuration
	TLS *TLSConfig `json:"tls" yaml:"tls"`

	// Auth configuration
	Auth *AuthConfig `json:"auth" yaml:"auth"`

	// MaxConnectionAge is the maximum duration a connection may exist before it is closed
	MaxConnectionAge time.Duration `json:"max_connection_age" yaml:"max_connection_age"`

	// MaxConnectionAgeGrace is the grace period for which connections may stay open after MaxConnectionAge is reached
	MaxConnectionAgeGrace time.Duration `json:"max_connection_age_grace" yaml:"max_connection_age_grace"`

	// MaxConcurrentStreams is the maximum number of concurrent streams to each client
	MaxConcurrentStreams uint32 `json:"max_concurrent_streams" yaml:"max_concurrent_streams"`

	// InitialWindowSize is the initial window size for a stream
	InitialWindowSize int32 `json:"initial_window_size" yaml:"initial_window_size"`

	// InitialConnWindowSize is the initial window size for a connection
	InitialConnWindowSize int32 `json:"initial_conn_window_size" yaml:"initial_conn_window_size"`

	// KeepAlive configuration
	KeepAlive *KeepAliveConfig `json:"keep_alive" yaml:"keep_alive"`

	// EnableReflection enables gRPC reflection
	EnableReflection bool `json:"enable_reflection" yaml:"enable_reflection"`

	// EnableChannelz enables gRPC channelz
	EnableChannelz bool `json:"enable_channelz" yaml:"enable_channelz"`

	// ShutdownTimeout is the timeout for graceful shutdown
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
}

// TLSConfig contains TLS configuration for the gRPC server
type TLSConfig struct {
	// Enabled determines if TLS is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// CertFile is the path to the certificate file
	CertFile string `json:"cert_file" yaml:"cert_file" validate:"required_if=Enabled true"`

	// KeyFile is the path to the key file
	KeyFile string `json:"key_file" yaml:"key_file" validate:"required_if=Enabled true"`

	// ClientAuth determines if client authentication is required
	ClientAuth bool `json:"client_auth" yaml:"client_auth"`

	// ClientCAs is the path to the client CA file
	ClientCAs string `json:"client_cas" yaml:"client_cas" validate:"required_if=ClientAuth true"`
}

// KeepAliveConfig contains keep-alive configuration for the gRPC server
type KeepAliveConfig struct {
	// MaxConnectionIdle is the maximum amount of time a connection may be idle before it is closed
	MaxConnectionIdle time.Duration `json:"max_connection_idle" yaml:"max_connection_idle"`

	// MaxConnectionAge is the maximum amount of time a connection may exist before it is closed
	MaxConnectionAge time.Duration `json:"max_connection_age" yaml:"max_connection_age"`

	// MaxConnectionAgeGrace is the grace period for which connections may stay open after MaxConnectionAge is reached
	MaxConnectionAgeGrace time.Duration `json:"max_connection_age_grace" yaml:"max_connection_age_grace"`

	// Time is the duration between keep-alive pings
	Time time.Duration `json:"time" yaml:"time"`

	// Timeout is the duration the server waits for a response to a keep-alive ping
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// PermitWithoutStream determines if the server sends keep-alive pings even when there are no active streams
	PermitWithoutStream bool `json:"permit_without_stream" yaml:"permit_without_stream"`
}

// AuthConfig contains configuration for authentication
type AuthConfig struct {
	// Enabled determines if authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// JWKS endpoint URL for fetching public keys
	JWKSEndpoint string `json:"jwks_endpoint" yaml:"jwks_endpoint" validate:"required_if=Enabled true"`

	// Issuer is the expected issuer of the token
	Issuer string `json:"issuer" yaml:"issuer"`

	// Audience is the expected audience of the token
	Audience []string `json:"audience" yaml:"audience"`

	// BypassPaths are paths that should bypass authentication
	BypassPaths []string `json:"bypass_paths" yaml:"bypass_paths"`

	// RefreshInterval is the interval for refreshing the JWKS cache
	RefreshInterval time.Duration `json:"refresh_interval" yaml:"refresh_interval"`

	// RequiredScopes are scopes that are required for all endpoints (unless overridden)
	RequiredScopes []string `json:"required_scopes" yaml:"required_scopes"`
}

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Address:               "0.0.0.0",
		Host:                  "0.0.0.0", // For backward compatibility
		Port:                  9090,      // Changed to match test expectations
		MaxConnectionAge:      30 * time.Minute,
		MaxConnectionAgeGrace: 5 * time.Second,
		MaxConcurrentStreams:  100,
		InitialWindowSize:     65536,
		InitialConnWindowSize: 65536,
		EnableReflection:      true,
		EnableChannelz:        false,
		KeepAlive: &KeepAliveConfig{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               20 * time.Second,
			PermitWithoutStream:   true,
		},
		Auth: &AuthConfig{
			Enabled:         false,
			RefreshInterval: 1 * time.Hour,
			BypassPaths:     []string{"/grpc.health.v1.Health/", "/grpc.reflection.v1alpha.ServerReflection/"},
			RequiredScopes:  []string{},
		},
		ShutdownTimeout: 30 * time.Second, // Added for tests
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

// WithRequiredScopes configures the required scopes for all endpoints
func WithRequiredScopes(scopes []string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.Auth == nil {
			config.Auth = &AuthConfig{}
		}
		config.Auth.RequiredScopes = scopes
	}
}

// NewAuthenticator creates a new authenticator from the server configuration
func NewAuthenticator(config AuthConfig) (security.Authenticator, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Create security auth config from gRPC auth config
	securityConfig := security.AuthConfig{
		JWKSEndpoint:    config.JWKSEndpoint,
		Issuer:          config.Issuer,
		Audience:        config.Audience,
		BypassPaths:     config.BypassPaths,
		RefreshInterval: config.RefreshInterval,
	}

	// Create the authenticator
	return security.NewJWTAuthenticator(securityConfig, nil)
}

// WithPort configures the port for the server
func WithPort(port int) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.Port = port
	}
}

// WithHost configures the host for the server
func WithHost(host string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.Host = host
		config.Address = host // Update both for consistency
	}
}

// WithShutdownTimeout configures the shutdown timeout for the server
func WithShutdownTimeout(timeout time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.ShutdownTimeout = timeout
	}
}
