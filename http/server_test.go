package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
)

func TestNewServer(t *testing.T) {
	deps := ServerDependencies{}
	server := NewServer(deps)
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestNewServerWithConfig(t *testing.T) {
	config := DefaultServerConfig()
	config.Port = 8081
	deps := ServerDependencies{}

	server := NewServerWithConfig(config, deps)
	if server == nil {
		t.Fatal("NewServerWithConfig() returned nil")
	}

	if server.Address() != "0.0.0.0:8081" {
		t.Errorf("Expected address 0.0.0.0:8081, got %s", server.Address())
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	// Use a specific port for testing
	testPort := 8090
	deps := ServerDependencies{}
	server := NewServerWithOptions(deps, WithPort(testPort), WithHost("localhost"))

	// Register a test handler before starting the server
	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Start the server
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify the server is started
	if !server.IsStarted() {
		t.Fatal("Server should be started")
	}

	// Get the actual address
	addr := server.Address()
	if addr != fmt.Sprintf("localhost:%d", testPort) {
		t.Errorf("Expected address localhost:%d, got %s", testPort, addr)
	}

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to the server
	resp, err := http.Get(fmt.Sprintf("http://%s/test", addr))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "OK" {
		t.Errorf("Expected response body 'OK', got '%s'", string(body))
	}

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// Verify the server is stopped
	if server.IsStarted() {
		t.Fatal("Server should be stopped")
	}
}

func TestRegisterMiddleware(t *testing.T) {
	// Use a specific port for testing
	testPort := 8091
	deps := ServerDependencies{}
	server := NewServerWithOptions(deps, WithPort(testPort), WithHost("localhost"))

	// Create a middleware that adds a header
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "test-value")
			next.ServeHTTP(w, r)
		})
	}

	// Register the middleware
	server.RegisterMiddleware(middleware)

	// Register a test handler
	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Start the server
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown(ctx)

	// Get the actual address
	addr := server.Address()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to the server
	resp, err := http.Get(fmt.Sprintf("http://%s/test", addr))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the middleware was applied
	if resp.Header.Get("X-Test") != "test-value" {
		t.Errorf("Expected header X-Test to be 'test-value', got '%s'", resp.Header.Get("X-Test"))
	}
}

func TestBasePath(t *testing.T) {
	// Use a specific port for testing
	testPort := 8092
	deps := ServerDependencies{}
	server := NewServerWithOptions(deps, WithPort(testPort), WithHost("localhost"), WithBasePath("/api"))

	// Register a test handler
	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Start the server
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown(ctx)

	// Get the actual address
	addr := server.Address()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to the server with the base path
	resp, err := http.Get(fmt.Sprintf("http://%s/api/test", addr))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

// TestServerRecovery tests the recovery middleware in the server
func TestServerRecovery(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger3{
		errorMessages: []string{},
		errorFields:   []map[string]interface{}{},
	}

	// Use a specific port for testing
	testPort := 8093
	deps := ServerDependencies{
		Logger: logger,
	}
	server := NewServerWithOptions(deps, WithPort(testPort), WithHost("localhost"))

	// No need to register recovery middleware as it's registered by default

	// Register a handler that panics
	server.RegisterHandler("/panic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	// Start the server
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown(ctx)

	// Get the actual address
	addr := server.Address()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to the handler that panics
	resp, err := http.Get(fmt.Sprintf("http://%s/panic", addr))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response is a 500 Internal Server Error
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.errorMessages) == 0 {
		t.Error("Expected logger.Error to be called")
	}
}

// mockLogger3 is a mock implementation of the observability.Logger interface for testing
type mockLogger3 struct {
	infoMessages  []string
	errorMessages []string
	infoFields    []map[string]interface{}
	errorFields   []map[string]interface{}
}

func (l *mockLogger3) Info(msg string, fields ...observability.Field) {
	l.InfoContext(context.Background(), msg, fields...)
}

func (l *mockLogger3) InfoContext(ctx context.Context, msg string, fields ...observability.Field) {
	l.infoMessages = append(l.infoMessages, msg)
	fieldMap := make(map[string]interface{})
	for _, field := range fields {
		fieldMap[field.Key] = field.Value
	}
	l.infoFields = append(l.infoFields, fieldMap)
}

func (l *mockLogger3) Error(msg string, err error, fields ...observability.Field) {
	l.ErrorContext(context.Background(), msg, err, fields...)
}

func (l *mockLogger3) ErrorContext(ctx context.Context, msg string, err error, fields ...observability.Field) {
	l.errorMessages = append(l.errorMessages, msg)
	fieldMap := make(map[string]interface{})
	fieldMap["error"] = err.Error()
	for _, field := range fields {
		fieldMap[field.Key] = field.Value
	}
	l.errorFields = append(l.errorFields, fieldMap)
}

func (l *mockLogger3) Debug(msg string, fields ...observability.Field) {}

func (l *mockLogger3) DebugContext(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *mockLogger3) Warn(msg string, fields ...observability.Field) {}

func (l *mockLogger3) WarnContext(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *mockLogger3) WithContext(ctx context.Context) observability.Logger { return l }
