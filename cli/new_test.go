package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidServiceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid name", "my-service", true},
		{"valid name with numbers", "service123", true},
		{"valid name mixed", "my-service-123", true},
		{"empty name", "", false},
		{"invalid characters", "my_service", false},
		{"invalid characters space", "my service", false},
		{"invalid characters special", "my@service", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidServiceName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTemplateData(t *testing.T) {
	// Set test values
	serviceName = "test-service"
	withGRPC = true
	withJobs = false
	withCache = true
	withAuth = false

	data := getTemplateData()

	// Check required fields
	if data["ServiceName"] != "test-service" {
		t.Errorf("Expected ServiceName to be 'test-service', got %v", data["ServiceName"])
	}

	if data["WithGRPC"] != true {
		t.Errorf("Expected WithGRPC to be true, got %v", data["WithGRPC"])
	}

	if data["WithJobs"] != false {
		t.Errorf("Expected WithJobs to be false, got %v", data["WithJobs"])
	}

	if data["WithCache"] != true {
		t.Errorf("Expected WithCache to be true, got %v", data["WithCache"])
	}

	if data["WithAuth"] != false {
		t.Errorf("Expected WithAuth to be false, got %v", data["WithAuth"])
	}
}

func TestGenerateFile(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "microlib-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test template
	template := `Service: {{.ServiceName}}
WithGRPC: {{.WithGRPC}}`

	data := map[string]interface{}{
		"ServiceName": "test-service",
		"WithGRPC":    true,
	}

	filePath := filepath.Join(tmpDir, "test.txt")

	// Generate file
	err = generateFile(filePath, template, data)
	if err != nil {
		t.Fatalf("Failed to generate file: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	expected := `Service: test-service
WithGRPC: true`

	if string(content) != expected {
		t.Errorf("Generated content doesn't match expected.\nGot:\n%s\nExpected:\n%s", string(content), expected)
	}
}
