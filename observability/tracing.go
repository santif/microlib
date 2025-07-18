package observability

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// TextMapCarrier is an interface for propagating trace context via string key-value pairs.
// It's a copy of the propagation.TextMapCarrier interface from OpenTelemetry to avoid
// direct dependencies on the OpenTelemetry package in other parts of the codebase.
type TextMapCarrier interface {
	// Get returns the value associated with the passed key.
	Get(key string) string
	// Set stores the key-value pair.
	Set(key, value string)
	// Keys lists the keys stored in this carrier.
	Keys() []string
}

// Tracer is the interface for distributed tracing
type Tracer interface {
	// Start creates a new span and context
	Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// StartSpan creates a new span without modifying the context
	StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) trace.Span

	// Extract extracts trace context from carrier
	Extract(ctx context.Context, carrier TextMapCarrier) context.Context

	// Inject injects trace context into carrier
	Inject(ctx context.Context, carrier TextMapCarrier)

	// TracerProvider returns the underlying TracerProvider
	TracerProvider() trace.TracerProvider

	// Shutdown shuts down the tracer provider
	Shutdown(ctx context.Context) error
}

// TracingConfig contains configuration for tracing
type TracingConfig struct {
	// Enabled determines if tracing is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ServiceName is the name of the service
	ServiceName string `json:"service_name" yaml:"service_name" validate:"required_if=Enabled true"`

	// ServiceVersion is the version of the service
	ServiceVersion string `json:"service_version" yaml:"service_version"`

	// ServiceInstance is the instance ID of the service
	ServiceInstance string `json:"service_instance" yaml:"service_instance"`

	// ExporterType is the type of exporter to use (otlp, jaeger, zipkin)
	ExporterType string `json:"exporter_type" yaml:"exporter_type" validate:"required_if=Enabled true,oneof=otlp jaeger zipkin"`

	// Endpoint is the endpoint for the exporter
	Endpoint string `json:"endpoint" yaml:"endpoint" validate:"required_if=Enabled true"`

	// Protocol is the protocol to use for OTLP (grpc or http/protobuf)
	Protocol string `json:"protocol" yaml:"protocol" validate:"omitempty,oneof=grpc http"`

	// Headers are the headers to include in OTLP requests
	Headers map[string]string `json:"headers" yaml:"headers"`

	// SamplingRate is the sampling rate (0.0 to 1.0)
	SamplingRate float64 `json:"sampling_rate" yaml:"sampling_rate" validate:"min=0,max=1"`

	// PropagationFormat is the context propagation format (w3c, b3, jaeger)
	PropagationFormat string `json:"propagation_format" yaml:"propagation_format" validate:"omitempty,oneof=w3c b3 jaeger"`

	// BatchTimeout is the maximum time to wait for a batch export (in milliseconds)
	BatchTimeout int `json:"batch_timeout" yaml:"batch_timeout" validate:"min=0"`

	// MaxExportBatchSize is the maximum number of spans to export in a batch
	MaxExportBatchSize int `json:"max_export_batch_size" yaml:"max_export_batch_size" validate:"min=0"`

	// MaxQueueSize is the maximum queue size for spans waiting to be exported
	MaxQueueSize int `json:"max_queue_size" yaml:"max_queue_size" validate:"min=0"`
}

// DefaultTracingConfig returns the default tracing configuration
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:            false,
		ExporterType:       "otlp",
		Protocol:           "grpc",
		Endpoint:           "localhost:4317",
		SamplingRate:       1.0,
		PropagationFormat:  "w3c",
		BatchTimeout:       5000,
		MaxExportBatchSize: 512,
		MaxQueueSize:       2048,
	}
}

// otelTracer implements the Tracer interface using OpenTelemetry
type otelTracer struct {
	tracer         trace.Tracer
	tracerProvider trace.TracerProvider
	propagator     propagation.TextMapPropagator
	config         TracingConfig
	shutdown       func(context.Context) error
}

// NewTracer creates a new tracer with the default configuration
func NewTracer() (Tracer, error) {
	return NewTracerWithConfig(DefaultTracingConfig())
}

// NewTracerWithConfig creates a new tracer with the provided configuration
func NewTracerWithConfig(config TracingConfig) (Tracer, error) {
	if !config.Enabled {
		return &noopTracer{}, nil
	}

	// Create a resource describing the service
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.ServiceInstanceID(config.ServiceInstance),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create the exporter based on the configuration
	var exporter sdktrace.SpanExporter
	switch config.ExporterType {
	case "otlp":
		exporter, err = createOTLPExporter(config)
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.ExporterType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create the trace provider with the exporter
	bsp := sdktrace.NewBatchSpanProcessor(exporter,
		sdktrace.WithMaxExportBatchSize(config.MaxExportBatchSize),
		sdktrace.WithMaxQueueSize(config.MaxQueueSize),
	)

	// Configure the sampling rate
	var sampler sdktrace.Sampler
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Create the propagator based on the configuration
	var propagator propagation.TextMapPropagator
	switch config.PropagationFormat {
	case "b3":
		// B3 propagation is not included in the core OpenTelemetry package
		// You would need to import "go.opentelemetry.io/contrib/propagators/b3"
		// For now, we'll default to W3C
		propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	case "jaeger":
		// Jaeger propagation is not included in the core OpenTelemetry package
		// You would need to import "go.opentelemetry.io/contrib/propagators/jaeger"
		// For now, we'll default to W3C
		propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	default: // w3c
		propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}

	// Set the global propagator
	otel.SetTextMapPropagator(propagator)

	// Create the tracer
	tracer := tracerProvider.Tracer(
		config.ServiceName,
		trace.WithInstrumentationVersion(config.ServiceVersion),
	)

	return &otelTracer{
		tracer:         tracer,
		tracerProvider: tracerProvider,
		propagator:     propagator,
		config:         config,
		shutdown: func(ctx context.Context) error {
			return tracerProvider.Shutdown(ctx)
		},
	}, nil
}

// createOTLPExporter creates an OTLP exporter based on the configuration
func createOTLPExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	ctx := context.Background()

	switch config.Protocol {
	case "http":
		// Create HTTP exporter options
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(config.Endpoint),
		}

		// Add headers if provided
		if len(config.Headers) > 0 {
			headers := make(map[string]string)
			for k, v := range config.Headers {
				headers[k] = v
			}
			opts = append(opts, otlptracehttp.WithHeaders(headers))
		}

		client := otlptracehttp.NewClient(opts...)
		return otlptrace.New(ctx, client)

	default: // grpc
		// Create gRPC exporter options
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(config.Endpoint),
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		}

		// Add headers if provided
		if len(config.Headers) > 0 {
			headers := make(map[string]string)
			for k, v := range config.Headers {
				headers[k] = v
			}
			opts = append(opts, otlptracegrpc.WithHeaders(headers))
		}

		client := otlptracegrpc.NewClient(opts...)
		return otlptrace.New(ctx, client)
	}
}

// Start creates a new span and context
func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// StartSpan creates a new span without modifying the context
func (t *otelTracer) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) trace.Span {
	_, span := t.tracer.Start(ctx, spanName, opts...)
	return span
}

// Extract extracts trace context from carrier
func (t *otelTracer) Extract(ctx context.Context, carrier TextMapCarrier) context.Context {
	// Convert our TextMapCarrier to OpenTelemetry's propagation.TextMapCarrier
	return t.propagator.Extract(ctx, propagationCarrier{carrier})
}

// Inject injects trace context into carrier
func (t *otelTracer) Inject(ctx context.Context, carrier TextMapCarrier) {
	// Convert our TextMapCarrier to OpenTelemetry's propagation.TextMapCarrier
	t.propagator.Inject(ctx, propagationCarrier{carrier})
}

// TracerProvider returns the underlying TracerProvider
func (t *otelTracer) TracerProvider() trace.TracerProvider {
	return t.tracerProvider
}

// Shutdown shuts down the tracer provider
func (t *otelTracer) Shutdown(ctx context.Context) error {
	return t.shutdown(ctx)
}

// noopTracer is a no-op implementation of the Tracer interface
type noopTracer struct{}

// Start creates a new span and context
func (t *noopTracer) Start(ctx context.Context, _ string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

// StartSpan creates a new span without modifying the context
func (t *noopTracer) StartSpan(ctx context.Context, _ string, _ ...trace.SpanStartOption) trace.Span {
	return trace.SpanFromContext(ctx)
}

// Extract extracts trace context from carrier
func (t *noopTracer) Extract(ctx context.Context, _ TextMapCarrier) context.Context {
	return ctx
}

// Inject injects trace context into carrier
func (t *noopTracer) Inject(_ context.Context, _ TextMapCarrier) {}

// TracerProvider returns the underlying TracerProvider
func (t *noopTracer) TracerProvider() trace.TracerProvider {
	return trace.NewNoopTracerProvider()
}

// Shutdown shuts down the tracer provider
func (t *noopTracer) Shutdown(_ context.Context) error {
	return nil
}

// GlobalTracer is a package-level tracer for convenience
var GlobalTracer Tracer

// Initialize the global tracer with default configuration
func init() {
	var err error
	GlobalTracer, err = NewTracer()
	if err != nil {
		// If there's an error, use a no-op tracer
		GlobalTracer = &noopTracer{}
	}
}

// SetGlobalTracer sets the global tracer
func SetGlobalTracer(tracer Tracer) {
	GlobalTracer = tracer
}

// Start creates a new span and context using the global tracer
func Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GlobalTracer.Start(ctx, spanName, opts...)
}

// StartSpan creates a new span without modifying the context using the global tracer
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) trace.Span {
	return GlobalTracer.StartSpan(ctx, spanName, opts...)
}

// propagationCarrier adapts our TextMapCarrier to OpenTelemetry's propagation.TextMapCarrier
type propagationCarrier struct {
	carrier TextMapCarrier
}

// Get returns the value associated with the passed key.
func (pc propagationCarrier) Get(key string) string {
	return pc.carrier.Get(key)
}

// Set stores the key-value pair.
func (pc propagationCarrier) Set(key, value string) {
	pc.carrier.Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (pc propagationCarrier) Keys() []string {
	return pc.carrier.Keys()
}

// Extract extracts trace context from carrier using the global tracer
func Extract(ctx context.Context, carrier TextMapCarrier) context.Context {
	return GlobalTracer.Extract(ctx, carrier)
}

// Inject injects trace context into carrier using the global tracer
func Inject(ctx context.Context, carrier TextMapCarrier) {
	GlobalTracer.Inject(ctx, carrier)
}

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the given span
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// TracerManager manages tracers and provides dynamic configuration updates
type TracerManager struct {
	config configInterface
	tracer Tracer
	mu     sync.RWMutex
}

// NewTracerManager creates a new tracer manager
func NewTracerManager(cfg configInterface) (*TracerManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Get the observability configuration
	obsConfig, ok := cfg.Get().(*ObservabilityConfig)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	// Create the tracer with the configuration
	tracer, err := NewTracerWithConfig(obsConfig.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer: %w", err)
	}

	return &TracerManager{
		config: cfg,
		tracer: tracer,
	}, nil
}

// GetTracer returns the tracer instance
func (m *TracerManager) GetTracer() Tracer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tracer
}

// UpdateTracingConfig updates the tracing configuration
func (m *TracerManager) UpdateTracingConfig(tracingConfig TracingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the current configuration
	obsConfig, ok := m.config.Get().(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	// Update the tracing configuration
	obsConfig.Tracing = tracingConfig

	// Update the configuration
	if err := m.config.Update(obsConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	return nil
}

// Reload implements the config.Reloadable interface
func (m *TracerManager) Reload(newConfig interface{}) error {
	obsConfig, ok := newConfig.(*ObservabilityConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type, expected ObservabilityConfig")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Shutdown the current tracer
	if m.tracer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		_ = m.tracer.Shutdown(ctx)
	}

	// Create a new tracer with the updated configuration
	tracer, err := NewTracerWithConfig(obsConfig.Tracing)
	if err != nil {
		return fmt.Errorf("failed to create tracer: %w", err)
	}

	// Update the tracer
	m.tracer = tracer

	// Update the global tracer
	SetGlobalTracer(tracer)

	return nil
}

// defaultShutdownTimeout is the default timeout for shutting down the tracer
const defaultShutdownTimeout = 5 * 1000 // 5 seconds
