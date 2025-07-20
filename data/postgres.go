package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfig holds configuration for PostgreSQL connection
type PostgresConfig struct {
	Host            string        `yaml:"host" validate:"required"`
	Port            int           `yaml:"port" validate:"required,min=1,max=65535"`
	Username        string        `yaml:"username" validate:"required"`
	Password        string        `yaml:"password" validate:"required"`
	Database        string        `yaml:"database" validate:"required"`
	SSLMode         string        `yaml:"sslmode" validate:"oneof=disable require verify-ca verify-full"`
	MaxConnections  int           `yaml:"max_connections" validate:"min=1"`
	ConnTimeout     time.Duration `yaml:"conn_timeout" validate:"required"`
	MaxIdleTime     time.Duration `yaml:"max_idle_time"`
	MaxLifetime     time.Duration `yaml:"max_lifetime"`
	HealthCheckFreq time.Duration `yaml:"health_check_freq"`
}

// PostgresDB implements the Database interface for PostgreSQL
type PostgresDB struct {
	pool *pgxpool.Pool
	cfg  PostgresConfig
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(ctx context.Context, cfg PostgresConfig) (*PostgresDB, error) {
	// Set default values if not provided
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 10
	}
	if cfg.ConnTimeout <= 0 {
		cfg.ConnTimeout = 5 * time.Second
	}
	if cfg.MaxIdleTime <= 0 {
		cfg.MaxIdleTime = 5 * time.Minute
	}
	if cfg.MaxLifetime <= 0 {
		cfg.MaxLifetime = 30 * time.Minute
	}
	if cfg.HealthCheckFreq <= 0 {
		cfg.HealthCheckFreq = 1 * time.Minute
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	// Build connection string
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	// Configure connection pool
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MaxConnLifetime = cfg.MaxLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckFreq
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnTimeout

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return &PostgresDB{
		pool: pool,
		cfg:  cfg,
	}, nil
}

// Query executes a query that returns rows
func (db *PostgresDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query error: %w", err)
	}
	return &PostgresRows{rows: rows}, nil
}

// QueryRow executes a query that returns a single row
func (db *PostgresDB) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	row := db.pool.QueryRow(ctx, query, args...)
	return &PostgresRow{row: row}
}

// Exec executes a query that doesn't return rows
func (db *PostgresDB) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	cmdTag, err := db.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres exec error: %w", err)
	}
	return &PostgresResult{cmdTag: cmdTag}, nil
}

// Transaction starts a transaction and executes the provided function within it
func (db *PostgresDB) Transaction(ctx context.Context, fn func(Transaction) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create transaction wrapper
	pgTx := &PostgresTx{tx: tx}

	// Execute function within transaction
	err = fn(pgTx)
	if err != nil {
		// Attempt to rollback on error
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			// Log rollback error but return the original error
			return fmt.Errorf("transaction failed: %w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the database connection pool
func (db *PostgresDB) Close() error {
	db.pool.Close()
	return nil
}

// Ping verifies a connection to the database is still alive
func (db *PostgresDB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// PostgresTx implements the Transaction interface for PostgreSQL
type PostgresTx struct {
	tx pgx.Tx
}

// Query executes a query that returns rows within a transaction
func (t *PostgresTx) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres transaction query error: %w", err)
	}
	return &PostgresRows{rows: rows}, nil
}

// QueryRow executes a query that returns a single row within a transaction
func (t *PostgresTx) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	row := t.tx.QueryRow(ctx, query, args...)
	return &PostgresRow{row: row}
}

// Exec executes a query that doesn't return rows within a transaction
func (t *PostgresTx) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	cmdTag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres transaction exec error: %w", err)
	}
	return &PostgresResult{cmdTag: cmdTag}, nil
}

// PostgresRows implements the Rows interface for PostgreSQL
type PostgresRows struct {
	rows pgx.Rows
}

// Next prepares the next row for reading
func (r *PostgresRows) Next() bool {
	return r.rows.Next()
}

// Scan copies the columns in the current row into the values pointed at by dest
func (r *PostgresRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

// Close closes the rows iterator
func (r *PostgresRows) Close() error {
	r.rows.Close()
	return nil
}

// Err returns any error that occurred while iterating
func (r *PostgresRows) Err() error {
	return r.rows.Err()
}

// PostgresRow implements the Row interface for PostgreSQL
type PostgresRow struct {
	row pgx.Row
}

// Scan copies the columns in the current row into the values pointed at by dest
func (r *PostgresRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

// PostgresResult implements the Result interface for PostgreSQL
type PostgresResult struct {
	cmdTag pgconn.CommandTag
}

// LastInsertId returns the ID of the last inserted row
// Note: PostgreSQL doesn't support LastInsertId directly, use RETURNING instead
func (r *PostgresResult) LastInsertId() (int64, error) {
	return 0, errors.New("LastInsertId is not supported by PostgreSQL, use RETURNING clause instead")
}

// RowsAffected returns the number of rows affected by the query
func (r *PostgresResult) RowsAffected() (int64, error) {
	return r.cmdTag.RowsAffected(), nil
}
