package observability

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// NoOpLogger returns a no-op implementation of the Logger interface
func NoOpLogger() Logger {
	return &noopLogger{}
}

// noopLogger is a no-op implementation of the Logger interface
type noopLogger struct{}

func (l *noopLogger) Info(msg string, fields ...Field)                                         {}
func (l *noopLogger) InfoContext(ctx context.Context, msg string, fields ...Field)             {}
func (l *noopLogger) Error(msg string, err error, fields ...Field)                             {}
func (l *noopLogger) ErrorContext(ctx context.Context, msg string, err error, fields ...Field) {}
func (l *noopLogger) Debug(msg string, fields ...Field)                                        {}
func (l *noopLogger) DebugContext(ctx context.Context, msg string, fields ...Field)            {}
func (l *noopLogger) Warn(msg string, fields ...Field)                                         {}
func (l *noopLogger) WarnContext(ctx context.Context, msg string, fields ...Field)             {}
func (l *noopLogger) WithContext(ctx context.Context) Logger                                   { return l }

// NoOpMetrics returns a no-op implementation of the Metrics interface
func NoOpMetrics() Metrics {
	return &noopMetrics{}
}

// noopMetrics is a no-op implementation of the Metrics interface
type noopMetrics struct{}

func (m *noopMetrics) Counter(name string, help string, labels ...string) Counter {
	return &noopCounter{}
}

func (m *noopMetrics) Histogram(name string, help string, buckets []float64, labels ...string) Histogram {
	return &noopHistogram{}
}

func (m *noopMetrics) Gauge(name string, help string, labels ...string) Gauge {
	return &noopGauge{}
}

func (m *noopMetrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
}

func (m *noopMetrics) Registry() *prometheus.Registry {
	return prometheus.NewRegistry()
}
