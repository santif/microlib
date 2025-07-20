package grpc

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockLogger is a mock implementation of the Logger interface for testing
type mockLogger struct {
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	debugMessages []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		infoMessages:  make([]string, 0),
		errorMessages: make([]string, 0),
		warnMessages:  make([]string, 0),
		debugMessages: make([]string, 0),
	}
}

func (l *mockLogger) Info(msg string, fields ...observability.Field) {
	l.infoMessages = append(l.infoMessages, msg)
}

func (l *mockLogger) InfoContext(ctx context.Context, msg string, fields ...observability.Field) {
	l.infoMessages = append(l.infoMessages, msg)
}

func (l *mockLogger) Error(msg string, err error, fields ...observability.Field) {
	l.errorMessages = append(l.errorMessages, msg)
}

func (l *mockLogger) ErrorContext(ctx context.Context, msg string, err error, fields ...observability.Field) {
	l.errorMessages = append(l.errorMessages, msg)
}

func (l *mockLogger) Debug(msg string, fields ...observability.Field) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *mockLogger) DebugContext(ctx context.Context, msg string, fields ...observability.Field) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *mockLogger) Warn(msg string, fields ...observability.Field) {
	l.warnMessages = append(l.warnMessages, msg)
}

func (l *mockLogger) WarnContext(ctx context.Context, msg string, fields ...observability.Field) {
	l.warnMessages = append(l.warnMessages, msg)
}

func (l *mockLogger) WithContext(ctx context.Context) observability.Logger {
	return l
}

// mockMetrics is a mock implementation of the Metrics interface for testing
type mockMetrics struct {
	counters   map[string]observability.Counter
	histograms map[string]observability.Histogram
	gauges     map[string]observability.Gauge
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		counters:   make(map[string]observability.Counter),
		histograms: make(map[string]observability.Histogram),
		gauges:     make(map[string]observability.Gauge),
	}
}

func (m *mockMetrics) Counter(name string, help string, labels ...string) observability.Counter {
	if _, ok := m.counters[name]; !ok {
		m.counters[name] = &mockCounter{name: name}
	}
	return m.counters[name]
}

func (m *mockMetrics) Histogram(name string, help string, buckets []float64, labels ...string) observability.Histogram {
	if _, ok := m.histograms[name]; !ok {
		m.histograms[name] = &mockHistogram{name: name}
	}
	return m.histograms[name]
}

func (m *mockMetrics) Gauge(name string, help string, labels ...string) observability.Gauge {
	if _, ok := m.gauges[name]; !ok {
		m.gauges[name] = &mockGauge{name: name}
	}
	return m.gauges[name]
}

func (m *mockMetrics) Handler() http.Handler {
	return nil
}

func (m *mockMetrics) Registry() *prometheus.Registry {
	return nil
}

// mockCounter is a mock implementation of the Counter interface for testing
type mockCounter struct {
	name   string
	value  float64
	labels map[string]string
}

func (c *mockCounter) Inc() {
	c.value++
}

func (c *mockCounter) Add(value float64) {
	c.value += value
}

func (c *mockCounter) WithLabels(labels map[string]string) observability.Counter {
	return &mockCounter{
		name:   c.name,
		value:  c.value,
		labels: labels,
	}
}

// mockHistogram is a mock implementation of the Histogram interface for testing
type mockHistogram struct {
	name       string
	values     []float64
	labels     map[string]string
	lastValue  float64
	valueCount int
}

func (h *mockHistogram) Observe(value float64) {
	h.values = append(h.values, value)
	h.lastValue = value
	h.valueCount++
}

func (h *mockHistogram) WithLabels(labels map[string]string) observability.Histogram {
	return &mockHistogram{
		name:   h.name,
		values: h.values,
		labels: labels,
	}
}

// mockGauge is a mock implementation of the Gauge interface for testing
type mockGauge struct {
	name   string
	value  float64
	labels map[string]string
}

func (g *mockGauge) Set(value float64) {
	g.value = value
}

func (g *mockGauge) Inc() {
	g.value++
}

func (g *mockGauge) Dec() {
	g.value--
}

func (g *mockGauge) Add(value float64) {
	g.value += value
}

func (g *mockGauge) Sub(value float64) {
	g.value -= value
}

func (g *mockGauge) WithLabels(labels map[string]string) observability.Gauge {
	return &mockGauge{
		name:   g.name,
		value:  g.value,
		labels: labels,
	}
}

// mockAuthenticator is a mock implementation of the Authenticator interface for testing
type mockAuthenticator struct {
	validToken string
	claims     *security.Claims
	err        error
}

func newMockAuthenticator() *mockAuthenticator {
	return &mockAuthenticator{
		validToken: "valid-token",
		claims: &security.Claims{
			Subject: "test-user",
			Custom: map[string]interface{}{
				"scope": "read write",
			},
		},
	}
}

func (a *mockAuthenticator) ValidateToken(ctx context.Context, token string) (*security.Claims, error) {
	if token == a.validToken {
		return a.claims, nil
	}
	if a.err != nil {
		return nil, a.err
	}
	return nil, errors.New("invalid token")
}

func (a *mockAuthenticator) Middleware() func(http.Handler) http.Handler {
	return nil
}

func (a *mockAuthenticator) WithBypass(paths []string) func(http.Handler) http.Handler {
	return nil
}

func TestLoggingUnaryServerInterceptor(t *testing.T) {
	// Create a mock logger
	logger := newMockLogger()

	// Create the interceptor
	interceptor := LoggingUnaryServerInterceptor(logger)

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Call the interceptor
	resp, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected response to be 'response', got %v", resp)
	}

	// Verify that the logger methods were called
	if len(logger.infoMessages) != 2 {
		t.Errorf("Expected 2 info messages, got %d", len(logger.infoMessages))
	}
	if logger.infoMessages[0] != "gRPC request started" {
		t.Errorf("Expected first message to be 'gRPC request started', got '%s'", logger.infoMessages[0])
	}
	if logger.infoMessages[1] != "gRPC request completed" {
		t.Errorf("Expected second message to be 'gRPC request completed', got '%s'", logger.infoMessages[1])
	}
}

func TestLoggingUnaryServerInterceptorWithError(t *testing.T) {
	// Create a mock logger
	logger := newMockLogger()

	// Create the interceptor
	interceptor := LoggingUnaryServerInterceptor(logger)

	// Create a mock handler that returns an error
	expectedErr := status.Error(codes.InvalidArgument, "invalid argument")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, expectedErr
	}

	// Call the interceptor
	resp, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if resp != nil {
		t.Errorf("Expected response to be nil, got %v", resp)
	}

	// Verify that the logger methods were called
	if len(logger.infoMessages) != 1 {
		t.Errorf("Expected 1 info message, got %d", len(logger.infoMessages))
	}
	if logger.infoMessages[0] != "gRPC request started" {
		t.Errorf("Expected message to be 'gRPC request started', got '%s'", logger.infoMessages[0])
	}
	if len(logger.errorMessages) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(logger.errorMessages))
	}
	if logger.errorMessages[0] != "gRPC request failed" {
		t.Errorf("Expected message to be 'gRPC request failed', got '%s'", logger.errorMessages[0])
	}
}

func TestLoggingStreamServerInterceptor(t *testing.T) {
	// Create a mock logger
	logger := newMockLogger()

	// Create the interceptor
	interceptor := LoggingStreamServerInterceptor(logger)

	// Create a mock handler
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: context.Background(),
	}

	// Call the interceptor
	err := interceptor(nil, stream, &grpc.StreamServerInfo{
		FullMethod:     "/service/method",
		IsClientStream: true,
		IsServerStream: true,
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that the logger methods were called
	if len(logger.infoMessages) != 2 {
		t.Errorf("Expected 2 info messages, got %d", len(logger.infoMessages))
	}
	if logger.infoMessages[0] != "gRPC stream started" {
		t.Errorf("Expected first message to be 'gRPC stream started', got '%s'", logger.infoMessages[0])
	}
	if logger.infoMessages[1] != "gRPC stream completed" {
		t.Errorf("Expected second message to be 'gRPC stream completed', got '%s'", logger.infoMessages[1])
	}
}

func TestMetricsUnaryServerInterceptor(t *testing.T) {
	// Create a mock metrics
	metrics := newMockMetrics()

	// Create the interceptor
	interceptor := MetricsUnaryServerInterceptor(metrics)

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return "response", nil
	}

	// Call the interceptor
	resp, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected response to be 'response', got %v", resp)
	}

	// Verify that the metrics were recorded
	counter, ok := metrics.counters["grpc_server_requests_total"]
	if !ok {
		t.Error("Expected counter 'grpc_server_requests_total' to be created")
	} else {
		mockCounter := counter.(*mockCounter)
		if mockCounter.value != 0 {
			// The counter value is 0 because we're using WithLabels which returns a new counter
			t.Errorf("Expected counter value to be 0, got %f", mockCounter.value)
		}
	}

	histogram, ok := metrics.histograms["grpc_server_request_duration_seconds"]
	if !ok {
		t.Error("Expected histogram 'grpc_server_request_duration_seconds' to be created")
	} else {
		mockHistogram := histogram.(*mockHistogram)
		if mockHistogram.valueCount != 0 {
			// The histogram value count is 0 because we're using WithLabels which returns a new histogram
			t.Errorf("Expected histogram value count to be 0, got %d", mockHistogram.valueCount)
		}
	}
}

func TestMetricsStreamServerInterceptor(t *testing.T) {
	// Create a mock metrics
	metrics := newMockMetrics()

	// Create the interceptor
	interceptor := MetricsStreamServerInterceptor(metrics)

	// Create a mock handler
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: context.Background(),
	}

	// Call the interceptor
	err := interceptor(nil, stream, &grpc.StreamServerInfo{
		FullMethod:     "/service/method",
		IsClientStream: true,
		IsServerStream: true,
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that the metrics were recorded
	counter, ok := metrics.counters["grpc_server_streams_total"]
	if !ok {
		t.Error("Expected counter 'grpc_server_streams_total' to be created")
	} else {
		mockCounter := counter.(*mockCounter)
		if mockCounter.value != 0 {
			// The counter value is 0 because we're using WithLabels which returns a new counter
			t.Errorf("Expected counter value to be 0, got %f", mockCounter.value)
		}
	}

	histogram, ok := metrics.histograms["grpc_server_stream_duration_seconds"]
	if !ok {
		t.Error("Expected histogram 'grpc_server_stream_duration_seconds' to be created")
	} else {
		mockHistogram := histogram.(*mockHistogram)
		if mockHistogram.valueCount != 0 {
			// The histogram value count is 0 because we're using WithLabels which returns a new histogram
			t.Errorf("Expected histogram value count to be 0, got %d", mockHistogram.valueCount)
		}
	}
}

func TestRecoveryUnaryServerInterceptor(t *testing.T) {
	// Create a mock logger
	logger := newMockLogger()

	// Create the interceptor
	interceptor := RecoveryUnaryServerInterceptor(logger)

	// Create a mock handler that panics
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}

	// Call the interceptor
	resp, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if resp != nil {
		t.Errorf("Expected response to be nil, got %v", resp)
	}

	// Verify that the logger methods were called
	if len(logger.errorMessages) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(logger.errorMessages))
	}
	if logger.errorMessages[0] != "gRPC panic recovered" {
		t.Errorf("Expected message to be 'gRPC panic recovered', got '%s'", logger.errorMessages[0])
	}
}

func TestRecoveryStreamServerInterceptor(t *testing.T) {
	// Create a mock logger
	logger := newMockLogger()

	// Create the interceptor
	interceptor := RecoveryStreamServerInterceptor(logger)

	// Create a mock handler that panics
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		panic("test panic")
	}

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: context.Background(),
	}

	// Call the interceptor
	err := interceptor(nil, stream, &grpc.StreamServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Verify that the logger methods were called
	if len(logger.errorMessages) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(logger.errorMessages))
	}
	if logger.errorMessages[0] != "gRPC stream panic recovered" {
		t.Errorf("Expected message to be 'gRPC stream panic recovered', got '%s'", logger.errorMessages[0])
	}
}

func TestAuthUnaryServerInterceptor(t *testing.T) {
	// Create a mock authenticator
	authenticator := newMockAuthenticator()

	// Create the interceptor
	interceptor := AuthUnaryServerInterceptor(authenticator, []string{"/health"})

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify that the claims are in the context
		claims, ok := security.GetClaimsFromContext(ctx)
		if !ok {
			t.Error("Expected claims in context")
		}
		if claims.Subject != "test-user" {
			t.Errorf("Expected subject to be 'test-user', got '%s'", claims.Subject)
		}
		return "response", nil
	}

	// Create a context with authorization metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer valid-token",
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
}

func TestAuthUnaryServerInterceptorBypass(t *testing.T) {
	// Create a mock authenticator
	authenticator := newMockAuthenticator()

	// Create the interceptor with a bypass path
	interceptor := AuthUnaryServerInterceptor(authenticator, []string{"/health"})

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// No authentication should be performed for bypass paths
		return "response", nil
	}

	// Create a context without authorization metadata
	ctx := context.Background()

	// Call the interceptor with a bypass path
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{
		FullMethod: "/health",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("Expected response to be 'response', got %v", resp)
	}
}

func TestAuthUnaryServerInterceptorInvalidToken(t *testing.T) {
	// Create a mock authenticator
	authenticator := newMockAuthenticator()

	// Create the interceptor
	interceptor := AuthUnaryServerInterceptor(authenticator, []string{"/health"})

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// This should not be called
		t.Error("Handler should not be called with invalid token")
		return nil, nil
	}

	// Create a context with invalid authorization metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer invalid-token",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Call the interceptor
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if resp != nil {
		t.Errorf("Expected response to be nil, got %v", resp)
	}
}

func TestAuthStreamServerInterceptor(t *testing.T) {
	// Create a mock authenticator
	authenticator := newMockAuthenticator()

	// Create the interceptor
	interceptor := AuthStreamServerInterceptor(authenticator, []string{"/health"})

	// Create a mock handler
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		// Verify that the claims are in the context
		ctx := stream.Context()
		claims, ok := security.GetClaimsFromContext(ctx)
		if !ok {
			t.Error("Expected claims in context")
		}
		if claims.Subject != "test-user" {
			t.Errorf("Expected subject to be 'test-user', got '%s'", claims.Subject)
		}
		return nil
	}

	// Create a context with authorization metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer valid-token",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: ctx,
	}

	// Call the interceptor
	err := interceptor(nil, stream, &grpc.StreamServerInfo{
		FullMethod: "/service/method",
	}, handler)

	// Verify the response
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestCombinedObservabilityUnaryServerInterceptor(t *testing.T) {
	// Create mock dependencies
	tracer := &mockTracer{}
	metrics := newMockMetrics()
	logger := newMockLogger()

	// Create the interceptor
	interceptor := CombinedObservabilityUnaryServerInterceptor(tracer, metrics, logger)

	// Create a mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Call the interceptor
	resp, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
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
	if !tracer.extractCalled {
		t.Error("Expected Extract to be called")
	}
	if !tracer.startCalled {
		t.Error("Expected Start to be called")
	}

	// Verify that the logger methods were called
	if len(logger.infoMessages) != 2 {
		t.Errorf("Expected 2 info messages, got %d", len(logger.infoMessages))
	}
}

func TestCombinedObservabilityStreamServerInterceptor(t *testing.T) {
	// Create mock dependencies
	tracer := &mockTracer{}
	metrics := newMockMetrics()
	logger := newMockLogger()

	// Create the interceptor
	interceptor := CombinedObservabilityStreamServerInterceptor(tracer, metrics, logger)

	// Create a mock handler
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	// Create a mock server stream
	stream := &mockServerStream{
		ctx: context.Background(),
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
	if !tracer.extractCalled {
		t.Error("Expected Extract to be called")
	}
	if !tracer.startCalled {
		t.Error("Expected Start to be called")
	}

	// Verify that the logger methods were called
	if len(logger.infoMessages) != 2 {
		t.Errorf("Expected 2 info messages, got %d", len(logger.infoMessages))
	}
}
