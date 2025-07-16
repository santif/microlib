# MicroLib - Go Microservices Framework

MicroLib is a Go microservices library designed to standardize and simplify the development of production-ready services. It provides a cohesive and unified framework that reduces time-to-market and ensures best architecture practices from day one.

## 🚀 Features

* **Service Core**: Full lifecycle management with graceful shutdown
* **Configuration**: Hierarchical config system with automatic validation
* **Observability**: Integrated structured logging, Prometheus metrics, and OpenTelemetry tracing
* **HTTP/gRPC**: Servers with a predefined middleware stack and OpenAPI support
* **Messaging**: Asynchronous communication with transparent Outbox pattern
* **Data Layer**: Abstractions for SQL and cache with optimized implementations
* **Jobs & Scheduling**: Distributed job system with leader election
* **Security**: Built-in JWT authentication and authorization hooks
* **CLI Tools**: Scaffolding tools for rapid development

## 📋 Requirements

* Go 1.22 or higher
* PostgreSQL (for persistence and Outbox pattern)
* Redis (for caching and job queues)

## 🏗️ Architecture

```
┌────────────────────────────────────────────────────────────-─┐
│                    MicroLib Framework                        │
├─────────────────────────────────────────────────────────────-┤
│  Service Core  │  Configuration  │  Observability  │ Security │
├─────────────────────────────────────────────────────────────-┤
│  HTTP/gRPC     │  Messaging      │  Data Layer               │
├────────────────────────────────────────────────────────────-─┤
│  Jobs/Scheduler│  CLI Tools      │  Extensions               │
└────────────────────────────────────────────────────────────-─┘
```

## 🚀 Quick Start

### 1. Install MicroLib CLI

```bash
go install github.com/santif/microlib/cli/microlib-cli@latest
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
    
    "github.com/santif/microlib/core"
    "github.com/santif/microlib/config"
    "github.com/santif/microlib/http"
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
import "github.com/santif/microlib/observability"

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
import "github.com/santif/microlib/security"

auth := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
})

// Use as middleware
httpServer.RegisterMiddleware(auth.Middleware())
```

## 📨 Messaging

### Outbox Pattern

```go
import "github.com/santif/microlib/messaging"

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
import "github.com/santif/microlib/jobs"

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

* **REST API Service**: Full HTTP service with authentication
* **gRPC Service**: gRPC service with interceptors
* **Event-Driven Service**: Messaging-based service with Outbox
* **Job Processing Service**: Background job processor

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License – see the [LICENSE](LICENSE) file for details.

## 🆘 Support

* 📖 [Documentation](https://microlib.dev/docs)
* 💬 [Discord Community](https://discord.gg/microlib)
* 🐛 [Issue Tracker](https://github.com/santif/microlib/issues)
* 📧 [Email Support](mailto:support@microlib.dev)
