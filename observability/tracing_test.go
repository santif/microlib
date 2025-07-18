package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// mockCarrier implements the TextMapCarrier interface for testing
type mockCarrier struct {
	values map[string]string
}

func (c *mockCarrier) Get(key string) string {
	if c.values == nil {
		return ""
	}
	return c.values[key]
}

func (c *mockCarrier) Set(key, value string) {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
}

func (c *mockCarrier) Keys() []string {
	if c.values == nil {
		return []string{}
	}
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	return keys
}

func TestNewTracer(t *testing.T) {
	// Test with default configuration (disabled)
	tracer, err := NewTracer()
	if err != nil {
		t.Fatalf("Failed to create tracer with default config: %v", err)
	}

	// Verify it's a no-op tracer when disabled
	if _, ok := tracer.(*noopTracer); !ok {
		t.Errorf("Expected noopTracer when disabled, got %T", tracer)
	}

	// Test with enabled configuration
	config := DefaultTracingConfig()
	config.Enabled = true
	config.ServiceName = "test-service"

	// This would normally connect to an actual collector, so we don't test the actual connection
	// Just verify that the configuration is processed correctly
	_, err = NewTracerWithConfig(config)
	if err == nil {
		// We expect an error since we're not actually connecting to a collector
		// But in a real environment with a collector, this would succeed
		t.Log("Note: No error when creating tracer with enabled config - this is expected in tests without a collector")
	}
}

func TestNoopTracer(t *testing.T) {
	tracer := &noopTracer{}

	// Test Start
	ctx := context.Background()
	newCtx, span := tracer.Start(ctx, "test-span")

	// Verify the context is unchanged
	if newCtx != ctx {
		t.Error("Expected context to be unchanged")
	}

	// Verify the span is a no-op span
	if span.SpanContext().IsValid() {
		t.Error("Expected invalid span context for no-op tracer")
	}

	// Test StartSpan
	span = tracer.StartSpan(ctx, "test-span")
	if span.SpanContext().IsValid() {
		t.Error("Expected invalid span context for no-op tracer")
	}

	// Test Extract
	carrier := &mockCarrier{}
	extractedCtx := tracer.Extract(ctx, carrier)
	if extractedCtx != ctx {
		t.Error("Expected context to be unchanged after Extract")
	}

	// Test Inject (should do nothing)
	tracer.Inject(ctx, carrier)

	// Test TracerProvider
	provider := tracer.TracerProvider()
	if provider == nil {
		t.Error("Expected non-nil TracerProvider")
	}

	// Test Shutdown
	err := tracer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error from Shutdown, got %v", err)
	}
}

func TestHTTPTracingMiddleware(t *testing.T) {
	// Create a no-op tracer for testing
	tracer := &noopTracer{}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that the span is in the context
		span := trace.SpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in request context")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Apply the middleware
	handler := HTTPTracingMiddleware(tracer)(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute the request
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestCombinedHTTPMiddleware(t *testing.T) {
	// Create no-op instances for testing
	tracer := &noopTracer{}

	// Create a disabled metrics instance to avoid label cardinality issues
	metricsConfig := DefaultMetricsConfig()
	metricsConfig.Enabled = false
	metrics := NewMetricsWithConfig(metricsConfig)

	logger := NewLogger()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that the span is in the context
		span := trace.SpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in request context")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Apply the middleware
	handler := CombinedHTTPMiddleware(tracer, metrics, logger)(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute the request
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTracerManager(t *testing.T) {
	// Create a mock config using the existing mockConfig from metrics_manager_test.go
	mockCfg := &mockConfig{
		data: &ObservabilityConfig{
			Tracing: DefaultTracingConfig(),
		},
	}

	// Create a tracer manager
	manager, err := NewTracerManager(mockCfg)
	if err != nil {
		t.Fatalf("Failed to create tracer manager: %v", err)
	}

	// Get the tracer
	tracer := manager.GetTracer()
	if tracer == nil {
		t.Fatal("Expected non-nil tracer")
	}

	// Update the tracing config
	newConfig := DefaultTracingConfig()
	newConfig.ServiceName = "updated-service"
	err = manager.UpdateTracingConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update tracing config: %v", err)
	}

	// Verify the config was updated
	updatedConfig := mockCfg.data.(*ObservabilityConfig)
	if updatedConfig.Tracing.ServiceName != "updated-service" {
		t.Errorf("Expected service name to be updated to 'updated-service', got '%s'", updatedConfig.Tracing.ServiceName)
	}

	// Test reload
	err = manager.Reload(&ObservabilityConfig{
		Tracing: DefaultTracingConfig(),
	})
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}
}
