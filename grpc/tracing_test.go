package grpc

import (
	"context"
	"testing"

	"github.com/santif/microlib/observability"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockTracer is a mock implementation of the Tracer interface for testing
type mockTracer struct {
	extractCalled bool
	injectCalled  bool
	startCalled   bool
	ctx           context.Context
	span          trace.Span
}

func (t *mockTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	t.startCalled = true
	t.ctx = ctx
	// Use the global no-op tracer for testing
	return trace.NewNoopTracerProvider().Tracer("test").Start(ctx, spanName, opts...)
}

func (t *mockTracer) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) trace.Span {
	_, span := t.Start(ctx, spanName, opts...)
	return span
}

func (t *mockTracer) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	t.extractCalled = true
	return ctx
}

func (t *mockTracer) Inject(ctx context.Context, carrier observability.TextMapCarrier) {
	t.injectCalled = true
	// Add some test metadata
	carrier.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
}

func (t *mockTracer) TracerProvider() trace.TracerProvider {
	return trace.NewNoopTracerProvider()
}

func (t *mockTracer) Shutdown(ctx context.Context) error {
	return nil
}

func TestUnaryServerTracingInterceptor(t *testing.T) {
	// Create a mock tracer
	mockTracer := &mockTracer{}

	// Create the interceptor
	interceptor := UnaryServerTracingInterceptor(mockTracer)

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify that the span is in the context
		span := trace.SpanFromContext(ctx)
		if span == nil {
			t.Error("Expected span in context")
		}
		return "response", nil
	}

	// Create a context with metadata
	md := metadata.New(map[string]string{
		"test-key": "test-value",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Call the interceptor
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected response to be 'response', got %v", resp)
	}

	// Verify that the tracer methods were called
	if !mockTracer.extractCalled {
		t.Error("Expected Extract to be called")
	}
	if !mockTracer.startCalled {
		t.Error("Expected Start to be called")
	}
}

func TestUnaryClientTracingInterceptor(t *testing.T) {
	// Create a mock tracer
	mockTracer := &mockTracer{}

	// Create the interceptor
	interceptor := UnaryClientTracingInterceptor(mockTracer)

	// Create a mock invoker
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		// Verify that the metadata contains the trace context
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Error("Expected metadata in context")
		}
		if len(md) == 0 {
			t.Error("Expected non-empty metadata")
		}
		return nil
	}

	// Call the interceptor
	err := interceptor(context.Background(), "/service/method", "request", "reply", &grpc.ClientConn{}, invoker)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that the tracer methods were called
	if !mockTracer.startCalled {
		t.Error("Expected Start to be called")
	}
	if !mockTracer.injectCalled {
		t.Error("Expected Inject to be called")
	}
}

func TestStreamServerTracingInterceptor(t *testing.T) {
	// Create a mock tracer
	mockTracer := &mockTracer{}

	// Create the interceptor
	interceptor := StreamServerTracingInterceptor(mockTracer)

	// Create a mock handler
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		// Verify that the span is in the context
		ctx := stream.Context()
		span := trace.SpanFromContext(ctx)
		if span == nil {
			t.Error("Expected span in context")
		}
		return nil
	}

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: metadata.NewIncomingContext(context.Background(), metadata.New(nil)),
	}

	// Call the interceptor
	err := interceptor(nil, stream, &grpc.StreamServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that the tracer methods were called
	if !mockTracer.extractCalled {
		t.Error("Expected Extract to be called")
	}
	if !mockTracer.startCalled {
		t.Error("Expected Start to be called")
	}
}

func TestStreamClientTracingInterceptor(t *testing.T) {
	// Create a mock tracer
	mockTracer := &mockTracer{}

	// Create the interceptor
	interceptor := StreamClientTracingInterceptor(mockTracer)

	// Create a mock streamer
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Verify that the metadata contains the trace context
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Error("Expected metadata in context")
		}
		if len(md) == 0 {
			t.Error("Expected non-empty metadata")
		}
		return &mockClientStream{}, nil
	}

	// Call the interceptor
	stream, err := interceptor(context.Background(), &grpc.StreamDesc{}, &grpc.ClientConn{}, "/service/method", streamer)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if stream == nil {
		t.Error("Expected non-nil stream")
	}

	// Verify that the tracer methods were called
	if !mockTracer.startCalled {
		t.Error("Expected Start to be called")
	}
	if !mockTracer.injectCalled {
		t.Error("Expected Inject to be called")
	}
}

func TestCombinedUnaryServerInterceptor(t *testing.T) {
	// Create a mock tracer
	mockTracer := &mockTracer{}

	// Create a mock logger
	logger := observability.NewLogger()

	// Create a mock metrics
	metrics := observability.NewMetrics()

	// Create the interceptor
	interceptor := CombinedUnaryServerInterceptor(mockTracer, metrics, logger)

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify that the span is in the context
		span := trace.SpanFromContext(ctx)
		if span == nil {
			t.Error("Expected span in context")
		}
		return "response", nil
	}

	// Create a context with metadata
	md := metadata.New(map[string]string{
		"test-key": "test-value",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Call the interceptor
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected response to be 'response', got %v", resp)
	}

	// Verify that the tracer methods were called
	if !mockTracer.extractCalled {
		t.Error("Expected Extract to be called")
	}
	if !mockTracer.startCalled {
		t.Error("Expected Start to be called")
	}
}

// mockServerStream is a mock implementation of grpc.ServerStream
type mockServerStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (s *mockServerStream) Context() context.Context {
	return s.ctx
}

func (s *mockServerStream) SendMsg(m interface{}) error {
	return nil
}

func (s *mockServerStream) RecvMsg(m interface{}) error {
	return nil
}

// mockClientStream is a mock implementation of grpc.ClientStream
type mockClientStream struct {
	grpc.ClientStream
}

func (s *mockClientStream) SendMsg(m interface{}) error {
	return nil
}

func (s *mockClientStream) RecvMsg(m interface{}) error {
	return nil
}

func (s *mockClientStream) CloseSend() error {
	return nil
}
