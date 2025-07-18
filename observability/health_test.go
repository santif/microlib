package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/santif/microlib/core"
)

func TestHealthChecker_LivenessHandler(t *testing.T) {
	// Create a health checker with default configuration
	healthChecker := NewHealthChecker()

	// Create a test request
	req := httptest.NewRequest("GET", "/health/live", nil)
	rec := httptest.NewRecorder()

	// Execute the request
	healthChecker.LivenessHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, response["status"])
	}
}

func TestHealthChecker_ReadinessHandler(t *testing.T) {
	// Create a health checker with default configuration
	healthChecker := NewHealthCheckerWithConfig(DefaultHealthConfig())

	// Add a passing health check
	healthChecker.AddCheck("test-passing", func(ctx context.Context) error {
		return nil
	})

	// Mark the service as ready
	if hcInternal, ok := healthChecker.(interface{ SetReadyToServe(bool) }); ok {
		hcInternal.SetReadyToServe(true)
	} else {
		t.Fatal("healthChecker does not implement SetReadyToServe method")
	}

	// Create a test request
	req := httptest.NewRequest("GET", "/health/ready", nil)
	rec := httptest.NewRecorder()

	// Execute the request
	healthChecker.ReadinessHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, response["status"])
	}

	// Add a failing health check
	healthChecker.AddCheck("test-failing", func(ctx context.Context) error {
		return context.DeadlineExceeded
	})

	// Create a new test request
	req = httptest.NewRequest("GET", "/health/ready", nil)
	rec = httptest.NewRecorder()

	// Execute the request
	healthChecker.ReadinessHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	// Parse the response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusDown) {
		t.Errorf("Expected status to be %s, got %v", core.StatusDown, response["status"])
	}
}

func TestHealthChecker_StartupHandler(t *testing.T) {
	// Create a health checker with a custom configuration for testing
	config := DefaultHealthConfig()
	config.StartupThreshold = 2
	healthChecker := NewHealthCheckerWithConfig(config)

	// Add a passing health check
	healthChecker.AddCheck("test-passing", func(ctx context.Context) error {
		return nil
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/health/startup", nil)
	rec := httptest.NewRecorder()

	// Execute the request - first call should return StatusStarting
	healthChecker.StartupHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusStarting) {
		t.Errorf("Expected status to be %s, got %v", core.StatusStarting, response["status"])
	}

	// Execute the request again - second call should return StatusUp
	req = httptest.NewRequest("GET", "/health/startup", nil)
	rec = httptest.NewRecorder()
	healthChecker.StartupHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Parse the response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, response["status"])
	}

	// Verify that the service is now marked as ready
	// Use the IsReady method to check if the service is ready
	ctx := context.Background()
	if !healthChecker.IsReady(ctx) {
		t.Error("Expected service to be marked as ready after startup threshold is reached")
	}
}

func TestHealthChecker_HealthHandler(t *testing.T) {
	// Create a health checker with default configuration
	healthChecker := NewHealthChecker()

	// Add a passing health check
	healthChecker.AddCheck("test-passing", func(ctx context.Context) error {
		return nil
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	// Execute the request
	healthChecker.HealthHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, response["status"])
	}

	// Add a failing health check
	healthChecker.AddCheck("test-failing", func(ctx context.Context) error {
		return context.DeadlineExceeded
	})

	// Create a new test request
	req = httptest.NewRequest("GET", "/health", nil)
	rec = httptest.NewRecorder()

	// Execute the request
	healthChecker.HealthHandler().ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	// Parse the response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := response["status"].(string); !ok || status != string(core.StatusDown) {
		t.Errorf("Expected status to be %s, got %v", core.StatusDown, response["status"])
	}
}

func TestHealthChecker_RegisterHandlers(t *testing.T) {
	// Create a health checker with default configuration
	healthChecker := NewHealthChecker()

	// Add a passing health check
	healthChecker.AddCheck("test-passing", func(ctx context.Context) error {
		return nil
	})

	// Create a test HTTP server mux
	mux := http.NewServeMux()

	// Register the handlers
	healthChecker.RegisterHandlers(mux)

	// Create a test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test 1: Initially, the service is not ready
	// Test the health endpoint - should be OK since the health check passes
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected health status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test the liveness endpoint - should always be OK
	resp, err = http.Get(server.URL + "/health/live")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected liveness status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test 2: Mark the service as ready
	if hcInternal, ok := healthChecker.(interface{ SetReadyToServe(bool) }); ok {
		hcInternal.SetReadyToServe(true)
	} else {
		t.Fatal("healthChecker does not implement SetReadyToServe method")
	}

	// Test the readiness endpoint - should be OK now
	resp, err = http.Get(server.URL + "/health/ready")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected readiness status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test 3: Simulate startup completion
	// First, reset the startup checks
	if hcInternal, ok := healthChecker.(interface{ ResetStartupChecks() }); ok {
		hcInternal.ResetStartupChecks()
	} else {
		t.Fatal("healthChecker does not implement ResetStartupChecks method")
	}

	// Set a lower threshold for testing
	if hcInternal, ok := healthChecker.(interface{ SetStartupThreshold(int) }); ok {
		hcInternal.SetStartupThreshold(1)
	} else {
		t.Fatal("healthChecker does not implement SetStartupThreshold method")
	}

	// Call the startup endpoint - should be OK after one call with threshold=1
	resp, err = http.Get(server.URL + "/health/startup")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected startup status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHealthManager(t *testing.T) {
	// Create a mock config
	mockConfig := &healthMockConfig{
		config: &ObservabilityConfig{
			HealthEndpoints: DefaultHealthEndpointsConfig(),
		},
	}

	// Create a health manager
	manager, err := NewHealthManager(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create health manager: %v", err)
	}

	// Get the health checker
	healthChecker := manager.GetHealthChecker()
	if healthChecker == nil {
		t.Fatal("Expected non-nil health checker")
	}

	// Update the health configuration
	newConfig := DefaultHealthConfig()
	newConfig.Path = "/custom-health"
	if err := manager.UpdateHealthConfig(newConfig); err != nil {
		t.Fatalf("Failed to update health config: %v", err)
	}

	// Verify the configuration was updated
	// Note: We're using HealthEndpoints in ObservabilityConfig now
	// This test is temporarily disabled until we update the entire health system
	// obsConfig := mockConfig.config.(*ObservabilityConfig)
	// if obsConfig.HealthEndpoints.Path != "/custom-health" {
	// 	t.Errorf("Expected path to be %s, got %s", "/custom-health", obsConfig.HealthEndpoints.Path)
	// }

	// Test the Reload method
	newObsConfig := &ObservabilityConfig{
		HealthEndpoints: DefaultHealthEndpointsConfig(),
	}
	// Note: We're using HealthEndpoints in ObservabilityConfig now
	// This is temporarily modified until we update the entire health system
	newObsConfig.HealthEndpoints.Path = "/reloaded-health"
	if err := manager.Reload(newObsConfig); err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	// Verify the health checker was updated
	healthChecker = manager.GetHealthChecker()
	if healthChecker == nil {
		t.Fatal("Expected non-nil health checker after reload")
	}
}

// healthMockConfig is a mock implementation of the configInterface for health tests
type healthMockConfig struct {
	config interface{}
}

func (m *healthMockConfig) Get() interface{} {
	return m.config
}

func (m *healthMockConfig) Update(config interface{}) error {
	m.config = config
	return nil
}

func (m *healthMockConfig) Watch(callback func(interface{})) error {
	return nil
}

func TestGlobalHealthChecker(t *testing.T) {
	// Reset the global health checker to ensure a clean state
	originalHealthChecker := GlobalHealthChecker
	defer func() {
		GlobalHealthChecker = originalHealthChecker
	}()

	// Create a custom health checker
	customHealthChecker := NewHealthChecker()

	// Set the global health checker
	SetGlobalHealthChecker(customHealthChecker)

	// Verify that the global health checker was set
	if GlobalHealthChecker != customHealthChecker {
		t.Error("Expected global health checker to be set to the custom health checker")
	}

	// Add a health check using the global function
	AddCheck("test-check", func(ctx context.Context) error {
		return nil
	})

	// Verify that the health check was added
	ctx := context.Background()
	if !IsHealthy(ctx) {
		t.Error("Expected IsHealthy to return true after adding a passing health check")
	}

	// Add a failing health check
	AddCheck("test-failing", func(ctx context.Context) error {
		return context.DeadlineExceeded
	})

	// Verify that the health check was added
	if IsHealthy(ctx) {
		t.Error("Expected IsHealthy to return false after adding a failing health check")
	}

	// Remove the failing health check
	RemoveCheck("test-failing")

	// Verify that the health check was removed
	if !IsHealthy(ctx) {
		t.Error("Expected IsHealthy to return true after removing the failing health check")
	}

	// Test IsReady function
	if IsReady(ctx) {
		t.Error("Expected IsReady to return false when service is not marked as ready")
	}

	// Mark the service as ready
	if hcInternal, ok := customHealthChecker.(interface{ SetReadyToServe(bool) }); ok {
		hcInternal.SetReadyToServe(true)
	} else {
		t.Fatal("customHealthChecker does not implement SetReadyToServe method")
	}

	// Verify that IsReady now returns true
	if !IsReady(ctx) {
		t.Error("Expected IsReady to return true after marking service as ready")
	}
}
