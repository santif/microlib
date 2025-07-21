# ADR-008: Database Abstraction Layer

## Status

Accepted

## Context

Microservices need consistent database access patterns while supporting different database technologies. The abstraction should provide type safety, transaction management, and connection pooling without sacrificing performance or flexibility.

Key requirements:
- Support for PostgreSQL as the primary database
- Transaction management with proper rollback
- Connection pooling and health checking
- Migration and seeding capabilities
- Type-safe query execution
- Context-based cancellation
- Integration with observability systems

## Decision

We will implement a database abstraction layer with the following components:

### 1. Database Interface
A unified interface for database operations:

```go
type Database interface {
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...interface{}) Row
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Transaction(ctx context.Context, fn func(Transaction) error) error
    Ping(ctx context.Context) error
    Close() error
}
```

### 2. Transaction Interface
Separate interface for transaction operations:

```go
type Transaction interface {
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...interface{}) Row
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Commit() error
    Rollback() error
}
```

### 3. PostgreSQL Implementation
Primary implementation using pgx driver:

```go
type PostgresDB struct {
    pool   *pgxpool.Pool
    config DatabaseConfig
    logger observability.Logger
}

func NewPostgresDB(config DatabaseConfig) (*PostgresDB, error) {
    poolConfig, err := pgxpool.ParseConfig(config.ConnectionString())
    if err != nil {
        return nil, fmt.Errorf("failed to parse database config: %w", err)
    }
    
    // Configure connection pool
    poolConfig.MaxConns = int32(config.MaxConnections)
    poolConfig.MinConns = int32(config.MinConnections)
    poolConfig.MaxConnLifetime = config.MaxConnLifetime
    poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
    
    pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    return &PostgresDB{
        pool:   pool,
        config: config,
        logger: observability.NewLogger(),
    }, nil
}
```

### 4. Migration System
Built-in migration management:

```go
type Migrator interface {
    Up(ctx context.Context) error
    Down(ctx context.Context) error
    Status(ctx context.Context) ([]MigrationStatus, error)
}

type Migration struct {
    Version     int
    Description string
    UpSQL       string
    DownSQL     string
}
```

### 5. Seeding System
Database seeding for development and testing:

```go
type Seeder interface {
    Seed(ctx context.Context) error
    Clear(ctx context.Context) error
}

type SeedData struct {
    Table string
    Data  []map[string]interface{}
}
```

### 6. Configuration
Comprehensive database configuration:

```go
type DatabaseConfig struct {
    Host               string        `yaml:"host" env:"DB_HOST" validate:"required"`
    Port               int           `yaml:"port" env:"DB_PORT" validate:"required,min=1,max=65535"`
    Name               string        `yaml:"name" env:"DB_NAME" validate:"required"`
    User               string        `yaml:"user" env:"DB_USER" validate:"required"`
    Password           string        `yaml:"password" env:"DB_PASSWORD" validate:"required"`
    SSLMode            string        `yaml:"ssl_mode" env:"DB_SSL_MODE"`
    MaxConnections     int           `yaml:"max_connections" env:"DB_MAX_CONNECTIONS"`
    MinConnections     int           `yaml:"min_connections" env:"DB_MIN_CONNECTIONS"`
    MaxConnLifetime    time.Duration `yaml:"max_conn_lifetime" env:"DB_MAX_CONN_LIFETIME"`
    MaxConnIdleTime    time.Duration `yaml:"max_conn_idle_time" env:"DB_MAX_CONN_IDLE_TIME"`
    MigrationsPath     string        `yaml:"migrations_path" env:"DB_MIGRATIONS_PATH"`
    SeedsPath          string        `yaml:"seeds_path" env:"DB_SEEDS_PATH"`
}
```

### 7. Observability Integration
Automatic instrumentation and health checking:

```go
func (db *PostgresDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        db.metrics.Histogram("db_query_duration").Observe(duration.Seconds())
        db.logger.Debug("Database query executed",
            observability.Field("query", query),
            observability.Field("duration", duration),
            observability.Field("args_count", len(args)),
        )
    }()
    
    return db.pool.Query(ctx, query, args...)
}

func (db *PostgresDB) HealthCheck(ctx context.Context) error {
    return db.pool.Ping(ctx)
}
```

## Consequences

### Positive
- Consistent database access patterns across services
- Built-in connection pooling and health checking
- Transaction safety with automatic rollback
- Migration and seeding capabilities
- Observability integration out of the box
- Context-based cancellation support
- Type-safe query execution
- Easy testing with interface mocking

### Negative
- Abstraction overhead compared to direct driver usage
- PostgreSQL-specific optimizations may be hidden
- Additional learning curve for the abstraction layer
- Potential performance impact from instrumentation
- Limited to SQL databases (no NoSQL support)

## Alternatives Considered

### 1. Direct pgx Usage
Use pgx driver directly without abstraction.
- **Rejected**: Leads to inconsistent patterns and duplicated connection management

### 2. GORM ORM
Use GORM as the database abstraction layer.
- **Rejected**: Heavy ORM with magic behavior, performance overhead, less control

### 3. SQLx Library
Use jmoiron/sqlx for database operations.
- **Rejected**: Less feature-rich than pgx, doesn't provide connection pooling

### 4. Multiple Database Support
Support multiple databases (MySQL, SQLite) from the start.
- **Rejected**: Adds complexity without immediate need, can be added later

### 5. Repository Pattern
Implement repository pattern within the framework.
- **Rejected**: Too opinionated for data access patterns, should be left to applications

### 6. Query Builder
Include a query builder in the abstraction.
- **Rejected**: Adds complexity and learning curve, raw SQL is more transparent