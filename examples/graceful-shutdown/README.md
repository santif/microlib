# Graceful Shutdown Example

This example demonstrates the graceful shutdown mechanism in MicroLib, showing how to:

1. Handle SIGTERM and SIGINT signals
2. Register shutdown hooks for cleanup
3. Configure shutdown timeout
4. Properly clean up resources during shutdown

## Running the Example

### Normal Mode
```bash
go run main.go
```

The service will start and wait for a shutdown signal. You can test graceful shutdown by:

1. Pressing `Ctrl+C` (SIGINT)
2. Sending `kill <pid>` (SIGTERM) from another terminal

### Test Mode (Non-blocking)
```bash
go run main.go --test
```

In test mode, the service will automatically trigger a shutdown after 1 second, demonstrating the programmatic shutdown capability.

## Features Demonstrated

- **Signal Handling**: Automatic handling of SIGTERM and SIGINT signals
- **Shutdown Hooks**: Registration of cleanup functions that run during shutdown
- **Configurable Timeout**: 30-second timeout for graceful shutdown
- **Resource Cleanup**: Proper cleanup of database connections, HTTP servers, etc.
- **Logging**: Structured logging during shutdown process

## Expected Output

```
2024/01/15 10:30:00 INFO Service starting name=graceful-shutdown-demo version=1.0.0
2024/01/15 10:30:00 INFO Service started successfully
2024/01/15 10:30:00 INFO Waiting for shutdown signal...
^C2024/01/15 10:30:05 INFO Shutdown signal received signal=interrupt
2024/01/15 10:30:05 INFO Executing shutdown hook name=http-server
2024/01/15 10:30:05 INFO HTTP server stopped
2024/01/15 10:30:05 INFO Executing shutdown hook name=database
2024/01/15 10:30:05 INFO Database connections closed
2024/01/15 10:30:05 INFO Service shutdown completed
```