package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockConfigInterface is an interface that matches the minimal requirements for the MetricsManager
type mockConfigInterface interface {
	Get() interface{}
	Update(interface{}) error
	Watch(func(interface{})) error
}

// mockConfig is a simple implementation for testing
type mockConfig struct {
	data interface{}
}

func (m *mockConfig) Get() interface{} {
	return m.data
}

func (m *mockConfig) Update(data interface{}) error {
	m.data = data
	return nil
}

func (m *mockConfig) Watch(callback func(interface{})) error {
	return nil
}

func TestNewMetricsManager(t *testing.T) {
	// Create a mock config with observability configuration
	obsConfig := DefaultObservabilityConfig()
	obsConfig.Metrics.Namespace = "test"
	obsConfig.Metrics.Subsystem = "manager"
	
	mockCfg := &mockConfig{data: &obsConfig}
	
	// Create a new metrics manager
	manager, err := NewMetricsManager(mockCfg)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}
	
	// Verify that the metrics instance was created
	metrics := manager.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics instance")
	}
	
	// Verify that the metrics configuration was applied
	handler := metrics.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
	
	// Create a counter to verify the namespace and subsystem
	counter := metrics.Counter("test_counter", "Test counter")
	counter.Inc()
	
	// Check the metrics output
	handler.ServeHTTP(w, req)
	body := w.Body.String()
	
	// The metrics should use the configured namespace and subsystem
	if !strings.Contains(body, "test_manager_") {
		t.Errorf("Expected response to contain metrics with namespace_subsystem prefix, got: %s", body)
	}
}

func TestMetricsManagerReload(t *testing.T) {
	// Create a mock config with observability configuration
	obsConfig := DefaultObservabilityConfig()
	obsConfig.Metrics.Namespace = "test"
	obsConfig.Metrics.Subsystem = "original"
	
	mockCfg := &mockConfig{data: &obsConfig}
	
	// Create a new metrics manager
	manager, err := NewMetricsManager(mockCfg)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}
	
	// Create a new configuration with updated metrics settings
	newObsConfig := DefaultObservabilityConfig()
	newObsConfig.Metrics.Namespace = "test"
	newObsConfig.Metrics.Subsystem = "updated"
	
	// Reload the manager with the new configuration
	err = manager.Reload(&newObsConfig)
	if err != nil {
		t.Fatalf("Failed to reload metrics manager: %v", err)
	}
	
	// Verify that the metrics configuration was updated
	metrics := manager.GetMetrics()
	handler := metrics.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	// Create a counter to verify the new namespace and subsystem
	counter := metrics.Counter("test_counter", "Test counter")
	counter.Inc()
	
	// Check the metrics output
	handler.ServeHTTP(w, req)
	body := w.Body.String()
	
	// The new metrics should use the updated subsystem
	if !strings.Contains(body, "test_updated_") {
		t.Errorf("Expected response to contain metrics with updated namespace_subsystem prefix, got: %s", body)
	}
}

func TestRegisterMetricsHandler(t *testing.T) {
	// Create a mock config with observability configuration
	obsConfig := DefaultObservabilityConfig()
	obsConfig.Metrics.Path = "/custom-metrics"
	
	mockCfg := &mockConfig{data: &obsConfig}
	
	// Create a new metrics manager
	manager, err := NewMetricsManager(mockCfg)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}
	
	// Create a test HTTP server
	mux := http.NewServeMux()
	manager.RegisterMetricsHandler(mux)
	
	// Create a test counter to verify metrics are collected
	metrics := manager.GetMetrics()
	counter := metrics.Counter("test_counter", "Test counter")
	counter.Inc()
	
	// Test the metrics endpoint
	server := httptest.NewServer(mux)
	defer server.Close()
	
	// Make a request to the custom metrics path
	resp, err := http.Get(server.URL + "/custom-metrics")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
	
	// Verify that metrics are disabled when configured
	obsConfig.Metrics.Enabled = false
	mockCfg.Update(&obsConfig)
	
	// Create a new mux and register the handler
	mux = http.NewServeMux()
	manager.RegisterMetricsHandler(mux)
	
	// The metrics endpoint should not be registered when disabled
	server = httptest.NewServer(mux)
	defer server.Close()
	
	// Make a request to the custom metrics path
	resp, err = http.Get(server.URL + "/custom-metrics")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()
	
	// Since the metrics are disabled, the handler should not be registered
	// and the default mux handler should return 404
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}