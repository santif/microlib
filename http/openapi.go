package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/santif/microlib/observability"
)

// OpenAPIConfig contains configuration for OpenAPI integration
type OpenAPIConfig struct {
	// Enabled determines if OpenAPI is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// SpecPath is the path to the OpenAPI specification file
	SpecPath string `json:"spec_path" yaml:"spec_path"`

	// ServePath is the path where the OpenAPI specification will be served
	ServePath string `json:"serve_path" yaml:"serve_path"`

	// UIEnabled determines if Swagger UI is enabled
	UIEnabled bool `json:"ui_enabled" yaml:"ui_enabled"`

	// UIPath is the path where Swagger UI will be served
	UIPath string `json:"ui_path" yaml:"ui_path"`

	// ValidateRequests determines if requests should be validated against the OpenAPI spec
	ValidateRequests bool `json:"validate_requests" yaml:"validate_requests"`

	// ValidateResponses determines if responses should be validated against the OpenAPI spec
	ValidateResponses bool `json:"validate_responses" yaml:"validate_responses"`
}

// DefaultOpenAPIConfig returns the default OpenAPI configuration
func DefaultOpenAPIConfig() OpenAPIConfig {
	return OpenAPIConfig{
		Enabled:           true,
		SpecPath:          "",
		ServePath:         "/openapi.json",
		UIEnabled:         true,
		UIPath:            "/docs",
		ValidateRequests:  true,
		ValidateResponses: false,
	}
}

// OpenAPISpec represents an OpenAPI specification
type OpenAPISpec struct {
	spec   map[string]interface{}
	logger observability.Logger
}

// NewOpenAPISpec creates a new OpenAPI specification from a JSON file
func NewOpenAPISpec(specPath string, logger observability.Logger) (*OpenAPISpec, error) {
	// Read the OpenAPI specification file
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI specification: %w", err)
	}

	// Parse the JSON data
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI specification: %w", err)
	}

	return &OpenAPISpec{
		spec:   spec,
		logger: logger,
	}, nil
}

// NewOpenAPISpecFromBytes creates a new OpenAPI specification from a byte array
func NewOpenAPISpecFromBytes(data []byte, logger observability.Logger) (*OpenAPISpec, error) {
	// Parse the JSON data
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI specification: %w", err)
	}

	return &OpenAPISpec{
		spec:   spec,
		logger: logger,
	}, nil
}

// Spec returns the OpenAPI specification as a map
func (s *OpenAPISpec) Spec() map[string]interface{} {
	return s.spec
}

// JSON returns the OpenAPI specification as JSON
func (s *OpenAPISpec) JSON() ([]byte, error) {
	return json.MarshalIndent(s.spec, "", "  ")
}

// RegisterOpenAPIHandlers registers handlers for serving the OpenAPI specification and Swagger UI
func RegisterOpenAPIHandlers(server Server, config OpenAPIConfig, spec *OpenAPISpec, logger observability.Logger) {
	if !config.Enabled {
		return
	}

	// Register handler for serving the OpenAPI specification
	server.RegisterHandler(config.ServePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Marshal the spec to JSON
		data, err := spec.JSON()
		if err != nil {
			logger.ErrorContext(r.Context(), "Failed to marshal OpenAPI specification", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Write the response
		w.Write(data)
	}))

	// Register handler for serving Swagger UI if enabled
	if config.UIEnabled {
		// Serve Swagger UI
		server.RegisterHandler(config.UIPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If the path is exactly the UI path, redirect to the UI index
			if r.URL.Path == config.UIPath || r.URL.Path == config.UIPath+"/" {
				// Serve the Swagger UI HTML
				serveSwaggerUI(w, r, config.ServePath)
				return
			}

			// Otherwise, serve static files from the embedded Swagger UI
			http.NotFound(w, r)
		}))

		logger.Info("Registered OpenAPI handlers",
			observability.NewField("spec_path", config.ServePath),
			observability.NewField("ui_path", config.UIPath),
		)
	} else {
		logger.Info("Registered OpenAPI specification handler",
			observability.NewField("path", config.ServePath),
		)
	}
}

// serveSwaggerUI serves the Swagger UI HTML
func serveSwaggerUI(w http.ResponseWriter, r *http.Request, specPath string) {
	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Write the Swagger UI HTML
	w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css">
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "%s",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "StandaloneLayout",
        docExpansion: "list",
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        displayRequestDuration: true,
        filter: true,
        syntaxHighlight: {
          activate: true,
          theme: "agate"
        }
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`, specPath)))
}

// OpenAPIMiddleware creates middleware for OpenAPI validation
func OpenAPIMiddleware(spec *OpenAPISpec, config OpenAPIConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation if not enabled
			if !config.Enabled || (!config.ValidateRequests && !config.ValidateResponses) {
				next.ServeHTTP(w, r)
				return
			}

			// Create a response writer wrapper for response validation if needed
			var wrapper http.ResponseWriter = w
			if config.ValidateResponses {
				wrapper = &openAPIResponseWriter{
					ResponseWriter: w,
					spec:           spec,
					path:           r.URL.Path,
					method:         r.Method,
				}
			}

			// Validate request if enabled
			if config.ValidateRequests {
				// TODO: Implement request validation against OpenAPI spec
				// This would involve:
				// 1. Finding the path in the OpenAPI spec
				// 2. Validating the request method
				// 3. Validating request parameters
				// 4. Validating request body if present
				// 5. Returning an error if validation fails
			}

			// Process the request
			next.ServeHTTP(wrapper, r)
		})
	}
}

// openAPIResponseWriter is a wrapper around http.ResponseWriter that validates responses against an OpenAPI spec
type openAPIResponseWriter struct {
	http.ResponseWriter
	spec   *OpenAPISpec
	path   string
	method string
	body   []byte
}

// Write captures the response body for validation
func (w *openAPIResponseWriter) Write(b []byte) (int, error) {
	// TODO: Implement response validation against OpenAPI spec
	// This would involve:
	// 1. Finding the path in the OpenAPI spec
	// 2. Validating the response status code
	// 3. Validating the response body against the schema
	// 4. Logging an error if validation fails but still writing the response

	// Write the response
	return w.ResponseWriter.Write(b)
}

// WithOpenAPI configures OpenAPI for the server
func WithOpenAPI(enabled bool, specPath string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.OpenAPI == nil {
			config.OpenAPI = &OpenAPIConfig{}
		}
		config.OpenAPI.Enabled = enabled
		config.OpenAPI.SpecPath = specPath
	}
}

// WithOpenAPIUI configures Swagger UI for the server
func WithOpenAPIUI(enabled bool, uiPath string) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.OpenAPI == nil {
			config.OpenAPI = &OpenAPIConfig{}
		}
		config.OpenAPI.UIEnabled = enabled
		config.OpenAPI.UIPath = uiPath
	}
}

// WithOpenAPIValidation configures OpenAPI validation for the server
func WithOpenAPIValidation(validateRequests, validateResponses bool) func(*ServerConfig) {
	return func(config *ServerConfig) {
		if config.OpenAPI == nil {
			config.OpenAPI = &OpenAPIConfig{}
		}
		config.OpenAPI.ValidateRequests = validateRequests
		config.OpenAPI.ValidateResponses = validateResponses
	}
}
