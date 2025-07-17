package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsEndpoint tests that the metrics endpoint works correctly
func TestMetricsEndpoint(t *testing.T) {
	// Create metrics with a unique namespace to avoid conflicts
	config := MetricsConfig{
		Enabled:   true,
		Path:      "/custom-metrics",
		Namespace: "test_endpoint",
		Subsystem: "metrics",
	}
	metrics := NewMetricsWithConfig(config)

	// Create a simple counter without labels
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "test_endpoint",
		Subsystem: "metrics",
		Name:      "simple_counter",
		Help:      "A simple counter without labels",
	})

	// Register the counter with the registry
	metrics.Registry().MustRegister(counter)

	// Increment the counter
	counter.Inc()

	// Verify metrics are exposed
	handler := metrics.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test_endpoint_metrics_simple_counter") {
		t.Errorf("Expected response to contain simple_counter metric, got: %s", body)
	}
}
