# Getting Started with MicroLib

This guide will walk you through creating your first microservice using MicroLib, from installation to deployment.

## Prerequisites

Before you begin, ensure you have:

- Go 1.22 or higher installed
- PostgreSQL 13+ running locally or accessible
- Redis 6+ running locally or accessible
- Basic familiarity with Go programming

## Step 1: Install MicroLib CLI

The MicroLib CLI tool helps you scaffold new services quickly with best practices built-in.

```bash
go install github.com/santif/microlib/cli/microlib-cli@latest
```

Verify the installation:

```bash
microlib-cli version
```

## Step 2: Create Your First Service

Let's create a simple user management service:

```bash
microlib-cli new service user-service --type=api
cd user-service
```

This creates a complete service structure:

```
user-service/
├── cmd/
│   └── server.go          # Main application entry point
├── internal/
│   ├── config/
│   │   └── config.go      # Configuration structures
│   ├── handlers/
│   │   ├── handlers.go    # HTTP handlers setup
│   │   └── users.go       # User-specific handlers
│   ├── models/
│   │   └── user.go        # Data models
│   ├── repositories/
│   │   └── user_repository.go  # Data access layer
│   └── services/
│       └── user_service.go     # Business logic layer
├── api/
│   └── openapi.yaml       # API specification
├── migrations/
│   └── 001_create_users_table.sql
├── config.yaml            # Default configuration
├── docker-compose.yml     # Local development setup
├── Dockerfile             # Container image
├── Makefile              # Build and development tasks
└── go.mod                # Go module definition
```

## Step 3: Understand the Configuration

MicroLib uses a hierarchical configuration system. Open `config.yaml`:

```yaml
service:
  name: "user-service"
  version: "1.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  name: "userdb"
  user: "postgres"
  password: "postgres"
  ssl_mode: "disable"
  max_connections: 25

cache:
  redis_url: "redis://localhost:6379"
  default_ttl: "1h"

logging:
  level: "info"
  format: "json"

observability:
  metrics_enabled: true
  tracing_enabled: true
  health_checks_enabled: true

security:
  jwt:
    jwks_endpoint: "https://your-auth-provider.com/.well-known/jwks.json"
    audience: "user-service"
```

Configuration can be overridden by environment variables:

```bash
export DATABASE_HOST=prod-db.example.com
export DATABASE_PASSWORD=secure-password
export LOGGING_LEVEL=debug
```

## Step 4: Set Up Local Development Environment

Start the required services using Docker Compose:

```bash
docker-compose up -d postgres redis
```

Run database migrations:

```bash
make migrate-up
```

## Step 5: Examine the Generated Code

Let's look at the main service entry point in `cmd/server.go`:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/santif/microlib/core"
    "github.com/santif/microlib/config"
    "github.com/santif/microlib/data"
    "github.com/santif/microlib/http"
    "github.com/santif/microlib/observability"
    
    "user-service/internal/config"
    "user-service/internal/handlers"
    "user-service/internal/repositories"
    "user-service/internal/services"
)

func main() {
    // Load configuration
    cfg := &config.Config{}
    if err := config.Load(cfg); err != nil {
        log.Fatal("Failed to load configuration:", err)
    }

    // Initialize observability
    logger := observability.NewLogger(cfg.Logging)
    metrics := observability.NewMetrics()
    tracer := observability.NewTracer(cfg.Observability)

    // Create service metadata
    metadata := core.ServiceMetadata{
        Name:      cfg.Service.Name,
        Version:   cfg.Service.Version,
        Instance:  os.Getenv("HOSTNAME"),
        BuildHash: os.Getenv("BUILD_HASH"),
    }

    // Initialize data layer
    db, err := data.NewPostgresDB(cfg.Database)
    if err != nil {
        logger.Error("Failed to connect to database", err)
        os.Exit(1)
    }

    cache, err := data.NewRedisCache(cfg.Cache)
    if err != nil {
        logger.Error("Failed to connect to cache", err)
        os.Exit(1)
    }

    // Initialize repositories and services
    userRepo := repositories.NewUserRepository(db)
    userService := services.NewUserService(userRepo, cache, logger)

    // Create HTTP server
    httpServer := http.NewServer(cfg.HTTP)
    
    // Register middleware
    httpServer.RegisterMiddleware(observability.LoggingMiddleware(logger))
    httpServer.RegisterMiddleware(observability.MetricsMiddleware(metrics))
    httpServer.RegisterMiddleware(observability.TracingMiddleware(tracer))

    // Register handlers
    handlers.RegisterHandlers(httpServer, userService, logger)

    // Create and configure service
    service := core.NewService(metadata)
    service.AddDependency("database", db)
    service.AddDependency("cache", cache)
    service.AddServer(httpServer)

    // Start service
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        logger.Info("Shutdown signal received")
        cancel()
    }()

    if err := service.Start(ctx); err != nil {
        logger.Error("Service failed to start", err)
        os.Exit(1)
    }

    logger.Info("Service started successfully")
    <-ctx.Done()
    
    logger.Info("Service shutting down")
    if err := service.Shutdown(context.Background()); err != nil {
        logger.Error("Error during shutdown", err)
    }
}
```

## Step 6: Implement Your First Handler

Let's examine the user handler in `internal/handlers/users.go`:

```go
package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"
    "github.com/santif/microlib/observability"
    
    "user-service/internal/models"
    "user-service/internal/services"
)

type UserHandler struct {
    userService *services.UserService
    logger      observability.Logger
}

func NewUserHandler(userService *services.UserService, logger observability.Logger) *UserHandler {
    return &UserHandler{
        userService: userService,
        logger:      logger,
    }
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    var req models.CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.logger.Error("Failed to decode request", err)
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    user, err := h.userService.CreateUser(ctx, req)
    if err != nil {
        h.logger.Error("Failed to create user", err, 
            observability.Field("email", req.Email))
        http.Error(w, "Failed to create user", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    vars := mux.Vars(r)
    
    userID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    user, err := h.userService.GetUser(ctx, userID)
    if err != nil {
        if err == services.ErrUserNotFound {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        h.logger.Error("Failed to get user", err,
            observability.Field("user_id", userID))
        http.Error(w, "Failed to get user", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

## Step 7: Run Your Service

Build and run the service:

```bash
make build
make run
```

Or run directly:

```bash
go run cmd/server.go
```

## Step 8: Test Your Service

Test the health endpoints:

```bash
# Liveness check
curl http://localhost:8080/health/live

# Readiness check  
curl http://localhost:8080/health/ready

# Full health check
curl http://localhost:8080/health
```

Test the user endpoints:

```bash
# Create a user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com"}'

# Get a user
curl http://localhost:8080/users/1
```

## Step 9: Observe Your Service

### View Metrics

Visit `http://localhost:8080/metrics` to see Prometheus metrics.

### Check Logs

Logs are structured JSON by default:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "Request completed",
  "method": "POST",
  "path": "/users",
  "status": 201,
  "duration": "45ms",
  "trace_id": "abc123def456"
}
```

### View API Documentation

The OpenAPI specification is served at `http://localhost:8080/openapi.json`.

## Step 10: Add Authentication

Enable JWT authentication by updating your configuration:

```yaml
security:
  jwt:
    enabled: true
    jwks_endpoint: "https://your-auth-provider.com/.well-known/jwks.json"
    audience: "user-service"
```

Then add the authentication middleware:

```go
// In cmd/server.go
if cfg.Security.JWT.Enabled {
    auth := security.NewJWTAuthenticator(cfg.Security.JWT)
    httpServer.RegisterMiddleware(auth.Middleware())
}
```

## Step 11: Add Messaging

To add event publishing when users are created:

```go
// In services/user_service.go
func (s *UserService) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
    return s.db.Transaction(ctx, func(tx data.Transaction) (*models.User, error) {
        // Create user in database
        user, err := s.userRepo.Create(ctx, tx, req)
        if err != nil {
            return nil, err
        }

        // Publish event via Outbox pattern
        event := models.UserCreatedEvent{
            UserID: user.ID,
            Name:   user.Name,
            Email:  user.Email,
        }
        
        if err := s.broker.Publish(ctx, "user.created", event); err != nil {
            return nil, err
        }

        return user, nil
    })
}
```

## Step 12: Deploy Your Service

Build a Docker image:

```bash
make docker-build
```

Deploy to Kubernetes:

```bash
kubectl apply -f k8s/
```

## Next Steps

Now that you have a working service, explore these advanced features:

1. **[gRPC Services](grpc-guide.md)** - Add gRPC endpoints
2. **[Job Processing](jobs-guide.md)** - Add background job processing
3. **[Messaging](messaging-guide.md)** - Implement event-driven architecture
4. **[Security](security-guide.md)** - Advanced authentication and authorization
5. **[Observability](observability-guide.md)** - Advanced monitoring and tracing
6. **[Testing](testing-guide.md)** - Comprehensive testing strategies

## Troubleshooting

### Common Issues

**Service won't start:**
- Check database connectivity
- Verify configuration values
- Check port availability

**Database connection errors:**
- Ensure PostgreSQL is running
- Verify connection parameters
- Check network connectivity

**Authentication failures:**
- Verify JWKS endpoint is accessible
- Check JWT token format
- Validate audience claims

For more help, see our [troubleshooting guide](troubleshooting.md) or join our [Discord community](https://discord.gg/microlib).