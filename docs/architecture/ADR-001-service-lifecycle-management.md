# ADR-001: Service Lifecycle Management

## Status

Accepted

## Context

Microservices need consistent lifecycle management including startup, health checking, graceful shutdown, and dependency management. Without a standardized approach, each service implements these concerns differently, leading to inconsistent behavior and operational complexity.

Key requirements:
- Consistent service startup sequence
- Health check integration for Kubernetes probes
- Graceful shutdown with configurable timeouts
- Dependency health validation before accepting traffic
- Service metadata management for observability

## Decision

We will implement a centralized `Service` struct in the `core` package that manages the complete service lifecycle:

1. **Service Metadata**: Each service must provide metadata including name, version, instance ID, and build hash
2. **Dependency Management**: Services register dependencies that implement a `HealthCheck` interface
3. **Server Management**: Services can register multiple servers (HTTP, gRPC) that implement a common `Server` interface
4. **Startup Sequence**: 
   - Validate configuration
   - Check dependency health
   - Start all registered servers
   - Begin accepting traffic
5. **Shutdown Sequence**:
   - Stop accepting new requests
   - Complete in-flight requests (with timeout)
   - Close connections and cleanup resources

```go
type Service struct {
    metadata ServiceMetadata
    deps     []Dependency
    servers  []Server
}

type Dependency interface {
    Name() string
    HealthCheck(ctx context.Context) error
}

type Server interface {
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

## Consequences

### Positive
- Consistent service behavior across all microservices
- Built-in support for Kubernetes health probes
- Simplified service implementation - developers focus on business logic
- Standardized observability metadata
- Graceful shutdown prevents data loss and connection errors

### Negative
- Additional abstraction layer that developers must learn
- Potential performance overhead from health checking
- Less flexibility for services with unique lifecycle requirements

## Alternatives Considered

### 1. No Framework Approach
Let each service implement its own lifecycle management.
- **Rejected**: Leads to inconsistent implementations and duplicated code

### 2. External Process Manager
Use a process manager like systemd or supervisor for lifecycle management.
- **Rejected**: Doesn't provide application-level health checking or graceful shutdown hooks

### 3. Embedding in HTTP/gRPC Servers
Build lifecycle management into individual server implementations.
- **Rejected**: Doesn't support services with multiple servers or complex dependencies

### 4. Event-Driven Lifecycle
Use an event system for lifecycle management.
- **Rejected**: Adds complexity without significant benefits for this use case