package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	// StatusUnknown indicates the health status is unknown
	StatusUnknown HealthStatus = "UNKNOWN"
	// StatusUp indicates the component is healthy
	StatusUp HealthStatus = "UP"
	// StatusDown indicates the component is unhealthy
	StatusDown HealthStatus = "DOWN"
	// StatusStarting indicates the component is starting up
	StatusStarting HealthStatus = "STARTING"
)

// HealthCheck represents a health check function
type HealthCheck func(ctx context.Context) error

// HealthResult represents the result of a health check
type HealthResult struct {
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Error     string       `json:"error,omitempty"`
}

// HealthChecker manages health checks for the service
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]HealthCheck
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheck),
	}
}

// AddCheck adds a health check with the given name
func (h *HealthChecker) AddCheck(name string, check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// RemoveCheck removes a health check with the given name
func (h *HealthChecker) RemoveCheck(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checks, name)
}

// RunChecks runs all registered health checks
func (h *HealthChecker) RunChecks(ctx context.Context) map[string]HealthResult {
	h.mu.RLock()
	checks := make(map[string]HealthCheck, len(h.checks))
	for name, check := range h.checks {
		checks[name] = check
	}
	h.mu.RUnlock()

	results := make(map[string]HealthResult, len(checks))
	for name, check := range checks {
		result := HealthResult{
			Status:    StatusUnknown,
			Timestamp: time.Now(),
		}

		err := check(ctx)
		if err != nil {
			result.Status = StatusDown
			result.Message = fmt.Sprintf("Health check failed: %v", err)
			result.Error = err.Error()
		} else {
			result.Status = StatusUp
			result.Message = "Health check passed"
		}

		results[name] = result
	}

	return results
}

// IsHealthy returns true if all health checks pass
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	results := h.RunChecks(ctx)
	for _, result := range results {
		if result.Status != StatusUp {
			return false
		}
	}
	return true
}

// HealthSummary provides a summary of all health checks
func (h *HealthChecker) HealthSummary(ctx context.Context) HealthResult {
	results := h.RunChecks(ctx)
	if len(results) == 0 {
		return HealthResult{
			Status:    StatusUnknown,
			Message:   "No health checks registered",
			Timestamp: time.Now(),
		}
	}

	for _, result := range results {
		if result.Status == StatusDown {
			return HealthResult{
				Status:    StatusDown,
				Message:   "One or more health checks failed",
				Timestamp: time.Now(),
			}
		}
	}

	return HealthResult{
		Status:    StatusUp,
		Message:   "All health checks passed",
		Timestamp: time.Now(),
	}
}