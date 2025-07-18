package observability

// ObservabilityConfig contains configuration for all observability components
type ObservabilityConfig struct {
	// Logger contains configuration for the logger
	Logger LoggerConfig `json:"logger" yaml:"logger"`

	// Metrics contains configuration for metrics collection
	Metrics MetricsConfig `json:"metrics" yaml:"metrics"`

	// Tracing contains configuration for distributed tracing
	Tracing TracingConfig `json:"tracing" yaml:"tracing"`

	// HealthEndpoints contains configuration for health check endpoints
	HealthEndpoints HealthEndpointsConfig `json:"health_endpoints" yaml:"health_endpoints"`
}

// DefaultObservabilityConfig returns the default observability configuration
func DefaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		Logger:          DefaultLoggerConfig(),
		Metrics:         DefaultMetricsConfig(),
		Tracing:         DefaultTracingConfig(),
		HealthEndpoints: DefaultHealthEndpointsConfig(),
	}
}

// HealthEndpointsConfig contains configuration for health check endpoints
type HealthEndpointsConfig struct {
	// Enabled determines if health checks are enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Path is the HTTP path for the health endpoint
	Path string `json:"path" yaml:"path" validate:"required"`

	// LivenessPath is the HTTP path for the liveness endpoint
	LivenessPath string `json:"liveness_path" yaml:"liveness_path" validate:"required"`

	// ReadinessPath is the HTTP path for the readiness endpoint
	ReadinessPath string `json:"readiness_path" yaml:"readiness_path" validate:"required"`

	// StartupPath is the HTTP path for the startup endpoint
	StartupPath string `json:"startup_path" yaml:"startup_path" validate:"required"`
}

// DefaultHealthEndpointsConfig returns the default health endpoints configuration
func DefaultHealthEndpointsConfig() HealthEndpointsConfig {
	return HealthEndpointsConfig{
		Enabled:       true,
		Path:          "/health",
		LivenessPath:  "/health/live",
		ReadinessPath: "/health/ready",
		StartupPath:   "/health/startup",
	}
}
