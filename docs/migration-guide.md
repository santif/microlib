# Migration Guide

This guide helps you migrate existing Go microservices to use the MicroLib framework, or upgrade between MicroLib versions.

## Table of Contents

- [Migrating from Existing Go Services](#migrating-from-existing-go-services)
- [Version Upgrade Guides](#version-upgrade-guides)
- [Common Migration Patterns](#common-migration-patterns)
- [Troubleshooting](#troubleshooting)

## Migrating from Existing Go Services

### Prerequisites

Before starting the migration:

1. Ensure your service uses Go 1.22 or higher
2. Have PostgreSQL and Redis available for testing
3. Back up your existing service code
4. Review the [Architecture Decision Records](architecture/) to understand MicroLib's design

### Step 1: Analyze Your Current Service

Identify the components in your existing service:

- **HTTP Server**: Gin, Echo, net/http, etc.
- **Configuration**: Viper, environment variables, config files
- **Logging**: logrus, zap, standard log
- **Database**: database/sql, GORM, custom drivers
- **Metrics**: Prometheus client, custom metrics
- **Authentication**: Custom JWT, OAuth libraries
- **Background Jobs**: Custom implementations, third-party libraries

### Step 2: Create Migration Plan

Create a phased migration plan:

1. **Phase 1**: Core service structure and configuration
2. **Phase 2**: HTTP server and middleware
3. **Phase 3**: Database and data layer
4. **Phase 4**: Observability (logging, metrics, tracing)
5. **Phase 5**: Authentication and authorization
6. **Phase 6**: Messaging and background jobs

### Step 3: Phase 1 - Core Service Structure

#### Before (Example with Gin)
```go
func main() {
    // Load config
    viper.SetConfigName("config")
    viper.AddConfigPath(".")
    if err := viper.ReadInConfig(); err != nil {
        log.Fatal(err)
    }
    
    // Setup router
    r := gin.Default()
    r.GET("/health", healthHandler)
    r.POST("/users", createUserHandler)
    
    // Start server
    log.Fatal(r.Run(":8080"))
}
```

#### After (MicroLib)
```go
func main() {
    // Load configuration
    cfg := &Config{}
    if err := config.Load(cfg); err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // Create service
    service := core.NewService(core.ServiceMetadata{
        Name:    cfg.Service.Name,
        Version: cfg.Service.Version,
    })

    // Setup HTTP server
    httpServer := http.NewServer(cfg.HTTP)
    httpServer.RegisterHandler("/health", healthHandler)
    httpServer.RegisterHandler("/users", createUserHandler)
    
    service.AddServer(httpServer)

    // Start service with graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        cancel()
    }()

    if err := service.Start(ctx); err != nil {
        log.Fatal("Service failed to start:", err)
    }
}
```

### Step 4: Phase 2 - HTTP Server Migration

#### Migrating from Gin

**Before:**
```go
r := gin.Default()
r.Use(gin.Logger())
r.Use(gin.Recovery())

// Custom middleware
r.Use(func(c *gin.Context) {
    // Custom logic
    c.Next()
})

r.GET("/users/:id", func(c *gin.Context) {
    id := c.Param("id")
    // Handler logic
    c.JSON(200, gin.H{"id": id})
})
```

**After:**
```go
server := http.NewServer(cfg.HTTP)

// Built-in middleware
server.RegisterMiddleware(http.LoggingMiddleware(logger))
server.RegisterMiddleware(http.RecoveryMiddleware(logger))

// Custom middleware
server.RegisterMiddleware(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Custom logic
        next.ServeHTTP(w, r)
    })
})

// Handler registration
server.RegisterHandler("/users/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["id"]
    // Handler logic
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"id": id})
}))
```

#### Migrating from Echo

**Before:**
```go
e := echo.New()
e.Use(middleware.Logger())
e.Use(middleware.Recover())

e.GET("/users/:id", func(c echo.Context) error {
    id := c.Param("id")
    return c.JSON(http.StatusOK, map[string]string{"id": id})
})
```

**After:** (Same as Gin migration above)

### Step 5: Phase 3 - Database Migration

#### Migrating from database/sql

**Before:**
```go
db, err := sql.Open("postgres", connectionString)
if err != nil {
    log.Fatal(err)
}

func createUser(ctx context.Context, user *User) error {
    query := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id"
    err := db.QueryRowContext(ctx, query, user.Name, user.Email).Scan(&user.ID)
    return err
}
```

**After:**
```go
db, err := data.NewPostgresDB(cfg.Database)
if err != nil {
    log.Fatal(err)
}

func createUser(ctx context.Context, user *User) error {
    query := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id"
    row := db.QueryRow(ctx, query, user.Name, user.Email)
    return row.Scan(&user.ID)
}
```

#### Migrating from GORM

**Before:**
```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
if err != nil {
    log.Fatal(err)
}

func createUser(ctx context.Context, user *User) error {
    return db.WithContext(ctx).Create(user).Error
}
```

**After:**
```go
db, err := data.NewPostgresDB(cfg.Database)
if err != nil {
    log.Fatal(err)
}

func createUser(ctx context.Context, user *User) error {
    query := "INSERT INTO users (name, email, created_at) VALUES ($1, $2, $3) RETURNING id"
    row := db.QueryRow(ctx, query, user.Name, user.Email, time.Now())
    return row.Scan(&user.ID)
}
```

### Step 6: Phase 4 - Observability Migration

#### Migrating from logrus

**Before:**
```go
import "github.com/sirupsen/logrus"

logger := logrus.New()
logger.SetFormatter(&logrus.JSONFormatter{})

logger.WithFields(logrus.Fields{
    "user_id": 123,
    "action":  "create",
}).Info("User created")
```

**After:**
```go
import "github.com/santif/microlib/observability"

logger := observability.NewLogger(cfg.Logging)

logger.Info("User created",
    observability.Field("user_id", 123),
    observability.Field("action", "create"),
)
```

#### Migrating Prometheus Metrics

**Before:**
```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "status"},
    )
)

func init() {
    prometheus.MustRegister(requestsTotal)
}

// In handler
requestsTotal.WithLabelValues("POST", "200").Inc()
```

**After:**
```go
import "github.com/santif/microlib/observability"

metrics := observability.NewMetrics()
requestsTotal := metrics.Counter("http_requests_total", "method", "status")

// In handler
requestsTotal.WithLabels(map[string]string{
    "method": "POST",
    "status": "200",
}).Inc()
```

### Step 7: Phase 5 - Authentication Migration

#### Migrating Custom JWT

**Before:**
```go
import "github.com/golang-jwt/jwt/v4"

func validateToken(tokenString string) (*jwt.Token, error) {
    return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Key validation logic
        return publicKey, nil
    })
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenString := extractToken(r)
        token, err := validateToken(tokenString)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        // Add claims to context
        next.ServeHTTP(w, r)
    })
}
```

**After:**
```go
import "github.com/santif/microlib/security"

auth, err := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
    Audience:     "my-service",
})
if err != nil {
    log.Fatal(err)
}

server.RegisterMiddleware(auth.Middleware())
```

### Step 8: Phase 6 - Background Jobs Migration

#### Migrating Custom Job Processing

**Before:**
```go
func processJobs() {
    for {
        // Poll for jobs
        jobs := getJobsFromQueue()
        for _, job := range jobs {
            go func(j Job) {
                if err := j.Execute(); err != nil {
                    log.Printf("Job failed: %v", err)
                }
            }(job)
        }
        time.Sleep(5 * time.Second)
    }
}
```

**After:**
```go
import "github.com/santif/microlib/jobs"

scheduler := jobs.NewScheduler(cfg.Jobs)
queue := jobs.NewJobQueue(cfg.Jobs)

// Schedule recurring job
scheduler.Schedule("0 */5 * * * *", jobs.JobFunc(func(ctx context.Context) error {
    log.Println("Running scheduled task")
    return nil
}))

// Process job queue
go func() {
    queue.Process(ctx, func(ctx context.Context, job jobs.Job) error {
        return job.Execute(ctx)
    })
}()
```

## Version Upgrade Guides

### Upgrading from v1.0 to v1.1

#### Breaking Changes
- Configuration structure changed for observability settings
- Logger interface added `WithFields` method

#### Migration Steps

1. Update configuration:
```yaml
# Before
logging:
  level: "info"
  format: "json"

# After
observability:
  logging:
    level: "info"
    format: "json"
  metrics:
    enabled: true
  tracing:
    enabled: true
```

2. Update logger usage:
```go
// Before
logger.Info("Message", observability.Field("key", "value"))

// After
logger.WithFields(observability.Field("key", "value")).Info("Message")
```

## Common Migration Patterns

### Pattern 1: Gradual HTTP Handler Migration

Migrate handlers one by one while keeping the old server running:

```go
// Create both old and new servers
oldServer := gin.Default()
newServer := http.NewServer(cfg.HTTP)

// Migrate handlers gradually
newServer.RegisterHandler("/users", newUserHandler)
oldServer.Any("/users", func(c *gin.Context) {
    // Proxy to new handler
    proxyToNewHandler(c)
})

// Keep other handlers on old server
oldServer.GET("/orders", oldOrderHandler)
```

### Pattern 2: Database Transaction Migration

Wrap existing database operations in MicroLib transactions:

```go
// Before
tx, err := db.Begin()
if err != nil {
    return err
}
defer tx.Rollback()

// ... operations ...

return tx.Commit()

// After
return db.Transaction(ctx, func(tx data.Transaction) error {
    // ... same operations using tx ...
    return nil
})
```

### Pattern 3: Configuration Migration

Gradually migrate configuration while maintaining backward compatibility:

```go
type Config struct {
    // New structure
    Service ServiceConfig `yaml:"service"`
    HTTP    HTTPConfig    `yaml:"http"`
    
    // Legacy fields (deprecated)
    Port int    `yaml:"port,omitempty"`
    Host string `yaml:"host,omitempty"`
}

func (c *Config) migrate() {
    // Handle legacy configuration
    if c.Port != 0 && c.HTTP.Port == 0 {
        c.HTTP.Port = c.Port
    }
    if c.Host != "" && c.HTTP.Host == "" {
        c.HTTP.Host = c.Host
    }
}
```

## Troubleshooting

### Common Issues

#### Issue: Service won't start after migration
**Symptoms**: Service exits immediately or fails to bind to port

**Solutions**:
1. Check configuration format and required fields
2. Verify port availability
3. Check database connectivity
4. Review service dependencies

#### Issue: HTTP handlers not working
**Symptoms**: 404 errors for existing endpoints

**Solutions**:
1. Verify handler registration syntax
2. Check URL pattern format (use `{id}` instead of `:id`)
3. Ensure middleware order is correct
4. Test with simple handlers first

#### Issue: Database queries failing
**Symptoms**: SQL errors or connection issues

**Solutions**:
1. Verify connection string format
2. Check PostgreSQL version compatibility
3. Update query syntax for pgx driver
4. Test database connectivity separately

#### Issue: Authentication not working
**Symptoms**: All requests return 401 Unauthorized

**Solutions**:
1. Verify JWKS endpoint accessibility
2. Check JWT token format and claims
3. Validate audience and issuer configuration
4. Test with skip_paths for debugging

### Migration Checklist

- [ ] Go version 1.22+ installed
- [ ] Dependencies updated in go.mod
- [ ] Configuration migrated to new format
- [ ] HTTP server migrated with proper handlers
- [ ] Database operations updated
- [ ] Logging migrated to structured format
- [ ] Metrics collection updated
- [ ] Authentication middleware configured
- [ ] Health checks implemented
- [ ] Graceful shutdown implemented
- [ ] Tests updated for new interfaces
- [ ] Documentation updated

### Getting Help

If you encounter issues during migration:

1. Check the [troubleshooting guide](troubleshooting.md)
2. Review [examples](../examples/) for reference implementations
3. Join our [Discord community](https://discord.gg/microlib)
4. Open an issue on [GitHub](https://github.com/santif/microlib/issues)