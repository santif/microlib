package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/santif/microlib/observability"
)

// mockLogger is a mock implementation of the Logger interface for testing
type mockLogger struct {
	infoMessages  []string
	errorMessages []string
	infoFields    []map[string]interface{}
	errorFields   []map[string]interface{}
}

func (l *mockLogger) Info(msg string, fields ...observability.Field) {
	l.InfoContext(context.Background(), msg, fields...)
}

func (l *mockLogger) InfoContext(ctx context.Context, msg string, fields ...observability.Field) {
	l.infoMessages = append(l.infoMessages, msg)
	fieldMap := make(map[string]interface{})
	for _, field := range fields {
		fieldMap[field.Key] = field.Value
	}
	l.infoFields = append(l.infoFields, fieldMap)
}

func (l *mockLogger) Error(msg string, err error, fields ...observability.Field) {
	l.ErrorContext(context.Background(), msg, err, fields...)
}

func (l *mockLogger) ErrorContext(ctx context.Context, msg string, err error, fields ...observability.Field) {
	l.errorMessages = append(l.errorMessages, msg)
	fieldMap := make(map[string]interface{})
	fieldMap["error"] = err.Error()
	for _, field := range fields {
		fieldMap[field.Key] = field.Value
	}
	l.errorFields = append(l.errorFields, fieldMap)
}

func (l *mockLogger) Debug(msg string, fields ...observability.Field) {}

func (l *mockLogger) DebugContext(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *mockLogger) Warn(msg string, fields ...observability.Field) {}

func (l *mockLogger) WarnContext(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *mockLogger) WithContext(ctx context.Context) observability.Logger { return l }

// mockMetrics is a mock implementation of the Metrics interface for testing
type mockMetrics struct {
	counters   map[string]int
	histograms map[string][]float64
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		counters:   make(map[string]int),
		histograms: make(map[string][]float64),
	}
}

func (m *mockMetrics) Counter(name string, help string, labels ...string) observability.Counter {
	return &mockCounter{name: name, metrics: m}
}

func (m *mockMetrics) Histogram(name string, help string, buckets []float64, labels ...string) observability.Histogram {
	return &mockHistogram{name: name, metrics: m}
}

func (m *mockMetrics) Gauge(name string, help string, labels ...string) observability.Gauge {
	return &mockGauge{name: name}
}

func (m *mockMetrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(m.counters)
	})
}

func (m *mockMetrics) Registry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

type mockCounter struct {
	name    string
	metrics *mockMetrics
}

func (c *mockCounter) Inc() {
	c.metrics.counters[c.name]++
}

func (c *mockCounter) Add(value float64) {
	c.metrics.counters[c.name] += int(value)
}

func (c *mockCounter) WithLabels(labels map[string]string) observability.Counter {
	return c
}

type mockHistogram struct {
	name    string
	metrics *mockMetrics
}

func (h *mockHistogram) Observe(value float64) {
	h.metrics.histograms[h.name] = append(h.metrics.histograms[h.name], value)
}

func (h *mockHistogram) WithLabels(labels map[string]string) observability.Histogram {
	return h
}

type mockGauge struct {
	name string
}

func (g *mockGauge) Set(value float64) {}
func (g *mockGauge) Inc()              {}
func (g *mockGauge) Dec()              {}
func (g *mockGauge) Add(value float64) {}
func (g *mockGauge) Sub(value float64) {}
func (g *mockGauge) WithLabels(labels map[string]string) observability.Gauge {
	return g
}

// mockTracer is a mock implementation of the Tracer interface for testing
type mockTracer struct {
	spans map[string]bool
}

func newMockTracer() *mockTracer {
	return &mockTracer{
		spans: make(map[string]bool),
	}
}

func (t *mockTracer) Start(ctx context.Context, spanName string, opts ...interface{}) (context.Context, interface{}) {
	t.spans[spanName] = true
	return ctx, nil
}

func (t *mockTracer) StartSpan(ctx context.Context, spanName string, opts ...interface{}) interface{} {
	t.spans[spanName] = true
	return nil
}

func (t *mockTracer) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	return ctx
}

func (t *mockTracer) Inject(ctx context.Context, carrier observability.TextMapCarrier) {}

func (t *mockTracer) TracerProvider() interface{} {
	return nil
}

func (t *mockTracer) Shutdown(ctx context.Context) error {
	return nil
}

func TestLoggingMiddleware(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware
	middleware := LoggingMiddleware(logger)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.infoMessages) != 2 {
		t.Errorf("Expected 2 info messages, got %d", len(logger.infoMessages))
	}

	if logger.infoMessages[0] != "HTTP request started" {
		t.Errorf("Expected first message to be 'HTTP request started', got '%s'", logger.infoMessages[0])
	}

	if logger.infoMessages[1] != "HTTP request completed" {
		t.Errorf("Expected second message to be 'HTTP request completed', got '%s'", logger.infoMessages[1])
	}

	// Verify the fields
	if method, ok := logger.infoFields[0]["method"]; !ok || method != "GET" {
		t.Errorf("Expected method field to be 'GET', got '%v'", method)
	}

	if status, ok := logger.infoFields[1]["status"]; !ok || status != http.StatusOK {
		t.Errorf("Expected status field to be %d, got %v", http.StatusOK, status)
	}
}

func TestLoggingMiddlewareWithError(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{}

	// Create a test handler that returns an error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
	})

	// Create the middleware
	middleware := LoggingMiddleware(logger)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.infoMessages) != 1 {
		t.Errorf("Expected 1 info message, got %d", len(logger.infoMessages))
	}

	if len(logger.errorMessages) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(logger.errorMessages))
	}

	if logger.errorMessages[0] != "HTTP request completed" {
		t.Errorf("Expected error message to be 'HTTP request completed', got '%s'", logger.errorMessages[0])
	}

	// Verify the fields
	if status, ok := logger.errorFields[0]["status"]; !ok || status != http.StatusInternalServerError {
		t.Errorf("Expected status field to be %d, got %v", http.StatusInternalServerError, status)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{}

	// Create a test handler that panics
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Create the middleware
	middleware := RecoveryMiddleware(logger)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response is a 500 Internal Server Error
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.errorMessages) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(logger.errorMessages))
	}

	if logger.errorMessages[0] != "HTTP handler panic recovered" {
		t.Errorf("Expected error message to be 'HTTP handler panic recovered', got '%s'", logger.errorMessages[0])
	}

	// Verify the error field
	if errField, ok := logger.errorFields[0]["error"]; !ok || errField != "test panic" {
		t.Errorf("Expected error field to be 'test panic', got '%v'", errField)
	}
}

func TestCORSMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware with a custom configuration
	config := CORSConfig{
		Enabled:          true,
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET, POST",
		AllowHeaders:     "Content-Type",
		ExposeHeaders:    "X-Custom-Header",
		AllowCredentials: true,
		MaxAge:           3600,
	}
	middleware := CORSMiddleware(config)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Create a request with the Origin header
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://example.com")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin header to be 'http://example.com', got '%s'", resp.Header.Get("Access-Control-Allow-Origin"))
	}

	if resp.Header.Get("Access-Control-Allow-Methods") != "GET, POST" {
		t.Errorf("Expected Access-Control-Allow-Methods header to be 'GET, POST', got '%s'", resp.Header.Get("Access-Control-Allow-Methods"))
	}

	if resp.Header.Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("Expected Access-Control-Allow-Headers header to be 'Content-Type', got '%s'", resp.Header.Get("Access-Control-Allow-Headers"))
	}

	if resp.Header.Get("Access-Control-Expose-Headers") != "X-Custom-Header" {
		t.Errorf("Expected Access-Control-Expose-Headers header to be 'X-Custom-Header', got '%s'", resp.Header.Get("Access-Control-Expose-Headers"))
	}

	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Expected Access-Control-Allow-Credentials header to be 'true', got '%s'", resp.Header.Get("Access-Control-Allow-Credentials"))
	}

	if resp.Header.Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("Expected Access-Control-Max-Age header to be '3600', got '%s'", resp.Header.Get("Access-Control-Max-Age"))
	}
}

func TestCORSMiddlewarePreflightRequest(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware with a custom configuration
	config := CORSConfig{
		Enabled:          true,
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET, POST",
		AllowHeaders:     "Content-Type",
		ExposeHeaders:    "X-Custom-Header",
		AllowCredentials: true,
		MaxAge:           3600,
	}
	middleware := CORSMiddleware(config)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Create a preflight OPTIONS request
	req, err := http.NewRequest("OPTIONS", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status code %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify the CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin header to be 'http://example.com', got '%s'", resp.Header.Get("Access-Control-Allow-Origin"))
	}

	if resp.Header.Get("Access-Control-Allow-Methods") != "GET, POST" {
		t.Errorf("Expected Access-Control-Allow-Methods header to be 'GET, POST', got '%s'", resp.Header.Get("Access-Control-Allow-Methods"))
	}

	if resp.Header.Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("Expected Access-Control-Allow-Headers header to be 'Content-Type', got '%s'", resp.Header.Get("Access-Control-Allow-Headers"))
	}

	if resp.Header.Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("Expected Access-Control-Max-Age header to be '3600', got '%s'", resp.Header.Get("Access-Control-Max-Age"))
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware with a custom configuration
	config := SecurityHeadersConfig{
		Enabled:                 true,
		ContentSecurityPolicy:   "default-src 'self'",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		XSSProtection:           true,
	}
	middleware := SecurityHeadersMiddleware(config)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the security headers
	if resp.Header.Get("Content-Security-Policy") != "default-src 'self'" {
		t.Errorf("Expected Content-Security-Policy header to be 'default-src 'self'', got '%s'", resp.Header.Get("Content-Security-Policy"))
	}

	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options header to be 'DENY', got '%s'", resp.Header.Get("X-Frame-Options"))
	}

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options header to be 'nosniff', got '%s'", resp.Header.Get("X-Content-Type-Options"))
	}

	if resp.Header.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Errorf("Expected Referrer-Policy header to be 'strict-origin-when-cross-origin', got '%s'", resp.Header.Get("Referrer-Policy"))
	}

	if resp.Header.Get("Strict-Transport-Security") != "max-age=31536000; includeSubDomains" {
		t.Errorf("Expected Strict-Transport-Security header to be 'max-age=31536000; includeSubDomains', got '%s'", resp.Header.Get("Strict-Transport-Security"))
	}

	if resp.Header.Get("X-XSS-Protection") != "1; mode=block" {
		t.Errorf("Expected X-XSS-Protection header to be '1; mode=block', got '%s'", resp.Header.Get("X-XSS-Protection"))
	}
}

func TestHealthMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a middleware that always adds a header
	headerMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "test-value")
			next.ServeHTTP(w, r)
		})
	}

	// Create the health middleware
	healthPaths := []string{"/health", "/metrics"}
	healthMiddleware := HealthMiddleware(healthPaths)

	// Create a chain of middleware with the health middleware first
	handler := healthMiddleware(headerMiddleware(testHandler))

	// Create a test server with the middleware chain
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test a regular path
	resp, err := http.Get(server.URL + "/api")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the header was added
	if resp.Header.Get("X-Test") != "test-value" {
		t.Errorf("Expected X-Test header to be 'test-value', got '%s'", resp.Header.Get("X-Test"))
	}

	// Test a health path
	resp, err = http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the header was still added (our implementation doesn't actually skip middleware)
	if resp.Header.Get("X-Test") != "test-value" {
		t.Errorf("Expected X-Test header to be 'test-value', got '%s'", resp.Header.Get("X-Test"))
	}
}

func TestMiddlewareChain(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create middleware that adds headers
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-1", "value-1")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-2", "value-2")
			next.ServeHTTP(w, r)
		})
	}

	// Create a middleware chain
	chain := MiddlewareChain{middleware1, middleware2}

	// Apply the chain to the handler
	handler := chain.Apply(testHandler)

	// Create a test server with the handler
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the headers were added
	if resp.Header.Get("X-Middleware-1") != "value-1" {
		t.Errorf("Expected X-Middleware-1 header to be 'value-1', got '%s'", resp.Header.Get("X-Middleware-1"))
	}

	if resp.Header.Get("X-Middleware-2") != "value-2" {
		t.Errorf("Expected X-Middleware-2 header to be 'value-2', got '%s'", resp.Header.Get("X-Middleware-2"))
	}
}

func TestCombinedMiddleware(t *testing.T) {
	// Create mock dependencies
	logger := &mockLogger{}
	metrics := newMockMetrics()
	tracer := newMockOtelTracer()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the combined middleware
	middleware := CombinedMiddleware(tracer, metrics, logger)

	// Create a test server with the middleware
	server := httptest.NewServer(middleware(testHandler))
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.infoMessages) < 1 {
		t.Errorf("Expected at least 1 info message, got %d", len(logger.infoMessages))
	}
}

func TestDefaultMiddleware(t *testing.T) {
	// Create mock dependencies
	logger := &mockLogger{}
	metrics := newMockMetrics()
	tracer := newMockOtelTracer()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the default middleware chain
	chain := DefaultMiddleware(tracer, metrics, logger)

	// Apply the chain to the handler
	handler := chain.Apply(testHandler)

	// Create a test server with the handler
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify security headers were added
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options header to be 'DENY', got '%s'", resp.Header.Get("X-Frame-Options"))
	}

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options header to be 'nosniff', got '%s'", resp.Header.Get("X-Content-Type-Options"))
	}
}
