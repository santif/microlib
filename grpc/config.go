package grpc

import (
	"time"
)

// ServerConfig contains configuration for the gRPC server
type ServerConfig struct {
	// Host is the host to bind to
	Host string `json:"host" yaml:"host" validate:"required"`

	// Port is the port to listen on
	Port int `json:"port" yaml:"port" validate:"required,min=1,max=65535"`

	// ShutdownTimeout is the maximum duration to wait for the server to shutdown gracefully
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" validate:"required"`

	// MaxConnectionIdle is the maximum amount of time a connection may be idle
	MaxConnectionIdle time.Duration `json:"max_connection_idle" yaml:"max_connection_idle"`

	// MaxConnectionAge is the maximum amount of time a connection may exist
	MaxConnectionAge time.Duration `json:"max_connection_age" yaml:"max_connection_age"`

	// MaxConnectionAgeGrace is an additive period after MaxConnectionAge after which the connection will be forcibly closed
	MaxConnectionAgeGrace time.Duration `json:"max_connection_age_grace" yaml:"max_connection_age_grace"`

	// KeepAlive is the time after which a keepalive ping is sent
	KeepAlive time.Duration `json:"keep_alive" yaml:"keep_alive"`

	// KeepAliveTimeout is the timeout after which a keepalive ping is considered failed
	KeepAliveTimeout time.Duration `json:"keep_alive_timeout" yaml:"keep_alive_timeout"`

	// MaxConcurrentStreams is the maximum number of concurrent streams to each client
	MaxConcurrentStreams uint32 `json:"max_concurrent_streams" yaml:"max_concurrent_streams"`

	// EnableTLS enables TLS for the server
	EnableTLS bool `json:"enable_tls" yaml:"enable_tls"`

	// TLSCertFile is the path to the TLS certificate file
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file" validate:"required_if=EnableTLS true"`

	// TLSKeyFile is the path to the TLS key file
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file" validate:"required_if=EnableTLS true"`

	// HealthPaths are paths that should bypass certain middleware (like authentication)
	HealthPaths []string `json:"health_paths" yaml:"health_paths"`

	// Auth contains authentication configuration
	Auth *AuthConfig `json:"auth" yaml:"auth"`
}

// AuthConfig contains authentication configuration for the gRPC server
type AuthConfig struct {
	// Enabled determines if authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// JWKSEndpoint is the endpoint for JWKS
	JWKSEndpoint string `json:"jwks_endpoint" yaml:"jwks_endpoint" validate:"required_if=Enabled true"`

	// Audience is the expected audience for tokens
	Audience string `json:"audience" yaml:"audience"`

	// Issuer is the expected issuer for tokens
	Issuer string `json:"issuer" yaml:"issuer"`
}

// DefaultAuthConfig returns the default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled: false,
	}
}

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() ServerConfig {
	authConfig := DefaultAuthConfig()
	return ServerConfig{
		Host:                  "0.0.0.0",
		Port:                  9090,
		ShutdownTimeout:       30 * time.Second,
		MaxConnectionIdle:     15 * time.Minute,
		MaxConnectionAge:      30 * time.Minute,
		MaxConnectionAgeGrace: 5 * time.Minute,
		KeepAlive:             5 * time.Minute,
		KeepAliveTimeout:      20 * time.Second,
		MaxConcurrentStreams:  100,
		EnableTLS:             false,
		HealthPaths:           []string{"/grpc.health.v1.Health"},
		Auth:                  &authConfig,
	}
}

// WithTLS configures the server to use TLS
func WithTLS(certFile, keyFile string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.EnableTLS = true
		config.TLSCertFile = certFile
		config.TLSKeyFile = keyFile
	}
}

// WithPort configures the server port
func WithPort(port int) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.Port = port
	}
}

// WithHost configures the server host
func WithHost(host string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.Host = host
	}
}

// WithShutdownTimeout configures the server shutdown timeout
func WithShutdownTimeout(timeout time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.ShutdownTimeout = timeout
	}
}

// WithConnectionTimeouts configures the server connection timeouts
func WithConnectionTimeouts(idle, age, ageGrace time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.MaxConnectionIdle = idle
		config.MaxConnectionAge = age
		config.MaxConnectionAgeGrace = ageGrace
	}
}

// WithKeepAlive configures the server keep-alive settings
func WithKeepAlive(keepAlive, timeout time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.KeepAlive = keepAlive
		config.KeepAliveTimeout = timeout
	}
}

// WithMaxConcurrentStreams configures the maximum number of concurrent streams
func WithMaxConcurrentStreams(max uint32) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.MaxConcurrentStreams = max
	}
}
