package observability

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/santif/microlib/config"
)

// ObservabilityConfig contains configuration for all observability components
type ObservabilityConfig struct {
	// Logging contains configuration for the logger
	Logging LoggerConfig `json:"logging" yaml:"logging" validate:"required"`
	
	// Metrics contains configuration for metrics (will be implemented in a future task)
	// Metrics MetricsConfig `json:"metrics" yaml:"metrics"`
	
	// Tracing contains configuration for tracing (will be implemented in a future task)
	// Tracing TracingConfig `json:"tracing" yaml:"tracing"`
}

// DefaultObservabilityConfig returns the default observability configuration
func DefaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		Logging: DefaultLoggerConfig(),
	}
}

// LoggerManager manages loggers and provides dynamic configuration updates
type LoggerManager struct {
	config      *config.Config
	defaultLogger Logger
	loggers     map[string]Logger
	mu          sync.RWMutex
}

// NewLoggerManager creates a new logger manager
func NewLoggerManager(cfg *config.Config) (*LoggerManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	
	// Get the observability configuration
	obsConfig, ok := cfg.Get().(*ObservabilityConfig)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}
	
	// Create the default logger
	defaultLogger := NewLoggerWithConfig(obsConfig.Logging)
	
	return &LoggerManager{
		config:      cfg,
		defaultLogger: defaultLogger,
		loggers:     make(map[string]Logger),
	}, nil
}

// GetLogger returns a logger with the given name
func (m *LoggerManager) GetLogger(name string) Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if logger, ok := m.loggers[name]; ok {
		return logger
	}
	
	// If no logger exists with this name, return the default logger
	return m.defaultLogger
}

// RegisterLogger registers a logger with the given name
func (m *LoggerManager) RegisterLogger(name string, logger Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loggers[name] = logger
}

// UpdateLogLevel updates the log level for all loggers
func (m *LoggerManager) UpdateLogLevel(level LogLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Get the current configuration
	obsConfig, ok := m.config.Get().(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}
	
	// Update the log level
	obsConfig.Logging.Level = level
	
	// Update the configuration
	if err := m.config.Update(obsConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}
	
	return nil
}

// Reload implements the config.Reloadable interface
func (m *LoggerManager) Reload(newConfig interface{}) error {
	obsConfig, ok := newConfig.(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update the default logger
	m.defaultLogger = NewLoggerWithConfig(obsConfig.Logging)
	
	// Reset all registered loggers
	m.loggers = make(map[string]Logger)
	
	return nil
}

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

// Error logs an error message using the global logger
func Error(msg string, err error, fields ...Field) {
	GlobalLogger.Error(msg, err, fields...)
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	GlobalLogger.Debug(msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	GlobalLogger.Warn(msg, fields...)
}

// WithContext returns a new logger with context using the global logger
func WithContext(ctx context.Context) Logger {
	return GlobalLogger.WithContext(ctx)
}