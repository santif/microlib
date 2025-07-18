package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// Logger is the interface for structured logging
type Logger interface {
	// Info logs an informational message
	Info(msg string, fields ...Field)

	// InfoContext logs an informational message with context
	InfoContext(ctx context.Context, msg string, fields ...Field)

	// Error logs an error message
	Error(msg string, err error, fields ...Field)

	// ErrorContext logs an error message with context
	ErrorContext(ctx context.Context, msg string, err error, fields ...Field)

	// Debug logs a debug message
	Debug(msg string, fields ...Field)

	// DebugContext logs a debug message with context
	DebugContext(ctx context.Context, msg string, fields ...Field)

	// Warn logs a warning message
	Warn(msg string, fields ...Field)

	// WarnContext logs a warning message with context
	WarnContext(ctx context.Context, msg string, fields ...Field)

	// WithContext returns a new logger with context
	WithContext(ctx context.Context) Logger
}

// LogLevel represents the logging level
type LogLevel string

const (
	// LogLevelDebug is the debug log level
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo is the info log level
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn is the warning log level
	LogLevelWarn LogLevel = "warn"

	// LogLevelError is the error log level
	LogLevelError LogLevel = "error"
)

// LogFormat represents the logging format
type LogFormat string

const (
	// LogFormatJSON is the JSON log format
	LogFormatJSON LogFormat = "json"

	// LogFormatText is the text log format
	LogFormatText LogFormat = "text"
)

// LoggerConfig contains configuration for the logger
type LoggerConfig struct {
	// Level is the minimum log level to output
	Level LogLevel `json:"level" yaml:"level" validate:"required,oneof=debug info warn error"`

	// Format is the log format (json or text)
	Format LogFormat `json:"format" yaml:"format" validate:"required,oneof=json text"`

	// ServiceName is the name of the service
	ServiceName string `json:"service_name" yaml:"service_name"`

	// ServiceVersion is the version of the service
	ServiceVersion string `json:"service_version" yaml:"service_version"`

	// ServiceInstance is the instance ID of the service
	ServiceInstance string `json:"service_instance" yaml:"service_instance"`
}

// DefaultLoggerConfig returns the default logger configuration
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
	}
}

// slogLogger is the implementation of the Logger interface using slog
type slogLogger struct {
	logger *slog.Logger
	config LoggerConfig
	mu     sync.RWMutex
}

// NewLogger creates a new logger with the default configuration
func NewLogger() Logger {
	return NewLoggerWithConfig(DefaultLoggerConfig())
}

// NewLoggerWithConfig creates a new logger with the provided configuration
func NewLoggerWithConfig(config LoggerConfig) Logger {
	var handler slog.Handler

	// Create the appropriate handler based on the format
	if config.Format == LogFormatJSON {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: getLogLevel(config.Level),
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: getLogLevel(config.Level),
		})
	}

	// Add service metadata to the handler if available
	if config.ServiceName != "" || config.ServiceVersion != "" || config.ServiceInstance != "" {
		handler = handler.WithAttrs([]slog.Attr{
			slog.String("service", config.ServiceName),
			slog.String("version", config.ServiceVersion),
			slog.String("instance", config.ServiceInstance),
		})
	}

	return &slogLogger{
		logger: slog.New(handler),
		config: config,
	}
}

// NewLoggerWithWriter creates a new logger with a custom writer
func NewLoggerWithWriter(w io.Writer, config LoggerConfig) Logger {
	var handler slog.Handler

	// Create the appropriate handler based on the format
	if config.Format == LogFormatJSON {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: getLogLevel(config.Level),
		})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: getLogLevel(config.Level),
		})
	}

	// Add service metadata to the handler if available
	if config.ServiceName != "" || config.ServiceVersion != "" || config.ServiceInstance != "" {
		handler = handler.WithAttrs([]slog.Attr{
			slog.String("service", config.ServiceName),
			slog.String("version", config.ServiceVersion),
			slog.String("instance", config.ServiceInstance),
		})
	}

	return &slogLogger{
		logger: slog.New(handler),
		config: config,
	}
}

// getLogLevel converts a LogLevel to a slog.Level
func getLogLevel(level LogLevel) slog.Level {
	switch level {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fieldsToAttrs converts a slice of Fields to a slice of slog.Attr
func fieldsToAttrs(fields []Field) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, field := range fields {
		attrs = append(attrs, slog.Any(field.Key, field.Value))
	}
	return attrs
}

// Info logs an informational message
func (l *slogLogger) Info(msg string, fields ...Field) {
	l.InfoContext(context.Background(), msg, fields...)
}

// InfoContext logs an informational message with context
func (l *slogLogger) InfoContext(ctx context.Context, msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Extract trace information from context
	logger := l.logger

	// Add trace ID if available
	if traceID := extractTraceID(ctx); traceID != "" {
		logger = logger.With(slog.String("trace_id", traceID))
	}

	// Add span ID if available
	if spanID := extractSpanID(ctx); spanID != "" {
		logger = logger.With(slog.String("span_id", spanID))
	}

	// Enrich with other context values
	logger = enrichLoggerFromContext(ctx, logger)

	// Log the message with the enriched logger
	logger.LogAttrs(ctx, slog.LevelInfo, msg, fieldsToAttrs(fields)...)
}

// Error logs an error message
func (l *slogLogger) Error(msg string, err error, fields ...Field) {
	l.ErrorContext(context.Background(), msg, err, fields...)
}

// ErrorContext logs an error message with context
func (l *slogLogger) ErrorContext(ctx context.Context, msg string, err error, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Extract trace information from context
	logger := l.logger

	// Add trace ID if available
	if traceID := extractTraceID(ctx); traceID != "" {
		logger = logger.With(slog.String("trace_id", traceID))
	}

	// Add span ID if available
	if spanID := extractSpanID(ctx); spanID != "" {
		logger = logger.With(slog.String("span_id", spanID))
	}

	// Enrich with other context values
	logger = enrichLoggerFromContext(ctx, logger)

	// Add the error to the fields if it's not nil
	attrs := fieldsToAttrs(fields)
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}

	// Log the message with the enriched logger
	logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

// Debug logs a debug message
func (l *slogLogger) Debug(msg string, fields ...Field) {
	l.DebugContext(context.Background(), msg, fields...)
}

// DebugContext logs a debug message with context
func (l *slogLogger) DebugContext(ctx context.Context, msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Extract trace information from context
	logger := l.logger

	// Add trace ID if available
	if traceID := extractTraceID(ctx); traceID != "" {
		logger = logger.With(slog.String("trace_id", traceID))
	}

	// Add span ID if available
	if spanID := extractSpanID(ctx); spanID != "" {
		logger = logger.With(slog.String("span_id", spanID))
	}

	// Enrich with other context values
	logger = enrichLoggerFromContext(ctx, logger)

	// Log the message with the enriched logger
	logger.LogAttrs(ctx, slog.LevelDebug, msg, fieldsToAttrs(fields)...)
}

// Warn logs a warning message
func (l *slogLogger) Warn(msg string, fields ...Field) {
	l.WarnContext(context.Background(), msg, fields...)
}

// WarnContext logs a warning message with context
func (l *slogLogger) WarnContext(ctx context.Context, msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Extract trace information from context
	logger := l.logger

	// Add trace ID if available
	if traceID := extractTraceID(ctx); traceID != "" {
		logger = logger.With(slog.String("trace_id", traceID))
	}

	// Add span ID if available
	if spanID := extractSpanID(ctx); spanID != "" {
		logger = logger.With(slog.String("span_id", spanID))
	}

	// Enrich with other context values
	logger = enrichLoggerFromContext(ctx, logger)

	// Log the message with the enriched logger
	logger.LogAttrs(ctx, slog.LevelWarn, msg, fieldsToAttrs(fields)...)
}

// WithContext returns a new logger with context
func (l *slogLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Create a new logger with trace correlation information
	newLogger := l.logger

	// Extract trace ID from context if available
	traceID := extractTraceID(ctx)
	if traceID != "" {
		newLogger = newLogger.With(slog.String("trace_id", traceID))
	}

	// Extract span ID from context if available
	spanID := extractSpanID(ctx)
	if spanID != "" {
		newLogger = newLogger.With(slog.String("span_id", spanID))
	}

	// Enrich the logger with additional context values
	newLogger = enrichLoggerFromContext(ctx, newLogger)

	// Create a new logger instance with the enriched slog.Logger
	return &slogLogger{
		logger: newLogger,
		config: l.config,
	}
}

// Common context keys for trace information
const (
	// TraceIDKey is the context key for trace ID
	TraceIDKey = "trace_id"

	// SpanIDKey is the context key for span ID
	SpanIDKey = "span_id"

	// RequestIDKey is the context key for request ID
	RequestIDKey = "request_id"

	// UserIDKey is the context key for user ID
	UserIDKey = "user_id"

	// ServiceNameKey is the context key for service name
	ServiceNameKey = "service_name"

	// ServiceVersionKey is the context key for service version
	ServiceVersionKey = "service_version"

	// ServiceInstanceKey is the context key for service instance
	ServiceInstanceKey = "service_instance"

	// CorrelationIDKey is the context key for correlation ID
	CorrelationIDKey = "correlation_id"
)

// extractTraceID extracts the trace ID from the context
func extractTraceID(ctx context.Context) string {
	// First, try to get trace ID from OpenTelemetry span context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	// Check for trace ID in various formats as fallback

	// Check our standard key
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		return traceID
	}

	// Check for OpenTelemetry trace ID
	if traceID, ok := ctx.Value("otel-trace-id").(string); ok && traceID != "" {
		return traceID
	}

	// Check for OpenTelemetry standard context key
	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		return traceID
	}

	// Check for W3C trace parent format
	if traceparent, ok := ctx.Value("traceparent").(string); ok && traceparent != "" {
		// Extract trace ID from traceparent format
		// Format: 00-<trace-id>-<span-id>-<trace-flags>
		parts := strings.Split(traceparent, "-")
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Check for correlation ID as fallback
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		return correlationID
	}

	return ""
}

// extractSpanID extracts the span ID from the context
func extractSpanID(ctx context.Context) string {
	// First, try to get span ID from OpenTelemetry span context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}

	// Check for span ID in various formats as fallback

	// Check our standard key
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok && spanID != "" {
		return spanID
	}

	// Check for OpenTelemetry span ID
	if spanID, ok := ctx.Value("otel-span-id").(string); ok && spanID != "" {
		return spanID
	}

	// Check for OpenTelemetry standard context key
	if spanID, ok := ctx.Value("span_id").(string); ok && spanID != "" {
		return spanID
	}

	// Check for W3C trace parent format
	if traceparent, ok := ctx.Value("traceparent").(string); ok && traceparent != "" {
		// Extract span ID from traceparent format
		// Format: 00-<trace-id>-<span-id>-<trace-flags>
		parts := strings.Split(traceparent, "-")
		if len(parts) >= 3 {
			return parts[2]
		}
	}

	return ""
}

// enrichLoggerFromContext adds additional context values to the logger
func enrichLoggerFromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	// Extract common values from context that should be included in logs

	// Add service metadata if available in context
	if serviceName, ok := ctx.Value(ServiceNameKey).(string); ok && serviceName != "" {
		logger = logger.With(slog.String("service", serviceName))
	}

	if serviceVersion, ok := ctx.Value(ServiceVersionKey).(string); ok && serviceVersion != "" {
		logger = logger.With(slog.String("version", serviceVersion))
	}

	if serviceInstance, ok := ctx.Value(ServiceInstanceKey).(string); ok && serviceInstance != "" {
		logger = logger.With(slog.String("instance", serviceInstance))
	}

	// Add correlation ID if available
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		logger = logger.With(slog.String("correlation_id", correlationID))
	}

	// Add request ID if available
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		logger = logger.With(slog.String("request_id", requestID))
	}

	// Add user ID if available for audit logs
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		logger = logger.With(slog.String("user_id", userID))
	}

	// Extract any additional context values with the "log:" prefix
	// This allows arbitrary values to be included in logs when they're added to the context
	// with a key that starts with "log:"
	type contextKey string
	ctx.Value(struct{}{}) // This does nothing but allows us to iterate through context values

	// Note: In a real implementation, we can't iterate through context values directly
	// as the standard context doesn't expose this functionality.
	// This would require a custom context implementation or middleware that explicitly
	// adds values to the logger.

	return logger
}

// NewField creates a new log field
func NewField(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}
