package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/santif/microlib/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Common errors
var (
	ErrServerAlreadyStarted = errors.New("server already started")
	ErrServerNotStarted     = errors.New("server not started")
)

// Server is the interface for the gRPC server
type Server interface {
	// RegisterService registers a gRPC service with the server
	RegisterService(desc *grpc.ServiceDesc, impl interface{})

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

// ServerDependencies contains the dependencies for the gRPC server
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
	server      *grpc.Server
	listener    net.Listener
	started     bool
	startedMu   sync.RWMutex
	shutdownErr error
	deps        ServerDependencies
	services    []serviceRegistration
}

// serviceRegistration holds a service to be registered with the gRPC server
type serviceRegistration struct {
	desc *grpc.ServiceDesc
	impl interface{}
}

// NewServer creates a new gRPC server with the default configuration
func NewServer(deps ServerDependencies) Server {
	return NewServerWithConfig(DefaultServerConfig(), deps)
}

// NewServerWithConfig creates a new gRPC server with the provided configuration
func NewServerWithConfig(config ServerConfig, deps ServerDependencies) Server {
	s := &server{
		config:   config,
		deps:     deps,
		services: make([]serviceRegistration, 0),
	}
	return s
}

// RegisterService registers a gRPC service with the server
func (s *server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if s.started {
		if s.deps.Logger != nil {
			s.deps.Logger.Warn("Cannot register service after server has started",
				observability.NewField("service", desc.ServiceName))
		}
		return
	}

	// Store the service for registration when the server starts
	s.services = append(s.services, serviceRegistration{
		desc: desc,
		impl: impl,
	})
}

// Start starts the server
func (s *server) Start(ctx context.Context) error {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if s.started {
		return ErrServerAlreadyStarted
	}

	// Create the server options
	opts := s.createServerOptions()

	// Create the gRPC server with the options
	s.server = grpc.NewServer(opts...)

	// Register all services
	for _, service := range s.services {
		s.server.RegisterService(service.desc, service.impl)
	}

	// Register the health service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s.server, healthServer)

	// Set all services as serving
	for _, service := range s.services {
		healthServer.SetServingStatus(service.desc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	}

	// Set the overall health status
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Register reflection service for grpcurl and other tools
	reflection.Register(s.server)

	// Create the server address
	addr := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)

	// Create the listener
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Log server startup
	if s.deps.Logger != nil {
		s.deps.Logger.Info("Starting gRPC server",
			observability.NewField("address", addr),
			observability.NewField("tls_enabled", s.config.TLS != nil && s.config.TLS.Enabled),
		)
	}

	// Start the server in a goroutine
	go func() {
		// Check if listener is still valid before serving
		if s.listener != nil {
			err := s.server.Serve(s.listener)
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				s.startedMu.Lock()
				s.shutdownErr = err
				s.startedMu.Unlock()
				if s.deps.Logger != nil {
					s.deps.Logger.Error("gRPC server error", err)
				}
			}
		}
	}()

	// Mark the server as started
	s.started = true

	return nil
}

// createServerOptions creates the gRPC server options
func (s *server) createServerOptions() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     s.config.KeepAlive.MaxConnectionIdle,
			MaxConnectionAge:      s.config.MaxConnectionAge,
			MaxConnectionAgeGrace: s.config.MaxConnectionAgeGrace,
			Time:                  s.config.KeepAlive.Time,
			Timeout:               s.config.KeepAlive.Timeout,
		}),
		grpc.MaxConcurrentStreams(s.config.MaxConcurrentStreams),
	}

	// Add TLS if enabled
	if s.config.TLS != nil && s.config.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(s.config.TLS.CertFile, s.config.TLS.KeyFile)
		if err != nil {
			if s.deps.Logger != nil {
				s.deps.Logger.Error("Failed to load TLS certificates", err,
					observability.NewField("cert_file", s.config.TLS.CertFile),
					observability.NewField("key_file", s.config.TLS.KeyFile),
				)
			}
		} else {
			opts = append(opts, grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
		}
	}

	// Add interceptors
	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}

	// Add recovery interceptors first to catch panics in other interceptors
	if s.deps.Logger != nil {
		unaryInterceptors = append(unaryInterceptors, RecoveryUnaryServerInterceptor(s.deps.Logger))
		streamInterceptors = append(streamInterceptors, RecoveryStreamServerInterceptor(s.deps.Logger))
	}

	// Add authentication interceptors if auth is enabled
	if s.config.Auth != nil && s.config.Auth.Enabled {
		// Create authenticator from the server configuration
		authenticator, err := NewAuthenticator(*s.config.Auth)
		if err == nil {
			// Add authentication interceptors
			unaryInterceptors = append(unaryInterceptors, AuthUnaryServerInterceptor(authenticator, s.config.Auth.BypassPaths))
			streamInterceptors = append(streamInterceptors, AuthStreamServerInterceptor(authenticator, s.config.Auth.BypassPaths))

			// Add scope requirement interceptors if required scopes are specified
			if len(s.config.Auth.RequiredScopes) > 0 {
				for _, scope := range s.config.Auth.RequiredScopes {
					unaryInterceptors = append(unaryInterceptors, RequireScopeUnaryInterceptor(scope))
					streamInterceptors = append(streamInterceptors, RequireScopeStreamInterceptor(scope))
				}
			}
		} else if s.deps.Logger != nil {
			s.deps.Logger.Error("Failed to create JWT authenticator", err,
				observability.NewField("jwks_endpoint", s.config.Auth.JWKSEndpoint))
		}
	}

	// Add tracing interceptors if tracer is available
	if s.deps.Tracer != nil {
		unaryInterceptors = append(unaryInterceptors, UnaryServerTracingInterceptor(s.deps.Tracer))
		streamInterceptors = append(streamInterceptors, StreamServerTracingInterceptor(s.deps.Tracer))
	}

	// Add metrics interceptors if metrics are available
	if s.deps.Metrics != nil {
		unaryInterceptors = append(unaryInterceptors, MetricsUnaryServerInterceptor(s.deps.Metrics))
		streamInterceptors = append(streamInterceptors, MetricsStreamServerInterceptor(s.deps.Metrics))
	}

	// Add logging interceptors if logger is available
	if s.deps.Logger != nil {
		unaryInterceptors = append(unaryInterceptors, LoggingUnaryServerInterceptor(s.deps.Logger))
		streamInterceptors = append(streamInterceptors, LoggingStreamServerInterceptor(s.deps.Logger))
	}

	// Use combined interceptors if all dependencies are available
	if s.deps.Tracer != nil && s.deps.Metrics != nil && s.deps.Logger != nil {
		unaryInterceptors = []grpc.UnaryServerInterceptor{
			CombinedObservabilityUnaryServerInterceptor(s.deps.Tracer, s.deps.Metrics, s.deps.Logger),
		}
		streamInterceptors = []grpc.StreamServerInterceptor{
			CombinedObservabilityStreamServerInterceptor(s.deps.Tracer, s.deps.Metrics, s.deps.Logger),
		}
	}

	// Add the interceptors to the options
	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	return opts
}

// Shutdown gracefully shuts down the server
// It first attempts a graceful shutdown, and if that times out, it forces a shutdown.
// The method ensures all resources are properly released, including the gRPC server and listener.
//
// IMPORTANT: We get the server address before acquiring the lock to avoid deadlock.
// Previously, this method would call s.Address() while holding the write lock,
// but s.Address() tries to acquire a read lock on the same mutex, causing a deadlock.
func (s *server) Shutdown(ctx context.Context) error {
	// Get the server address before acquiring the lock to avoid deadlock
	// since Address() also acquires the lock
	serverAddr := s.Address()

	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if !s.started {
		return ErrServerNotStarted
	}

	// Log server shutdown
	if s.deps.Logger != nil {
		s.deps.Logger.Info("Shutting down gRPC server",
			observability.NewField("address", serverAddr),
		)
	}

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Default to 30 seconds
	defer cancel()

	// Create a channel to signal when the server has stopped
	stopped := make(chan struct{})

	// Stop the server in a goroutine
	go func() {
		// GracefulStop stops the server gracefully
		s.server.GracefulStop()
		close(stopped)
	}()

	// Wait for the server to stop or the context to be canceled
	var forceShutdown bool
	select {
	case <-stopped:
		// Server stopped gracefully
		if s.deps.Logger != nil {
			s.deps.Logger.Info("gRPC server stopped gracefully")
		}
	case <-shutdownCtx.Done():
		// Context canceled, force stop the server
		forceShutdown = true
		if s.deps.Logger != nil {
			s.deps.Logger.Warn("gRPC server graceful shutdown timed out, forcing stop")
		}
		s.server.Stop()
	}

	// Close the listener if it's still open
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			if s.deps.Logger != nil {
				s.deps.Logger.Error("Error closing gRPC listener", err)
			}
		}
		s.listener = nil
	}

	// Mark the server as stopped
	s.started = false
	s.server = nil

	// Return any error that occurred during server operation
	if s.shutdownErr != nil {
		return fmt.Errorf("server error: %w", s.shutdownErr)
	}

	// Return timeout error if we had to force shutdown
	if forceShutdown {
		return fmt.Errorf("server shutdown timed out")
	}

	return nil
}

// Address returns the server's address
// Note: This method acquires a read lock on startedMu.
// Be careful not to call this method while holding a write lock on startedMu
// as it will cause a deadlock.
func (s *server) Address() string {
	s.startedMu.RLock()
	defer s.startedMu.RUnlock()

	if !s.started || s.listener == nil {
		return fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	}

	return s.listener.Addr().String()
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

// NewServerWithOptions creates a new gRPC server with the provided options
func NewServerWithOptions(deps ServerDependencies, options ...func(*ServerConfig)) Server {
	config := DefaultServerConfig()

	// Apply options
	for _, option := range options {
		option(&config)
	}

	return NewServerWithConfig(config, deps)
}
