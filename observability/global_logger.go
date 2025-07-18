package observability

import (
	"context"
)

// GlobalLogger is a package-level logger for convenience
var GlobalLogger Logger

// Initialize the global logger with default configuration
func init() {
	GlobalLogger = NewLogger()
}

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger Logger) {
	GlobalLogger = logger
}

// Info logs an informational message using the global logger
func Info(msg string, fields ...Field) {
	GlobalLogger.Info(msg, fields...)
}

// InfoContext logs an informational message with context using the global logger
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	GlobalLogger.InfoContext(ctx, msg, fields...)
}

// Error logs an error message using the global logger
func Error(msg string, err error, fields ...Field) {
	GlobalLogger.Error(msg, err, fields...)
}

// ErrorContext logs an error message with context using the global logger
func ErrorContext(ctx context.Context, msg string, err error, fields ...Field) {
	GlobalLogger.ErrorContext(ctx, msg, err, fields...)
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	GlobalLogger.Debug(msg, fields...)
}

// DebugContext logs a debug message with context using the global logger
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	GlobalLogger.DebugContext(ctx, msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	GlobalLogger.Warn(msg, fields...)
}

// WarnContext logs a warning message with context using the global logger
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	GlobalLogger.WarnContext(ctx, msg, fields...)
}

// WithContext returns a new logger with context using the global logger
func WithContext(ctx context.Context) Logger {
	return GlobalLogger.WithContext(ctx)
}
