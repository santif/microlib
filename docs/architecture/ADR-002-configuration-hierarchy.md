# ADR-002: Configuration Hierarchy and Validation

## Status

Accepted

## Context

Microservices need flexible configuration management that works across different environments (development, staging, production) while maintaining type safety and validation. Configuration sources include environment variables, configuration files, and command-line flags.

Key requirements:
- Support multiple configuration sources with clear precedence
- Type-safe configuration with compile-time checking
- Automatic validation with clear error messages
- Hot-reload capability for non-critical settings
- Environment-specific overrides

## Decision

We will implement a hierarchical configuration system with the following precedence (highest to lowest):

1. **Environment Variables** (highest priority)
2. **Configuration Files** (YAML/TOML)
3. **Command Line Flags** (lowest priority)

Configuration will use Go structs with tags for validation and source mapping:

```go
type Config struct {
    Port int    `yaml:"port" env:"PORT" flag:"port" validate:"required,min=1,max=65535"`
    Host string `yaml:"host" env:"HOST" flag:"host" validate:"required"`
}
```

Key components:
- `Manager` interface for loading and validating configuration
- `Source` interface for different configuration sources
- Struct tag-based validation using `go-playground/validator`
- Thread-safe configuration access with RWMutex
- Optional hot-reload for specific configuration sections

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

## Consequences

### Positive
- Clear configuration precedence eliminates ambiguity
- Type safety prevents runtime configuration errors
- Automatic validation with descriptive error messages
- Consistent configuration patterns across all services
- Environment-specific overrides without code changes
- Hot-reload capability for operational flexibility

### Negative
- Additional complexity compared to simple environment variable approach
- Struct tags can become verbose for complex configurations
- Hot-reload adds complexity and potential race conditions
- Learning curve for developers unfamiliar with the tag system

## Alternatives Considered

### 1. Environment Variables Only
Use only environment variables for configuration.
- **Rejected**: Difficult to manage complex nested configurations and lacks type safety

### 2. Configuration Files Only
Use only YAML/TOML configuration files.
- **Rejected**: Doesn't support environment-specific overrides without file modifications

### 3. Viper Library
Use the popular Viper configuration library.
- **Rejected**: Heavy dependency with features we don't need, less control over validation

### 4. JSON Configuration
Use JSON instead of YAML/TOML for configuration files.
- **Rejected**: JSON doesn't support comments and is less human-readable

### 5. Runtime Configuration Service
Use an external configuration service like Consul or etcd.
- **Rejected**: Adds operational complexity and external dependencies for basic configuration needs