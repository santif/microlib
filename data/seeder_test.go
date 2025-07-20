package data

import (
	"context"
	"testing"
)

func TestPostgresSeeder_Initialize(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			// Verify that the query creates the seeds table
			if !containsIgnoreCase(query, "CREATE TABLE IF NOT EXISTS schema_seeds") {
				t.Errorf("Expected query to create seeds table, got: %s", query)
			}
			return &MockResult{}, nil
		},
	}

	// Create seeder
	seeder := NewPostgresSeeder(mockDB)

	// Initialize
	err := seeder.Initialize(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresSeeder_AddSeed(t *testing.T) {
	// Create seeder
	seeder := NewPostgresSeeder(nil)

	// Test valid seed
	err := seeder.AddSeed(Seed{
		Name:     "users",
		SQL:      "INSERT INTO users (name) VALUES ('admin')",
		Priority: 1,
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test duplicate name
	err = seeder.AddSeed(Seed{
		Name:     "users",
		SQL:      "INSERT INTO users (name) VALUES ('user')",
		Priority: 2,
	})
	if err == nil {
		t.Error("Expected error for duplicate name, got nil")
	}

	// Test invalid seed (empty name)
	err = seeder.AddSeed(Seed{
		Name:     "",
		SQL:      "INSERT INTO users (name) VALUES ('admin')",
		Priority: 1,
	})
	if err == nil {
		t.Error("Expected error for empty name, got nil")
	}

	// Test invalid seed (empty SQL)
	err = seeder.AddSeed(Seed{
		Name:     "admins",
		SQL:      "",
		Priority: 1,
	})
	if err == nil {
		t.Error("Expected error for empty SQL, got nil")
	}
}

func TestPostgresSeeder_Apply(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			return &MockResult{}, nil
		},
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (Rows, error) {
			// Return empty rows for schema_seeds query
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

	// Create seeder
	seeder := NewPostgresSeeder(mockDB)

	// Add seeds
	seeder.AddSeed(Seed{
		Name:     "users",
		SQL:      "INSERT INTO users (name) VALUES ('admin')",
		Priority: 1,
	})
	seeder.AddSeed(Seed{
		Name:     "posts",
		SQL:      "INSERT INTO posts (title) VALUES ('First post')",
		Priority: 2,
	})

	// Apply seeds
	err := seeder.Apply(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresSeeder_Reset(t *testing.T) {
	// Create mock database
	mockDB := &MockDatabase{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (Result, error) {
			// Verify that the query deletes from schema_seeds
			if containsIgnoreCase(query, "DELETE FROM schema_seeds") {
				return &MockResult{rowsAffected: 2}, nil
			}
			return &MockResult{}, nil
		},
	}

	// Create seeder
	seeder := NewPostgresSeeder(mockDB)

	// Reset seeds
	err := seeder.Reset(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPostgresSeeder_parseSeedContent(t *testing.T) {
	// Create seeder
	seeder := NewPostgresSeeder(nil)

	// Test valid seed content
	content := `
	INSERT INTO users (name) VALUES ('admin');
	INSERT INTO users (name) VALUES ('user');
	`
	err := seeder.parseSeedContent("S01_users.sql", content)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test invalid filename format
	err = seeder.parseSeedContent("users.sql", content)
	if err == nil {
		t.Error("Expected error for invalid filename format, got nil")
	}

	// Test invalid priority in filename
	err = seeder.parseSeedContent("Sx_users.sql", content)
	if err == nil {
		t.Error("Expected error for invalid priority in filename, got nil")
	}
}
