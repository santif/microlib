package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	
	slogLogger, ok := logger.(*slogLogger)
	if !ok {
		t.Fatal("Logger is not a slogLogger")
	}
	
	if slogLogger.config.Level != LogLevelInfo {
		t.Errorf("Expected default log level %s, got %s", LogLevelInfo, slogLogger.config.Level)
	}
	
	if slogLogger.config.Format != LogFormatJSON {
		t.Errorf("Expected default log format %s, got %s", LogFormatJSON, slogLogger.config.Format)
	}
}

func TestNewLoggerWithConfig(t *testing.T) {
	config := LoggerConfig{
		Level:          LogLevelDebug,
		Format:         LogFormatText,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		ServiceInstance: "test-1",
	}
	
	logger := NewLoggerWithConfig(config)
	if logger == nil {
		t.Fatal("NewLoggerWithConfig returned nil")
	}
	
	slogLogger, ok := logger.(*slogLogger)
	if !ok {
		t.Fatal("Logger is not a slogLogger")
	}
	
	if slogLogger.config.Level != LogLevelDebug {
		t.Errorf("Expected log level %s, got %s", LogLevelDebug, slogLogger.config.Level)
	}
	
	if slogLogger.config.Format != LogFormatText {
		t.Errorf("Expected log format %s, got %s", LogFormatText, slogLogger.config.Format)
	}
	
	if slogLogger.config.ServiceName != "test-service" {
		t.Errorf("Expected service name %s, got %s", "test-service", slogLogger.config.ServiceName)
	}
}

func TestLoggerWithWriter(t *testing.T) {
	var buf bytes.Buffer
	
	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
	}
	
	logger := NewLoggerWithWriter(&buf, config)
	if logger == nil {
		t.Fatal("NewLoggerWithWriter returned nil")
	}
	
	// Log a message
	logger.Info("test message", NewField("key", "value"))
	
	// Check that the message was written to the buffer
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log message to contain 'test message', got %s", output)
	}
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}
	
	// Check that the message and level are correct
	if msg, ok := logEntry["msg"].(string); !ok || msg != "test message" {
		t.Errorf("Expected message 'test message', got %v", logEntry["msg"])
	}
	
	if level, ok := logEntry["level"].(string); !ok || level != "INFO" {
		t.Errorf("Expected level 'INFO', got %v", logEntry["level"])
	}
	
	// Check that the field was included
	if value, ok := logEntry["key"].(string); !ok || value != "value" {
		t.Errorf("Expected field 'key' with value 'value', got %v", logEntry["key"])
	}
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	
	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
	}
	
	logger := NewLoggerWithWriter(&buf, config)
	
	// Test info level
	buf.Reset()
	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("Expected info message to be logged")
	}
	
	// Test error level
	buf.Reset()
	logger.Error("error message", errors.New("test error"))
	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected error message to be logged")
	}
	if !strings.Contains(output, "test error") {
		t.Errorf("Expected error to be included in log")
	}
	
	// Test debug level (should not be logged with INFO level)
	buf.Reset()
	logger.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Errorf("Debug message should not be logged at INFO level")
	}
	
	// Test warn level
	buf.Reset()
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Errorf("Expected warn message to be logged")
	}
	
	// Create a new logger with DEBUG level
	debugConfig := LoggerConfig{
		Level:  LogLevelDebug,
		Format: LogFormatJSON,
	}
	
	debugLogger := NewLoggerWithWriter(&buf, debugConfig)
	
	// Test debug level with DEBUG logger
	buf.Reset()
	debugLogger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("Expected debug message to be logged with DEBUG level")
	}
}

func TestLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer
	
	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
	}
	
	logger := NewLoggerWithWriter(&buf, config)
	
	// Create a context with a trace ID
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-id")
	
	// Get a logger with context
	ctxLogger := logger.WithContext(ctx)
	
	// Log a message
	buf.Reset()
	ctxLogger.Info("context message")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}
	
	// Check that the trace ID was included
	if traceID, ok := logEntry["trace_id"].(string); !ok || traceID != "test-trace-id" {
		t.Errorf("Expected trace_id 'test-trace-id', got %v", logEntry["trace_id"])
	}
	
	// Test with nil context
	nilCtxLogger := logger.WithContext(nil)
	if nilCtxLogger == nil {
		t.Fatal("WithContext(nil) returned nil")
	}
}

func TestFieldsToAttrs(t *testing.T) {
	fields := []Field{
		NewField("string", "value"),
		NewField("int", 123),
		NewField("bool", true),
		NewField("float", 3.14),
	}
	
	attrs := fieldsToAttrs(fields)
	
	if len(attrs) != len(fields) {
		t.Errorf("Expected %d attrs, got %d", len(fields), len(attrs))
	}
	
	// Check that the keys match
	for i, field := range fields {
		if attrs[i].Key != field.Key {
			t.Errorf("Expected key %s, got %s", field.Key, attrs[i].Key)
		}
	}
}

func TestGetLogLevel(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{"unknown", "INFO"}, // Default to INFO for unknown levels
	}
	
	for _, tc := range testCases {
		t.Run(string(tc.level), func(t *testing.T) {
			level := getLogLevel(tc.level)
			if level.String() != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, level.String())
			}
		})
	}
}

func TestGlobalLogger(t *testing.T) {
	// Save the original global logger
	originalLogger := GlobalLogger
	defer func() {
		// Restore the original global logger
		GlobalLogger = originalLogger
	}()
	
	var buf bytes.Buffer
	
	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
	}
	
	logger := NewLoggerWithWriter(&buf, config)
	
	// Set the global logger
	SetGlobalLogger(logger)
	
	// Test global logger functions
	buf.Reset()
	Info("global info")
	if !strings.Contains(buf.String(), "global info") {
		t.Errorf("Expected global info message to be logged")
	}
	
	buf.Reset()
	Error("global error", errors.New("global test error"))
	if !strings.Contains(buf.String(), "global error") {
		t.Errorf("Expected global error message to be logged")
	}
	
	buf.Reset()
	Debug("global debug")
	if strings.Contains(buf.String(), "global debug") {
		t.Errorf("Global debug message should not be logged at INFO level")
	}
	
	buf.Reset()
	Warn("global warn")
	if !strings.Contains(buf.String(), "global warn") {
		t.Errorf("Expected global warn message to be logged")
	}
	
	// Test WithContext
	ctx := context.WithValue(context.Background(), "trace_id", "global-trace-id")
	ctxLogger := WithContext(ctx)
	
	buf.Reset()
	ctxLogger.Info("global context message")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}
	
	// Check that the trace ID was included
	if traceID, ok := logEntry["trace_id"].(string); !ok || traceID != "global-trace-id" {
		t.Errorf("Expected trace_id 'global-trace-id', got %v", logEntry["trace_id"])
	}
}