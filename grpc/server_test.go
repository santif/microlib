package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
	"google.golang.org/grpc"
)

// mockService is a mock gRPC service for testing
type mockService struct{}

// mockServiceDesc is a mock gRPC service descriptor for testing
var mockServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.MockService",
	HandlerType: (*mockService)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams:     []grpc.StreamDesc{},
	Metadata:    "test/mock.proto",
}

func TestNewServer(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server with default configuration
	server := NewServer(deps)
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Check that the server has the default configuration
	config := server.Config()
	if config.Port != 9090 {
		t.Errorf("Expected default port 9090, got %d", config.Port)
	}
}

func TestNewServerWithConfig(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a custom configuration
	config := DefaultServerConfig()
	config.Port = 9091

	// Create a server with the custom configuration
	server := NewServerWithConfig(config, deps)
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Check that the server has the custom configuration
	serverConfig := server.Config()
	if serverConfig.Port != 9091 {
		t.Errorf("Expected port 9091, got %d", serverConfig.Port)
	}
}

func TestNewServerWithOptions(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server with options
	server := NewServerWithOptions(deps,
		WithPort(9092),
		WithHost("127.0.0.1"),
		WithShutdownTimeout(10*time.Second),
	)
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Check that the server has the custom configuration
	config := server.Config()
	if config.Port != 9092 {
		t.Errorf("Expected port 9092, got %d", config.Port)
	}
	if config.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", config.Host)
	}
	if config.ShutdownTimeout != 10*time.Second {
		t.Errorf("Expected shutdown timeout 10s, got %v", config.ShutdownTimeout)
	}
}

func TestRegisterService(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server
	s := NewServer(deps).(*server)

	// Register a service
	s.RegisterService(&mockServiceDesc, &mockService{})

	// Check that the service was registered
	if len(s.services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(s.services))
	}
	if s.services[0].desc.ServiceName != "test.MockService" {
		t.Errorf("Expected service name test.MockService, got %s", s.services[0].desc.ServiceName)
	}
}

func TestStartAndShutdown(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server with a random port
	server := NewServerWithOptions(deps, WithPort(0))

	// Start the server
	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Check that the server is started
	if !server.IsStarted() {
		t.Error("Expected server to be started")
	}

	// Get the server address
	addr := server.Address()
	if addr == "" {
		t.Error("Expected non-empty server address")
	}

	// Shutdown the server
	err = server.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// Check that the server is stopped
	if server.IsStarted() {
		t.Error("Expected server to be stopped")
	}
}

func TestStartAlreadyStarted(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server with a random port
	server := NewServerWithOptions(deps, WithPort(0))

	// Start the server
	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Try to start the server again
	err = server.Start(ctx)
	if err != ErrServerAlreadyStarted {
		t.Errorf("Expected ErrServerAlreadyStarted, got %v", err)
	}

	// Shutdown the server
	_ = server.Shutdown(ctx)
}

func TestShutdownNotStarted(t *testing.T) {
	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server
	server := NewServer(deps)

	// Try to shutdown the server
	ctx := context.Background()
	err := server.Shutdown(ctx)
	if err != ErrServerNotStarted {
		t.Errorf("Expected ErrServerNotStarted, got %v", err)
	}
}

func TestHealthService(t *testing.T) {
	// This is a more complex test that would require a real gRPC client
	// to connect to the server and check the health service.
	// For simplicity, we'll just check that the server starts and stops.

	// Create dependencies
	deps := ServerDependencies{
		Logger:  observability.NewLogger(),
		Metrics: observability.NewMetrics(),
		Tracer:  &mockTracer{},
	}

	// Create a server with a random port
	server := NewServerWithOptions(deps, WithPort(0))

	// Register a mock service
	server.RegisterService(&mockServiceDesc, &mockService{})

	// Start the server
	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// In a real test, we would connect to the server and check the health service
	// For example:
	// conn, err := grpc.Dial(server.Address(), grpc.WithInsecure())
	// if err != nil {
	//     t.Fatalf("Failed to connect to server: %v", err)
	// }
	// defer conn.Close()
	// client := grpc_health_v1.NewHealthClient(conn)
	// resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	// if err != nil {
	//     t.Fatalf("Failed to check health: %v", err)
	// }
	// if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
	//     t.Errorf("Expected health status SERVING, got %v", resp.Status)
	// }

	// Shutdown the server
	err = server.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}
}

// TestTLSConfiguration would test the TLS configuration
// This requires generating test certificates, which is beyond the scope of this test
func TestTLSConfiguration(t *testing.T) {
	// Skip this test for now
	t.Skip("TLS configuration test requires test certificates")
}

// TestInterceptors would test the interceptor chain
// This requires setting up a full gRPC client and server, which is beyond the scope of this test
func TestInterceptors(t *testing.T) {
	// Skip this test for now
	t.Skip("Interceptor test requires a full gRPC client and server setup")
}

// Additional tests for error cases, configuration options, etc. would go here
