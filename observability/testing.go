package observability

import (
	"io"
)

// NewNoOpLogger creates a logger that discards all output, useful for testing
func NewNoOpLogger() Logger {
	config := DefaultLoggerConfig()
	config.Level = LogLevelError // Set to error level to minimize output
	return NewLoggerWithWriter(io.Discard, config)
}

// NewTestLogger creates a logger for testing that writes to the provided writer
func NewTestLogger(w io.Writer) Logger {
	config := DefaultLoggerConfig()
	config.Format = LogFormatJSON
	return NewLoggerWithWriter(w, config)
}
