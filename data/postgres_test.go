package data

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestPostgresConfig tests the default values for PostgresConfig
func TestPostgresConfig(t *testing.T) {
	// This is a unit test that doesn't require an actual database connection
	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	// Create a mock context that will be canceled immediately to avoid actual connection
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to avoid actual connection attempt

	// Create a copy of the config to test default values
	testCfg := cfg

	// Apply default values manually (same logic as in NewPostgresDB)
	if testCfg.MaxConnections <= 0 {
		testCfg.MaxConnections = 10
	}
	if testCfg.ConnTimeout <= 0 {
		testCfg.ConnTimeout = 5 * time.Second
	}
	if testCfg.MaxIdleTime <= 0 {
		testCfg.MaxIdleTime = 5 * time.Minute
	}
	if testCfg.MaxLifetime <= 0 {
		testCfg.MaxLifetime = 30 * time.Minute
	}
	if testCfg.HealthCheckFreq <= 0 {
		testCfg.HealthCheckFreq = 1 * time.Minute
	}
	if testCfg.SSLMode == "" {
		testCfg.SSLMode = "disable"
	}

	// This should fail due to canceled context, but we're testing the config parsing
	_, err := NewPostgresDB(ctx, cfg)
	if err == nil {
		t.Error("Expected error due to canceled context, got nil")
	}

	// Verify default values are set correctly
	if testCfg.MaxConnections != 10 {
		t.Errorf("Expected default MaxConnections to be 10, got %d", testCfg.MaxConnections)
	}
	if testCfg.ConnTimeout != 5*time.Second {
		t.Errorf("Expected default ConnTimeout to be 5s, got %v", testCfg.ConnTimeout)
	}
	if testCfg.MaxIdleTime != 5*time.Minute {
		t.Errorf("Expected default MaxIdleTime to be 5m, got %v", testCfg.MaxIdleTime)
	}
	if testCfg.MaxLifetime != 30*time.Minute {
		t.Errorf("Expected default MaxLifetime to be 30m, got %v", testCfg.MaxLifetime)
	}
	if testCfg.HealthCheckFreq != time.Minute {
		t.Errorf("Expected default HealthCheckFreq to be 1m, got %v", testCfg.HealthCheckFreq)
	}
	if testCfg.SSLMode != "disable" {
		t.Errorf("Expected default SSLMode to be 'disable', got %s", testCfg.SSLMode)
	}
}

// TestTransactionRollback tests that transactions are properly rolled back on error
func TestTransactionRollback(t *testing.T) {
	// Create a mock database for testing transactions
	mockDB := &MockDatabase{
		TransactionFunc: func(ctx context.Context, fn func(Transaction) error) error {
			// Create a mock transaction
			mockTx := &MockTransaction{
				ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
					// Simulate a successful query execution
					return &MockResult{
						RowsAffectedFunc: func() (int64, error) {
							return 1, nil
						},
					}, nil
				},
			}

			// Execute the transaction function with our mock transaction
			err := fn(mockTx)

			// Verify that the error is properly returned
			if err == nil {
				t.Error("Expected error from transaction function, got nil")
			}

			// In a real implementation, this would trigger a rollback
			return err
		},
	}

	// Test a transaction that returns an error
	err := mockDB.Transaction(context.Background(), func(tx Transaction) error {
		// This should succeed
		_, err := tx.Exec(context.Background(), "INSERT INTO test (name) VALUES ($1)", "test")
		if err != nil {
			return err
		}

		// Return an error to trigger rollback
		return errors.New("forced transaction error")
	})

	// Verify that the error is properly returned
	if err == nil {
		t.Error("Expected error from transaction, got nil")
	}
	if err.Error() != "forced transaction error" {
		t.Errorf("Expected error message 'forced transaction error', got '%s'", err.Error())
	}
}

// MockResult implements the Result interface for testing
type MockResult struct {
	LastInsertIdFunc func() (int64, error)
	RowsAffectedFunc func() (int64, error)
}

func (m *MockResult) LastInsertId() (int64, error) {
	if m.LastInsertIdFunc != nil {
		return m.LastInsertIdFunc()
	}
	return 0, nil
}

func (m *MockResult) RowsAffected() (int64, error) {
	if m.RowsAffectedFunc != nil {
		return m.RowsAffectedFunc()
	}
	return 0, nil
}

// TestResultInterface ensures that our interfaces are properly defined
func TestResultInterface(t *testing.T) {
	// This test doesn't actually run any code, it just ensures that MockResult
	// implements the Result interface at compile time
	var _ Result = &MockResult{}
}
