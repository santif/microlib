package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateServiceStructure(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "microlib-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set test values
	serviceName = "test-integration"
	serviceType = "api"
	withGRPC = true
	withJobs = true
	withCache = true
	withAuth = false

	serviceDir := filepath.Join(tmpDir, serviceName)

	// For testing, use the legacy template generation approach
	// since embedded templates may not be available during testing
	err = generateServiceStructureLegacy(serviceDir)
	if err != nil {
		t.Fatalf("Failed to generate service structure: %v", err)
	}

	// Check that required files exist (using legacy template structure)
	expectedFiles := []string{
		"go.mod",
		"main.go",
		"cmd/server.go",
		"internal/config/config.go",
		"internal/handlers/health.go",
		"internal/handlers/handlers.go",
		"Makefile",
		"Dockerfile",
		"docker-compose.yml",
		".gitignore",
		"README.md",
		"config.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(serviceDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}

	// Check that optional files exist when flags are set
	if withGRPC {
		grpcFile := filepath.Join(serviceDir, "internal/handlers/grpc.go")
		if _, err := os.Stat(grpcFile); os.IsNotExist(err) {
			t.Errorf("Expected gRPC file does not exist when --grpc flag is set")
		}
	}

	if withJobs {
		jobsFile := filepath.Join(serviceDir, "internal/jobs/jobs.go")
		if _, err := os.Stat(jobsFile); os.IsNotExist(err) {
			t.Errorf("Expected jobs file does not exist when --jobs flag is set")
		}
	}

	// Verify go.mod content
	goModPath := filepath.Join(serviceDir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	expectedModule := "test-integration-service"
	if !contains(string(content), expectedModule) {
		t.Errorf("go.mod does not contain expected module name %s", expectedModule)
	}

	// Verify main.go content
	mainPath := filepath.Join(serviceDir, "main.go")
	content, err = os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !contains(string(content), expectedModule+"/cmd") {
		t.Errorf("main.go does not contain expected import %s/cmd", expectedModule)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
