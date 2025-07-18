package observability

import (
	"net/http"
	"strings"
)

// HealthPathConfig contains configuration for health check paths
type HealthPathConfig struct {
	// BasePath is the base path for health endpoints
	BasePath string `json:"base_path" yaml:"base_path"`

	// LivenessPath is the path for liveness probe
	LivenessPath string `json:"liveness_path" yaml:"liveness_path"`

	// ReadinessPath is the path for readiness probe
	ReadinessPath string `json:"readiness_path" yaml:"readiness_path"`

	// StartupPath is the path for startup probe
	StartupPath string `json:"startup_path" yaml:"startup_path"`
}

// DefaultHealthPathConfig returns the default health path configuration
func DefaultHealthPathConfig() HealthPathConfig {
	return HealthPathConfig{
		BasePath:      "/health",
		LivenessPath:  "/health/live",
		ReadinessPath: "/health/ready",
		StartupPath:   "/health/startup",
	}
}

// IsHealthPath checks if the request path is a health check path
func IsHealthPath(r *http.Request, config HealthPathConfig) bool {
	path := r.URL.Path
	return path == config.BasePath ||
		path == config.LivenessPath ||
		path == config.ReadinessPath ||
		path == config.StartupPath
}

// HealthBypassMiddleware creates a middleware that bypasses the wrapped handler for health check paths
// This is useful for bypassing authentication or other middleware for health check endpoints
func HealthBypassMiddleware(config HealthPathConfig, healthHandler http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsHealthPath(r, config) {
				healthHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HealthMiddleware creates a middleware that adds health check headers to responses
func HealthMiddleware(healthChecker HealthChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add health check headers
			ctx := r.Context()
			isHealthy := healthChecker.IsHealthy(ctx)
			isReady := healthChecker.IsReady(ctx)

			// Add headers
			w.Header().Set("X-Health-Status", boolToStatus(isHealthy))
			w.Header().Set("X-Ready-Status", boolToStatus(isReady))

			// Continue with the request
			next.ServeHTTP(w, r)
		})
	}
}

// boolToStatus converts a boolean to a status string
func boolToStatus(b bool) string {
	if b {
		return "UP"
	}
	return "DOWN"
}

// RegisterHealthHandlers registers health check handlers with the provided HTTP server
func RegisterHealthHandlers(mux *http.ServeMux, healthChecker HealthChecker, config HealthConfig) {
	if !config.Enabled {
		return
	}

	// Register the handlers
	mux.Handle(config.Path, healthChecker.HealthHandler())
	mux.Handle(config.LivenessPath, healthChecker.LivenessHandler())
	mux.Handle(config.ReadinessPath, healthChecker.ReadinessHandler())
	mux.Handle(config.StartupPath, healthChecker.StartupHandler())
}

// IsHealthEndpoint checks if the request path is a health check endpoint
func IsHealthEndpoint(r *http.Request, config HealthConfig) bool {
	path := r.URL.Path
	return path == config.Path ||
		path == config.LivenessPath ||
		path == config.ReadinessPath ||
		path == config.StartupPath
}

// HealthAuthBypassMiddleware creates a middleware that bypasses authentication for health check endpoints
func HealthAuthBypassMiddleware(config HealthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsHealthEndpoint(r, config) {
				// Set a header to indicate that authentication was bypassed
				w.Header().Set("X-Auth-Bypass", "health-endpoint")
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HealthPrefixMiddleware creates a middleware that adds a prefix to health check paths
// This is useful for adding a version prefix or API gateway path
func HealthPrefixMiddleware(prefix string, config HealthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Check if the path starts with the prefix and matches a health endpoint
			if strings.HasPrefix(path, prefix) {
				// Remove the prefix
				unprefixedPath := strings.TrimPrefix(path, prefix)

				// Check if the unprefixed path is a health endpoint
				if unprefixedPath == config.Path ||
					unprefixedPath == config.LivenessPath ||
					unprefixedPath == config.ReadinessPath ||
					unprefixedPath == config.StartupPath {

					// Update the request path
					r.URL.Path = unprefixedPath
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
