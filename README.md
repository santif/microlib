# MicroLib - Go Microservices Framework

MicroLib es una librería de microservicios para Go diseñada para estandarizar y simplificar el desarrollo de microservicios productivos. Proporciona un framework cohesivo y unificado que reduce el time-to-market y asegura la implementación de las mejores prácticas de arquitectura desde el inicio.

## 🚀 Features

- **Service Core**: Gestión completa del ciclo de vida del servicio con graceful shutdown
- **Configuration**: Sistema de configuración jerárquico con validación automática
- **Observability**: Logging estructurado, métricas Prometheus y tracing OpenTelemetry integrados
- **HTTP/gRPC**: Servidores con middleware stack predefinido y soporte OpenAPI
- **Messaging**: Comunicación asincrónica con patrón Outbox transparente
- **Data Layer**: Abstracciones para SQL y cache con implementaciones optimizadas
- **Jobs & Scheduling**: Sistema de jobs distribuidos con elección de líder
- **Security**: Autenticación JWT y hooks de autorización integrados
- **CLI Tools**: Herramientas de scaffolding para desarrollo rápido

## 📋 Requirements

- Go 1.22 or higher
- PostgreSQL (for data persistence and Outbox pattern)
- Redis (for caching and job queues)

## 🏗️ Architecture

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

## 🚀 Quick Start

### 1. Install MicroLib CLI

```bash
go install github.com/microlib/microlib/cli/microlib-cli@latest
```

### 2. Create a New Service

```bash
microlib-cli new service my-service
cd my-service
```

### 3. Basic Service Example

```go
package main

import (
    "context"
    "log"
    
    "github.com/microlib/microlib/core"
    "github.com/microlib/microlib/config"
    "github.com/microlib/microlib/http"
)

func main() {
    // Load configuration
    cfg := &Config{}
    if err := config.Load(cfg); err != nil {
        log.Fatal("Failed to load config:", err)
    }
    
    // Create service
    service := core.NewService(core.ServiceMetadata{
        Name:    "my-service",
        Version: "1.0.0",
    })
    
    // Setup HTTP server
    httpServer := http.NewServer(cfg.HTTP)
    httpServer.RegisterHandler("/health", http.HealthHandler())
    
    // Start service
    if err := service.Start(context.Background()); err != nil {
        log.Fatal("Failed to start service:", err)
    }
}
```

## 📦 Package Structure

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

## 🔧 Configuration

MicroLib uses a hierarchical configuration system:

1. **Environment Variables** (highest priority)
2. **Configuration Files** (YAML/TOML)
3. **Command Line Flags** (lowest priority)

Example configuration:

```yaml
service:
  name: "my-service"
  version: "1.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  name: "mydb"

cache:
  redis_url: "redis://localhost:6379"

logging:
  level: "info"
  format: "json"
```

## 📊 Observability

### Logging

```go
import "github.com/microlib/microlib/observability"

logger := observability.NewLogger()
logger.Info("Service started", 
    observability.Field("port", 8080),
    observability.Field("version", "1.0.0"),
)
```

### Metrics

```go
metrics := observability.NewMetrics()
counter := metrics.Counter("requests_total")
counter.Inc()
```

### Health Checks

```go
health := observability.NewHealthChecker()
health.AddCheck("database", func(ctx context.Context) error {
    return db.Ping(ctx)
})
```

## 🔐 Security

### JWT Authentication

```go
import "github.com/microlib/microlib/security"

auth := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
})

// Use as middleware
httpServer.RegisterMiddleware(auth.Middleware())
```

## 📨 Messaging

### Outbox Pattern

```go
import "github.com/microlib/microlib/messaging"

broker := messaging.NewBroker(cfg.Messaging)

// Publish within a database transaction
err := db.Transaction(ctx, func(tx data.Transaction) error {
    // Your business logic here
    user := &User{Name: "John"}
    if err := userRepo.Create(ctx, tx, user); err != nil {
        return err
    }
    
    // Message will be sent reliably via Outbox pattern
    return broker.Publish(ctx, "user.created", &UserCreatedEvent{
        UserID: user.ID,
        Name:   user.Name,
    })
})
```

## 🔄 Jobs & Scheduling

```go
import "github.com/microlib/microlib/jobs"

scheduler := jobs.NewScheduler()

// Cron job
scheduler.Schedule("0 */5 * * * *", jobs.JobFunc(func(ctx context.Context) error {
    log.Println("Running scheduled task")
    return nil
}))

// Distributed job queue
queue := jobs.NewJobQueue(cfg.Jobs)
queue.Enqueue(ctx, &EmailJob{
    To:      "user@example.com",
    Subject: "Welcome!",
})
```

## 🛠️ Development

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

### Building

```bash
make build
```

## 📚 Examples

Check the `examples/` directory for complete service implementations:

- **REST API Service**: Complete HTTP service with authentication
- **gRPC Service**: gRPC service with interceptors
- **Event-Driven Service**: Service using messaging and Outbox pattern
- **Job Processing Service**: Background job processing

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- 📖 [Documentation](https://microlib.dev/docs)
- 💬 [Discord Community](https://discord.gg/microlib)
- 🐛 [Issue Tracker](https://github.com/microlib/microlib/issues)
- 📧 [Email Support](mailto:support@microlib.dev)