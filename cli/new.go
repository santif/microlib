package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var (
	serviceName string
	outputDir   string
	withGRPC    bool
	withJobs    bool
	withCache   bool
	withAuth    bool
)

func init() {
	// New command
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Create new MicroLib components",
		Long:  `Commands for creating new services, handlers, and other components using MicroLib templates.`,
	}

	// New service command
	serviceCmd := &cobra.Command{
		Use:   "service [name]",
		Short: "Create a new MicroLib service",
		Long: `Create a new MicroLib service with complete project structure and best practices.
This command generates a fully functional service with:
- Core service setup with graceful shutdown
- Configuration management
- Observability (logging, metrics, tracing, health checks)
- HTTP server with middleware
- Optional gRPC server
- Optional job scheduling
- Optional caching
- Optional authentication
- Database integration
- Docker configuration
- Makefile with common tasks`,
		Args: cobra.ExactArgs(1),
		Run:  createService,
	}

	// Add flags
	serviceCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the new service")
	serviceCmd.Flags().BoolVar(&withGRPC, "grpc", false, "Include gRPC server setup")
	serviceCmd.Flags().BoolVar(&withJobs, "jobs", false, "Include job scheduling setup")
	serviceCmd.Flags().BoolVar(&withCache, "cache", false, "Include Redis cache setup")
	serviceCmd.Flags().BoolVar(&withAuth, "auth", false, "Include JWT authentication setup")

	// Add commands
	newCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(newCmd)
}

// createService creates a new MicroLib service
func createService(cmd *cobra.Command, args []string) {
	serviceName = args[0]

	// Validate service name
	if !isValidServiceName(serviceName) {
		fmt.Fprintf(os.Stderr, "Error: Service name must contain only letters, numbers, and hyphens\n")
		os.Exit(1)
	}

	// Create service directory
	serviceDir := filepath.Join(outputDir, serviceName)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating service directory: %v\n", err)
		os.Exit(1)
	}

	// Generate service files
	if err := generateServiceStructure(serviceDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating service structure: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Service '%s' created successfully in %s\n", serviceName, serviceDir)
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", serviceName)
	fmt.Println("  make deps     # Install dependencies")
	fmt.Println("  make build    # Build the service")
	fmt.Println("  make test     # Run tests")
	fmt.Println("  make run      # Run the service")
}

// generateServiceStructure creates the complete service structure
func generateServiceStructure(serviceDir string) error {
	// Create directory structure
	dirs := []string{
		"cmd",
		"internal/config",
		"internal/handlers",
		"internal/models",
		"internal/services",
		"internal/repositories",
		"migrations",
		"seeds",
		"api",
		"docs",
		"scripts",
		"deployments",
	}

	// Add optional directories based on flags
	if withJobs {
		dirs = append(dirs, "internal/jobs")
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(serviceDir, dir), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Generate files
	files := []struct {
		path     string
		template string
		data     interface{}
	}{
		{"go.mod", goModTemplate, getTemplateData()},
		{"main.go", mainTemplate, getTemplateData()},
		{"cmd/server.go", serverTemplate, getTemplateData()},
		{"internal/config/config.go", configTemplate, getTemplateData()},
		{"internal/handlers/health.go", healthHandlerTemplate, getTemplateData()},
		{"internal/handlers/handlers.go", handlersTemplate, getTemplateData()},
		{"Makefile", makefileTemplate, getTemplateData()},
		{"Dockerfile", dockerfileTemplate, getTemplateData()},
		{"docker-compose.yml", dockerComposeTemplate, getTemplateData()},
		{".gitignore", gitignoreTemplate, getTemplateData()},
		{"README.md", readmeTemplate, getTemplateData()},
		{"config.yaml", configYamlTemplate, getTemplateData()},
	}

	// Add optional files based on flags
	if withGRPC {
		files = append(files,
			struct {
				path     string
				template string
				data     interface{}
			}{"internal/handlers/grpc.go", grpcHandlerTemplate, getTemplateData()},
		)
	}

	if withJobs {
		files = append(files,
			struct {
				path     string
				template string
				data     interface{}
			}{"internal/jobs/jobs.go", jobsTemplate, getTemplateData()},
		)
	}

	// Generate each file
	for _, file := range files {
		if err := generateFile(filepath.Join(serviceDir, file.path), file.template, file.data); err != nil {
			return fmt.Errorf("generating file %s: %w", file.path, err)
		}
	}

	return nil
}

// generateFile creates a file from a template
func generateFile(filePath, templateStr string, data interface{}) error {
	tmpl, err := template.New("file").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	return nil
}

// getTemplateData returns data for template execution
func getTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"ServiceName":    serviceName,
		"ServiceNameCap": strings.Title(serviceName),
		"ModuleName":     fmt.Sprintf("%s-service", serviceName),
		"WithGRPC":       withGRPC,
		"WithJobs":       withJobs,
		"WithCache":      withCache,
		"WithAuth":       withAuth,
	}
}

// isValidServiceName validates the service name
func isValidServiceName(name string) bool {
	if len(name) == 0 {
		return false
	}

	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}

	return true
}
