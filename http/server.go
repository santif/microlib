package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"
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
}

// NewServer creates a new HTTP server with the default configuration
func NewServer() Server {
	return NewServerWithConfig(DefaultServerConfig())
}

// NewServerWithConfig creates a new HTTP server with the provided configuration
func NewServerWithConfig(config ServerConfig) Server {
	mux := http.NewServeMux()

	return &server{
		config:     config,
		mux:        mux,
		middleware: make(MiddlewareChain, 0),
		started:    false,
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

// NewServerWithOptions creates a new HTTP server with the provided options
func NewServerWithOptions(options ...func(*ServerConfig)) Server {
	config := DefaultServerConfig()

	// Apply options
	for _, option := range options {
		option(&config)
	}

	return NewServerWithConfig(config)
}
