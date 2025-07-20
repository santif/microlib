package data

import (
	"context"
	"testing"
)

// MockDatabase implements the Database interface for testing
type MockDatabase struct {
	QueryFunc       func(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowFunc    func(ctx context.Context, query string, args ...interface{}) Row
	ExecFunc        func(ctx context.Context, query string, args ...interface{}) (Result, error)
	TransactionFunc func(ctx context.Context, fn func(Transaction) error) error
	CloseFunc       func() error
	PingFunc        func(ctx context.Context) error
}

func (m *MockDatabase) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, query, args...)
	}
	return nil
}

func (m *MockDatabase) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDatabase) Transaction(ctx context.Context, fn func(Transaction) error) error {
	if m.TransactionFunc != nil {
		return m.TransactionFunc(ctx, fn)
	}
	return nil
}

func (m *MockDatabase) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockDatabase) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// TestDatabaseInterface ensures that our interfaces are properly defined
func TestDatabaseInterface(t *testing.T) {
	// This test doesn't actually run any code, it just ensures that MockDatabase
	// implements the Database interface at compile time
	var _ Database = &MockDatabase{}
}

// MockTransaction implements the Transaction interface for testing
type MockTransaction struct {
	QueryFunc    func(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowFunc func(ctx context.Context, query string, args ...interface{}) Row
	ExecFunc     func(ctx context.Context, query string, args ...interface{}) (Result, error)
}

func (m *MockTransaction) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, query, args...)
	}
	return nil
}

func (m *MockTransaction) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, query, args...)
	}
	return nil, nil
}

// TestTransactionInterface ensures that our interfaces are properly defined
func TestTransactionInterface(t *testing.T) {
	// This test doesn't actually run any code, it just ensures that MockTransaction
	// implements the Transaction interface at compile time
	var _ Transaction = &MockTransaction{}
}

// MockResult implements the Result interface for testing
type MockResult struct {
	lastInsertId     int64
	rowsAffected     int64
	LastInsertIdFunc func() (int64, error)
	RowsAffectedFunc func() (int64, error)
}

func (m *MockResult) LastInsertId() (int64, error) {
	if m.LastInsertIdFunc != nil {
		return m.LastInsertIdFunc()
	}
	return m.lastInsertId, nil
}

func (m *MockResult) RowsAffected() (int64, error) {
	if m.RowsAffectedFunc != nil {
		return m.RowsAffectedFunc()
	}
	return m.rowsAffected, nil
}
