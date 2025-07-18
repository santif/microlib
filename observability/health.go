package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/santif/microlib/core"
)

// Common errors
var (
	ErrNilConfig         = errors.New("config cannot be nil")
	ErrInvalidConfigType = errors.New("invalid configuration type, expected ObservabilityConfig")
)

// Using configInterface from metrics.go

// HealthConfig contains configuration for health checks
type HealthConfig struct {
	// Enabled determines if health endpoints are enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Path is the base path for health endpoints
	Path string `json:"path" yaml:"path" validate:"required_if=Enabled true"`

	// LivenessPath is the path for liveness probe
	LivenessPath string `json:"liveness_path" yaml:"liveness_path" validate:"required_if=Enabled true"`

	// ReadinessPath is the path for readiness probe
	ReadinessPath string `json:"readiness_path" yaml:"readiness_path" validate:"required_if=Enabled true"`

	// StartupPath is the path for startup probe
	StartupPath string `json:"startup_path" yaml:"startup_path" validate:"required_if=Enabled true"`

	// StartupThreshold is the number of consecutive successful checks required for startup
	StartupThreshold int `json:"startup_threshold" yaml:"startup_threshold" validate:"min=1"`

	// StartupInterval is the interval between startup checks in milliseconds
	StartupInterval int `json:"startup_interval" yaml:"startup_interval" validate:"min=1"`

	// ReadinessTimeout is the timeout for readiness checks in milliseconds
	ReadinessTimeout int `json:"readiness_timeout" yaml:"readiness_timeout" validate:"min=1"`

	// LivenessTimeout is the timeout for liveness checks in milliseconds
	LivenessTimeout int `json:"liveness_timeout" yaml:"liveness_timeout" validate:"min=1"`
}

// DefaultHealthConfig returns the default health configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:          true,
		Path:             "/health",
		LivenessPath:     "/health/live",
		ReadinessPath:    "/health/ready",
		StartupPath:      "/health/startup",
		StartupThreshold: 3,
		StartupInterval:  1000, // 1 second
		ReadinessTimeout: 5000, // 5 seconds
		LivenessTimeout:  2000, // 2 seconds
	}
}

// HealthCheck represents a health check function
type HealthCheck func(ctx context.Context) error

// HealthChecker is the interface for health check management
type HealthChecker interface {
	// AddCheck adds a health check with the given name
	AddCheck(name string, check HealthCheck)

	// RemoveCheck removes a health check with the given name
	RemoveCheck(name string)

	// LivenessHandler returns an HTTP handler for liveness probe
	LivenessHandler() http.Handler

	// ReadinessHandler returns an HTTP handler for readiness probe
	ReadinessHandler() http.Handler

	// StartupHandler returns an HTTP handler for startup probe
	StartupHandler() http.Handler

	// HealthHandler returns an HTTP handler for overall health status
	HealthHandler() http.Handler

	// IsHealthy returns true if all health checks pass
	IsHealthy(ctx context.Context) bool

	// IsReady returns true if the service is ready to serve traffic
	IsReady(ctx context.Context) bool

	// RegisterHandlers registers all health check handlers with the provided HTTP server
	RegisterHandlers(mux *http.ServeMux)
}

// healthChecker implements the HealthChecker interface
type healthChecker struct {
	coreHealthChecker *core.HealthChecker
	config            HealthConfig
	startupChecks     int
	startupMu         sync.RWMutex
	readyToServe      bool
	readyMu           sync.RWMutex
}

// NewHealthChecker creates a new health checker with the default configuration
func NewHealthChecker() HealthChecker {
	return NewHealthCheckerWithConfig(DefaultHealthConfig())
}

// NewHealthCheckerWithConfig creates a new health checker with the provided configuration
func NewHealthCheckerWithConfig(config HealthConfig) HealthChecker {
	return &healthChecker{
		coreHealthChecker: core.NewHealthChecker(),
		config:            config,
		startupChecks:     0,
		readyToServe:      false,
	}
}

// AddCheck adds a health check with the given name
func (h *healthChecker) AddCheck(name string, check HealthCheck) {
	// Convert our HealthCheck to core.HealthCheck
	coreCheck := func(ctx context.Context) error {
		return check(ctx)
	}
	h.coreHealthChecker.AddCheck(name, coreCheck)
}

// RemoveCheck removes a health check with the given name
func (h *healthChecker) RemoveCheck(name string) {
	h.coreHealthChecker.RemoveCheck(name)
}

// LivenessHandler returns an HTTP handler for liveness probe
func (h *healthChecker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For liveness, we just check if the service is running, not if all dependencies are healthy
		// We don't need a timeout context for this simple check
		result := core.HealthResult{
			Status:    core.StatusUp,
			Message:   "Service is alive",
			Timestamp: time.Now(),
		}

		// Set the content type header before writing the status code
		w.Header().Set("Content-Type", "application/json")

		// Set the response status code based on the health status
		if result.Status != core.StatusUp {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Write the response
		json.NewEncoder(w).Encode(result)
	})
}

// ReadinessHandler returns an HTTP handler for readiness probe
func (h *healthChecker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a context with timeout for readiness checks
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.config.ReadinessTimeout)*time.Millisecond)
		defer cancel()

		// Check if the service is ready to serve traffic
		h.readyMu.RLock()
		isReady := h.readyToServe
		h.readyMu.RUnlock()

		// Run all health checks
		results := h.coreHealthChecker.RunChecks(ctx)

		// Determine overall status
		status := core.StatusUp
		message := "Service is ready"

		// If not ready to serve, set status to down
		if !isReady {
			status = core.StatusDown
			message = "Service is not ready to serve traffic"
		} else {
			// Check if any dependency is down
			for _, result := range results {
				if result.Status != core.StatusUp {
					status = core.StatusDown
					message = "One or more dependencies are not healthy"
					break
				}
			}
		}

		// Create the response
		response := map[string]interface{}{
			"status":    status,
			"message":   message,
			"timestamp": time.Now(),
			"checks":    results,
		}

		// Set the content type header before writing the status code
		w.Header().Set("Content-Type", "application/json")

		// Set the response status code based on the health status
		if status != core.StatusUp {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Write the response
		json.NewEncoder(w).Encode(response)
	})
}

// StartupHandler returns an HTTP handler for startup probe
func (h *healthChecker) StartupHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a context with timeout for startup checks
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.config.ReadinessTimeout)*time.Millisecond)
		defer cancel()

		// Run all health checks
		results := h.coreHealthChecker.RunChecks(ctx)

		// Determine overall status
		status := core.StatusUp
		message := "Service startup complete"

		// Check if any dependency is down
		for _, result := range results {
			if result.Status != core.StatusUp {
				status = core.StatusDown
				message = "Service startup in progress"
				break
			}
		}

		// If all checks pass, increment the startup counter
		if status == core.StatusUp {
			h.startupMu.Lock()
			h.startupChecks++
			count := h.startupChecks
			h.startupMu.Unlock()

			// If we've reached the threshold, mark the service as ready
			if count >= h.config.StartupThreshold {
				h.readyMu.Lock()
				h.readyToServe = true
				h.readyMu.Unlock()
			} else {
				// Still waiting for more successful checks
				status = core.StatusStarting
				message = "Service startup in progress"
			}
		} else {
			// Reset the counter if any check fails
			h.startupMu.Lock()
			h.startupChecks = 0
			h.startupMu.Unlock()
		}

		// Create the response
		response := map[string]interface{}{
			"status":    status,
			"message":   message,
			"timestamp": time.Now(),
			"checks":    results,
		}

		// Set the content type header before writing the status code
		w.Header().Set("Content-Type", "application/json")

		// Set the response status code based on the health status
		if status != core.StatusUp {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Write the response
		json.NewEncoder(w).Encode(response)
	})
}

// HealthHandler returns an HTTP handler for overall health status
func (h *healthChecker) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a context with timeout for health checks
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.config.ReadinessTimeout)*time.Millisecond)
		defer cancel()

		// Run all health checks
		results := h.coreHealthChecker.RunChecks(ctx)

		// Get the overall health summary
		summary := h.coreHealthChecker.HealthSummary(ctx)

		// Create the response
		response := map[string]interface{}{
			"status":    summary.Status,
			"message":   summary.Message,
			"timestamp": summary.Timestamp,
			"checks":    results,
		}

		// Set the content type header before writing the status code
		w.Header().Set("Content-Type", "application/json")

		// Set the response status code based on the health status
		if summary.Status != core.StatusUp {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Write the response
		json.NewEncoder(w).Encode(response)
	})
}

// IsHealthy returns true if all health checks pass
func (h *healthChecker) IsHealthy(ctx context.Context) bool {
	return h.coreHealthChecker.IsHealthy(ctx)
}

// IsReady returns true if the service is ready to serve traffic
func (h *healthChecker) IsReady(ctx context.Context) bool {
	h.readyMu.RLock()
	defer h.readyMu.RUnlock()
	return h.readyToServe && h.IsHealthy(ctx)
}

// SetReadyToServe sets whether the service is ready to serve traffic (for testing)
func (h *healthChecker) SetReadyToServe(ready bool) {
	h.readyMu.Lock()
	defer h.readyMu.Unlock()
	h.readyToServe = ready
}

// ResetStartupChecks resets the startup checks counter (for testing)
func (h *healthChecker) ResetStartupChecks() {
	h.startupMu.Lock()
	defer h.startupMu.Unlock()
	h.startupChecks = 0
}

// SetStartupThreshold sets the startup threshold (for testing)
func (h *healthChecker) SetStartupThreshold(threshold int) {
	h.config.StartupThreshold = threshold
}

// RegisterHandlers registers all health check handlers with the provided HTTP server
func (h *healthChecker) RegisterHandlers(mux *http.ServeMux) {
	if !h.config.Enabled {
		return
	}

	// Register the handlers
	mux.Handle(h.config.Path, h.HealthHandler())
	mux.Handle(h.config.LivenessPath, h.LivenessHandler())
	mux.Handle(h.config.ReadinessPath, h.ReadinessHandler())
	mux.Handle(h.config.StartupPath, h.StartupHandler())
}

// HealthManager manages health checks and provides dynamic configuration updates
type HealthManager struct {
	config        configInterface
	healthChecker HealthChecker
	mu            sync.RWMutex
}

// NewHealthManager creates a new health manager
func NewHealthManager(cfg configInterface) (*HealthManager, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	// Get the observability configuration
	_, ok := cfg.Get().(*ObservabilityConfig)
	if !ok {
		return nil, ErrInvalidConfigType
	}

	// Create the health checker with the configuration
	healthChecker := NewHealthCheckerWithConfig(DefaultHealthConfig())

	return &HealthManager{
		config:        cfg,
		healthChecker: healthChecker,
	}, nil
}

// GetHealthChecker returns the health checker instance
func (m *HealthManager) GetHealthChecker() HealthChecker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthChecker
}

// UpdateHealthConfig updates the health configuration
func (m *HealthManager) UpdateHealthConfig(healthConfig HealthConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the current configuration
	_, ok := m.config.Get().(*ObservabilityConfig)
	if !ok {
		return ErrInvalidConfigType
	}

	// Note: We're using HealthEndpoints in ObservabilityConfig now
	// This is a temporary fix until we update the entire health system
	// Just use the default config for now

	// We're not actually updating the configuration since we've changed the structure
	// This method will be updated when we refactor the health system

	return nil
}

// Reload implements the config.Reloadable interface
func (m *HealthManager) Reload(newConfig interface{}) error {
	_, ok := newConfig.(*ObservabilityConfig)
	if !ok {
		return ErrInvalidConfigType
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a new health checker with the updated configuration
	// Note: We're using HealthEndpoints in ObservabilityConfig now
	// This is a temporary fix until we update the entire health system
	healthChecker := NewHealthCheckerWithConfig(DefaultHealthConfig())

	// Update the health checker
	m.healthChecker = healthChecker

	return nil
}

// GlobalHealthChecker is a package-level health checker for convenience
var GlobalHealthChecker HealthChecker

// Initialize the global health checker with default configuration
func init() {
	GlobalHealthChecker = NewHealthChecker()
}

// SetGlobalHealthChecker sets the global health checker
func SetGlobalHealthChecker(healthChecker HealthChecker) {
	GlobalHealthChecker = healthChecker
}

// AddCheck adds a health check to the global health checker
func AddCheck(name string, check HealthCheck) {
	GlobalHealthChecker.AddCheck(name, check)
}

// RemoveCheck removes a health check from the global health checker
func RemoveCheck(name string) {
	GlobalHealthChecker.RemoveCheck(name)
}

// IsHealthy returns true if all health checks pass using the global health checker
func IsHealthy(ctx context.Context) bool {
	return GlobalHealthChecker.IsHealthy(ctx)
}

// IsReady returns true if the service is ready to serve traffic using the global health checker
func IsReady(ctx context.Context) bool {
	return GlobalHealthChecker.IsReady(ctx)
}
