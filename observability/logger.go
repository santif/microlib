package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
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
	
	// Error logs an error message
	Error(msg string, err error, fields ...Field)
	
	// Debug logs a debug message
	Debug(msg string, fields ...Field)
	
	// Warn logs a warning message
	Warn(msg string, fields ...Field)
	
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
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.LogAttrs(context.Background(), slog.LevelInfo, msg, fieldsToAttrs(fields)...)
}

// Error logs an error message
func (l *slogLogger) Error(msg string, err error, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// Add the error to the fields if it's not nil
	attrs := fieldsToAttrs(fields)
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	
	l.logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

// Debug logs a debug message
func (l *slogLogger) Debug(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.LogAttrs(context.Background(), slog.LevelDebug, msg, fieldsToAttrs(fields)...)
}

// Warn logs a warning message
func (l *slogLogger) Warn(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.LogAttrs(context.Background(), slog.LevelWarn, msg, fieldsToAttrs(fields)...)
}

// WithContext returns a new logger with context
func (l *slogLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}
	
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// Extract trace ID from context if available
	traceID := extractTraceID(ctx)
	
	// Create a new logger with the trace ID
	newLogger := l.logger
	if traceID != "" {
		newLogger = newLogger.With(slog.String("trace_id", traceID))
	}
	
	// Extract any other context values that should be included in logs
	newLogger = enrichLoggerFromContext(ctx, newLogger)
	
	return &slogLogger{
		logger: newLogger,
		config: l.config,
	}
}

// extractTraceID extracts the trace ID from the context
// This is a placeholder implementation that will be enhanced in the tracing task
func extractTraceID(ctx context.Context) string {
	// This will be implemented in the tracing task
	// For now, we'll check for a trace ID in the context
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return traceID
	}
	return ""
}

// enrichLoggerFromContext adds additional context values to the logger
// This is a placeholder implementation that will be enhanced in the tracing task
func enrichLoggerFromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	// This will be implemented in the tracing task
	// For now, we'll just return the logger as is
	return logger
}

// NewField creates a new log field
func NewField(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}