package core

import (
	"context"
	"errors"
	"sync"
	"syscall"
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
	
	// Check default shutdown timeout
	if service.shutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, service.shutdownTimeout)
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

func TestWaitForShutdown(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Test that WaitForShutdown returns the signal channel
	shutdownChan := service.WaitForShutdown()
	if shutdownChan == nil {
		t.Error("WaitForShutdown should return a valid channel")
	}

	// Cleanup
	err = service.Shutdown(1 * time.Second)
	if err != nil {
		t.Errorf("Failed to shutdown service: %v", err)
	}
}

func TestShutdownTimeout(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Register a slow shutdown hook that takes longer than timeout
	service.RegisterShutdownHook(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown with short timeout
	start := time.Now()
	err = service.Shutdown(500 * time.Millisecond)
	duration := time.Since(start)

	// Should timeout and return error
	if err == nil {
		t.Error("Expected timeout error during shutdown")
	}

	// Should not take much longer than timeout
	if duration > 1*time.Second {
		t.Errorf("Shutdown took too long: %v", duration)
	}
}

func TestShutdownHooksOrder(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var executionOrder []int
	
	// Register multiple shutdown hooks
	for i := 1; i <= 5; i++ {
		hookNum := i
		service.RegisterShutdownHook(func(ctx context.Context) error {
			executionOrder = append(executionOrder, hookNum)
			return nil
		})
	}

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

	// Verify hooks were called in reverse order (LIFO)
	expectedOrder := []int{5, 4, 3, 2, 1}
	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d hooks executed, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("Expected hook %d at position %d, got %d", expected, i, executionOrder[i])
		}
	}
}

func TestConcurrentShutdown(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Try concurrent shutdowns
	var wg sync.WaitGroup
	errors := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := service.Shutdown(1 * time.Second)
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	// At least one should succeed, others should be no-ops
	var successCount, errorCount int
	for err := range errors {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	// Should handle concurrent shutdowns gracefully
	if successCount == 0 {
		t.Error("At least one shutdown should succeed")
	}
}

func TestShutdownHookContextCancellation(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var contextCancelled bool
	
	// Register shutdown hook that checks context cancellation
	service.RegisterShutdownHook(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			contextCancelled = true
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown with very short timeout to trigger context cancellation
	err = service.Shutdown(50 * time.Millisecond)
	
	// Should get context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if !contextCancelled {
		t.Error("Context should have been cancelled in shutdown hook")
	}
}

func TestServiceRun(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var hookCalled bool
	service.RegisterShutdownHook(func(ctx context.Context) error {
		hookCalled = true
		return nil
	})

	// Run service in a goroutine since it blocks
	done := make(chan error, 1)
	go func() {
		ctx := context.Background()
		err := service.Run(ctx)
		done <- err
	}()

	// Wait a bit for service to start
	time.Sleep(100 * time.Millisecond)

	// Verify service is started
	if !service.IsStarted() {
		t.Error("Service should be started after Run() call")
	}

	// Send shutdown signal
	service.shutdown <- syscall.SIGTERM

	// Wait for Run to complete
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error from Run(), got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run() did not complete within timeout")
	}

	// Verify shutdown hook was called
	if !hookCalled {
		t.Error("Shutdown hook should have been called")
	}

	// Verify service is no longer started
	if service.IsStarted() {
		t.Error("Service should not be started after Run() completes")
	}
}

func TestServiceRun_StartFailure(t *testing.T) {
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

	// Run should fail during start
	ctx := context.Background()
	err := service.Run(ctx)
	if err == nil {
		t.Error("Expected error when Run() fails to start")
	}

	expectedMsg := "failed to start service"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestSignalHandling_SIGTERM(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var shutdownCalled bool
	service.RegisterShutdownHook(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Send SIGTERM signal directly to the service's shutdown channel
	go func() {
		service.shutdown <- syscall.SIGTERM
	}()

	// Wait for signal to be received
	shutdownChan := service.WaitForShutdown()
	select {
	case sig := <-shutdownChan:
		if sig != syscall.SIGTERM {
			t.Errorf("Expected SIGTERM, got %v", sig)
		}
	case <-time.After(1 * time.Second):
		t.Error("Signal was not received within timeout")
	}

	// Perform shutdown to test hooks
	err = service.Shutdown(1 * time.Second)
	if err != nil {
		t.Errorf("Failed to shutdown service: %v", err)
	}

	if !shutdownCalled {
		t.Error("Shutdown hook should have been called")
	}
}

func TestSignalHandling_SIGINT(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var shutdownCalled bool
	service.RegisterShutdownHook(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Send SIGINT signal directly to the service's shutdown channel
	go func() {
		service.shutdown <- syscall.SIGINT
	}()

	// Wait for signal to be received
	shutdownChan := service.WaitForShutdown()
	select {
	case sig := <-shutdownChan:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(1 * time.Second):
		t.Error("Signal was not received within timeout")
	}

	// Perform shutdown to test hooks
	err = service.Shutdown(1 * time.Second)
	if err != nil {
		t.Errorf("Failed to shutdown service: %v", err)
	}

	if !shutdownCalled {
		t.Error("Shutdown hook should have been called")
	}
}

func TestGracefulShutdownTimeout_MultipleHooks(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Register multiple hooks with different delays
	service.RegisterShutdownHook(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	
	service.RegisterShutdownHook(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	
	service.RegisterShutdownHook(func(ctx context.Context) error {
		time.Sleep(300 * time.Millisecond)
		return nil
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown with sufficient timeout
	start := time.Now()
	err = service.Shutdown(1 * time.Second)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error during shutdown, got %v", err)
	}

	// Should complete within reasonable time (all hooks should execute)
	expectedDuration := 600 * time.Millisecond // Sum of all hook delays
	if duration < expectedDuration {
		t.Errorf("Shutdown completed too quickly: %v, expected at least %v", duration, expectedDuration)
	}

	if duration > expectedDuration+200*time.Millisecond {
		t.Errorf("Shutdown took too long: %v, expected around %v", duration, expectedDuration)
	}
}

func TestShutdownHook_ContextTimeout(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var contextDeadlineExceeded bool
	
	// Register hook that respects context timeout
	service.RegisterShutdownHook(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				contextDeadlineExceeded = true
			}
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown with short timeout
	err = service.Shutdown(100 * time.Millisecond)
	
	if err == nil {
		t.Error("Expected timeout error during shutdown")
	}

	if !contextDeadlineExceeded {
		t.Error("Context deadline should have been exceeded")
	}
}

func TestShutdownHooksExecutionOrder_WithErrors(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var executionOrder []string
	
	// Register hooks, some with errors
	service.RegisterShutdownHook(func(ctx context.Context) error {
		executionOrder = append(executionOrder, "hook1")
		return nil
	})
	
	service.RegisterShutdownHook(func(ctx context.Context) error {
		executionOrder = append(executionOrder, "hook2-error")
		return errors.New("hook2 failed")
	})
	
	service.RegisterShutdownHook(func(ctx context.Context) error {
		executionOrder = append(executionOrder, "hook3")
		return nil
	})

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Shutdown service - should fail on first error
	err = service.Shutdown(5 * time.Second)
	if err == nil {
		t.Error("Expected error during shutdown due to failing hook")
	}

	// Should execute hooks in reverse order: hook3, then hook2-error (which fails)
	// The implementation stops at the first error, so hook1 should not be executed
	expectedOrder := []string{"hook3", "hook2-error"}
	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d hooks executed, got %d. Execution order: %v", len(expectedOrder), len(executionOrder), executionOrder)
	}

	for i, expected := range expectedOrder {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("Expected hook %s at position %d, got %s", expected, i, executionOrder[i])
		}
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	// Default timeout should be set
	if service.shutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultShutdownTimeout, service.shutdownTimeout)
	}

	// Set custom timeout
	customTimeout := 10 * time.Second
	service.WithShutdownTimeout(customTimeout)

	// Check if timeout was updated
	if service.shutdownTimeout != customTimeout {
		t.Errorf("Expected custom timeout %v, got %v", customTimeout, service.shutdownTimeout)
	}
}

func TestRunWithTimeout(t *testing.T) {
	service := NewService(ServiceMetadata{
		Name:      "test-service",
		Version:   "1.0.0",
		Instance:  "test-instance-1",
		BuildHash: "abc123",
	})

	var hookCalled bool
	service.RegisterShutdownHook(func(ctx context.Context) error {
		hookCalled = true
		return nil
	})

	// Run service in a goroutine since it blocks
	done := make(chan error, 1)
	customTimeout := 5 * time.Second
	go func() {
		ctx := context.Background()
		err := service.RunWithTimeout(ctx, customTimeout)
		done <- err
	}()

	// Wait a bit for service to start
	time.Sleep(100 * time.Millisecond)

	// Verify service is started
	if !service.IsStarted() {
		t.Error("Service should be started after RunWithTimeout() call")
	}

	// Verify timeout was set
	if service.shutdownTimeout != customTimeout {
		t.Errorf("Expected shutdown timeout to be set to %v, got %v", customTimeout, service.shutdownTimeout)
	}

	// Send shutdown signal
	service.shutdown <- syscall.SIGTERM

	// Wait for Run to complete
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error from RunWithTimeout(), got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("RunWithTimeout() did not complete within timeout")
	}

	// Verify shutdown hook was called
	if !hookCalled {
		t.Error("Shutdown hook should have been called")
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