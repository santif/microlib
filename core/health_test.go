package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewHealthChecker(t *testing.T) {
	checker := NewHealthChecker()
	if checker == nil {
		t.Fatal("NewHealthChecker returned nil")
	}
	
	if checker.checks == nil {
		t.Fatal("HealthChecker.checks should be initialized")
	}
}

func TestAddCheck(t *testing.T) {
	checker := NewHealthChecker()
	
	// Add a check
	checker.AddCheck("test", func(ctx context.Context) error {
		return nil
	})
	
	// Verify check was added
	if len(checker.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checker.checks))
	}
	
	// Add another check
	checker.AddCheck("test2", func(ctx context.Context) error {
		return errors.New("check failed")
	})
	
	// Verify second check was added
	if len(checker.checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(checker.checks))
	}
}

func TestRemoveCheck(t *testing.T) {
	checker := NewHealthChecker()
	
	// Add a check
	checker.AddCheck("test", func(ctx context.Context) error {
		return nil
	})
	
	// Verify check was added
	if len(checker.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checker.checks))
	}
	
	// Remove the check
	checker.RemoveCheck("test")
	
	// Verify check was removed
	if len(checker.checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(checker.checks))
	}
	
	// Remove a non-existent check (should not error)
	checker.RemoveCheck("nonexistent")
}

func TestRunChecks(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()
	
	// Add a passing check
	checker.AddCheck("passing", func(ctx context.Context) error {
		return nil
	})
	
	// Add a failing check
	checker.AddCheck("failing", func(ctx context.Context) error {
		return errors.New("check failed")
	})
	
	// Run checks
	results := checker.RunChecks(ctx)
	
	// Verify results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	
	// Verify passing check
	passingResult, ok := results["passing"]
	if !ok {
		t.Error("Expected result for 'passing' check")
	} else {
		if passingResult.Status != StatusUp {
			t.Errorf("Expected status %s, got %s", StatusUp, passingResult.Status)
		}
		if passingResult.Error != "" {
			t.Errorf("Expected no error, got %s", passingResult.Error)
		}
	}
	
	// Verify failing check
	failingResult, ok := results["failing"]
	if !ok {
		t.Error("Expected result for 'failing' check")
	} else {
		if failingResult.Status != StatusDown {
			t.Errorf("Expected status %s, got %s", StatusDown, failingResult.Status)
		}
		if failingResult.Error != "check failed" {
			t.Errorf("Expected error 'check failed', got %s", failingResult.Error)
		}
	}
}

func TestIsHealthy(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()
	
	// Empty checker should be healthy
	if !checker.IsHealthy(ctx) {
		t.Error("Empty health checker should be healthy")
	}
	
	// Add a passing check
	checker.AddCheck("passing", func(ctx context.Context) error {
		return nil
	})
	
	// Should still be healthy
	if !checker.IsHealthy(ctx) {
		t.Error("Health checker with passing check should be healthy")
	}
	
	// Add a failing check
	checker.AddCheck("failing", func(ctx context.Context) error {
		return errors.New("check failed")
	})
	
	// Should now be unhealthy
	if checker.IsHealthy(ctx) {
		t.Error("Health checker with failing check should be unhealthy")
	}
	
	// Remove failing check
	checker.RemoveCheck("failing")
	
	// Should be healthy again
	if !checker.IsHealthy(ctx) {
		t.Error("Health checker with only passing check should be healthy")
	}
}

func TestHealthSummary(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()
	
	// Empty checker should return unknown status
	summary := checker.HealthSummary(ctx)
	if summary.Status != StatusUnknown {
		t.Errorf("Expected status %s, got %s", StatusUnknown, summary.Status)
	}
	
	// Add a passing check
	checker.AddCheck("passing", func(ctx context.Context) error {
		return nil
	})
	
	// Should return up status
	summary = checker.HealthSummary(ctx)
	if summary.Status != StatusUp {
		t.Errorf("Expected status %s, got %s", StatusUp, summary.Status)
	}
	
	// Add a failing check
	checker.AddCheck("failing", func(ctx context.Context) error {
		return errors.New("check failed")
	})
	
	// Should return down status
	summary = checker.HealthSummary(ctx)
	if summary.Status != StatusDown {
		t.Errorf("Expected status %s, got %s", StatusDown, summary.Status)
	}
}

func TestHealthResult(t *testing.T) {
	// Test creation of health result
	result := HealthResult{
		Status:    StatusUp,
		Message:   "All good",
		Timestamp: time.Now(),
	}
	
	if result.Status != StatusUp {
		t.Errorf("Expected status %s, got %s", StatusUp, result.Status)
	}
	
	if result.Message != "All good" {
		t.Errorf("Expected message 'All good', got %s", result.Message)
	}
	
	if result.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}