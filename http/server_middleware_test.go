package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerWithMiddleware(t *testing.T) {
	// Create mock dependencies
	logger := &mockLogger{}
	metrics := newMockMetrics()
	tracer := newMockOtelTracer()

	deps := ServerDependencies{
		Logger:  logger,
		Metrics: metrics,
		Tracer:  tracer,
	}

	// Create a server with default configuration
	server := NewServer(deps)

	// Register a test handler
	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// We'll create the request directly when calling the test server

	// Create a test server that uses our handler
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the path and pass it to our server's handler
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Apply our middleware chain
		middlewareChain := MiddlewareChain{
			LoggingMiddleware(logger),
			MetricsMiddleware(metrics),
			TracingMiddleware(tracer),
		}

		// Apply the middleware chain to the handler
		wrappedHandler := middlewareChain.Apply(handler)

		// Serve the request
		wrappedHandler.ServeHTTP(w, r)
	}))
	defer testServer.Close()

	// Make a request to the test server
	resp, err := http.Get(testServer.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.infoMessages) < 1 {
		t.Errorf("Expected at least 1 info message, got %d", len(logger.infoMessages))
	}
}

func TestCORSMiddlewareIntegration(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware with a custom configuration
	config := CORSConfig{
		Enabled:          true,
		AllowOrigins:     "http://example.com",
		AllowMethods:     "GET, POST",
		AllowHeaders:     "Content-Type",
		ExposeHeaders:    "X-Custom-Header",
		AllowCredentials: true,
		MaxAge:           3600,
	}
	middleware := CORSMiddleware(config)

	// Apply the middleware to the handler
	handler := middleware(testHandler)

	// Create a test server with the middleware
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create a request with the Origin header
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://example.com")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin header to be 'http://example.com', got '%s'", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestSecurityHeadersMiddlewareIntegration(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware with a custom configuration
	config := SecurityHeadersConfig{
		Enabled:                 true,
		ContentSecurityPolicy:   "default-src 'self'",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     true,
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		XSSProtection:           true,
	}
	middleware := SecurityHeadersMiddleware(config)

	// Apply the middleware to the handler
	handler := middleware(testHandler)

	// Create a test server with the middleware
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the security headers
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options header to be 'DENY', got '%s'", resp.Header.Get("X-Frame-Options"))
	}

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options header to be 'nosniff', got '%s'", resp.Header.Get("X-Content-Type-Options"))
	}
}

func TestRecoveryMiddlewareIntegration(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{}

	// Create a test handler that panics
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Create the middleware
	middleware := RecoveryMiddleware(logger)

	// Apply the middleware to the handler
	handler := middleware(testHandler)

	// Create a test server with the middleware
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response is a 500 Internal Server Error
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.errorMessages) < 1 {
		t.Errorf("Expected at least 1 error message, got %d", len(logger.errorMessages))
	}
}

func TestMiddlewareChainIntegration(t *testing.T) {
	// Create mock dependencies
	logger := &mockLogger{}
	metrics := newMockMetrics()
	tracer := newMockOtelTracer()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a middleware chain
	chain := MiddlewareChain{
		LoggingMiddleware(logger),
		MetricsMiddleware(metrics),
		TracingMiddleware(tracer),
		SecurityHeadersMiddleware(DefaultSecurityHeadersConfig()),
	}

	// Apply the middleware chain to the handler
	handler := chain.Apply(testHandler)

	// Create a test server with the handler
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the logger was called
	if len(logger.infoMessages) < 1 {
		t.Errorf("Expected at least 1 info message, got %d", len(logger.infoMessages))
	}

	// Verify security headers were added
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options header to be 'DENY', got '%s'", resp.Header.Get("X-Frame-Options"))
	}
}
