# ADR-006: Error Handling Strategy

## Status

Accepted

## Context

Consistent error handling is crucial for microservices, especially for debugging, monitoring, and API responses. Go's error handling is explicit but can lead to inconsistent error types and messages across services without proper guidelines.

Key requirements:
- Consistent error types across all packages
- Structured error information for debugging
- HTTP status code mapping for API responses
- Error wrapping to maintain context
- Integration with observability systems
- Clear error categorization

## Decision

We will implement a structured error handling strategy with the following components:

### 1. ServiceError Type
A standard error type used throughout the framework:

```go
type ServiceError struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Cause   error                  `json:"-"`
    Context map[string]interface{} `json:"context,omitempty"`
}

func (e *ServiceError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ServiceError) Unwrap() error {
    return e.Cause
}
```

### 2. Error Categories
Predefined error codes for common scenarios:

```go
const (
    ErrCodeConfigInvalid           = "CONFIG_INVALID"
    ErrCodeDependencyUnhealthy     = "DEPENDENCY_UNHEALTHY"
    ErrCodeAuthTokenInvalid        = "AUTH_TOKEN_INVALID"
    ErrCodeAuthInsufficientPerms   = "AUTH_INSUFFICIENT_PERMISSIONS"
    ErrCodeDBConnectionFailed      = "DB_CONNECTION_FAILED"
    ErrCodeCacheConnectionFailed   = "CACHE_CONNECTION_FAILED"
    ErrCodeMessagePublishFailed    = "MESSAGE_PUBLISH_FAILED"
    ErrCodeJobExecutionFailed      = "JOB_EXECUTION_FAILED"
    ErrCodeValidationFailed        = "VALIDATION_FAILED"
    ErrCodeResourceNotFound        = "RESOURCE_NOT_FOUND"
    ErrCodeResourceConflict        = "RESOURCE_CONFLICT"
)
```

### 3. Error Construction Helpers
Helper functions for creating common error types:

```go
func NewConfigError(message string, cause error) *ServiceError {
    return &ServiceError{
        Code:    ErrCodeConfigInvalid,
        Message: message,
        Cause:   cause,
    }
}

func NewValidationError(message string, context map[string]interface{}) *ServiceError {
    return &ServiceError{
        Code:    ErrCodeValidationFailed,
        Message: message,
        Context: context,
    }
}
```

### 4. Error Wrapping
Use Go 1.13+ error wrapping with `fmt.Errorf`:

```go
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}
```

### 5. HTTP Error Mapping
Automatic mapping of error codes to HTTP status codes:

```go
var errorCodeToHTTPStatus = map[string]int{
    ErrCodeValidationFailed:        http.StatusBadRequest,
    ErrCodeAuthTokenInvalid:        http.StatusUnauthorized,
    ErrCodeAuthInsufficientPerms:   http.StatusForbidden,
    ErrCodeResourceNotFound:        http.StatusNotFound,
    ErrCodeResourceConflict:        http.StatusConflict,
    ErrCodeConfigInvalid:           http.StatusInternalServerError,
    ErrCodeDependencyUnhealthy:     http.StatusServiceUnavailable,
}
```

### 6. Observability Integration
Automatic error logging and metrics:

```go
func (h *Handler) handleError(ctx context.Context, err error) {
    if serviceErr, ok := err.(*ServiceError); ok {
        h.logger.Error("Service error occurred", err,
            observability.Field("error_code", serviceErr.Code),
            observability.Field("context", serviceErr.Context),
        )
        h.metrics.Counter("errors_total").WithLabels(map[string]string{
            "error_code": serviceErr.Code,
        }).Inc()
    }
}
```

## Consequences

### Positive
- Consistent error handling across all services
- Structured error information for debugging
- Automatic HTTP status code mapping
- Integration with observability systems
- Clear error categorization for monitoring
- Context preservation through error wrapping
- Easy error testing and assertion

### Negative
- Additional abstraction layer over Go's native errors
- Potential performance overhead from error context
- Learning curve for developers unfamiliar with the pattern
- Risk of over-categorizing errors

## Alternatives Considered

### 1. Native Go Errors Only
Use only Go's built-in error type.
- **Rejected**: Lacks structure and consistency across services

### 2. Third-Party Error Library
Use a library like `pkg/errors` or `emperror`.
- **Rejected**: External dependency for core functionality, less control

### 3. Exception-Style Error Handling
Implement panic/recover-based error handling.
- **Rejected**: Goes against Go idioms and makes error handling implicit

### 4. Result Type Pattern
Use a Result<T, E> type similar to Rust.
- **Rejected**: Not idiomatic in Go, would require significant API changes

### 5. Error Codes as Constants Only
Use simple string constants without structured error types.
- **Rejected**: Lacks context and integration capabilities