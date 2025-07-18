package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santif/microlib/core"
)

func TestHealthEndpointsIntegration(t *testing.T) {
	// Create a health checker with default configuration
	healthChecker := NewHealthChecker()

	// Add a passing health check
	healthChecker.AddCheck("database", func(ctx context.Context) error {
		return nil
	})

	// Add a health check that simulates a dependency with varying health
	var dependencyHealthy bool
	healthChecker.AddCheck("api-dependency", func(ctx context.Context) error {
		if !dependencyHealthy {
			return context.DeadlineExceeded
		}
		return nil
	})

	// Create a test HTTP server mux
	mux := http.NewServeMux()

	// Register the handlers
	healthChecker.RegisterHandlers(mux)

	// Create a test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test 1: Initially, the dependency is unhealthy
	dependencyHealthy = false

	// Test the health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Since one dependency is unhealthy, this should return 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Parse the response
	var healthResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := healthResponse["status"].(string); !ok || status != string(core.StatusDown) {
		t.Errorf("Expected status to be %s, got %v", core.StatusDown, healthResponse["status"])
	}

	// Test 2: Make the dependency healthy
	dependencyHealthy = true

	// Test the health endpoint again
	resp, err = http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Now all dependencies are healthy, this should return 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the response
	if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := healthResponse["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, healthResponse["status"])
	}

	// Test 3: Test the startup probe
	// First, reset the health checker to ensure a clean state
	// We need to access the internal fields of the healthChecker
	// This is only for testing purposes
	type healthCheckerInternal interface {
		SetReadyToServe(bool)
		ResetStartupChecks()
		SetStartupThreshold(int)
	}

	// Add methods to healthChecker to expose internal state for testing
	if hcInternal, ok := healthChecker.(healthCheckerInternal); ok {
		hcInternal.ResetStartupChecks()
		hcInternal.SetReadyToServe(false)
		hcInternal.SetStartupThreshold(2)
	} else {
		t.Fatal("healthChecker does not implement healthCheckerInternal")
	}

	// First call to startup endpoint
	resp, err = http.Get(server.URL + "/health/startup")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	// Since we haven't reached the threshold, this should return 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Second call to startup endpoint
	resp, err = http.Get(server.URL + "/health/startup")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	// Now we should have reached the threshold, this should return 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test 4: Test the readiness endpoint after startup is complete
	resp, err = http.Get(server.URL + "/health/ready")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Since startup is complete and all dependencies are healthy, this should return 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test 5: Make the dependency unhealthy again
	dependencyHealthy = false

	// Test the readiness endpoint
	resp, err = http.Get(server.URL + "/health/ready")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Since one dependency is unhealthy, this should return 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Test 6: Test the liveness endpoint
	resp, err = http.Get(server.URL + "/health/live")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Liveness should always return 200 if the service is running
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHealthEndpointsWithServiceIntegration(t *testing.T) {
	// Create a service with metadata
	service := core.NewService(core.ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance",
		BuildHash: "abcdef123456",
	})

	// Create a health checker with default configuration
	healthChecker := NewHealthChecker()

	// Register the health checker with the service
	service.RegisterStartupHook(func(ctx context.Context) error {
		// Add a health check for a database dependency
		healthChecker.AddCheck("database", func(ctx context.Context) error {
			// Simulate a database check
			return nil
		})

		// Add a health check for an API dependency
		healthChecker.AddCheck("api", func(ctx context.Context) error {
			// Simulate an API check
			return nil
		})

		return nil
	})

	// Create a test HTTP server mux
	mux := http.NewServeMux()

	// Register the handlers
	healthChecker.RegisterHandlers(mux)

	// Create a test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Start the service
	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Mark the service as ready to serve traffic
	if hcInternal, ok := healthChecker.(interface{ SetReadyToServe(bool) }); ok {
		hcInternal.SetReadyToServe(true)
	} else {
		t.Fatal("healthChecker does not implement SetReadyToServe method")
	}

	// Test the health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Since all dependencies are healthy, this should return 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the response
	var healthResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify the response fields
	if status, ok := healthResponse["status"].(string); !ok || status != string(core.StatusUp) {
		t.Errorf("Expected status to be %s, got %v", core.StatusUp, healthResponse["status"])
	}

	// Verify that the checks field contains our registered health checks
	if checks, ok := healthResponse["checks"].(map[string]interface{}); !ok {
		t.Error("Expected checks field to be a map")
	} else {
		if _, ok := checks["database"]; !ok {
			t.Error("Expected database check to be present")
		}
		if _, ok := checks["api"]; !ok {
			t.Error("Expected api check to be present")
		}
	}

	// Shutdown the service
	if err := service.Shutdown(5 * time.Second); err != nil {
		t.Fatalf("Failed to shutdown service: %v", err)
	}
}
