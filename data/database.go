package data

import (
	"context"
)

// Database defines the interface for database operations
type Database interface {
	// Query executes a query that returns rows
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)

	// QueryRow executes a query that returns a single row
	QueryRow(ctx context.Context, query string, args ...interface{}) Row

	// Exec executes a query that doesn't return rows
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)

	// Transaction starts a transaction and executes the provided function within it
	Transaction(ctx context.Context, fn func(Transaction) error) error

	// Close closes the database connection
	Close() error

	// Ping verifies a connection to the database is still alive
	Ping(ctx context.Context) error
}

// Transaction represents a database transaction
type Transaction interface {
	// Query executes a query that returns rows within a transaction
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)

	// QueryRow executes a query that returns a single row within a transaction
	QueryRow(ctx context.Context, query string, args ...interface{}) Row

	// Exec executes a query that doesn't return rows within a transaction
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
}

// Rows represents the result set of a query
type Rows interface {
	// Next prepares the next row for reading
	Next() bool

	// Scan copies the columns in the current row into the values pointed at by dest
	Scan(dest ...interface{}) error

	// Close closes the rows iterator
	Close() error

	// Err returns any error that occurred while iterating
	Err() error
}

// Row represents a single row returned from a query
type Row interface {
	// Scan copies the columns in the current row into the values pointed at by dest
	Scan(dest ...interface{}) error
}

// Result represents the result of a query execution
type Result interface {
	// LastInsertId returns the ID of the last inserted row
	LastInsertId() (int64, error)

	// RowsAffected returns the number of rows affected by the query
	RowsAffected() (int64, error)
}
