package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/santif/microlib/observability"
)

// Common errors
var (
	ErrServerAlreadyStarted = errors.New("server already started")
	ErrServerNotStarted     = errors.New("server not started")
)

// Server is the interface for the HTTP server
type Server interface {
	// RegisterHandler registers a handler for the given pattern
	RegisterHandler(pattern string, handler http.Handler)

	// RegisterMiddleware registers middleware to be applied to all handlers
	RegisterMiddleware(middleware Middleware)

	// Start starts the server
	Start(ctx context.Context) error

	// Shutdown gracefully shuts down the server
	Shutdown(ctx context.Context) error

	// Address returns the server's address
	Address() string

	// IsStarted returns whether the server is started
	IsStarted() bool

	// Config returns the server configuration
	Config() ServerConfig
}

// ServerDependencies contains the dependencies for the HTTP server
type ServerDependencies struct {
	// Logger is the logger to use for the server
	Logger observability.Logger

	// Metrics is the metrics collector to use for the server
	Metrics observability.Metrics

	// Tracer is the tracer to use for the server
	Tracer observability.Tracer
}

// server implements the Server interface
type server struct {
	config      ServerConfig
	server      *http.Server
	mux         *http.ServeMux
	middleware  MiddlewareChain
	started     bool
	startedMu   sync.RWMutex
	shutdownErr error
	deps        ServerDependencies
}

// NewServer creates a new HTTP server with the default configuration
func NewServer(deps ServerDependencies) Server {
	return NewServerWithConfig(DefaultServerConfig(), deps)
}

// NewServerWithConfig creates a new HTTP server with the provided configuration
func NewServerWithConfig(config ServerConfig, deps ServerDependencies) Server {
	mux := http.NewServeMux()
	s := &server{
		config:     config,
		mux:        mux,
		middleware: make(MiddlewareChain, 0),
		started:    false,
		deps:       deps,
	}

	// Register default middleware based on configuration
	s.registerDefaultMiddleware()

	return s
}

// registerDefaultMiddleware registers the default middleware stack based on the configuration
func (s *server) registerDefaultMiddleware() {
	// Recovery middleware should always be first to catch panics in other middleware
	s.RegisterMiddleware(RecoveryMiddleware(s.deps.Logger))

	// Add security headers if enabled
	if s.config.SecurityHeaders.Enabled {
		s.RegisterMiddleware(SecurityHeadersMiddleware(s.config.SecurityHeaders))
	}

	// Add CORS if enabled
	if s.config.CORS.Enabled {
		s.RegisterMiddleware(CORSMiddleware(s.config.CORS))
	}

	// Add tracing if available
	if s.deps.Tracer != nil {
		s.RegisterMiddleware(TracingMiddleware(s.deps.Tracer))
	}

	// Add metrics if available
	if s.deps.Metrics != nil {
		s.RegisterMiddleware(MetricsMiddleware(s.deps.Metrics))
	}

	// Add logging if available
	if s.deps.Logger != nil {
		s.RegisterMiddleware(LoggingMiddleware(s.deps.Logger))
	}
}

// RegisterHandler registers a handler for the given pattern
func (s *server) RegisterHandler(pattern string, handler http.Handler) {
	// Prepend the base path if configured
	if s.config.BasePath != "" {
		pattern = path.Join(s.config.BasePath, pattern)
	}

	// Apply middleware to the handler
	wrappedHandler := s.middleware.Apply(handler)

	// Register the handler with the mux
	s.mux.Handle(pattern, wrappedHandler)
}

// RegisterMiddleware registers middleware to be applied to all handlers
func (s *server) RegisterMiddleware(middleware Middleware) {
	s.middleware = append(s.middleware, middleware)
}

// Start starts the server
func (s *server) Start(ctx context.Context) error {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if s.started {
		return ErrServerAlreadyStarted
	}

	// Create the server address
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Create the HTTP server
	s.server = &http.Server{
		Addr:           addr,
		Handler:        s.mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	// Log server startup
	if s.deps.Logger != nil {
		s.deps.Logger.Info("Starting HTTP server",
			observability.NewField("address", addr),
			observability.NewField("tls_enabled", s.config.EnableTLS),
		)
	}

	// Start the server in a goroutine
	go func() {
		var err error

		// Start the server with or without TLS
		if s.config.EnableTLS {
			err = s.server.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
		} else {
			err = s.server.ListenAndServe()
		}

		// If the server was shut down gracefully, err will be http.ErrServerClosed
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.shutdownErr = err
			if s.deps.Logger != nil {
				s.deps.Logger.Error("HTTP server error", err)
			}
		}
	}()

	// Mark the server as started
	s.started = true

	return nil
}

// Shutdown gracefully shuts down the server
func (s *server) Shutdown(ctx context.Context) error {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if !s.started {
		return ErrServerNotStarted
	}

	// Log server shutdown
	if s.deps.Logger != nil {
		s.deps.Logger.Info("Shutting down HTTP server",
			observability.NewField("address", s.server.Addr),
			observability.NewField("timeout", s.config.ShutdownTimeout.String()),
		)
	}

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	// Shutdown the server
	err := s.server.Shutdown(shutdownCtx)

	// Mark the server as stopped
	s.started = false

	// Return any shutdown error
	if err != nil {
		return fmt.Errorf("failed to shutdown server gracefully: %w", err)
	}

	// Return any error that occurred during server operation
	if s.shutdownErr != nil {
		return fmt.Errorf("server error: %w", s.shutdownErr)
	}

	return nil
}

// Address returns the server's address
func (s *server) Address() string {
	s.startedMu.RLock()
	defer s.startedMu.RUnlock()

	if !s.started || s.server == nil {
		return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	}

	// If the server is started, return the configured address
	// Note: http.Server doesn't expose its listener publicly
	return s.server.Addr
}

// IsStarted returns whether the server is started
func (s *server) IsStarted() bool {
	s.startedMu.RLock()
	defer s.startedMu.RUnlock()
	return s.started
}

// Config returns the server configuration
func (s *server) Config() ServerConfig {
	return s.config
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

// WithBasePath configures the server base path
func WithBasePath(basePath string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.BasePath = basePath
	}
}

// WithTimeouts configures the server timeouts
func WithTimeouts(read, write, idle, shutdown time.Duration) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.ReadTimeout = read
		config.WriteTimeout = write
		config.IdleTimeout = idle
		config.ShutdownTimeout = shutdown
	}
}

// WithCORS configures CORS for the server
func WithCORS(enabled bool, allowOrigins string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.CORS.Enabled = enabled
		if allowOrigins != "" {
			config.CORS.AllowOrigins = allowOrigins
		}
	}
}

// WithSecurityHeaders configures security headers for the server
func WithSecurityHeaders(enabled bool) func(*ServerConfig) {
	return func(config *ServerConfig) {
		config.SecurityHeaders.Enabled = enabled
	}
}

// NewServerWithOptions creates a new HTTP server with the provided options
func NewServerWithOptions(deps ServerDependencies, options ...func(*ServerConfig)) Server {
	config := DefaultServerConfig()

	// Apply options
	for _, option := range options {
		option(&config)
	}

	return NewServerWithConfig(config, deps)
}
