package microlib

// // TestServiceE2E tests a complete service scenario with HTTP server, observability, and graceful shutdown
// func TestServiceE2E(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping E2E test in short mode")
// 	}

// 	// Create service configuration
// 	serviceConfig := &TestServiceConfig{
// 		Service: ServiceConfig{
// 			Name:    "test-service",
// 			Version: "1.0.0",
// 		},
// 		HTTP: httpserver.Config{
// 			Address: "127.0.0.1:0", // Use random port
// 			Timeout: httpserver.TimeoutConfig{
// 				Read:  30 * time.Second,
// 				Write: 30 * time.Second,
// 				Idle:  60 * time.Second,
// 			},
// 		},
// 		Observability: observability.Config{
// 			Logging: observability.LogConfig{
// 				Level:  "info",
// 				Format: "json",
// 			},
// 			Metrics: observability.MetricsConfig{
// 				Enabled: true,
// 			},
// 			Tracing: observability.TracingConfig{
// 				Enabled: false, // Disable for testing
// 			},
// 		},
// 	}

// 	// Create logger and metrics
// 	logger := observability.NewLoggerWithConfig(&serviceConfig.Observability.Logging)
// 	metrics := observability.NewMetricsWithConfig(&serviceConfig.Observability.Metrics)

// 	// Create HTTP server
// 	httpServer := httpserver.NewServerWithConfig(&serviceConfig.HTTP, logger, metrics)

// 	// Register test endpoints
// 	httpServer.RegisterHandler("/api/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusOK)
// 		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
// 	}))

// 	httpServer.RegisterHandler("/api/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		switch r.Method {
// 		case http.MethodGet:
// 			users := []map[string]interface{}{
// 				{"id": 1, "name": "John Doe", "email": "john@example.com"},
// 				{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
// 			}
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(users)
// 		case http.MethodPost:
// 			var user map[string]interface{}
// 			if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
// 				http.Error(w, "Invalid JSON", http.StatusBadRequest)
// 				return
// 			}
// 			user["id"] = 3
// 			w.Header().Set("Content-Type", "application/json")
// 			w.WriteHeader(http.StatusCreated)
// 			json.NewEncoder(w).Encode(user)
// 		default:
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		}
// 	}))

// 	// Create service metadata
// 	metadata := core.ServiceMetadata{
// 		Name:      serviceConfig.Service.Name,
// 		Version:   serviceConfig.Service.Version,
// 		Instance:  "test-instance",
// 		BuildHash: "test-hash",
// 	}

// 	// Create service
// 	service := core.NewService(metadata)

// 	// Add HTTP server as a dependency
// 	service.AddDependency(&httpServerDependency{server: httpServer})

// 	// Start service in background
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	serviceErr := make(chan error, 1)
// 	go func() {
// 		serviceErr <- service.Run(ctx)
// 	}()

// 	// Wait for service to start
// 	time.Sleep(100 * time.Millisecond)

// 	// Get the actual server address
// 	serverAddr := httpServer.Address()
// 	if serverAddr == "" {
// 		t.Fatal("Server address is empty")
// 	}

// 	baseURL := fmt.Sprintf("http://%s", serverAddr)

// 	// Test health endpoint
// 	t.Run("HealthEndpoint", func(t *testing.T) {
// 		resp, err := http.Get(baseURL + "/api/health")
// 		if err != nil {
// 			t.Fatalf("Failed to call health endpoint: %v", err)
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode != http.StatusOK {
// 			t.Errorf("Expected status 200, got %d", resp.StatusCode)
// 		}

// 		var health map[string]string
// 		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
// 			t.Fatalf("Failed to decode health response: %v", err)
// 		}

// 		if health["status"] != "ok" {
// 			t.Errorf("Expected status 'ok', got '%s'", health["status"])
// 		}
// 	})

// 	// Test users endpoint - GET
// 	t.Run("GetUsers", func(t *testing.T) {
// 		resp, err := http.Get(baseURL + "/api/users")
// 		if err != nil {
// 			t.Fatalf("Failed to get users: %v", err)
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode != http.StatusOK {
// 			t.Errorf("Expected status 200, got %d", resp.StatusCode)
// 		}

// 		var users []map[string]interface{}
// 		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
// 			t.Fatalf("Failed to decode users response: %v", err)
// 		}

// 		if len(users) != 2 {
// 			t.Errorf("Expected 2 users, got %d", len(users))
// 		}
// 	})

// 	// Test users endpoint - POST
// 	t.Run("CreateUser", func(t *testing.T) {
// 		userJSON := `{"name": "Bob Wilson", "email": "bob@example.com"}`
// 		resp, err := http.Post(baseURL+"/api/users", "application/json",
// 			NewBufferString(userJSON))
// 		if err != nil {
// 			t.Fatalf("Failed to create user: %v", err)
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode != http.StatusCreated {
// 			t.Errorf("Expected status 201, got %d", resp.StatusCode)
// 		}

// 		var user map[string]interface{}
// 		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
// 			t.Fatalf("Failed to decode user response: %v", err)
// 		}

// 		if user["name"] != "Bob Wilson" {
// 			t.Errorf("Expected name 'Bob Wilson', got '%v'", user["name"])
// 		}

// 		if user["id"] != float64(3) { // JSON numbers are float64
// 			t.Errorf("Expected id 3, got %v", user["id"])
// 		}
// 	})

// 	// Test metrics endpoint
// 	t.Run("MetricsEndpoint", func(t *testing.T) {
// 		resp, err := http.Get(baseURL + "/metrics")
// 		if err != nil {
// 			t.Fatalf("Failed to get metrics: %v", err)
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode != http.StatusOK {
// 			t.Errorf("Expected status 200, got %d", resp.StatusCode)
// 		}

// 		// Check content type
// 		contentType := resp.Header.Get("Content-Type")
// 		if contentType != "text/plain; version=0.0.4; charset=utf-8" {
// 			t.Errorf("Expected Prometheus content type, got %s", contentType)
// 		}
// 	})

// 	// Test graceful shutdown
// 	t.Run("GracefulShutdown", func(t *testing.T) {
// 		// Cancel context to trigger shutdown
// 		cancel()

// 		// Wait for service to shutdown
// 		select {
// 		case err := <-serviceErr:
// 			if err != nil && err != context.Canceled {
// 				t.Errorf("Service shutdown with error: %v", err)
// 			}
// 		case <-time.After(5 * time.Second):
// 			t.Error("Service did not shutdown within timeout")
// 		}

// 		// Verify server is no longer accepting connections
// 		_, err := http.Get(baseURL + "/api/health")
// 		if err == nil {
// 			t.Error("Expected error when calling shutdown server")
// 		}
// 	})
// }

// // TestServiceConfig represents the configuration for the test service
// type TestServiceConfig struct {
// 	Service       ServiceConfig        `yaml:"service"`
// 	HTTP          httpserver.Config    `yaml:"http"`
// 	Observability observability.Config `yaml:"observability"`
// }

// // ServiceConfig represents basic service configuration
// type ServiceConfig struct {
// 	Name    string `yaml:"name" validate:"required"`
// 	Version string `yaml:"version" validate:"required"`
// }

// // httpServerDependency implements the Dependency interface for HTTP server
// type httpServerDependency struct {
// 	server *httpserver.Server
// }

// func (d *httpServerDependency) Name() string {
// 	return "http-server"
// }

// func (d *httpServerDependency) HealthCheck(ctx context.Context) error {
// 	// Simple health check - verify server is not nil and can be started
// 	if d.server == nil {
// 		return fmt.Errorf("HTTP server is nil")
// 	}
// 	return nil
// }

// // TestConfigurationE2E tests the configuration system with multiple sources
// func TestConfigurationE2E(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping E2E test in short mode")
// 	}

// 	// Create temporary config file
// 	configContent := `
// service:
//   name: "config-test-service"
//   version: "2.0.0"
// http:
//   address: "0.0.0.0:8080"
//   timeout:
//     read: "30s"
//     write: "30s"
//     idle: "60s"
// observability:
//   logging:
//     level: "debug"
//     format: "json"
//   metrics:
//     enabled: true
// `

// 	configFile := t.TempDir() + "/config.yaml"
// 	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
// 		t.Fatalf("Failed to write config file: %v", err)
// 	}

// 	// Create configuration manager
// 	manager := config.NewManager()

// 	// Add file source
// 	fileSource := config.NewFileSource(configFile)
// 	manager.AddSource(fileSource)

// 	// Add environment source (higher priority)
// 	envSource := config.NewEnvSource("TEST")
// 	manager.AddSource(envSource)

// 	// Set environment variable to override config
// 	t.Setenv("TEST_SERVICE_NAME", "env-override-service")
// 	t.Setenv("TEST_HTTP_ADDRESS", "127.0.0.1:9090")

// 	// Load configuration
// 	var cfg TestServiceConfig
// 	err := manager.Load(&cfg)
// 	if err != nil {
// 		t.Fatalf("Failed to load configuration: %v", err)
// 	}

// 	// Verify environment variables override file values
// 	if cfg.Service.Name != "env-override-service" {
// 		t.Errorf("Expected service name 'env-override-service', got '%s'", cfg.Service.Name)
// 	}

// 	if cfg.HTTP.Address != "127.0.0.1:9090" {
// 		t.Errorf("Expected HTTP address '127.0.0.1:9090', got '%s'", cfg.HTTP.Address)
// 	}

// 	// Verify file values are used when no env override
// 	if cfg.Service.Version != "2.0.0" {
// 		t.Errorf("Expected service version '2.0.0', got '%s'", cfg.Service.Version)
// 	}

// 	if cfg.Observability.Logging.Level != "debug" {
// 		t.Errorf("Expected log level 'debug', got '%s'", cfg.Observability.Logging.Level)
// 	}

// 	// Test configuration validation
// 	t.Run("ConfigValidation", func(t *testing.T) {
// 		// Test with invalid configuration
// 		var invalidCfg TestServiceConfig
// 		invalidCfg.Service.Name = "" // Required field is empty

// 		err := manager.Validate(&invalidCfg)
// 		if err == nil {
// 			t.Error("Expected validation error for empty service name")
// 		}
// 	})
// }

// // Helper function to create a buffer from string for HTTP requests
// func NewBufferString(s string) *strings.Reader {
// 	return strings.NewReader(s)
// }
