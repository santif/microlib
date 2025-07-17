package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics instance")
	}
}

func TestNewMetricsWithConfig(t *testing.T) {
	config := MetricsConfig{
		Enabled:         true,
		Path:            "/custom-metrics",
		Namespace:       "test",
		Subsystem:       "unit",
		ServiceName:     "test-service",
		ServiceVersion:  "1.0.0",
		ServiceInstance: "instance-1",
	}

	metrics := NewMetricsWithConfig(config)
	if metrics == nil {
		t.Fatal("Expected non-nil metrics instance")
	}

	// Test that the registry has the service info metric
	handler := metrics.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check that the response contains the service info metric
	body := w.Body.String()
	if !strings.Contains(body, "test_unit_service_info") {
		t.Errorf("Expected response to contain service info metric, got: %s", body)
	}
}

func TestCounter(t *testing.T) {
	metrics := NewMetrics()
	counter := metrics.Counter("test_counter", "Test counter", "label1", "label2")

	// Test Inc
	counter.Inc()

	// Test Add
	counter.Add(5)

	// Test WithLabels
	labeledCounter := counter.WithLabels(map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	labeledCounter.Inc()
	labeledCounter.Add(10)

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
	if !strings.Contains(body, "microlib_service_test_counter") {
		t.Errorf("Expected response to contain counter metric, got: %s", body)
	}

	// Verify that labels are correctly applied
	if !strings.Contains(body, `label1="value1"`) {
		t.Errorf("Expected response to contain label1=\"value1\", got: %s", body)
	}
	if !strings.Contains(body, `label2="value2"`) {
		t.Errorf("Expected response to contain label2=\"value2\", got: %s", body)
	}
}

func TestHistogram(t *testing.T) {
	metrics := NewMetrics()
	histogram := metrics.Histogram("test_histogram", "Test histogram",
		[]float64{0.01, 0.1, 1, 10}, "label1", "label2")

	// Test Observe
	histogram.Observe(0.5)

	// Test WithLabels
	labeledHistogram := histogram.WithLabels(map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	labeledHistogram.Observe(5)

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
	if !strings.Contains(body, "microlib_service_test_histogram") {
		t.Errorf("Expected response to contain histogram metric, got: %s", body)
	}

	// Verify that labels are correctly applied
	if !strings.Contains(body, `label1="value1"`) {
		t.Errorf("Expected response to contain label1=\"value1\", got: %s", body)
	}
	if !strings.Contains(body, `label2="value2"`) {
		t.Errorf("Expected response to contain label2=\"value2\", got: %s", body)
	}
}

func TestGauge(t *testing.T) {
	metrics := NewMetrics()
	gauge := metrics.Gauge("test_gauge", "Test gauge", "label1", "label2")

	// Test Set
	gauge.Set(42)

	// Test Inc
	gauge.Inc()

	// Test Dec
	gauge.Dec()

	// Test Add
	gauge.Add(10)

	// Test Sub
	gauge.Sub(5)

	// Test WithLabels
	labeledGauge := gauge.WithLabels(map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	labeledGauge.Set(100)

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
	if !strings.Contains(body, "microlib_service_test_gauge") {
		t.Errorf("Expected response to contain gauge metric, got: %s", body)
	}

	// Verify that labels are correctly applied
	if !strings.Contains(body, `label1="value1"`) {
		t.Errorf("Expected response to contain label1=\"value1\", got: %s", body)
	}
	if !strings.Contains(body, `label2="value2"`) {
		t.Errorf("Expected response to contain label2=\"value2\", got: %s", body)
	}
}

func TestHTTPMetricsMiddleware(t *testing.T) {
	t.Skip("This test is replaced by TestMetricsEndpoint in http_metrics_test.go")
}

func TestDisabledMetrics(t *testing.T) {
	config := MetricsConfig{
		Enabled: false,
	}
	metrics := NewMetricsWithConfig(config)

	// Test that disabled metrics return no-op implementations
	counter := metrics.Counter("test_counter", "Test counter")
	counter.Inc() // Should not panic

	histogram := metrics.Histogram("test_histogram", "Test histogram", []float64{0.1, 1, 10})
	histogram.Observe(0.5) // Should not panic

	gauge := metrics.Gauge("test_gauge", "Test gauge")
	gauge.Set(42) // Should not panic

	// Test that the metrics endpoint returns 404 when disabled
	handler := metrics.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestGlobalMetrics(t *testing.T) {
	// Test global metrics functions
	counter := GetCounter("global_counter", "Global counter", "label")
	counter.Inc()

	histogram := GetHistogram("global_histogram", "Global histogram",
		[]float64{0.01, 0.1, 1, 10}, "label")
	histogram.Observe(0.5)

	gauge := GetGauge("global_gauge", "Global gauge", "label")
	gauge.Set(42)

	// Test setting custom global metrics
	customMetrics := NewMetricsWithConfig(MetricsConfig{
		Enabled:   true,
		Namespace: "custom",
		Subsystem: "global",
	})
	SetGlobalMetrics(customMetrics)

	// Verify the global metrics were changed
	if GlobalMetrics != customMetrics {
		t.Error("Expected GlobalMetrics to be updated")
	}

	// Test the metrics handler
	handler := MetricsHandler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
