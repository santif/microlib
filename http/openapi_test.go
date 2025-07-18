package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santif/microlib/observability"
)

func TestOpenAPISpec(t *testing.T) {
	// Create a temporary OpenAPI spec file
	specContent := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"summary": "Test endpoint",
					"responses": {
						"200": {
							"description": "OK"
						}
					}
				}
			}
		}
	}`

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "openapi-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the spec file
	specPath := filepath.Join(tempDir, "openapi.json")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Test loading the spec from a file
	spec, err := NewOpenAPISpec(specPath, logger)
	if err != nil {
		t.Fatalf("Failed to load OpenAPI spec: %v", err)
	}

	// Verify the spec was loaded correctly
	if spec == nil {
		t.Fatal("Expected spec to be non-nil")
	}

	// Verify the spec content
	if spec.spec["openapi"] != "3.0.0" {
		t.Errorf("Expected openapi version 3.0.0, got %v", spec.spec["openapi"])
	}

	// Test loading the spec from bytes
	specBytes := []byte(specContent)
	spec, err = NewOpenAPISpecFromBytes(specBytes, logger)
	if err != nil {
		t.Fatalf("Failed to load OpenAPI spec from bytes: %v", err)
	}

	// Verify the spec was loaded correctly
	if spec == nil {
		t.Fatal("Expected spec to be non-nil")
	}

	// Test JSON serialization
	jsonData, err := spec.JSON()
	if err != nil {
		t.Fatalf("Failed to serialize spec to JSON: %v", err)
	}

	// Verify the JSON data
	var parsedSpec map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsedSpec); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsedSpec["openapi"] != "3.0.0" {
		t.Errorf("Expected openapi version 3.0.0, got %v", parsedSpec["openapi"])
	}
}

func TestOpenAPIHandlers(t *testing.T) {
	// Create a simple OpenAPI spec
	specContent := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"summary": "Test endpoint",
					"responses": {
						"200": {
							"description": "OK"
						}
					}
				}
			}
		}
	}`

	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create the spec
	spec, err := NewOpenAPISpecFromBytes([]byte(specContent), logger)
	if err != nil {
		t.Fatalf("Failed to create OpenAPI spec: %v", err)
	}

	// Create a test server with mocked dependencies
	deps := ServerDependencies{
		Logger: logger,
	}
	server := NewServer(deps)

	// Create OpenAPI config
	config := OpenAPIConfig{
		Enabled:   true,
		ServePath: "/openapi.json",
		UIEnabled: true,
		UIPath:    "/docs",
	}

	// Register OpenAPI handlers
	RegisterOpenAPIHandlers(server, config, spec, logger)

	// Start the server
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	// Create a test HTTP server that directly uses the server's address
	serverAddr := server.Address()

	// Create a test HTTP server that forwards requests to our actual server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to our server
		url := "http://" + serverAddr + r.URL.Path
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}

		// Create a new request to forward
		req, err := http.NewRequest(r.Method, url, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy headers
		for name, values := range r.Header {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}

		// Send the request to our server
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Copy the response back
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}))
	defer testServer.Close()

	// Test the OpenAPI spec endpoint
	resp, err := http.Get(testServer.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("Failed to get OpenAPI spec: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify the response body
	var parsedSpec map[string]interface{}
	if err := json.Unmarshal(body, &parsedSpec); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsedSpec["openapi"] != "3.0.0" {
		t.Errorf("Expected openapi version 3.0.0, got %v", parsedSpec["openapi"])
	}

	// Test the Swagger UI endpoint
	resp, err = http.Get(testServer.URL + "/docs")
	if err != nil {
		t.Fatalf("Failed to get Swagger UI: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// Read the response body
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify the response contains Swagger UI HTML
	if !contains(string(body), "swagger-ui") {
		t.Errorf("Expected response to contain Swagger UI HTML")
	}
}

func TestOpenAPIMiddleware(t *testing.T) {
	// Create a simple OpenAPI spec
	specContent := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"summary": "Test endpoint",
					"responses": {
						"200": {
							"description": "OK",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"message": {
												"type": "string"
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create the spec
	spec, err := NewOpenAPISpecFromBytes([]byte(specContent), logger)
	if err != nil {
		t.Fatalf("Failed to create OpenAPI spec: %v", err)
	}

	// Create OpenAPI config
	config := OpenAPIConfig{
		Enabled:           true,
		ValidateRequests:  true,
		ValidateResponses: true,
	}

	// Create the middleware
	middleware := OpenAPIMiddleware(spec, config)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello, World!"}`))
	})

	// Apply the middleware
	handler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(rec, req)

	// Verify the response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rec.Code)
	}

	// Read the response body
	body := rec.Body.String()

	// Verify the response body
	if body != `{"message":"Hello, World!"}` {
		t.Errorf("Expected response body %q, got %q", `{"message":"Hello, World!"}`, body)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
