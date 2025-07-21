# Troubleshooting Guide

This guide helps you diagnose and resolve common issues when using the MicroLib framework.

## Table of Contents

- [Service Startup Issues](#service-startup-issues)
- [Configuration Problems](#configuration-problems)
- [Database Connection Issues](#database-connection-issues)
- [HTTP Server Problems](#http-server-problems)
- [Authentication Issues](#authentication-issues)
- [Messaging Problems](#messaging-problems)
- [Performance Issues](#performance-issues)
- [Observability Problems](#observability-problems)
- [Deployment Issues](#deployment-issues)

## Service Startup Issues

### Service Exits Immediately

**Symptoms:**
- Service starts and exits without error messages
- Process terminates with exit code 0 or 1

**Common Causes:**
1. Configuration validation failure
2. Missing required environment variables
3. Port already in use
4. Database connection failure

**Diagnosis:**
```bash
# Check service logs
docker logs <container-name>

# Run with debug logging
export LOGGING_LEVEL=debug
./your-service

# Check port usage
lsof -i :8080
netstat -tulpn | grep :8080
```

**Solutions:**
```go
// Add detailed startup logging
func main() {
    logger := observability.NewLogger(observability.LogConfig{
        Level:  "debug",
        Format: "json",
    })
    
    logger.Info("Starting service initialization")
    
    // Load configuration with error details
    cfg := &Config{}
    if err := config.Load(cfg); err != nil {
        logger.Error("Configuration loading failed", err)
        os.Exit(1)
    }
    logger.Info("Configuration loaded successfully")
    
    // Test database connection
    db, err := data.NewPostgresDB(cfg.Database)
    if err != nil {
        logger.Error("Database connection failed", err)
        os.Exit(1)
    }
    logger.Info("Database connection established")
    
    // Continue with service setup...
}
```

### Dependency Health Check Failures

**Symptoms:**
- Service fails to start with "dependency unhealthy" errors
- Health check endpoints return 503 Service Unavailable

**Diagnosis:**
```bash
# Test database connectivity
psql -h localhost -p 5432 -U postgres -d mydb -c "SELECT 1;"

# Test Redis connectivity
redis-cli -h localhost -p 6379 ping

# Check network connectivity
telnet database-host 5432
nc -zv redis-host 6379
```

**Solutions:**
```go
// Add timeout and retry logic to health checks
func setupHealthChecks(service *core.Service, db data.Database) {
    health := observability.NewHealthChecker()
    
    health.AddCheck("database", func(ctx context.Context) error {
        // Add timeout
        ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        
        // Retry logic
        var lastErr error
        for i := 0; i < 3; i++ {
            if err := db.Ping(ctx); err == nil {
                return nil
            } else {
                lastErr = err
                time.Sleep(time.Duration(i+1) * time.Second)
            }
        }
        return fmt.Errorf("database health check failed after 3 retries: %w", lastErr)
    })
    
    service.AddDependency("health", health)
}
```

## Configuration Problems

### Configuration Validation Errors

**Symptoms:**
- Service fails to start with validation error messages
- "required field missing" or "invalid value" errors

**Common Issues:**
```yaml
# Missing required fields
service:
  name: "user-service"
  # version: "1.0.0"  # Missing required field

database:
  host: "localhost"
  port: 5432
  # name: "mydb"      # Missing required field
```

**Solutions:**
```yaml
# Complete configuration example
service:
  name: "user-service"
  version: "1.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  name: "mydb"
  user: "postgres"
  password: "postgres"
  ssl_mode: "disable"
  max_connections: 25

logging:
  level: "info"
  format: "json"
```

### Environment Variable Override Issues

**Symptoms:**
- Environment variables not overriding configuration file values
- Unexpected configuration values

**Diagnosis:**
```bash
# Check environment variables
env | grep -E "(DB_|LOGGING_|SERVICE_)"

# Test configuration loading
export DB_HOST=test-host
export LOGGING_LEVEL=debug
./your-service --dry-run  # If you implement this flag
```

**Solutions:**
```go
// Debug configuration loading
type Config struct {
    Database DatabaseConfig `yaml:"database"`
    Logging  LoggingConfig  `yaml:"logging"`
}

func (c *Config) Debug() {
    fmt.Printf("Database Host: %s (from env: %s)\n", 
        c.Database.Host, os.Getenv("DB_HOST"))
    fmt.Printf("Logging Level: %s (from env: %s)\n", 
        c.Logging.Level, os.Getenv("LOGGING_LEVEL"))
}

// In main()
cfg := &Config{}
if err := config.Load(cfg); err != nil {
    log.Fatal(err)
}
cfg.Debug() // Remove in production
```

## Database Connection Issues

### Connection Pool Exhaustion

**Symptoms:**
- "connection pool exhausted" errors
- Slow database operations
- Timeouts on database queries

**Diagnosis:**
```sql
-- Check active connections
SELECT count(*) as active_connections 
FROM pg_stat_activity 
WHERE state = 'active';

-- Check connection limits
SHOW max_connections;

-- Check long-running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query 
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
```

**Solutions:**
```yaml
# Adjust connection pool settings
database:
  max_connections: 25        # Increase if needed
  min_connections: 5         # Keep some connections warm
  max_conn_lifetime: "1h"    # Rotate connections
  max_conn_idle_time: "30m"  # Close idle connections
```

```go
// Ensure proper connection cleanup
func (r *UserRepository) GetUsers(ctx context.Context) ([]*User, error) {
    rows, err := r.db.Query(ctx, "SELECT id, name, email FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close() // Always close rows!
    
    var users []*User
    for rows.Next() {
        user := &User{}
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, rows.Err()
}
```

### Migration Failures

**Symptoms:**
- Service fails to start with migration errors
- Database schema inconsistencies

**Diagnosis:**
```sql
-- Check migration status
SELECT * FROM schema_migrations ORDER BY version;

-- Check for failed migrations
SELECT * FROM schema_migrations WHERE dirty = true;
```

**Solutions:**
```bash
# Reset migrations (development only)
make migrate-reset

# Force migration version
migrate -path ./migrations -database "postgres://..." force 20240115000001

# Manual migration repair
psql -d mydb -c "UPDATE schema_migrations SET dirty = false WHERE version = 20240115000001;"
```

## HTTP Server Problems

### Port Binding Issues

**Symptoms:**
- "bind: address already in use" errors
- Service fails to start HTTP server

**Diagnosis:**
```bash
# Check what's using the port
lsof -i :8080
netstat -tulpn | grep :8080

# Kill process using the port
kill -9 $(lsof -t -i:8080)
```

**Solutions:**
```yaml
# Use different port
service:
  port: 8081  # Change from default 8080

# Or use environment variable
# export SERVICE_PORT=8081
```

### Middleware Chain Issues

**Symptoms:**
- Middleware not executing in expected order
- Authentication bypassed unexpectedly
- CORS errors

**Diagnosis:**
```go
// Add logging to middleware chain
func debugMiddleware(name string) http.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            log.Printf("Executing middleware: %s", name)
            next.ServeHTTP(w, r)
            log.Printf("Completed middleware: %s", name)
        })
    }
}

// Register in order
server.RegisterMiddleware(debugMiddleware("logging"))
server.RegisterMiddleware(debugMiddleware("auth"))
server.RegisterMiddleware(debugMiddleware("cors"))
```

**Solutions:**
```go
// Correct middleware order
server := http.NewServer(cfg.HTTP)

// 1. Logging (first to capture all requests)
server.RegisterMiddleware(http.LoggingMiddleware(logger))

// 2. Recovery (catch panics early)
server.RegisterMiddleware(http.RecoveryMiddleware(logger))

// 3. CORS (before auth to handle preflight)
server.RegisterMiddleware(http.CORSMiddleware(cfg.CORS))

// 4. Authentication (after CORS)
if cfg.Security.JWT.Enabled {
    auth := security.NewJWTAuthenticator(cfg.Security.JWT)
    server.RegisterMiddleware(auth.Middleware())
}

// 5. Metrics (last to measure everything)
server.RegisterMiddleware(http.MetricsMiddleware(metrics))
```

## Authentication Issues

### JWT Token Validation Failures

**Symptoms:**
- All requests return 401 Unauthorized
- "invalid token" errors in logs
- JWKS endpoint errors

**Diagnosis:**
```bash
# Test JWKS endpoint
curl -v https://auth.example.com/.well-known/jwks.json

# Decode JWT token (without verification)
echo "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9..." | base64 -d

# Test with curl
curl -H "Authorization: Bearer <token>" http://localhost:8080/users
```

**Solutions:**
```go
// Add detailed JWT debugging
auth, err := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
    Audience:     "user-service",
    Issuer:       "https://auth.example.com",
    SkipPaths:    []string{"/health", "/metrics", "/debug"}, // Add debug endpoint
})

// Add debug endpoint (remove in production)
server.RegisterHandler("/debug/auth", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    token := extractToken(r)
    if token == "" {
        http.Error(w, "No token provided", http.StatusBadRequest)
        return
    }
    
    claims, err := auth.ValidateToken(r.Context(), token)
    if err != nil {
        http.Error(w, fmt.Sprintf("Token validation failed: %v", err), http.StatusUnauthorized)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(claims)
}))
```

### JWKS Endpoint Issues

**Symptoms:**
- "failed to fetch JWKS" errors
- Intermittent authentication failures
- Certificate verification errors

**Solutions:**
```go
// Configure HTTP client for JWKS
jwksClient := &http.Client{
    Timeout: 10 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: false, // Set to true only for testing
        },
        DialContext: (&net.Dialer{
            Timeout: 5 * time.Second,
        }).DialContext,
    },
}

auth, err := security.NewJWTAuthenticator(security.JWTConfig{
    JWKSEndpoint: "https://auth.example.com/.well-known/jwks.json",
    HTTPClient:   jwksClient,
    CacheTTL:     5 * time.Minute, // Cache JWKS keys
})
```

## Messaging Problems

### Outbox Pattern Issues

**Symptoms:**
- Messages not being published
- Duplicate message delivery
- Outbox table growing indefinitely

**Diagnosis:**
```sql
-- Check outbox table
SELECT count(*) FROM outbox_messages WHERE processed_at IS NULL;

-- Check old processed messages
SELECT count(*) FROM outbox_messages 
WHERE processed_at < NOW() - INTERVAL '24 hours';

-- Check failed messages
SELECT * FROM outbox_messages 
WHERE processed_at IS NULL 
AND created_at < NOW() - INTERVAL '1 hour'
ORDER BY created_at;
```

**Solutions:**
```go
// Configure outbox relay properly
relay := messaging.NewOutboxRelay(messaging.RelayConfig{
    PollInterval:    5 * time.Second,
    BatchSize:      100,
    MaxRetries:     3,
    RetryBackoff:   time.Second,
    CleanupEnabled: true,
    CleanupAfter:   24 * time.Hour,
})

// Start relay process
go func() {
    if err := relay.Start(ctx); err != nil {
        logger.Error("Outbox relay failed", err)
    }
}()
```

### Message Broker Connection Issues

**Symptoms:**
- "connection refused" errors
- Messages not being delivered
- Consumer not receiving messages

**Diagnosis:**
```bash
# Test RabbitMQ connection
rabbitmqctl status
curl -u guest:guest http://localhost:15672/api/overview

# Test Kafka connection
kafka-topics.sh --bootstrap-server localhost:9092 --list
```

**Solutions:**
```yaml
# Configure broker with retry logic
messaging:
  broker: "rabbitmq"
  rabbitmq:
    url: "amqp://guest:guest@localhost:5672/"
    connection_timeout: "30s"
    heartbeat: "10s"
    max_retries: 3
    retry_delay: "5s"
```

## Performance Issues

### High Memory Usage

**Symptoms:**
- Service consuming excessive memory
- Out of memory errors
- Slow garbage collection

**Diagnosis:**
```bash
# Monitor memory usage
top -p $(pgrep your-service)
ps aux | grep your-service

# Go memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap
```

**Solutions:**
```go
// Add memory profiling endpoint (development only)
import _ "net/http/pprof"

func main() {
    // Start pprof server
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Your service code...
}

// Optimize database queries
func (r *UserRepository) GetUsers(ctx context.Context, limit int) ([]*User, error) {
    // Use LIMIT to prevent loading too much data
    query := "SELECT id, name, email FROM users ORDER BY id LIMIT $1"
    rows, err := r.db.Query(ctx, query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    // Pre-allocate slice with known capacity
    users := make([]*User, 0, limit)
    for rows.Next() {
        user := &User{}
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, nil
}
```

### Slow Database Queries

**Symptoms:**
- High response times
- Database timeout errors
- High CPU usage on database server

**Diagnosis:**
```sql
-- Enable query logging
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_min_duration_statement = 1000; -- Log queries > 1s

-- Check slow queries
SELECT query, mean_time, calls, total_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;

-- Check missing indexes
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'public'
AND n_distinct > 100
AND correlation < 0.1;
```

**Solutions:**
```sql
-- Add indexes for common queries
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_orders_user_id_status ON orders(user_id, status);

-- Optimize queries
-- Before: SELECT * FROM users WHERE email = 'user@example.com';
-- After: SELECT id, name, email FROM users WHERE email = 'user@example.com';
```

## Observability Problems

### Missing Logs

**Symptoms:**
- No logs appearing in output
- Logs missing context information
- Inconsistent log formats

**Solutions:**
```go
// Ensure logger is properly configured
logger := observability.NewLogger(observability.LogConfig{
    Level:  "info",
    Format: "json",
    Output: "stdout", // or "stderr"
})

// Use context-aware logging
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Extract trace ID from context
    logger := s.logger.WithContext(ctx)
    
    logger.Info("Creating user",
        observability.Field("email", req.Email),
        observability.Field("operation", "create_user"),
    )
    
    user, err := s.repo.Create(ctx, req)
    if err != nil {
        logger.Error("Failed to create user", err,
            observability.Field("email", req.Email),
        )
        return nil, err
    }
    
    logger.Info("User created successfully",
        observability.Field("user_id", user.ID),
        observability.Field("email", user.Email),
    )
    
    return user, nil
}
```

### Metrics Not Appearing

**Symptoms:**
- /metrics endpoint returns empty or minimal data
- Prometheus not scraping metrics
- Missing custom metrics

**Solutions:**
```go
// Ensure metrics are properly registered
metrics := observability.NewMetrics()

// Create metrics at service startup, not in handlers
var (
    usersCreated = metrics.Counter("users_created_total", "source")
    requestDuration = metrics.Histogram("http_request_duration_seconds", "method", "endpoint")
)

// Use metrics in handlers
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        requestDuration.WithLabels(map[string]string{
            "method":   r.Method,
            "endpoint": "/users",
        }).Observe(duration.Seconds())
    }()
    
    // Handler logic...
    
    usersCreated.WithLabels(map[string]string{
        "source": "api",
    }).Inc()
}
```

## Deployment Issues

### Container Startup Failures

**Symptoms:**
- Container exits immediately
- Health checks failing
- Service not responding to requests

**Diagnosis:**
```bash
# Check container logs
docker logs <container-id>

# Check container resource usage
docker stats <container-id>

# Inspect container
docker inspect <container-id>
```

**Solutions:**
```dockerfile
# Proper Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/server.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/config.yaml .

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

EXPOSE 8080
CMD ["./main"]
```

### Kubernetes Deployment Issues

**Symptoms:**
- Pods in CrashLoopBackOff state
- Service not accessible
- Load balancer not working

**Solutions:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
      - name: user-service
        image: user-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_HOST
          value: "postgres-service"
        - name: REDIS_URL
          value: "redis://redis-service:6379"
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
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        startupProbe:
          httpGet:
            path: /health/startup
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 30
```

## Getting Help

If you can't resolve an issue using this guide:

1. **Check the logs**: Enable debug logging and examine the output
2. **Review configuration**: Validate all configuration values
3. **Test dependencies**: Verify external services are accessible
4. **Consult examples**: Check the [examples directory](../examples/) for reference
5. **Search issues**: Look for similar issues on [GitHub](https://github.com/santif/microlib/issues)
6. **Ask for help**: Join our [Discord community](https://discord.gg/microlib) or open a GitHub issue

When asking for help, please include:
- MicroLib version
- Go version
- Operating system
- Complete error messages
- Relevant configuration
- Steps to reproduce the issue