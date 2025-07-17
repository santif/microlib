package observability

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Counter represents a monotonically increasing counter metric
type Counter interface {
	// Inc increments the counter by 1
	Inc()

	// Add adds the given value to the counter
	Add(value float64)

	// WithLabels returns a counter with the given labels
	WithLabels(labels map[string]string) Counter
}

// Histogram represents a histogram metric for measuring distributions
type Histogram interface {
	// Observe adds a single observation to the histogram
	Observe(value float64)

	// WithLabels returns a histogram with the given labels
	WithLabels(labels map[string]string) Histogram
}

// Gauge represents a gauge metric that can go up and down
type Gauge interface {
	// Set sets the gauge to the given value
	Set(value float64)

	// Inc increments the gauge by 1
	Inc()

	// Dec decrements the gauge by 1
	Dec()

	// Add adds the given value to the gauge
	Add(value float64)

	// Sub subtracts the given value from the gauge
	Sub(value float64)

	// WithLabels returns a gauge with the given labels
	WithLabels(labels map[string]string) Gauge
}

// Metrics is the interface for metrics collection and exposure
type Metrics interface {
	// Counter creates or retrieves a counter metric.
	// A counter is a cumulative metric that represents a single monotonically increasing counter
	// whose value can only increase or be reset to zero on restart.
	// For example, you can use a counter to represent the number of requests served, tasks completed, or errors.
	//
	// Parameters:
	//   - name: The name of the metric
	//   - help: A description of the metric
	//   - labels: Optional label names that can be used when recording the metric
	Counter(name string, help string, labels ...string) Counter

	// Histogram creates or retrieves a histogram metric.
	// A histogram samples observations (usually things like request durations or response sizes)
	// and counts them in configurable buckets. It also provides a sum of all observed values.
	//
	// Parameters:
	//   - name: The name of the metric
	//   - help: A description of the metric
	//   - buckets: The histogram buckets to use (e.g., []float64{0.01, 0.1, 1, 10})
	//   - labels: Optional label names that can be used when recording the metric
	Histogram(name string, help string, buckets []float64, labels ...string) Histogram

	// Gauge creates or retrieves a gauge metric.
	// A gauge is a metric that represents a single numerical value that can arbitrarily go up and down.
	// Gauges are typically used for measured values like temperatures or current memory usage.
	//
	// Parameters:
	//   - name: The name of the metric
	//   - help: A description of the metric
	//   - labels: Optional label names that can be used when recording the metric
	Gauge(name string, help string, labels ...string) Gauge

	// Handler returns an HTTP handler for the metrics endpoint.
	// This handler can be registered with an HTTP server to expose metrics in Prometheus format.
	// The default endpoint is /metrics.
	Handler() http.Handler

	// Registry returns the underlying Prometheus registry.
	// This can be used for advanced use cases where direct access to the registry is needed.
	Registry() *prometheus.Registry
}

// MetricsConfig contains configuration for metrics
type MetricsConfig struct {
	// Enabled determines if metrics collection is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Path is the HTTP path for the metrics endpoint
	Path string `json:"path" yaml:"path" validate:"required"`

	// Namespace is the namespace prefix for all metrics
	Namespace string `json:"namespace" yaml:"namespace"`

	// Subsystem is the subsystem prefix for all metrics
	Subsystem string `json:"subsystem" yaml:"subsystem"`

	// ServiceName is the name of the service
	ServiceName string `json:"service_name" yaml:"service_name"`

	// ServiceVersion is the version of the service
	ServiceVersion string `json:"service_version" yaml:"service_version"`

	// ServiceInstance is the instance ID of the service
	ServiceInstance string `json:"service_instance" yaml:"service_instance"`
}

// DefaultMetricsConfig returns the default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:   true,
		Path:      "/metrics",
		Namespace: "microlib",
		Subsystem: "service",
	}
}

// prometheusMetrics implements the Metrics interface using Prometheus
type prometheusMetrics struct {
	registry   *prometheus.Registry
	config     MetricsConfig
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
}

// NewMetrics creates a new metrics instance with the default configuration
func NewMetrics() Metrics {
	return NewMetricsWithConfig(DefaultMetricsConfig())
}

// NewMetricsWithConfig creates a new metrics instance with the provided configuration
func NewMetricsWithConfig(config MetricsConfig) Metrics {
	registry := prometheus.NewRegistry()

	m := &prometheusMetrics{
		registry:   registry,
		config:     config,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
	}

	// Register default metrics if enabled
	if config.Enabled {
		m.registerDefaultMetrics()
	}

	return m
}

// registerDefaultMetrics registers common service metrics
func (m *prometheusMetrics) registerDefaultMetrics() {
	// HTTP request metrics
	m.Counter("http_requests_total", "Total number of HTTP requests", "method", "path", "status")
	m.Histogram("http_request_duration_seconds", "HTTP request duration in seconds",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		"method", "path", "status")
	m.Counter("http_request_errors_total", "Total number of HTTP request errors", "method", "path", "status")

	// Service metrics
	m.Gauge("service_info", "Service information", "name", "version", "instance")
	m.Counter("service_started_total", "Total number of service starts", "name", "version")

	// Set service info
	if m.config.ServiceName != "" {
		serviceInfo := m.Gauge("service_info", "Service information", "name", "version", "instance")
		serviceInfo.WithLabels(map[string]string{
			"name":     m.config.ServiceName,
			"version":  m.config.ServiceVersion,
			"instance": m.config.ServiceInstance,
		}).Set(1)
	}
}

// Counter creates or retrieves a counter metric
func (m *prometheusMetrics) Counter(name string, help string, labels ...string) Counter {
	if !m.config.Enabled {
		return &noopCounter{}
	}

	fullName := m.buildMetricName(name)

	if counter, exists := m.counters[fullName]; exists {
		return &prometheusCounter{counter: counter}
	}

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      name,
			Help:      help,
		},
		labels,
	)

	m.registry.MustRegister(counter)
	m.counters[fullName] = counter

	return &prometheusCounter{counter: counter, labelNames: labels}
}

// Histogram creates or retrieves a histogram metric
func (m *prometheusMetrics) Histogram(name string, help string, buckets []float64, labels ...string) Histogram {
	if !m.config.Enabled {
		return &noopHistogram{}
	}

	fullName := m.buildMetricName(name)

	if histogram, exists := m.histograms[fullName]; exists {
		return &prometheusHistogram{histogram: histogram, labelNames: labels}
	}

	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      name,
			Help:      help,
			Buckets:   buckets,
		},
		labels,
	)

	m.registry.MustRegister(histogram)
	m.histograms[fullName] = histogram

	return &prometheusHistogram{histogram: histogram, labelNames: labels}
}

// Gauge creates or retrieves a gauge metric
func (m *prometheusMetrics) Gauge(name string, help string, labels ...string) Gauge {
	if !m.config.Enabled {
		return &noopGauge{}
	}

	fullName := m.buildMetricName(name)

	if gauge, exists := m.gauges[fullName]; exists {
		return &prometheusGauge{gauge: gauge, labelNames: labels}
	}

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      name,
			Help:      help,
		},
		labels,
	)

	m.registry.MustRegister(gauge)
	m.gauges[fullName] = gauge

	return &prometheusGauge{gauge: gauge, labelNames: labels}
}

// Handler returns an HTTP handler for the metrics endpoint
func (m *prometheusMetrics) Handler() http.Handler {
	if !m.config.Enabled {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	}

	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Registry returns the underlying Prometheus registry
func (m *prometheusMetrics) Registry() *prometheus.Registry {
	return m.registry
}

// buildMetricName builds the full metric name with namespace and subsystem
func (m *prometheusMetrics) buildMetricName(name string) string {
	if m.config.Namespace != "" && m.config.Subsystem != "" {
		return m.config.Namespace + "_" + m.config.Subsystem + "_" + name
	} else if m.config.Namespace != "" {
		return m.config.Namespace + "_" + name
	} else if m.config.Subsystem != "" {
		return m.config.Subsystem + "_" + name
	}
	return name
}

// prometheusCounter implements the Counter interface using Prometheus
type prometheusCounter struct {
	counter    *prometheus.CounterVec
	labelNames []string
}

func (c *prometheusCounter) Inc() {
	// For counters without specific labels, use empty label values
	if len(c.labelNames) == 0 {
		// If there are no labels, use the counter directly
		c.counter.WithLabelValues().Inc()
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(c.labelNames))
		c.counter.WithLabelValues(labelValues...).Inc()
	}
}

func (c *prometheusCounter) Add(value float64) {
	// For counters without specific labels, use empty label values
	if len(c.labelNames) == 0 {
		// If there are no labels, use the counter directly
		c.counter.WithLabelValues().Add(value)
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(c.labelNames))
		c.counter.WithLabelValues(labelValues...).Add(value)
	}
}

func (c *prometheusCounter) WithLabels(labels map[string]string) Counter {
	// If no label names are defined, return the counter itself
	if len(c.labelNames) == 0 {
		return c
	}

	labelValues := make([]string, len(c.labelNames))
	for i, name := range c.labelNames {
		if value, ok := labels[name]; ok {
			labelValues[i] = value
		}
	}
	return &prometheusCounterWithLabels{
		counter: c.counter.WithLabelValues(labelValues...),
	}
}

// prometheusCounterWithLabels is a counter with specific labels
type prometheusCounterWithLabels struct {
	counter prometheus.Counter
}

func (c *prometheusCounterWithLabels) Inc() {
	c.counter.Inc()
}

func (c *prometheusCounterWithLabels) Add(value float64) {
	c.counter.Add(value)
}

func (c *prometheusCounterWithLabels) WithLabels(labels map[string]string) Counter {
	// This implementation doesn't support nested labels
	return c
}

// prometheusHistogram implements the Histogram interface using Prometheus
type prometheusHistogram struct {
	histogram  *prometheus.HistogramVec
	labelNames []string
}

func (h *prometheusHistogram) Observe(value float64) {
	// For histograms without specific labels, use empty label values
	if len(h.labelNames) == 0 {
		// If there are no labels, use the histogram directly
		h.histogram.WithLabelValues().Observe(value)
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(h.labelNames))
		h.histogram.WithLabelValues(labelValues...).Observe(value)
	}
}

func (h *prometheusHistogram) WithLabels(labels map[string]string) Histogram {
	// If no label names are defined, return the histogram itself
	if len(h.labelNames) == 0 {
		return h
	}

	labelValues := make([]string, len(h.labelNames))
	for i, name := range h.labelNames {
		if value, ok := labels[name]; ok {
			labelValues[i] = value
		}
	}
	return &prometheusHistogramWithLabels{
		histogram: h.histogram.WithLabelValues(labelValues...),
	}
}

// prometheusHistogramWithLabels is a histogram with specific labels
type prometheusHistogramWithLabels struct {
	histogram prometheus.Observer
}

func (h *prometheusHistogramWithLabels) Observe(value float64) {
	h.histogram.Observe(value)
}

func (h *prometheusHistogramWithLabels) WithLabels(labels map[string]string) Histogram {
	// This implementation doesn't support nested labels
	return h
}

// prometheusGauge implements the Gauge interface using Prometheus
type prometheusGauge struct {
	gauge      *prometheus.GaugeVec
	labelNames []string
}

func (g *prometheusGauge) Set(value float64) {
	// For gauges without specific labels, use empty label values
	if len(g.labelNames) == 0 {
		// If there are no labels, use the gauge directly
		g.gauge.WithLabelValues().Set(value)
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(g.labelNames))
		g.gauge.WithLabelValues(labelValues...).Set(value)
	}
}

func (g *prometheusGauge) Inc() {
	// For gauges without specific labels, use empty label values
	if len(g.labelNames) == 0 {
		// If there are no labels, use the gauge directly
		g.gauge.WithLabelValues().Inc()
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(g.labelNames))
		g.gauge.WithLabelValues(labelValues...).Inc()
	}
}

func (g *prometheusGauge) Dec() {
	// For gauges without specific labels, use empty label values
	if len(g.labelNames) == 0 {
		// If there are no labels, use the gauge directly
		g.gauge.WithLabelValues().Dec()
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(g.labelNames))
		g.gauge.WithLabelValues(labelValues...).Dec()
	}
}

func (g *prometheusGauge) Add(value float64) {
	// For gauges without specific labels, use empty label values
	if len(g.labelNames) == 0 {
		// If there are no labels, use the gauge directly
		g.gauge.WithLabelValues().Add(value)
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(g.labelNames))
		g.gauge.WithLabelValues(labelValues...).Add(value)
	}
}

func (g *prometheusGauge) Sub(value float64) {
	// For gauges without specific labels, use empty label values
	if len(g.labelNames) == 0 {
		// If there are no labels, use the gauge directly
		g.gauge.WithLabelValues().Sub(value)
	} else {
		// Otherwise, use empty label values
		labelValues := make([]string, len(g.labelNames))
		g.gauge.WithLabelValues(labelValues...).Sub(value)
	}
}

func (g *prometheusGauge) WithLabels(labels map[string]string) Gauge {
	// If no label names are defined, return the gauge itself
	if len(g.labelNames) == 0 {
		return g
	}

	labelValues := make([]string, len(g.labelNames))
	for i, name := range g.labelNames {
		if value, ok := labels[name]; ok {
			labelValues[i] = value
		}
	}
	return &prometheusGaugeWithLabels{
		gauge: g.gauge.WithLabelValues(labelValues...),
	}
}

// prometheusGaugeWithLabels is a gauge with specific labels
type prometheusGaugeWithLabels struct {
	gauge prometheus.Gauge
}

func (g *prometheusGaugeWithLabels) Set(value float64) {
	g.gauge.Set(value)
}

func (g *prometheusGaugeWithLabels) Inc() {
	g.gauge.Inc()
}

func (g *prometheusGaugeWithLabels) Dec() {
	g.gauge.Dec()
}

func (g *prometheusGaugeWithLabels) Add(value float64) {
	g.gauge.Add(value)
}

func (g *prometheusGaugeWithLabels) Sub(value float64) {
	g.gauge.Sub(value)
}

func (g *prometheusGaugeWithLabels) WithLabels(labels map[string]string) Gauge {
	// This implementation doesn't support nested labels
	return g
}

// No-op implementations for when metrics are disabled

type noopCounter struct{}

func (c *noopCounter) Inc()                                        {}
func (c *noopCounter) Add(value float64)                           {}
func (c *noopCounter) WithLabels(labels map[string]string) Counter { return c }

type noopHistogram struct{}

func (h *noopHistogram) Observe(value float64)                         {}
func (h *noopHistogram) WithLabels(labels map[string]string) Histogram { return h }

type noopGauge struct{}

func (g *noopGauge) Set(value float64)                         {}
func (g *noopGauge) Inc()                                      {}
func (g *noopGauge) Dec()                                      {}
func (g *noopGauge) Add(value float64)                         {}
func (g *noopGauge) Sub(value float64)                         {}
func (g *noopGauge) WithLabels(labels map[string]string) Gauge { return g }

// HTTPMetricsMiddleware creates middleware for automatic HTTP metrics collection.
// This middleware automatically collects the following metrics for each HTTP request:
//   - http_requests_total: Counter of total requests with labels for method, path, and status code
//   - http_request_duration_seconds: Histogram of request durations with labels for method, path, and status code
//   - http_request_errors_total: Counter of requests with 4xx or 5xx status codes
//
// Usage:
//
//	router := http.NewServeMux()
//	metrics := observability.NewMetrics()
//	router.Handle("/metrics", metrics.Handler())
//
//	// Apply the middleware to your handlers
//	wrappedHandler := observability.HTTPMetricsMiddleware(metrics)(yourHandler)
//	router.Handle("/api", wrappedHandler)
//
// Parameters:
//   - metrics: The Metrics instance to use for recording metrics
//
// Returns:
//   - A middleware function that can be applied to an http.Handler
func HTTPMetricsMiddleware(metrics Metrics) func(http.Handler) http.Handler {
	// Create a custom implementation for the test to avoid label cardinality issues
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture the status code
			wrapper := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process the request
			next.ServeHTTP(wrapper, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Extract labels
			method := r.Method
			path := r.URL.Path
			status := strconv.Itoa(wrapper.statusCode)

			// Record metrics
			// Create metrics on-demand to avoid label cardinality issues
			requestsTotal := metrics.Counter("http_requests_total", "Total number of HTTP requests", "method", "path", "status")
			requestsTotal.WithLabels(map[string]string{
				"method": method,
				"path":   path,
				"status": status,
			}).Inc()

			requestDuration := metrics.Histogram("http_request_duration_seconds", "HTTP request duration in seconds",
				[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				"method", "path", "status")
			requestDuration.WithLabels(map[string]string{
				"method": method,
				"path":   path,
				"status": status,
			}).Observe(duration)

			// Record errors (4xx and 5xx status codes)
			if wrapper.statusCode >= 400 {
				requestErrors := metrics.Counter("http_request_errors_total", "Total number of HTTP request errors", "method", "path", "status")
				requestErrors.WithLabels(map[string]string{
					"method": method,
					"path":   path,
					"status": status,
				}).Inc()
			}
		})
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture the status code
// and implement additional interfaces that might be implemented by the wrapped ResponseWriter
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// WriteHeader captures the status code and delegates to the wrapped ResponseWriter
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the number of bytes written and delegates to the wrapped ResponseWriter
func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// Flush implements http.Flusher if the wrapped ResponseWriter implements it
func (w *responseWriterWrapper) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker if the wrapped ResponseWriter implements it
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// Push implements http.Pusher if the wrapped ResponseWriter implements it
func (w *responseWriterWrapper) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("underlying ResponseWriter does not implement http.Pusher")
}

// GlobalMetrics is a package-level metrics instance for convenience.
// This allows for easy access to metrics functionality without having to pass a metrics instance around.
var GlobalMetrics Metrics

// Initialize the global metrics with default configuration
func init() {
	GlobalMetrics = NewMetrics()
}

// SetGlobalMetrics sets the global metrics instance.
// This can be used to configure the global metrics with custom settings.
//
// Example:
//
//	config := observability.MetricsConfig{
//	    Namespace: "myapp",
//	    Subsystem: "api",
//	    ServiceName: "user-service",
//	}
//	customMetrics := observability.NewMetricsWithConfig(config)
//	observability.SetGlobalMetrics(customMetrics)
func SetGlobalMetrics(metrics Metrics) {
	GlobalMetrics = metrics
}

// GetCounter creates or retrieves a counter metric using the global metrics instance.
// This is a convenience function that uses the GlobalMetrics instance.
//
// Example:
//
//	requestCounter := observability.GetCounter("requests_total", "Total number of requests", "method", "path")
//	requestCounter.WithLabels(map[string]string{"method": "GET", "path": "/users"}).Inc()
func GetCounter(name string, help string, labels ...string) Counter {
	return GlobalMetrics.Counter(name, help, labels...)
}

// GetHistogram creates or retrieves a histogram metric using the global metrics instance.
// This is a convenience function that uses the GlobalMetrics instance.
//
// Example:
//
//	latencyHistogram := observability.GetHistogram("request_duration_seconds", "Request duration in seconds",
//	    []float64{0.01, 0.05, 0.1, 0.5, 1, 5}, "method", "path")
//	latencyHistogram.WithLabels(map[string]string{"method": "GET", "path": "/users"}).Observe(0.42)
func GetHistogram(name string, help string, buckets []float64, labels ...string) Histogram {
	return GlobalMetrics.Histogram(name, help, buckets, labels...)
}

// GetGauge creates or retrieves a gauge metric using the global metrics instance.
// This is a convenience function that uses the GlobalMetrics instance.
//
// Example:
//
//	activeConnectionsGauge := observability.GetGauge("active_connections", "Number of active connections")
//	activeConnectionsGauge.Inc() // Increment when a connection is established
//	activeConnectionsGauge.Dec() // Decrement when a connection is closed
func GetGauge(name string, help string, labels ...string) Gauge {
	return GlobalMetrics.Gauge(name, help, labels...)
}

// MetricsHandler returns an HTTP handler for the metrics endpoint using the global metrics instance.
// This handler can be registered with an HTTP server to expose metrics in Prometheus format.
//
// Example:
//
//	router := http.NewServeMux()
//	router.Handle("/metrics", observability.MetricsHandler())
func MetricsHandler() http.Handler {
	return GlobalMetrics.Handler()
}

// configInterface is an interface that matches the minimal requirements for the MetricsManager
type configInterface interface {
	Get() interface{}
	Update(interface{}) error
	Watch(func(interface{})) error
}

// MetricsManager manages metrics and provides dynamic configuration updates
type MetricsManager struct {
	config  configInterface
	metrics Metrics
	mu      sync.RWMutex
}

// NewMetricsManager creates a new metrics manager
func NewMetricsManager(cfg configInterface) (*MetricsManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Get the observability configuration
	obsConfig, ok := cfg.Get().(*ObservabilityConfig)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	// Create the metrics instance with the configuration
	metrics := NewMetricsWithConfig(obsConfig.Metrics)

	return &MetricsManager{
		config:  cfg,
		metrics: metrics,
	}, nil
}

// GetMetrics returns the metrics instance
func (m *MetricsManager) GetMetrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// UpdateMetricsConfig updates the metrics configuration
func (m *MetricsManager) UpdateMetricsConfig(metricsConfig MetricsConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the current configuration
	obsConfig, ok := m.config.Get().(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	// Update the metrics configuration
	obsConfig.Metrics = metricsConfig

	// Update the configuration
	if err := m.config.Update(obsConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	return nil
}

// Reload implements the config.Reloadable interface
func (m *MetricsManager) Reload(newConfig interface{}) error {
	obsConfig, ok := newConfig.(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a new metrics instance with the updated configuration
	m.metrics = NewMetricsWithConfig(obsConfig.Metrics)

	// Update the global metrics instance
	SetGlobalMetrics(m.metrics)

	return nil
}

// RegisterMetricsHandler registers the metrics handler with the provided HTTP server
// This is a convenience method for exposing the metrics endpoint
func (m *MetricsManager) RegisterMetricsHandler(mux *http.ServeMux) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get the metrics configuration
	obsConfig, ok := m.config.Get().(*ObservabilityConfig)
	if !ok || !obsConfig.Metrics.Enabled {
		return
	}

	// Register the metrics handler at the configured path
	path := obsConfig.Metrics.Path
	if path == "" {
		path = "/metrics"
	}

	mux.Handle(path, m.metrics.Handler())
}
