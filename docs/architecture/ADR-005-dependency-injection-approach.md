# ADR-005: Dependency Injection Approach

## Status

Accepted

## Context

Microservices typically have multiple dependencies (databases, caches, external services) that need to be injected for proper testing and modularity. Go doesn't have built-in dependency injection, so we need to decide on an approach that balances simplicity, testability, and maintainability.

Key requirements:
- Easy testing with mock dependencies
- Clear dependency relationships
- Minimal boilerplate code
- Support for different DI patterns
- No forced framework adoption

## Decision

We will support multiple dependency injection approaches without mandating a specific one:

### 1. Constructor Injection (Recommended)
The primary pattern using explicit constructors:

```go
type UserService struct {
    repo   UserRepository
    cache  Cache
    logger Logger
}

func NewUserService(repo UserRepository, cache Cache, logger Logger) *UserService {
    return &UserService{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}
```

### 2. Wire Integration (Optional)
Support for Google's Wire for compile-time DI:

```go
//go:build wireinject

func InitializeUserService(db Database, cache Cache, logger Logger) *UserService {
    wire.Build(
        NewUserRepository,
        NewUserService,
    )
    return nil
}
```

### 3. Fx Integration (Optional)
Support for Uber's Fx for runtime DI:

```go
func NewUserModule() fx.Option {
    return fx.Module("user",
        fx.Provide(NewUserRepository),
        fx.Provide(NewUserService),
    )
}
```

### 4. Interface-Based Design
All dependencies use interfaces for easy mocking:

```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int64) (*User, error)
}

type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
```

### 5. Framework Integration
The core Service struct accepts dependencies through its interface:

```go
service := core.NewService(metadata)
service.AddDependency("database", db)
service.AddDependency("cache", cache)
```

## Consequences

### Positive
- Flexibility to choose DI approach based on team preferences
- Easy unit testing with interface mocking
- Clear dependency relationships
- No forced framework adoption
- Compile-time safety with Wire
- Runtime flexibility with Fx
- Simple constructor pattern for basic cases

### Negative
- Multiple patterns may cause confusion
- Wire requires code generation step
- Fx adds runtime overhead
- Interface proliferation can be verbose
- No automatic dependency resolution without external tools

## Alternatives Considered

### 1. Mandatory Wire Usage
Force all services to use Wire for dependency injection.
- **Rejected**: Too opinionated, adds build complexity for simple cases

### 2. Mandatory Fx Usage
Force all services to use Fx for dependency injection.
- **Rejected**: Runtime overhead, complex for simple services

### 3. Custom DI Container
Build a custom dependency injection container.
- **Rejected**: Reinventing the wheel, maintenance burden

### 4. Service Locator Pattern
Use a global service locator for dependency resolution.
- **Rejected**: Anti-pattern that hides dependencies and makes testing difficult

### 5. No DI Support
Let developers handle dependency injection manually.
- **Rejected**: Leads to inconsistent patterns and testing difficulties