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
}

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() ServerConfig {
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
	}
}
