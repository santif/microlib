# API Reference

This document provides comprehensive documentation for all public interfaces in the MicroLib framework.

## Table of Contents

- [Core Package](#core-package)
- [Configuration Package](#configuration-package)
- [Observability Package](#observability-package)
- [HTTP Package](#http-package)
- [gRPC Package](#grpc-package)
- [Data Package](#data-package)
- [Messaging Package](#messaging-package)
- [Jobs Package](#jobs-package)
- [Security Package](#security-package)

## Core Package

The core package provides service lifecycle management and metadata handling.

### Types

#### ServiceMetadata

```go
type ServiceMetadata struct {
    Name      string    `json:"name" validate:"required"`
    Version   string    `json:"version" validate:"required,semver"`
    Instance  string    `json:"instance" validate:"required"`
    BuildHash string    `json:"build_hash" validate:"required"`
    StartTime time.Time `json:"start_time"`
}
```

Represents metadata about a service instance.

**Fields:**
- `Name`: Service name (required)
- `Version`: Semantic version (required)
- `Instance`: Unique instance identifier (required)
- `BuildHash`: Git commit hash or build identifier (required)
- `StartTime`: Service start timestamp

#### Service

```go
type Service struct {
    // private fields
}
```

Main service orchestrator that manages lifecycle, dependencies, and servers.

### Interfaces

#### Dependency

```go
type Dependency interface {
    Name() string
    HealthCheck(ctx context.Context) error
}
```

Represents a service dependency that can be health-checked.

**Methods:**
- `Name()`: Returns the dependency name
- `HealthCheck(ctx context.Context)`: Performs health check, returns error if unhealthy

#### Server

```go
type Server interface {
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

Represents a server (HTTP, gRPC, etc.) that can be started and stopped.

### Functions

#### NewService

```go
func NewService(metadata ServiceMetadata) *Service
```

Creates a new service instance with the provided metadata.

**Parameters:**
- `metadata`: Service metadata including name, version, etc.

**Returns:**
- `*Service`: New service instance

**Example:**
```go
service := core.NewService(core.ServiceMetadata{
    Name:      "user-service",
    Version:   "1.0.0",
    Instance:  "instance-1",
    BuildHash: "abc123",
})
```

### Methods

#### AddDependency

```go
func (s *Service) AddDependency(name string, dep Dependency)
```

Registers a dependency for health checking.

**Parameters:**
- `name`: Dependency name
- `dep`: Dependency implementation

#### AddServer

```go
func (s *Service) AddServer(server Server)
```

Registers a server to be managed by the service.

**Parameters:**
- `server`: Server implementation

#### Start

```go
func (s *Service) Start(ctx context.Context) error
```

Starts the service, performing health checks and starting all servers.

**Parameters:**
- `ctx`: Context for cancellation

**Returns:**
- `error`: Error if startup fails

#### Shutdown

```go
func (s *Service) Shutdown(ctx context.Context) error
```

Gracefully shuts down the service and all servers.

**Parameters:**
- `ctx`: Context with timeout for shutdown

**Returns:**
- `error`: Error if shutdown fails

## Configuration Package

The configuration package provides hierarchical configuration management with validation.

### Interfaces

#### Manager

```go
type Manager interface {
    Load(dest interface{}) error
    Validate(config interface{}) error
    Watch(callback func(interface{})) error
}
```

Manages configuration loading, validation, and watching.

**Methods:**
- `Load(dest interface{})`: Loads configuration into destination struct
- `Validate(config interface{})`: Validates configuration struct
- `Watch(callback func(interface{}))`: Watches for configuration changes

#### Source

```go
type Source interface {
    Load() (map[string]interface{}, error)
    Priority() int
}
```

Represents a configuration source (file, env vars, etc.).

**Methods:**
- `Load()`: Loads configuration data
- `Priority()`: Returns source priority (higher = more important)

### Functions

#### Load

```go
func Load(dest interface{}) error
```

Loads configuration from all sources into the destination struct.

**Parameters:**
- `dest`: Pointer to configuration struct

**Returns:**
- `error`: Error if loading fails

**Example:**
```go
type Config struct {
    Port int    `yaml:"port" env:"PORT" validate:"required,min=1,max=65535"`
    Host string `yaml:"host" env:"HOST" validate:"required"`
}

cfg := &Config{}
if err := config.Load(cfg); err != nil {
    log.Fatal(err)
}
```

#### NewManager

```go
func NewManager(sources ...Source) Manager
```

Creates a new configuration manager with the specified sources.

**Parameters:**
- `sources`: Variable number of configuration sources

**Returns:**
- `Manager`: Configuration manager instance

## Observability Package

The observability package provides logging, metrics, tracing, and health checking.

### Interfaces

#### Logger

```go
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    Debug(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    WithContext(ctx context.Context) Logger
    WithFields(fields ...Field) Logger
}
```

Structured logger interface.

**Methods:**
- `Info(msg, fields...)`: Logs info message with fields
- `Error(msg, err, fields...)`: Logs error message with error and fields
- `Debug(msg, fields...)`: Logs debug message with fields
- `Warn(msg, fields...)`: Logs warning message with fields
- `WithContext(ctx)`: Returns logger with context (trace ID, etc.)
- `WithFields(fields...)`: Returns logger with additional fields

#### Metrics

```go
type Metrics interface {
    Counter(name string, labels ...string) Counter
    Histogram(name string, labels ...string) Histogram
    Gauge(name string, labels ...string) Gauge
}
```

Metrics collection interface.

**Methods:**
- `Counter(name, labels...)`: Creates/gets a counter metric
- `Histogram(name, labels...)`: Creates/gets a histogram metric
- `Gauge(name, labels...)`: Creates/gets a gauge metric

#### Counter

```go
type Counter interface {
    Inc()
    Add(value float64)
    WithLabels(labels map[string]string) Counter
}
```

Counter metric interface.

#### Histogram

```go
type Histogram interface {
    Observe(value float64)
    WithLabels(labels map[string]string) Histogram
}
```

Histogram metric interface.

#### Gauge

```go
type Gauge interface {
    Set(value float64)
    Inc()
    Dec()
    Add(value float64)
    WithLabels(labels map[string]string) Gauge
}
```

Gauge metric interface.

#### HealthChecker

```go
type HealthChecker interface {
    AddCheck(name string, check HealthCheck)
    LivenessHandler() http.Handler
    ReadinessHandler() http.Handler
    StartupHandler() http.Handler
}
```

Health checking interface.

**Methods:**
- `AddCheck(name, check)`: Adds a named health check
- `LivenessHandler()`: Returns HTTP handler for liveness probe
- `ReadinessHandler()`: Returns HTTP handler for readiness probe
- `StartupHandler()`: Returns HTTP handler for startup probe

### Types

#### Field

```go
type Field struct {
    Key   string
    Value interface{}
}
```

Represents a structured logging field.

#### HealthCheck

```go
type HealthCheck func(ctx context.Context) error
```

Function type for health checks.

### Functions

#### NewLogger

```go
func NewLogger(config LogConfig) Logger
```

Creates a new structured logger.

#### NewMetrics

```go
func NewMetrics() Metrics
```

Creates a new metrics collector.

#### NewTracer

```go
func NewTracer(config TracingConfig) (trace.Tracer, error)
```

Creates a new OpenTelemetry tracer.

#### NewHealthChecker

```go
func NewHealthChecker() HealthChecker
```

Creates a new health checker.

#### Field

```go
func Field(key string, value interface{}) Field
```

Creates a new logging field.

**Example:**
```go
logger := observability.NewLogger(cfg.Logging)
logger.Info("User created", 
    observability.Field("user_id", 123),
    observability.Field("email", "user@example.com"),
)
```

## HTTP Package

The HTTP package provides HTTP server functionality with middleware support.

### Interfaces

#### Server

```go
type Server interface {
    RegisterHandler(pattern string, handler http.Handler)
    RegisterMiddleware(middleware Middleware)
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

HTTP server interface.

**Methods:**
- `RegisterHandler(pattern, handler)`: Registers HTTP handler for pattern
- `RegisterMiddleware(middleware)`: Registers middleware
- `Start(ctx)`: Starts the HTTP server
- `Shutdown(ctx)`: Gracefully shuts down the server

#### Middleware

```go
type Middleware func(http.Handler) http.Handler
```

HTTP middleware function type.

### Functions

#### NewServer

```go
func NewServer(config HTTPConfig) Server
```

Creates a new HTTP server.

**Parameters:**
- `config`: HTTP server configuration

**Returns:**
- `Server`: HTTP server instance

#### LoggingMiddleware

```go
func LoggingMiddleware(logger observability.Logger) Middleware
```

Creates request logging middleware.

#### MetricsMiddleware

```go
func MetricsMiddleware(metrics observability.Metrics) Middleware
```

Creates metrics collection middleware.

#### TracingMiddleware

```go
func TracingMiddleware(tracer trace.Tracer) Middleware
```

Creates distributed tracing middleware.

#### RecoveryMiddleware

```go
func RecoveryMiddleware(logger observability.Logger) Middleware
```

Creates panic recovery middleware.

#### CORSMiddleware

```go
func CORSMiddleware(config CORSConfig) Middleware
```

Creates CORS middleware.

**Example:**
```go
server := http.NewServer(cfg.HTTP)
server.RegisterMiddleware(http.LoggingMiddleware(logger))
server.RegisterMiddleware(http.MetricsMiddleware(metrics))
server.RegisterHandler("/users", userHandler)
```

## gRPC Package

The gRPC package provides gRPC server functionality with interceptors.

### Interfaces

#### Server

```go
type Server interface {
    RegisterService(desc *grpc.ServiceDesc, impl interface{})
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

gRPC server interface.

**Methods:**
- `RegisterService(desc, impl)`: Registers gRPC service
- `Start(ctx)`: Starts the gRPC server
- `Shutdown(ctx)`: Gracefully shuts down the server

### Functions

#### NewServer

```go
func NewServer(config GRPCConfig) Server
```

Creates a new gRPC server.

#### LoggingInterceptor

```go
func LoggingInterceptor(logger observability.Logger) grpc.UnaryServerInterceptor
```

Creates request logging interceptor.

#### MetricsInterceptor

```go
func MetricsInterceptor(metrics observability.Metrics) grpc.UnaryServerInterceptor
```

Creates metrics collection interceptor.

#### TracingInterceptor

```go
func TracingInterceptor(tracer trace.Tracer) grpc.UnaryServerInterceptor
```

Creates distributed tracing interceptor.

## Data Package

The data package provides database and cache abstractions.

### Interfaces

#### Database

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

Database interface for SQL operations.

#### Transaction

```go
type Transaction interface {
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...interface{}) Row
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Commit() error
    Rollback() error
}
```

Database transaction interface.

#### Cache

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Close() error
}
```

Cache interface for key-value operations.

### Functions

#### NewPostgresDB

```go
func NewPostgresDB(config DatabaseConfig) (Database, error)
```

Creates a new PostgreSQL database connection.

#### NewRedisCache

```go
func NewRedisCache(config CacheConfig) (Cache, error)
```

Creates a new Redis cache connection.

## Messaging Package

The messaging package provides asynchronous messaging with Outbox pattern support.

### Interfaces

#### Broker

```go
type Broker interface {
    Publish(ctx context.Context, topic string, message Message) error
    Subscribe(topic string, handler MessageHandler) error
    Close() error
}
```

Message broker interface.

#### Message

```go
type Message interface {
    ID() string
    Topic() string
    Body() []byte
    Headers() map[string]string
    Timestamp() time.Time
}
```

Message interface.

#### MessageHandler

```go
type MessageHandler func(ctx context.Context, message Message) error
```

Message handler function type.

#### OutboxStore

```go
type OutboxStore interface {
    SaveMessage(ctx context.Context, tx Transaction, message OutboxMessage) error
    GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error)
    MarkProcessed(ctx context.Context, messageID string) error
}
```

Outbox pattern storage interface.

### Functions

#### NewBroker

```go
func NewBroker(config MessagingConfig) (Broker, error)
```

Creates a new message broker.

#### NewMessage

```go
func NewMessage(topic string, body []byte, headers map[string]string) Message
```

Creates a new message.

## Jobs Package

The jobs package provides job scheduling and distributed job processing.

### Interfaces

#### Scheduler

```go
type Scheduler interface {
    Schedule(spec string, job Job) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Job scheduler interface.

#### JobQueue

```go
type JobQueue interface {
    Enqueue(ctx context.Context, job Job) error
    Process(ctx context.Context, handler JobHandler) error
    Close() error
}
```

Distributed job queue interface.

#### Job

```go
type Job interface {
    ID() string
    Execute(ctx context.Context) error
    RetryPolicy() RetryPolicy
}
```

Job interface.

#### JobHandler

```go
type JobHandler func(ctx context.Context, job Job) error
```

Job handler function type.

### Functions

#### NewScheduler

```go
func NewScheduler(config SchedulerConfig) Scheduler
```

Creates a new job scheduler.

#### NewJobQueue

```go
func NewJobQueue(config JobQueueConfig) (JobQueue, error)
```

Creates a new distributed job queue.

## Security Package

The security package provides authentication and authorization functionality.

### Interfaces

#### Authenticator

```go
type Authenticator interface {
    ValidateToken(ctx context.Context, token string) (*Claims, error)
    Middleware() http.Handler
}
```

Authentication interface.

#### Authorizer

```go
type Authorizer interface {
    Authorize(ctx context.Context, subject, action, resource string) error
}
```

Authorization interface.

### Types

#### Claims

```go
type Claims struct {
    Subject   string                 `json:"sub"`
    Audience  []string              `json:"aud"`
    ExpiresAt time.Time             `json:"exp"`
    IssuedAt  time.Time             `json:"iat"`
    Issuer    string                `json:"iss"`
    Custom    map[string]interface{} `json:"-"`
}
```

JWT claims structure.

### Functions

#### NewJWTAuthenticator

```go
func NewJWTAuthenticator(config JWTConfig) (Authenticator, error)
```

Creates a new JWT authenticator.

#### NewOPAAuthorizer

```go
func NewOPAAuthorizer(config OPAConfig) (Authorizer, error)
```

Creates a new OPA-based authorizer.

**Example:**
```go
auth, err := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
    Audience:     "my-service",
})
if err != nil {
    log.Fatal(err)
}

server.RegisterMiddleware(auth.Middleware())
```

## Error Handling

All packages follow consistent error handling patterns:

### ServiceError

```go
type ServiceError struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Cause   error                  `json:"-"`
    Context map[string]interface{} `json:"context,omitempty"`
}
```

Standard error type used throughout the framework.

### Error Codes

Common error codes used across packages:

- `CONFIG_INVALID`: Configuration validation failed
- `DEPENDENCY_UNHEALTHY`: Dependency health check failed
- `AUTH_TOKEN_INVALID`: JWT token validation failed
- `AUTH_INSUFFICIENT_PERMISSIONS`: Authorization failed
- `DB_CONNECTION_FAILED`: Database connection failed
- `CACHE_CONNECTION_FAILED`: Cache connection failed
- `MESSAGE_PUBLISH_FAILED`: Message publishing failed
- `JOB_EXECUTION_FAILED`: Job execution failed

### Error Wrapping

Use `fmt.Errorf` with `%w` verb to wrap errors:

```go
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}
```

This maintains error chains for debugging while providing context.