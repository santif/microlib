# Design Document - gRPC Server Test Fix

## Overview

This design document outlines the approach to fix the test timeout issue in the gRPC server implementation of the MicroLib Go framework. The `TestStartAndShutdown` test is currently timing out after 10 minutes, indicating a potential deadlock or resource leak in the server's shutdown process.

Based on the stack trace and code analysis, we've identified that the issue is likely related to a deadlock during the server shutdown process, specifically in the interaction between the `Address()` method and the `Shutdown()` method, both of which acquire locks on the same mutex.

## Architecture

The current architecture of the gRPC server component includes:

1. A `Server` interface that defines the contract for the gRPC server
2. A `server` struct that implements this interface
3. A mutex (`startedMu`) that protects access to the server state
4. Methods for starting, stopping, and querying the server

The issue appears to be in the synchronization mechanism during shutdown, where a deadlock occurs due to improper lock acquisition.

## Components and Interfaces

### Current Implementation

The current implementation uses a read-write mutex (`sync.RWMutex`) to protect access to the server state. The `Shutdown()` method acquires a write lock on this mutex, while the `Address()` method acquires a read lock.

```go
// Address returns the server's address
func (s *server) Address() string {
    s.startedMu.RLock()
    defer s.startedMu.RUnlock()

    if !s.started || s.listener == nil {
        return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
    }

    return s.listener.Addr().String()
}

// Shutdown gracefully shuts down the server
func (s *server) Shutdown(ctx context.Context) error {
    s.startedMu.Lock()
    defer s.startedMu.Unlock()

    if !s.started {
        return ErrServerNotStarted
    }

    // Log server shutdown
    if s.deps.Logger != nil {
        s.deps.Logger.Info("Shutting down gRPC server",
            observability.NewField("address", s.Address()), // This calls Address() while holding the write lock
            observability.NewField("timeout", s.config.ShutdownTimeout.String()),
        )
    }
    
    // ... rest of shutdown logic
}
```

The issue is that the `Shutdown()` method calls `s.Address()` while holding the write lock, and `Address()` tries to acquire a read lock on the same mutex, resulting in a deadlock.

### Proposed Solution

The solution is to avoid calling methods that acquire locks while already holding a lock. Specifically:

1. Modify the `Shutdown()` method to get the server address before acquiring the lock
2. Alternatively, refactor the code to avoid calling `Address()` from within `Shutdown()`
3. Ensure proper lock ordering throughout the codebase to prevent similar issues

## Data Models

No changes to data models are required for this fix.

## Error Handling

The error handling in the server implementation is already robust. We will maintain the existing error handling approach:

- Return specific error types for common error conditions
- Wrap errors with context for better debugging
- Log errors with appropriate context

## Testing Strategy

### Unit Testing

- Modify the existing `TestStartAndShutdown` test to ensure it completes within a reasonable time
- Add additional tests to verify proper resource cleanup during shutdown
- Add tests specifically for concurrent access to server methods

### Integration Testing

- Test the server with real gRPC clients to ensure proper shutdown behavior
- Test shutdown behavior under load to ensure it remains reliable

### Performance Testing

- Benchmark the shutdown process to ensure it completes within the expected timeframe
- Test with various timeout configurations to ensure proper behavior

## Implementation Considerations

### Thread Safety

The key consideration for this fix is ensuring thread safety during server shutdown. We need to be careful about:

1. Lock ordering to prevent deadlocks
2. Avoiding calling methods that acquire locks while already holding a lock
3. Ensuring all resources are properly released during shutdown

### Backward Compatibility

The fix should maintain backward compatibility with the existing API. No changes to the public interface should be required.

### Performance Impact

The fix should have minimal performance impact. The shutdown process is not a performance-critical path, but we should ensure that it completes efficiently.

## Alternative Approaches Considered

1. **Use a separate mutex for address access**: This would avoid the deadlock but increase complexity.
2. **Restructure the server implementation**: A more extensive refactoring could make the code more robust but would be more invasive.
3. **Add a timeout to the lock acquisition**: This would prevent indefinite blocking but wouldn't fix the root cause.

We've chosen the approach of fixing the lock ordering issue as it directly addresses the root cause with minimal changes to the codebase.