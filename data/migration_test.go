package data

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPostgresMigrationRunner_Initialize(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			// Verify that the query creates the migrations table
			if !containsIgnoreCase(query, "CREATE TABLE IF NOT EXISTS schema_migrations") {
				t.Errorf("Expected query to create migrations table, got: %s", query)
			}
			return &MockResult{}, nil
		},
	}

	// Create migration runner
	runner := NewPostgresMigrationRunner(mockDB)

	// Initialize
	err := runner.Initialize(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresMigrationRunner_AddMigration(t *testing.T) {
	// Create migration runner
	runner := NewPostgresMigrationRunner(nil)

	// Test valid migration
	err := runner.AddMigration(Migration{
		Version:     1,
		Description: "Test migration",
		UpSQL:       "CREATE TABLE test (id INT)",
		DownSQL:     "DROP TABLE test",
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test duplicate version
	err = runner.AddMigration(Migration{
		Version:     1,
		Description: "Another test migration",
		UpSQL:       "CREATE TABLE another_test (id INT)",
		DownSQL:     "DROP TABLE another_test",
	})
	if err == nil {
		t.Error("Expected error for duplicate version, got nil")
	}

	// Test invalid migration (version <= 0)
	err = runner.AddMigration(Migration{
		Version:     0,
		Description: "Invalid migration",
		UpSQL:       "CREATE TABLE invalid (id INT)",
		DownSQL:     "DROP TABLE invalid",
	})
	if err == nil {
		t.Error("Expected error for invalid version, got nil")
	}

	// Test invalid migration (empty description)
	err = runner.AddMigration(Migration{
		Version:     2,
		Description: "",
		UpSQL:       "CREATE TABLE test2 (id INT)",
		DownSQL:     "DROP TABLE test2",
	})
	if err == nil {
		t.Error("Expected error for empty description, got nil")
	}

	// Test invalid migration (empty UpSQL)
	err = runner.AddMigration(Migration{
		Version:     2,
		Description: "Test migration 2",
		UpSQL:       "",
		DownSQL:     "DROP TABLE test2",
	})
	if err == nil {
		t.Error("Expected error for empty UpSQL, got nil")
	}

	// Test invalid migration (empty DownSQL)
	err = runner.AddMigration(Migration{
		Version:     2,
		Description: "Test migration 2",
		UpSQL:       "CREATE TABLE test2 (id INT)",
		DownSQL:     "",
	})
	if err == nil {
		t.Error("Expected error for empty DownSQL, got nil")
	}
}

func TestPostgresMigrationRunner_Apply(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			return &MockResult{}, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (Rows, error) {
			// Return empty rows for schema_migrations query
			return &MockRows{
				data: [][]interface{}{},
				err:  nil,
			}, nil
		},
		TransactionFunc: func(ctx context.Context, fn func(Transaction) error) error {
			// Execute the transaction function with a mock transaction
			mockTx := &MockTransaction{
				ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
					return &MockResult{}, nil
				},
			}
			return fn(mockTx)
		},
	}

	// Create migration runner
	runner := NewPostgresMigrationRunner(mockDB)

	// Add migrations
	runner.AddMigration(Migration{
		Version:     1,
		Description: "Create users table",
		UpSQL:       "CREATE TABLE users (id INT)",
		DownSQL:     "DROP TABLE users",
	})
	runner.AddMigration(Migration{
		Version:     2,
		Description: "Create posts table",
		UpSQL:       "CREATE TABLE posts (id INT)",
		DownSQL:     "DROP TABLE posts",
	})

	// Apply migrations
	err := runner.Apply(context.Background(), 0)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresMigrationRunner_Rollback(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			return &MockResult{}, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (Rows, error) {
			// Return applied migrations
			return &MockRows{
				data: [][]interface{}{
					{int64(1)},
					{int64(2)},
				},
				err: nil,
			}, nil
		},
		TransactionFunc: func(ctx context.Context, fn func(Transaction) error) error {
			// Execute the transaction function with a mock transaction
			mockTx := &MockTransaction{
				ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
					return &MockResult{}, nil
				},
			}
			return fn(mockTx)
		},
	}

	// Create migration runner
	runner := NewPostgresMigrationRunner(mockDB)

	// Add migrations
	runner.AddMigration(Migration{
		Version:     1,
		Description: "Create users table",
		UpSQL:       "CREATE TABLE users (id INT)",
		DownSQL:     "DROP TABLE users",
	})
	runner.AddMigration(Migration{
		Version:     2,
		Description: "Create posts table",
		UpSQL:       "CREATE TABLE posts (id INT)",
		DownSQL:     "DROP TABLE posts",
	})

	// Rollback migrations
	err := runner.Rollback(context.Background(), 0)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresMigrationRunner_Status(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			return &MockResult{}, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (Rows, error) {
			// Return migration statuses
			if containsIgnoreCase(query, "SELECT version, description, applied_at") {
				return &MockRows{
					data: [][]interface{}{
						{int64(1), "Create users table", time.Now()},
						{int64(2), "Create posts table", time.Now()},
					},
					err: nil,
				}, nil
			}
			return &MockRows{data: [][]interface{}{}, err: nil}, nil
		},
	}

	// Create migration runner
	runner := NewPostgresMigrationRunner(mockDB)

	// Get migration status
	statuses, err := runner.Status(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify statuses
	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got: %d", len(statuses))
	}
}

func TestPostgresMigrationRunner_parseMigrationContent(t *testing.T) {
	// Create migration runner
	runner := NewPostgresMigrationRunner(nil)

	// Test valid migration content
	content := `
	CREATE TABLE users (id INT);
	
	-- DOWN
	
	DROP TABLE users;
	`
	err := runner.parseMigrationContent("V1__create_users_table.sql", content)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test invalid migration content (no DOWN separator)
	content = `
	CREATE TABLE users (id INT);
	`
	err = runner.parseMigrationContent("V1__create_users_table.sql", content)
	if err == nil {
		t.Error("Expected error for missing DOWN separator, got nil")
	}

	// Test invalid filename format
	err = runner.parseMigrationContent("create_users_table.sql", content)
	if err == nil {
		t.Error("Expected error for invalid filename format, got nil")
	}

	// Test invalid version in filename
	err = runner.parseMigrationContent("Vx__create_users_table.sql", content)
	if err == nil {
		t.Error("Expected error for invalid version in filename, got nil")
	}
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// Mock implementations for testing

type MockRows struct {
	data     [][]interface{}
	index    int
	err      error
	closed   bool
	scanFunc func(dest ...interface{}) error
}

func (m *MockRows) Next() bool {
	if m.closed || m.index >= len(m.data) {
		return false
	}
	m.index++
	return true
}

func (m *MockRows) Scan(dest ...interface{}) error {
	if m.closed {
		return errors.New("rows are closed")
	}
	if m.index <= 0 || m.index > len(m.data) {
		return errors.New("invalid row index")
	}
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	row := m.data[m.index-1]
	for i, val := range row {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *int64:
				if v, ok := val.(int64); ok {
					*d = v
				}
			case *string:
				if v, ok := val.(string); ok {
					*d = v
				}
			case *time.Time:
				if v, ok := val.(time.Time); ok {
					*d = v
				}
			}
		}
	}
	return nil
}

func (m *MockRows) Close() error {
	m.closed = true
	return nil
}

func (m *MockRows) Err() error {
	return m.err
}

// MockResult is defined in database_test.go
