package http

import (
	"context"

	"github.com/santif/microlib/observability"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockOtelTracer is a mock implementation of the observability.Tracer interface
type mockOtelTracer struct {
	spans map[string]bool
}

func newMockOtelTracer() *mockOtelTracer {
	return &mockOtelTracer{
		spans: make(map[string]bool),
	}
}

func (t *mockOtelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	t.spans[spanName] = true
	tracer := noop.NewTracerProvider().Tracer("")
	return tracer.Start(ctx, spanName)
}

func (t *mockOtelTracer) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) trace.Span {
	t.spans[spanName] = true
	tracer := noop.NewTracerProvider().Tracer("")
	_, span := tracer.Start(ctx, spanName)
	return span
}

func (t *mockOtelTracer) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	return ctx
}

func (t *mockOtelTracer) Inject(ctx context.Context, carrier observability.TextMapCarrier) {}

func (t *mockOtelTracer) TracerProvider() trace.TracerProvider {
	return noop.NewTracerProvider()
}

func (t *mockOtelTracer) Shutdown(ctx context.Context) error {
	return nil
}

// mockTextMapCarrier is a mock implementation of the observability.TextMapCarrier interface
type mockTextMapCarrier struct {
	values map[string]string
}

func newMockTextMapCarrier() *mockTextMapCarrier {
	return &mockTextMapCarrier{
		values: make(map[string]string),
	}
}

func (c *mockTextMapCarrier) Get(key string) string {
	return c.values[key]
}

func (c *mockTextMapCarrier) Set(key, value string) {
	c.values[key] = value
}

func (c *mockTextMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	return keys
}
