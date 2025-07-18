package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	server := NewServer()
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestNewServerWithConfig(t *testing.T) {
	config := DefaultServerConfig()
	config.Port = 8081

	server := NewServerWithConfig(config)
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
	server := NewServerWithOptions(WithPort(testPort), WithHost("localhost"))

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
	server := NewServerWithOptions(WithPort(testPort), WithHost("localhost"))

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
	server := NewServerWithOptions(WithPort(testPort), WithHost("localhost"), WithBasePath("/api"))

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

func TestRecoveryMiddleware(t *testing.T) {
	// Create a mock logger
	mockLogger := &mockLogger{}

	// Use a specific port for testing
	testPort := 8093
	server := NewServerWithOptions(WithPort(testPort), WithHost("localhost"))

	// Register the recovery middleware
	server.RegisterMiddleware(RecoveryMiddleware(mockLogger))

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
	if !mockLogger.errorCalled {
		t.Error("Expected logger.Error to be called")
	}
}

// mockLogger is a mock implementation of the logger interface
type mockLogger struct {
	errorCalled bool
}

func (l *mockLogger) Error(msg string, err error, fields ...interface{}) {
	l.errorCalled = true
}
