package http

import (
	"time"
)

// ServerConfig contains configuration for the HTTP server
type ServerConfig struct {
	// Host is the host to bind to
	Host string `json:"host" yaml:"host" validate:"required"`

	// Port is the port to listen on
	Port int `json:"port" yaml:"port" validate:"required,min=1,max=65535"`

	// ReadTimeout is the maximum duration for reading the entire request, including the body
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout" validate:"required"`

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" validate:"required"`

	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout" validate:"required"`

	// ShutdownTimeout is the maximum duration to wait for the server to shutdown gracefully
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" validate:"required"`

	// MaxHeaderBytes controls the maximum number of bytes the server will read parsing the request header
	MaxHeaderBytes int `json:"max_header_bytes" yaml:"max_header_bytes" validate:"min=0"`

	// EnableTLS enables TLS for the server
	EnableTLS bool `json:"enable_tls" yaml:"enable_tls"`

	// TLSCertFile is the path to the TLS certificate file
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file" validate:"required_if=EnableTLS true"`

	// TLSKeyFile is the path to the TLS key file
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file" validate:"required_if=EnableTLS true"`

	// BasePath is the base path for all routes
	BasePath string `json:"base_path" yaml:"base_path"`

	// CORS contains CORS configuration
	CORS CORSConfig `json:"cors" yaml:"cors"`

	// SecurityHeaders contains security headers configuration
	SecurityHeaders SecurityHeadersConfig `json:"security_headers" yaml:"security_headers"`

	// HealthPaths are paths that should bypass certain middleware (like authentication)
	HealthPaths []string `json:"health_paths" yaml:"health_paths"`

	// OpenAPI contains OpenAPI configuration
	OpenAPI *OpenAPIConfig `json:"openapi" yaml:"openapi"`

	// Auth contains authentication configuration
	Auth *AuthConfig `json:"auth" yaml:"auth"`
}

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() ServerConfig {
	openAPIConfig := DefaultOpenAPIConfig()
	authConfig := DefaultAuthConfig()
	return ServerConfig{
		Host:            "0.0.0.0",
		Port:            8080,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxHeaderBytes:  1 << 20, // 1 MB
		EnableTLS:       false,
		BasePath:        "",
		CORS:            DefaultCORSConfig(),
		SecurityHeaders: DefaultSecurityHeadersConfig(),
		HealthPaths:     []string{"/health", "/metrics"},
		OpenAPI:         &openAPIConfig,
		Auth:            &authConfig,
	}
}

// CORSConfig contains configuration for CORS middleware
type CORSConfig struct {
	// Enabled determines if CORS is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// AllowOrigins is a comma-separated list of origins that are allowed to access the resource
	AllowOrigins string `json:"allow_origins" yaml:"allow_origins"`

	// AllowMethods is a comma-separated list of methods that are allowed to access the resource
	AllowMethods string `json:"allow_methods" yaml:"allow_methods"`

	// AllowHeaders is a comma-separated list of headers that are allowed to be sent with the request
	AllowHeaders string `json:"allow_headers" yaml:"allow_headers"`

	// ExposeHeaders is a comma-separated list of headers that are allowed to be exposed to the client
	ExposeHeaders string `json:"expose_headers" yaml:"expose_headers"`

	// AllowCredentials indicates whether the request can include user credentials
	AllowCredentials bool `json:"allow_credentials" yaml:"allow_credentials"`

	// MaxAge indicates how long the results of a preflight request can be cached
	MaxAge int `json:"max_age" yaml:"max_age"`
}

// DefaultCORSConfig returns the default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		Enabled:          true,
		AllowOrigins:     "*",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS, PATCH",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		ExposeHeaders:    "Content-Length, Content-Type",
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// SecurityHeadersConfig contains configuration for security headers middleware
type SecurityHeadersConfig struct {
	// Enabled determines if security headers are enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ContentSecurityPolicy is the Content-Security-Policy header value
	ContentSecurityPolicy string `json:"content_security_policy" yaml:"content_security_policy"`

	// XFrameOptions is the X-Frame-Options header value
	XFrameOptions string `json:"x_frame_options" yaml:"x_frame_options"`

	// XContentTypeOptions enables the X-Content-Type-Options: nosniff header
	XContentTypeOptions bool `json:"x_content_type_options" yaml:"x_content_type_options"`

	// ReferrerPolicy is the Referrer-Policy header value
	ReferrerPolicy string `json:"referrer_policy" yaml:"referrer_policy"`

	// StrictTransportSecurity is the Strict-Transport-Security header value
	StrictTransportSecurity string `json:"strict_transport_security" yaml:"strict_transport_security"`

	// XSSProtection enables the X-XSS-Protection: 1; mode=block header
	XSSProtection bool `json:"xss_protection" yaml:"xss_protection"`
}

// DefaultSecurityHeadersConfig returns the default security headers configuration
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:                 true,
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		XSSProtection:           true,
	}
}
