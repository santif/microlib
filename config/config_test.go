package config

import (
	"sync"
	"testing"
	"time"
)

// TestConfigStruct is a test configuration struct
type TestConfigStruct struct {
	Service struct {
		Name    string `json:"name" yaml:"name" validate:"required"`
		Version string `json:"version" yaml:"version" validate:"required"`
	} `json:"service" yaml:"service"`
	HTTP struct {
		Port    int      `json:"port" yaml:"port" validate:"required,min=1,max=65535"`
		Timeout int      `json:"timeout" yaml:"timeout"`
		Hosts   []string `json:"hosts" yaml:"hosts"`
	} `json:"http" yaml:"http"`
	Features struct {
		EnableCache bool `json:"enable_cache" yaml:"enable_cache"`
		Debug       bool `json:"debug" yaml:"debug"`
	} `json:"features" yaml:"features"`
}

// TestConfig_Creation tests the creation of a thread-safe configuration
func TestConfig_Creation(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Check that the config was stored correctly
	if safeConfig == nil {
		t.Fatal("Thread-safe config is nil")
	}

	// Get the config and check values
	configData := safeConfig.Get()
	if configData == nil {
		t.Fatal("Config data is nil")
	}

	// Type assertion
	typedConfig, ok := configData.(*TestConfigStruct)
	if !ok {
		t.Fatal("Failed to cast config data to TestConfigStruct")
	}

	// Check values
	if typedConfig.Service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got '%s'", typedConfig.Service.Name)
	}
	if typedConfig.Service.Version != "1.0.0" {
		t.Errorf("Expected service version to be '1.0.0', got '%s'", typedConfig.Service.Version)
	}
	if typedConfig.HTTP.Port != 8080 {
		t.Errorf("Expected HTTP port to be 8080, got %d", typedConfig.HTTP.Port)
	}
}

// TestConfig_GetField tests getting a specific field from the configuration
func TestConfig_GetField(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080
	config.HTTP.Hosts = []string{"localhost", "example.com"}

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Get fields and check values
	serviceName, err := safeConfig.GetField("Service.Name")
	if err != nil {
		t.Fatalf("Failed to get Service.Name: %v", err)
	}
	if serviceName != "test-service" {
		t.Errorf("Expected Service.Name to be 'test-service', got '%v'", serviceName)
	}

	httpPort, err := safeConfig.GetField("HTTP.Port")
	if err != nil {
		t.Fatalf("Failed to get HTTP.Port: %v", err)
	}
	if httpPort != 8080 {
		t.Errorf("Expected HTTP.Port to be 8080, got %v", httpPort)
	}

	// Test getting a non-existent field
	_, err = safeConfig.GetField("NonExistent")
	if err == nil {
		t.Error("Expected error when getting non-existent field, got nil")
	}
}

// TestConfig_Update tests updating the entire configuration
func TestConfig_Update(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Create a new config with different values
	newConfig := &TestConfigStruct{}
	newConfig.Service.Name = "updated-service"
	newConfig.Service.Version = "2.0.0"
	newConfig.HTTP.Port = 9090

	// Update the config
	err = safeConfig.Update(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Get the updated config and check values
	updatedConfig := safeConfig.Get().(*TestConfigStruct)
	if updatedConfig.Service.Name != "updated-service" {
		t.Errorf("Expected service name to be 'updated-service', got '%s'", updatedConfig.Service.Name)
	}
	if updatedConfig.Service.Version != "2.0.0" {
		t.Errorf("Expected service version to be '2.0.0', got '%s'", updatedConfig.Service.Version)
	}
	if updatedConfig.HTTP.Port != 9090 {
		t.Errorf("Expected HTTP port to be 9090, got %d", updatedConfig.HTTP.Port)
	}
}

// TestConfig_UpdateField tests updating a specific field in the configuration
func TestConfig_UpdateField(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Update a field
	err = safeConfig.UpdateField("Service.Name", "updated-service")
	if err != nil {
		t.Fatalf("Failed to update Service.Name: %v", err)
	}

	// Get the updated field and check value
	serviceName, err := safeConfig.GetField("Service.Name")
	if err != nil {
		t.Fatalf("Failed to get Service.Name: %v", err)
	}
	if serviceName != "updated-service" {
		t.Errorf("Expected Service.Name to be 'updated-service', got '%v'", serviceName)
	}

	// Update another field
	err = safeConfig.UpdateField("HTTP.Port", 9090)
	if err != nil {
		t.Fatalf("Failed to update HTTP.Port: %v", err)
	}

	// Get the updated field and check value
	httpPort, err := safeConfig.GetField("HTTP.Port")
	if err != nil {
		t.Fatalf("Failed to get HTTP.Port: %v", err)
	}
	if httpPort != 9090 {
		t.Errorf("Expected HTTP.Port to be 9090, got %v", httpPort)
	}

	// Test updating a non-existent field
	err = safeConfig.UpdateField("NonExistent", "value")
	if err == nil {
		t.Error("Expected error when updating non-existent field, got nil")
	}
}

// TestConfig_ConcurrentAccess tests concurrent access to the configuration
func TestConfig_ConcurrentAccess(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080
	config.Features.Debug = false

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Number of concurrent goroutines
	numGoroutines := 100

	// Wait group to synchronize goroutines
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Readers and writers

	// Start reader goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Read the config multiple times
			for j := 0; j < 10; j++ {
				// Get the config
				configData := safeConfig.Get()
				if configData == nil {
					t.Errorf("Reader %d: Config data is nil", id)
					return
				}

				// Get a specific field
				_, err := safeConfig.GetField("Service.Name")
				if err != nil {
					t.Errorf("Reader %d: Failed to get Service.Name: %v", id, err)
					return
				}

				// Small delay to increase chance of concurrent access
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Start writer goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Update a field
			field := "Features.Debug"
			value := id%2 == 0 // Alternate between true and false

			err := safeConfig.UpdateField(field, value)
			if err != nil {
				t.Errorf("Writer %d: Failed to update %s: %v", id, field, err)
				return
			}

			// Small delay to increase chance of concurrent access
			time.Sleep(time.Millisecond)
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Final check
	configData := safeConfig.Get().(*TestConfigStruct)
	if configData == nil {
		t.Fatal("Final config data is nil")
	}
}

// TestInjectable tests the Injectable struct for configuration injection
func TestInjectable(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080

	// Create a thread-safe config
	safeConfig, err := NewConfig(config)
	if err != nil {
		t.Fatalf("Failed to create thread-safe config: %v", err)
	}

	// Create an injectable service
	service := &TestService{}

	// Inject the config
	service.InjectConfig(safeConfig)

	// Get the config from the service
	injectedConfig := service.GetConfig()
	if injectedConfig == nil {
		t.Fatal("Injected config is nil")
	}

	// Type assertion
	typedConfig, ok := injectedConfig.(*TestConfigStruct)
	if !ok {
		t.Fatal("Failed to cast injected config to TestConfigStruct")
	}

	// Check values
	if typedConfig.Service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got '%s'", typedConfig.Service.Name)
	}

	// Get a specific field
	httpPort, err := service.GetConfigField("HTTP.Port")
	if err != nil {
		t.Fatalf("Failed to get HTTP.Port: %v", err)
	}
	if httpPort != 8080 {
		t.Errorf("Expected HTTP.Port to be 8080, got %v", httpPort)
	}
}

// TestReloadable tests the Reloadable interface
func TestReloadable(t *testing.T) {
	// Create a test config
	config := &TestConfigStruct{}
	config.Service.Name = "test-service"
	config.Service.Version = "1.0.0"
	config.HTTP.Port = 8080

	// Create a manager
	manager := NewManager()

	// Create a reloadable service
	service := &TestService{
		done: make(chan bool, 1),
	}

	// Register the service as reloadable
	err := manager.RegisterReloadable(service)
	if err != nil {
		t.Fatalf("Failed to register reloadable service: %v", err)
	}

	// Notify watchers (which should call Reload on the service)
	manager.notifyWatchers(config)

	// Wait for the service to be reloaded or timeout
	select {
	case <-service.done:
		// Service was reloaded successfully
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Service was not reloaded within timeout")
	}

	// Check the reloaded config
	service.mu.RLock()
	lastConfig := service.lastConfig
	service.mu.RUnlock()

	if lastConfig == nil {
		t.Fatal("Reloaded config is nil")
	}

	// Type assertion
	typedConfig, ok := lastConfig.(*TestConfigStruct)
	if !ok {
		t.Fatal("Failed to cast reloaded config to TestConfigStruct")
	}

	// Check values
	if typedConfig.Service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got '%s'", typedConfig.Service.Name)
	}
}

// TestService is a test service that implements Reloadable
type TestService struct {
	Injectable
	mu         sync.RWMutex
	reloaded   bool
	lastConfig interface{}
	done       chan bool
}

// Reload implements the Reloadable interface
func (s *TestService) Reload(newConfig interface{}) error {
	s.mu.Lock()
	s.reloaded = true
	s.lastConfig = newConfig
	s.mu.Unlock()
	
	// Signal that reload is complete
	select {
	case s.done <- true:
	default:
		// Channel is full, ignore
	}
	
	return nil
}