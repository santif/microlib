# Best Practices Guide

This guide provides recommendations for building robust, maintainable, and scalable microservices using the MicroLib framework.

## Table of Contents

- [Service Design](#service-design)
- [Configuration Management](#configuration-management)
- [Error Handling](#error-handling)
- [Observability](#observability)
- [Security](#security)
- [Database Operations](#database-operations)
- [Messaging](#messaging)
- [Testing](#testing)
- [Performance](#performance)
- [Deployment](#deployment)

## Service Design

### Single Responsibility Principle

Each microservice should have a single, well-defined responsibility:

```go
// Good: User management service
type UserService struct {
    repo   UserRepository
    cache  Cache
    logger Logger
}

func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Focus only on user-related operations
}

// Avoid: Mixed responsibilities
type UserOrderService struct {
    userRepo  UserRepository
    orderRepo OrderRepository
    // Don't mix user and order concerns in one service
}
```

### Service Boundaries

Define clear service boundaries using domain-driven design:

```go
// Good: Clear domain boundaries
// user-service: User management, authentication
// order-service: Order processing, inventory
// notification-service: Email, SMS, push notifications

// Avoid: Overlapping responsibilities
// user-order-service: Mixed user and order logic
```

### Dependency Management

Use dependency injection for better testability:

```go
// Good: Constructor injection
func NewUserService(repo UserRepository, cache Cache, logger Logger) *UserService {
    return &UserService{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}

// Avoid: Global dependencies
var globalDB Database // Don't use global variables

func CreateUser(user *User) error {
    return globalDB.Create(user) // Hard to test
}
```

### Interface Design

Design interfaces from the consumer's perspective:

```go
// Good: Focused interface
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int64) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
}

// Avoid: Fat interfaces
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int64) (*User, error)
    // ... 20+ methods
    GenerateReport(ctx context.Context) (*Report, error) // Unrelated
}
```

## Configuration Management

### Environment-Specific Configuration

Use the configuration hierarchy effectively:

```yaml
# config.yaml (defaults)
service:
  name: "user-service"
  port: 8080

database:
  host: "localhost"
  port: 5432
  max_connections: 25

# Override with environment variables in production
# DATABASE_HOST=prod-db.example.com
# DATABASE_MAX_CONNECTIONS=100
```

### Configuration Validation

Always validate configuration at startup:

```go
type DatabaseConfig struct {
    Host           string `yaml:"host" env:"DB_HOST" validate:"required,hostname"`
    Port           int    `yaml:"port" env:"DB_PORT" validate:"required,min=1,max=65535"`
    MaxConnections int    `yaml:"max_connections" env:"DB_MAX_CONNECTIONS" validate:"min=1,max=1000"`
}

// Validation happens automatically with config.Load()
cfg := &Config{}
if err := config.Load(cfg); err != nil {
    log.Fatal("Invalid configuration:", err)
}
```

### Secrets Management

Never store secrets in configuration files:

```go
// Good: Use environment variables or secret management
type Config struct {
    Database DatabaseConfig `yaml:"database"`
    JWT      JWTConfig      `yaml:"jwt"`
}

type DatabaseConfig struct {
    Host     string `yaml:"host" env:"DB_HOST"`
    Password string `env:"DB_PASSWORD" validate:"required"` // Only from env
}

// Better: Use secret management
secrets := security.NewVaultSecrets(cfg.Vault)
dbPassword, err := secrets.GetSecret(ctx, "database/password")
```

## Error Handling

### Use Structured Errors

Always use structured errors with context:

```go
// Good: Structured error with context
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    if err := s.validateUser(req); err != nil {
        return nil, &ServiceError{
            Code:    "VALIDATION_FAILED",
            Message: "User validation failed",
            Cause:   err,
            Context: map[string]interface{}{
                "email": req.Email,
                "field": "email",
            },
        }
    }
    
    user, err := s.repo.Create(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    
    return user, nil
}

// Avoid: Generic errors
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    user, err := s.repo.Create(ctx, req)
    if err != nil {
        return nil, err // No context
    }
    return user, nil
}
```

### Error Logging

Log errors with appropriate levels and context:

```go
// Good: Contextual error logging
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.service.CreateUser(r.Context(), req)
    if err != nil {
        if serviceErr, ok := err.(*ServiceError); ok {
            switch serviceErr.Code {
            case "VALIDATION_FAILED":
                h.logger.Warn("User validation failed", 
                    observability.Field("error", err),
                    observability.Field("email", req.Email),
                )
                http.Error(w, serviceErr.Message, http.StatusBadRequest)
            default:
                h.logger.Error("Failed to create user", err,
                    observability.Field("email", req.Email),
                )
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
            return
        }
    }
}
```

## Observability

### Structured Logging

Use structured logging consistently:

```go
// Good: Structured logging with context
logger := observability.NewLogger(cfg.Logging)

logger.Info("User created successfully",
    observability.Field("user_id", user.ID),
    observability.Field("email", user.Email),
    observability.Field("duration", time.Since(start)),
)

// Include trace context
logger.WithContext(ctx).Info("Processing user request",
    observability.Field("operation", "create_user"),
)

// Avoid: Unstructured logging
log.Printf("User %d created with email %s", user.ID, user.Email)
```

### Metrics Collection

Collect meaningful business and technical metrics:

```go
// Good: Business and technical metrics
metrics := observability.NewMetrics()

// Business metrics
usersCreated := metrics.Counter("users_created_total", "source")
usersCreated.WithLabels(map[string]string{"source": "api"}).Inc()

// Technical metrics
requestDuration := metrics.Histogram("http_request_duration_seconds", "method", "endpoint")
requestDuration.WithLabels(map[string]string{
    "method":   "POST",
    "endpoint": "/users",
}).Observe(duration.Seconds())

// Error metrics
errorRate := metrics.Counter("errors_total", "type", "endpoint")
errorRate.WithLabels(map[string]string{
    "type":     "validation_error",
    "endpoint": "/users",
}).Inc()
```

### Distributed Tracing

Use tracing for complex operations:

```go
// Good: Trace complex operations
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    ctx, span := tracer.Start(ctx, "UserService.CreateUser")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("user.email", req.Email),
        attribute.String("operation", "create_user"),
    )
    
    // Validation span
    if err := s.validateUser(ctx, req); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return nil, err
    }
    
    // Database span (automatically created by data layer)
    user, err := s.repo.Create(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "database error")
        return nil, err
    }
    
    span.SetAttributes(attribute.Int64("user.id", user.ID))
    return user, nil
}
```

## Security

### Authentication Best Practices

Implement robust authentication:

```go
// Good: Proper JWT configuration
auth, err := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
    Audience:     "user-service",
    Issuer:       "https://auth.example.com",
    SkipPaths:    []string{"/health", "/metrics"}, // Public endpoints
})

// Apply to protected endpoints only
server.RegisterMiddleware(auth.Middleware())

// Extract claims in handlers
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
    secCtx := security.FromContext(r.Context())
    if secCtx == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    userID := secCtx.Claims.Subject
    // Use userID for authorization
}
```

### Input Validation

Always validate input data:

```go
// Good: Comprehensive validation
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,containsany=!@#$%^&*"`
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if err := validator.Struct(&req); err != nil {
        http.Error(w, "Validation failed", http.StatusBadRequest)
        return
    }
    
    // Sanitize input
    req.Name = strings.TrimSpace(req.Name)
    req.Email = strings.ToLower(strings.TrimSpace(req.Email))
}
```

### Authorization

Implement proper authorization checks:

```go
// Good: Role-based authorization
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
    secCtx := security.FromContext(r.Context())
    userID := mux.Vars(r)["id"]
    
    // Check if user can delete this resource
    if !h.canDeleteUser(secCtx, userID) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
    
    // Proceed with deletion
}

func (h *UserHandler) canDeleteUser(secCtx *security.SecurityContext, userID string) bool {
    // Users can delete their own account
    if secCtx.Claims.Subject == userID {
        return true
    }
    
    // Admins can delete any user
    for _, role := range secCtx.Roles {
        if role == "admin" {
            return true
        }
    }
    
    return false
}
```

## Database Operations

### Transaction Management

Use transactions for data consistency:

```go
// Good: Proper transaction usage
func (s *UserService) CreateUserWithProfile(ctx context.Context, req CreateUserRequest) (*User, error) {
    return s.db.Transaction(ctx, func(tx data.Transaction) (*User, error) {
        // Create user
        user, err := s.userRepo.CreateTx(ctx, tx, req.User)
        if err != nil {
            return nil, fmt.Errorf("failed to create user: %w", err)
        }
        
        // Create profile
        profile := &Profile{
            UserID: user.ID,
            Bio:    req.Profile.Bio,
        }
        if err := s.profileRepo.CreateTx(ctx, tx, profile); err != nil {
            return nil, fmt.Errorf("failed to create profile: %w", err)
        }
        
        // Publish event (uses Outbox pattern automatically)
        if err := s.broker.Publish(ctx, "user.created", UserCreatedEvent{
            UserID: user.ID,
            Email:  user.Email,
        }); err != nil {
            return nil, fmt.Errorf("failed to publish event: %w", err)
        }
        
        return user, nil
    })
}
```

### Query Optimization

Write efficient database queries:

```go
// Good: Efficient queries with proper indexing
func (r *UserRepository) GetActiveUsers(ctx context.Context, limit, offset int) ([]*User, error) {
    query := `
        SELECT id, name, email, created_at 
        FROM users 
        WHERE active = true 
        ORDER BY created_at DESC 
        LIMIT $1 OFFSET $2
    `
    
    rows, err := r.db.Query(ctx, query, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("failed to query users: %w", err)
    }
    defer rows.Close()
    
    var users []*User
    for rows.Next() {
        user := &User{}
        if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan user: %w", err)
        }
        users = append(users, user)
    }
    
    return users, nil
}

// Avoid: N+1 queries
func (r *UserRepository) GetUsersWithProfiles(ctx context.Context) ([]*User, error) {
    users, err := r.GetAllUsers(ctx)
    if err != nil {
        return nil, err
    }
    
    for _, user := range users {
        // This creates N+1 queries
        profile, err := r.GetProfileByUserID(ctx, user.ID)
        if err != nil {
            return nil, err
        }
        user.Profile = profile
    }
    
    return users, nil
}
```

### Caching Strategy

Implement effective caching:

```go
// Good: Cache with proper TTL and invalidation
func (s *UserService) GetUser(ctx context.Context, userID int64) (*User, error) {
    cacheKey := fmt.Sprintf("user:%d", userID)
    
    // Try cache first
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        var user User
        if err := json.Unmarshal(cached, &user); err == nil {
            return &user, nil
        }
    }
    
    // Fallback to database
    user, err := s.repo.GetByID(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    if data, err := json.Marshal(user); err == nil {
        s.cache.Set(ctx, cacheKey, data, 15*time.Minute)
    }
    
    return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, userID int64, req UpdateUserRequest) (*User, error) {
    user, err := s.repo.Update(ctx, userID, req)
    if err != nil {
        return nil, err
    }
    
    // Invalidate cache
    cacheKey := fmt.Sprintf("user:%d", userID)
    s.cache.Delete(ctx, cacheKey)
    
    return user, nil
}
```

## Messaging

### Event Design

Design events for loose coupling:

```go
// Good: Well-designed events
type UserCreatedEvent struct {
    UserID    int64     `json:"user_id"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    Version   string    `json:"version"` // For schema evolution
}

type OrderStatusChangedEvent struct {
    OrderID   int64  `json:"order_id"`
    UserID    int64  `json:"user_id"`
    OldStatus string `json:"old_status"`
    NewStatus string `json:"new_status"`
    ChangedAt time.Time `json:"changed_at"`
    Version   string    `json:"version"`
}

// Avoid: Coupled events
type UserCreatedEvent struct {
    User *User `json:"user"` // Exposes internal structure
}
```

### Message Handling

Implement idempotent message handlers:

```go
// Good: Idempotent message handling
func (h *NotificationHandler) HandleUserCreated(ctx context.Context, msg messaging.Message) error {
    var event UserCreatedEvent
    if err := json.Unmarshal(msg.Body(), &event); err != nil {
        return fmt.Errorf("failed to unmarshal event: %w", err)
    }
    
    // Check if already processed (idempotency)
    processed, err := h.repo.IsEventProcessed(ctx, msg.ID())
    if err != nil {
        return fmt.Errorf("failed to check event status: %w", err)
    }
    if processed {
        h.logger.Info("Event already processed", 
            observability.Field("event_id", msg.ID()),
        )
        return nil
    }
    
    // Process the event
    if err := h.sendWelcomeEmail(ctx, event.Email, event.Name); err != nil {
        return fmt.Errorf("failed to send welcome email: %w", err)
    }
    
    // Mark as processed
    if err := h.repo.MarkEventProcessed(ctx, msg.ID()); err != nil {
        return fmt.Errorf("failed to mark event as processed: %w", err)
    }
    
    return nil
}
```

## Testing

### Unit Testing

Write comprehensive unit tests:

```go
// Good: Comprehensive unit test
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        req     CreateUserRequest
        mockFn  func(*mocks.UserRepository, *mocks.Cache)
        want    *User
        wantErr bool
    }{
        {
            name: "successful creation",
            req: CreateUserRequest{
                Name:  "John Doe",
                Email: "john@example.com",
            },
            mockFn: func(repo *mocks.UserRepository, cache *mocks.Cache) {
                repo.On("Create", mock.Anything, mock.AnythingOfType("*User")).
                    Return(&User{ID: 1, Name: "John Doe", Email: "john@example.com"}, nil)
            },
            want: &User{ID: 1, Name: "John Doe", Email: "john@example.com"},
        },
        {
            name: "duplicate email error",
            req: CreateUserRequest{
                Name:  "John Doe",
                Email: "existing@example.com",
            },
            mockFn: func(repo *mocks.UserRepository, cache *mocks.Cache) {
                repo.On("Create", mock.Anything, mock.AnythingOfType("*User")).
                    Return(nil, errors.New("duplicate email"))
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &mocks.UserRepository{}
            cache := &mocks.Cache{}
            logger := observability.NewTestLogger()
            
            tt.mockFn(repo, cache)
            
            service := NewUserService(repo, cache, logger)
            got, err := service.CreateUser(context.Background(), tt.req)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
            repo.AssertExpectations(t)
        })
    }
}
```

### Integration Testing

Use testcontainers for integration tests:

```go
// Good: Integration test with real database
func TestUserRepository_Integration(t *testing.T) {
    // Start PostgreSQL container
    ctx := context.Background()
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "postgres:15",
            ExposedPorts: []string{"5432/tcp"},
            Env: map[string]string{
                "POSTGRES_DB":       "testdb",
                "POSTGRES_USER":     "test",
                "POSTGRES_PASSWORD": "test",
            },
            WaitingFor: wait.ForLog("database system is ready to accept connections"),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)
    
    // Get connection details
    host, err := postgres.Host(ctx)
    require.NoError(t, err)
    port, err := postgres.MappedPort(ctx, "5432")
    require.NoError(t, err)
    
    // Connect to database
    config := data.DatabaseConfig{
        Host:     host,
        Port:     port.Int(),
        Name:     "testdb",
        User:     "test",
        Password: "test",
        SSLMode:  "disable",
    }
    
    db, err := data.NewPostgresDB(config)
    require.NoError(t, err)
    defer db.Close()
    
    // Run migrations
    migrator := data.NewMigrator(db, "../../migrations")
    require.NoError(t, migrator.Up(ctx))
    
    // Test repository
    repo := NewUserRepository(db)
    
    user := &User{
        Name:  "John Doe",
        Email: "john@example.com",
    }
    
    err = repo.Create(ctx, user)
    assert.NoError(t, err)
    assert.NotZero(t, user.ID)
    
    retrieved, err := repo.GetByID(ctx, user.ID)
    assert.NoError(t, err)
    assert.Equal(t, user.Name, retrieved.Name)
    assert.Equal(t, user.Email, retrieved.Email)
}
```

## Performance

### Connection Pooling

Configure appropriate connection pools:

```yaml
# Good: Proper connection pool configuration
database:
  max_connections: 25      # Based on expected load
  min_connections: 5       # Keep minimum connections warm
  max_conn_lifetime: "1h"  # Rotate connections
  max_conn_idle_time: "30m" # Close idle connections

cache:
  redis_url: "redis://localhost:6379"
  pool_size: 10           # Redis connection pool
  min_idle_conns: 2       # Minimum idle connections
```

### Request Timeouts

Set appropriate timeouts:

```go
// Good: Proper timeout configuration
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Set timeout for database operations
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    return s.repo.Create(ctx, req)
}

// HTTP client with timeout
client := &http.Client{
    Timeout: 10 * time.Second,
    Transport: &http.Transport{
        DialContext: (&net.Dialer{
            Timeout: 5 * time.Second,
        }).DialContext,
        ResponseHeaderTimeout: 5 * time.Second,
    },
}
```

### Memory Management

Avoid memory leaks:

```go
// Good: Proper resource cleanup
func (s *UserService) ProcessUsers(ctx context.Context) error {
    rows, err := s.db.Query(ctx, "SELECT id, name, email FROM users")
    if err != nil {
        return err
    }
    defer rows.Close() // Always close rows
    
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            return err
        }
        
        // Process user
        if err := s.processUser(ctx, &user); err != nil {
            s.logger.Error("Failed to process user", err,
                observability.Field("user_id", user.ID),
            )
            continue // Don't fail entire batch
        }
    }
    
    return rows.Err()
}
```

## Deployment

### Health Checks

Implement comprehensive health checks:

```go
// Good: Comprehensive health checks
func setupHealthChecks(service *core.Service, db data.Database, cache data.Cache) {
    health := observability.NewHealthChecker()
    
    // Database health check
    health.AddCheck("database", func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
        defer cancel()
        return db.Ping(ctx)
    })
    
    // Cache health check
    health.AddCheck("cache", func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
        defer cancel()
        return cache.Ping(ctx)
    })
    
    // External service health check
    health.AddCheck("auth_service", func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
        defer cancel()
        
        resp, err := http.Get("https://auth.example.com/health")
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("auth service unhealthy: %d", resp.StatusCode)
        }
        return nil
    })
    
    service.AddDependency("health", health)
}
```

### Graceful Shutdown

Implement proper shutdown handling:

```go
// Good: Graceful shutdown with timeout
func main() {
    service := core.NewService(metadata)
    
    // Setup service components...
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutdown signal received")
        
        // Give service time to finish current requests
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer shutdownCancel()
        
        if err := service.Shutdown(shutdownCtx); err != nil {
            logger.Error("Error during shutdown", err)
        }
        
        cancel()
    }()
    
    if err := service.Start(ctx); err != nil {
        logger.Error("Service failed to start", err)
        os.Exit(1)
    }
    
    <-ctx.Done()
    logger.Info("Service stopped")
}
```

### Resource Limits

Set appropriate resource limits:

```yaml
# Kubernetes deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  template:
    spec:
      containers:
      - name: user-service
        image: user-service:latest
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

Following these best practices will help you build robust, maintainable, and scalable microservices with MicroLib. Remember to adapt these guidelines to your specific use case and requirements.