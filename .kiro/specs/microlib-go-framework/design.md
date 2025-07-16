# Design Document - MicroLib Go Framework

## Overview

MicroLib es un framework opinado para microservicios en Go que proporciona una base sólida y estandarizada para el desarrollo de servicios productivos. El diseño sigue principios de arquitectura limpia, separación de responsabilidades y configuración por convención sobre configuración.

### Core Design Principles

- **Opinado pero Extensible**: Proporciona implementaciones por defecto robustas mientras permite personalización
- **Observabilidad First**: Logging, métricas y tracing integrados desde el inicio
- **Production Ready**: Manejo de ciclo de vida, health checks y graceful shutdown incluidos
- **Developer Experience**: CLI tools y scaffolding para reducir time-to-market
- **Go Idiomático**: Interfaces claras, manejo explícito de errores, y patrones estándar de Go

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    MicroLib Framework                        │
├─────────────────────────────────────────────────────────────┤
│  Service Core  │  Config  │  Observability  │  Security     │
├─────────────────────────────────────────────────────────────┤
│  HTTP/gRPC     │  Messaging      │  Data Layer              │
├─────────────────────────────────────────────────────────────┤
│  Jobs/Scheduler│  CLI Tools      │  Extensions              │
└─────────────────────────────────────────────────────────────┘
```

### Package Structure

```
microlib/
├── core/           # Service lifecycle and metadata
├── config/         # Configuration management
├── observability/  # Logging, metrics, tracing, health
├── http/           # HTTP server and middleware
├── grpc/           # gRPC server and interceptors
├── messaging/      # Async messaging with Outbox pattern
├── data/           # Database and cache abstractions
├── jobs/           # Scheduled tasks and distributed jobs
├── security/       # Authentication and authorization
├── cli/            # Command-line tools and scaffolding
└── examples/       # Reference implementations
```

## Components and Interfaces

### 1. Service Core (`core` package)

**Design Decision**: Centralizar el ciclo de vida del servicio en un componente core que maneje inicialización, health checks y graceful shutdown.

```go
type Service struct {
    metadata ServiceMetadata
    deps     []Dependency
    server   Server
    shutdown chan os.Signal
}

type ServiceMetadata struct {
    Name      string
    Version   string
    Instance  string
    BuildHash string
}

type Dependency interface {
    Name() string
    HealthCheck(ctx context.Context) error
}
```

**Rationale**: Un servicio debe tener un punto de entrada único que maneje toda la infraestructura común, permitiendo al desarrollador enfocarse en lógica de negocio.

### 2. Configuration (`config` package)

**Design Decision**: Sistema de configuración jerárquico con validación automática usando struct tags.

```go
type Manager interface {
    Load(dest interface{}) error
    Validate(config interface{}) error
    Watch(callback func(interface{})) error
}

type Source interface {
    Load() (map[string]interface{}, error)
    Priority() int
}
```

**Rationale**: La configuración debe ser type-safe, validada y seguir una jerarquía clara para diferentes entornos.

### 3. Observability (`observability` package)

**Design Decision**: Integración completa de logging estructurado, métricas Prometheus y tracing OpenTelemetry.

```go
// Logging
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    WithContext(ctx context.Context) Logger
}

// Metrics
type Metrics interface {
    Counter(name string) Counter
    Histogram(name string) Histogram
    Gauge(name string) Gauge
}

// Health
type HealthChecker interface {
    AddCheck(name string, check HealthCheck)
    LivenessHandler() http.Handler
    ReadinessHandler() http.Handler
}
```

**Rationale**: La observabilidad debe ser transparente y automática, enriqueciendo logs con contexto de tracing sin intervención manual.

### 4. HTTP Server (`http` package)

**Design Decision**: Servidor HTTP con middleware stack predefinido y soporte para OpenAPI.

```go
type Server interface {
    RegisterHandler(pattern string, handler http.Handler)
    RegisterMiddleware(middleware Middleware)
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}

type Middleware func(http.Handler) http.Handler
```

**Middlewares incluidos**:
- Request logging con trace correlation
- Métricas automáticas (latencia, errores, throughput)
- Panic recovery
- Authentication/Authorization hooks
- CORS y security headers

**Rationale**: El middleware stack debe ser consistente entre servicios y proporcionar observabilidad automática.

### 5. gRPC Server (`grpc` package)

**Design Decision**: Interceptores equivalentes a los middlewares HTTP para consistencia.

```go
type Server interface {
    RegisterService(desc *grpc.ServiceDesc, impl interface{})
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

**Interceptores incluidos**:
- Logging con trace correlation
- Métricas automáticas
- Panic recovery
- Authentication/Authorization

### 6. Messaging (`messaging` package)

**Design Decision**: Abstracción de messaging con implementación transparente del patrón Outbox.

```go
type Broker interface {
    Publish(ctx context.Context, topic string, message Message) error
    Subscribe(topic string, handler MessageHandler) error
    Close() error
}

type Message interface {
    ID() string
    Body() []byte
    Headers() map[string]string
}

type OutboxStore interface {
    SaveMessage(ctx context.Context, tx Transaction, message OutboxMessage) error
    GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error)
    MarkProcessed(ctx context.Context, messageID string) error
}
```

**Rationale**: El patrón Outbox debe ser transparente para el desarrollador, garantizando consistencia eventual sin complejidad adicional.

### 7. Data Layer (`data` package)

**Design Decision**: Abstracciones para SQL y cache con implementaciones optimizadas.

```go
type Database interface {
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Transaction(ctx context.Context, fn func(Transaction) error) error
}

type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}
```

**Rationale**: Las abstracciones deben ser simples pero potentes, con implementaciones optimizadas para PostgreSQL y Redis.

### 8. Jobs and Scheduling (`jobs` package)

**Design Decision**: Sistema de jobs distribuidos con elección de líder y retry policies.

```go
type Scheduler interface {
    Schedule(spec string, job Job) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type JobQueue interface {
    Enqueue(ctx context.Context, job Job) error
    Process(ctx context.Context, handler JobHandler) error
}

type Job interface {
    ID() string
    Execute(ctx context.Context) error
    RetryPolicy() RetryPolicy
}
```

**Rationale**: Los jobs distribuidos requieren coordinación entre instancias y políticas de retry robustas.

### 9. Security (`security` package)

**Design Decision**: Middleware de autenticación JWT con soporte OIDC y hooks para autorización.

```go
type Authenticator interface {
    ValidateToken(ctx context.Context, token string) (*Claims, error)
    Middleware() Middleware
}

type Authorizer interface {
    Authorize(ctx context.Context, subject, action, resource string) error
}

type Claims struct {
    Subject   string
    Audience  []string
    ExpiresAt time.Time
    Custom    map[string]interface{}
}
```

**Rationale**: La seguridad debe ser pluggable pero con implementaciones seguras por defecto.

## Data Models

### Service Metadata Model

```go
type ServiceMetadata struct {
    Name      string    `json:"name" validate:"required"`
    Version   string    `json:"version" validate:"required,semver"`
    Instance  string    `json:"instance" validate:"required"`
    BuildHash string    `json:"build_hash" validate:"required"`
    StartTime time.Time `json:"start_time"`
}
```

### Configuration Model

```go
type Config struct {
    Service ServiceConfig `yaml:"service" validate:"required"`
    HTTP    HTTPConfig    `yaml:"http"`
    GRPC    GRPCConfig    `yaml:"grpc"`
    DB      DBConfig      `yaml:"database"`
    Cache   CacheConfig   `yaml:"cache"`
    Logging LogConfig     `yaml:"logging"`
}
```

### Outbox Message Model

```go
type OutboxMessage struct {
    ID          string            `db:"id"`
    Topic       string            `db:"topic"`
    Payload     []byte            `db:"payload"`
    Headers     map[string]string `db:"headers"`
    CreatedAt   time.Time         `db:"created_at"`
    ProcessedAt *time.Time        `db:"processed_at"`
}
```

## Error Handling

### Error Strategy

**Design Decision**: Usar errores tipados con wrapping para mantener contexto y stack traces.

```go
type ServiceError struct {
    Code    string
    Message string
    Cause   error
    Context map[string]interface{}
}

func (e *ServiceError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ServiceError) Unwrap() error {
    return e.Cause
}
```

### Error Categories

- **ConfigurationError**: Errores de configuración inválida
- **DependencyError**: Errores de dependencias externas
- **ValidationError**: Errores de validación de datos
- **AuthenticationError**: Errores de autenticación
- **AuthorizationError**: Errores de autorización

**Rationale**: Los errores deben ser informativos y permitir debugging efectivo en producción.

## Testing Strategy

### Unit Testing

- Cada package debe tener >90% de cobertura
- Usar interfaces para facilitar mocking
- Tests de configuración con diferentes fuentes
- Tests de middleware con requests simulados

### Integration Testing

- Tests con bases de datos reales usando testcontainers
- Tests de messaging con brokers reales
- Tests de health checks con dependencias simuladas

### End-to-End Testing

- Servicios completos con todas las dependencias
- Tests de graceful shutdown
- Tests de observabilidad (métricas, logs, traces)

### Performance Testing

- Benchmarks para operaciones críticas
- Load testing para HTTP/gRPC endpoints
- Memory profiling para detectar leaks

**Rationale**: Una estrategia de testing comprehensiva asegura la calidad y confiabilidad del framework en producción.

## Implementation Considerations

### Dependency Injection

**Design Decision**: Soporte para wire y fx como opciones de DI, pero no requerido.

```go
// Wire example
func NewService(cfg Config, db Database, cache Cache) *Service {
    return &Service{
        config: cfg,
        db:     db,
        cache:  cache,
    }
}
```

### Graceful Shutdown

**Design Decision**: Timeout configurable con shutdown hooks para cleanup.

```go
type ShutdownHook func(ctx context.Context) error

func (s *Service) RegisterShutdownHook(hook ShutdownHook) {
    s.shutdownHooks = append(s.shutdownHooks, hook)
}
```

### Configuration Hot Reload

**Design Decision**: Soporte opcional para hot reload de configuración no crítica.

```go
type Reloadable interface {
    Reload(newConfig interface{}) error
}
```

**Rationale**: Estas decisiones de diseño priorizan la simplicidad y robustez mientras mantienen flexibilidad para casos avanzados.