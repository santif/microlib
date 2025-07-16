package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockDependency implements the Dependency interface for testing
type mockDependency struct {
	name        string
	healthError error
}

func (m *mockDependency) Name() string {
	return m.name
}

func (m *mockDependency) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func TestNewService(t *testing.T) {
	metadata := ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	}

	service := NewService(metadata)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.metadata.Name != metadata.Name {
		t.Errorf("Expected name %s, got %s", metadata.Name, service.metadata.Name)
	}

	if service.metadata.Version != metadata.Version {
		t.Errorf("Expected version %s, got %s", metadata.Version, service.metadata.Version)
	}

	if service.metadata.Instance != metadata.Instance {
		t.Errorf("Expected instance %s, got %s", metadata.Instance, service.metadata.Instance)
	}

	if service.metadata.BuildHash != metadata.BuildHash {
		t.Errorf("Expected build hash %s, got %s", metadata.BuildHash, service.metadata.BuildHash)
	}

	if service.metadata.StartTime.IsZero() {
		t.Error("StartTime should be set automatically")
	}

	if service.IsStarted() {
		t.Error("Service should not be started initially")
	}
}

func TestServiceMetadata(t *testing.T) {
	metadata := ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	}

	service := NewService(metadata)
	retrievedMetadata := service.Metadata()

	if retrievedMetadata.Name != metadata.Name {
		t.Errorf("Expected name %s, got %s", metadata.Name, retrievedMetadata.Name)
	}
}

func TestAddDependency(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	dep := &mockDependency{name: "test-dep"}
	service.AddDependency(dep)

	// Verify dependency was added by checking validation
	ctx := context.Background()
	err := service.ValidateDependencies(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestValidateDependencies_Success(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Add healthy dependencies
	dep1 := &mockDependency{name: "dep1", healthError: nil}
	dep2 := &mockDependency{name: "dep2", healthError: nil}
	
	service.AddDependency(dep1)
	service.AddDependency(dep2)

	ctx := context.Background()
	err := service.ValidateDependencies(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestValidateDependencies_Failure(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Add unhealthy dependency
	dep := &mockDependency{
		name:        "failing-dep",
		healthError: errors.New("connection failed"),
	}
	service.AddDependency(dep)

	ctx := context.Background()
	err := service.ValidateDependencies(ctx)
	if err == nil {
		t.Error("Expected error for failing dependency")
	}

	expectedMsg := "dependency failing-dep health check failed"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestServiceStart(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !service.IsStarted() {
		t.Error("Service should be started after Start() call")
	}

	// Test double start
	err = service.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already started service")
	}
}

func TestServiceStart_WithFailingDependency(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Add failing dependency
	dep := &mockDependency{
		name:        "failing-dep",
		healthError: errors.New("connection failed"),
	}
	service.AddDependency(dep)

	ctx := context.Background()
	err := service.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting with failing dependency")
	}

	if service.IsStarted() {
		t.Error("Service should not be started when dependency validation fails")
	}
}

func TestShutdownHooks(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var hooksCalled []string
	
	// Register shutdown hooks
	service.RegisterShutdownHook(func(ctx context.Context) error {
		hooksCalled = append(hooksCalled, "hook1")
		return nil
	})
	
	service.RegisterShutdownHook(func(ctx context.Context) error {
		hooksCalled = append(hooksCalled, "hook2")
		return nil
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown service
	err = service.Shutdown(5 * time.Second)
	if err != nil {
		t.Errorf("Expected no error during shutdown, got %v", err)
	}

	// Verify hooks were called in reverse order
	expectedOrder := []string{"hook2", "hook1"}
	if len(hooksCalled) != len(expectedOrder) {
		t.Errorf("Expected %d hooks called, got %d", len(expectedOrder), len(hooksCalled))
	}

	for i, expected := range expectedOrder {
		if i >= len(hooksCalled) || hooksCalled[i] != expected {
			t.Errorf("Expected hook %s at position %d, got %s", expected, i, hooksCalled[i])
		}
	}

	if service.IsStarted() {
		t.Error("Service should not be started after shutdown")
	}
}

func TestShutdownHooks_WithError(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Register failing shutdown hook
	service.RegisterShutdownHook(func(ctx context.Context) error {
		return errors.New("shutdown failed")
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown service
	err = service.Shutdown(5 * time.Second)
	if err == nil {
		t.Error("Expected error during shutdown with failing hook")
	}
}

func TestShutdown_NotStarted(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Shutdown without starting
	err := service.Shutdown(5 * time.Second)
	if err != nil {
		t.Errorf("Expected no error when shutting down non-started service, got %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}