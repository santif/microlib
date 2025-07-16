package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ServiceMetadata contains metadata about the service instance
type ServiceMetadata struct {
	Name      string    `json:"name" validate:"required"`
	Version   string    `json:"version" validate:"required,semver"`
	Instance  string    `json:"instance" validate:"required"`
	BuildHash string    `json:"build_hash" validate:"required"`
	StartTime time.Time `json:"start_time"`
}

// Dependency represents a service dependency that can be health checked
type Dependency interface {
	Name() string
	HealthCheck(ctx context.Context) error
}

// ShutdownHook is a function that gets called during graceful shutdown
type ShutdownHook func(ctx context.Context) error

// Service represents the core service with lifecycle management
type Service struct {
	metadata        ServiceMetadata
	dependencies    []Dependency
	shutdownHooks   []ShutdownHook
	shutdown        chan os.Signal
	mu              sync.RWMutex
	started         bool
	shutdownTimeout time.Duration
}

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 30 * time.Second

// NewService creates a new service instance with the provided metadata
func NewService(metadata ServiceMetadata) *Service {
	metadata.StartTime = time.Now()
	
	return &Service{
		metadata:        metadata,
		dependencies:    make([]Dependency, 0),
		shutdownHooks:   make([]ShutdownHook, 0),
		shutdown:        make(chan os.Signal, 1),
		started:         false,
		shutdownTimeout: DefaultShutdownTimeout,
	}
}

// WithShutdownTimeout sets the timeout for graceful shutdown
func (s *Service) WithShutdownTimeout(timeout time.Duration) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownTimeout = timeout
	return s
}

// Metadata returns the service metadata
func (s *Service) Metadata() ServiceMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata
}

// AddDependency registers a dependency for health checking
func (s *Service) AddDependency(dep Dependency) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dependencies = append(s.dependencies, dep)
}

// RegisterShutdownHook registers a function to be called during graceful shutdown
func (s *Service) RegisterShutdownHook(hook ShutdownHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownHooks = append(s.shutdownHooks, hook)
}

// ValidateDependencies checks all registered dependencies before accepting traffic
func (s *Service) ValidateDependencies(ctx context.Context) error {
	s.mu.RLock()
	dependencies := make([]Dependency, len(s.dependencies))
	copy(dependencies, s.dependencies)
	s.mu.RUnlock()

	for _, dep := range dependencies {
		if err := dep.HealthCheck(ctx); err != nil {
			return fmt.Errorf("dependency %s health check failed: %w", dep.Name(), err)
		}
	}
	return nil
}

// Start initializes the service and validates dependencies
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("service %s is already started", s.metadata.Name)
	}
	s.mu.Unlock()

	// Validate dependencies before accepting traffic
	if err := s.ValidateDependencies(ctx); err != nil {
		return fmt.Errorf("failed to validate dependencies: %w", err)
	}

	// Only mark as started after successful validation
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	// Set up signal handling for graceful shutdown
	signal.Notify(s.shutdown, syscall.SIGTERM, syscall.SIGINT)

	return nil
}

// Shutdown performs graceful shutdown with configurable timeout
func (s *Service) Shutdown(timeout time.Duration) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute shutdown hooks in reverse order
	s.mu.RLock()
	hooks := make([]ShutdownHook, len(s.shutdownHooks))
	copy(hooks, s.shutdownHooks)
	s.mu.RUnlock()

	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i](ctx); err != nil {
			return fmt.Errorf("shutdown hook failed: %w", err)
		}
	}

	// Stop signal handling
	signal.Stop(s.shutdown)
	close(s.shutdown)

	return nil
}

// WaitForShutdown blocks until a shutdown signal is received
func (s *Service) WaitForShutdown() <-chan os.Signal {
	return s.shutdown
}

// Trigger sends a shutdown signal programmatically, useful for testing
// or for controlled shutdown without relying on OS signals
func (s *Service) Trigger(sig os.Signal) {
	if sig == nil {
		sig = syscall.SIGTERM
	}
	s.shutdown <- sig
}

// IsStarted returns whether the service has been started
func (s *Service) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// Run starts the service and blocks until a shutdown signal is received,
// then performs graceful shutdown with the configured timeout.
// This is a convenience method that combines Start, WaitForShutdown, and Shutdown.
func (s *Service) Run(ctx context.Context) error {
	// Start the service
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Wait for shutdown signal
	sig := <-s.WaitForShutdown()

	// Get the configured shutdown timeout
	s.mu.RLock()
	timeout := s.shutdownTimeout
	s.mu.RUnlock()

	// Log the signal received (in a real implementation, this would use the logger)
	fmt.Printf("Received signal %v, initiating graceful shutdown with timeout %v\n", sig, timeout)

	// Perform graceful shutdown
	if err := s.Shutdown(timeout); err != nil {
		return fmt.Errorf("failed to shutdown service gracefully: %w", err)
	}

	return nil
}

// RunWithTimeout starts the service and blocks until a shutdown signal is received,
// then performs graceful shutdown with the specified timeout.
// This is a convenience method that combines Start, WaitForShutdown, and Shutdown.
func (s *Service) RunWithTimeout(ctx context.Context, shutdownTimeout time.Duration) error {
	// Override the configured shutdown timeout
	s.WithShutdownTimeout(shutdownTimeout)
	
	return s.Run(ctx)
}