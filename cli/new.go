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
	serviceName   string
	serviceType   string
	outputDir     string
	withGRPC      bool
	withJobs      bool
	withCache     bool
	withAuth      bool
	withMessaging bool
)

func init() {
	// New command
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Create new MicroLib components",
		Long:  `Commands for creating new services, handlers, and other components using MicroLib templates.`,
	}

	// List service types command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available service types",
		Long:  `List all available service types with their descriptions and features.`,
		Run:   listServiceTypes,
	}

	// New service command
	serviceCmd := &cobra.Command{
		Use:   "service [name]",
		Short: "Create a new MicroLib service",
		Long: `Create a new MicroLib service with complete project structure and best practices.

Available service types:
- api: REST API service with HTTP endpoints, database, and observability
- worker: Background worker service for processing jobs and tasks
- scheduler: Scheduled task service for running cron jobs
- gateway: API gateway with routing, authentication, and rate limiting
- event: Event-driven service with messaging and outbox pattern

This command generates a fully functional service with:
- Core service setup with graceful shutdown
- Configuration management
- Observability (logging, metrics, tracing, health checks)
- Service-specific components based on type
- Optional features (gRPC, caching, authentication, messaging)
- Database integration (where applicable)
- Docker configuration
- Comprehensive documentation
- Makefile with common tasks`,
		Args: cobra.ExactArgs(1),
		Run:  createService,
	}

	// Add flags
	serviceCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the new service")
	serviceCmd.Flags().StringVarP(&serviceType, "type", "t", "api", "Service type (api, worker, scheduler, gateway, event)")
	serviceCmd.Flags().BoolVar(&withGRPC, "grpc", false, "Include gRPC server setup")
	serviceCmd.Flags().BoolVar(&withJobs, "jobs", false, "Include job scheduling setup")
	serviceCmd.Flags().BoolVar(&withCache, "cache", false, "Include Redis cache setup")
	serviceCmd.Flags().BoolVar(&withAuth, "auth", false, "Include JWT authentication setup")
	serviceCmd.Flags().BoolVar(&withMessaging, "messaging", false, "Include messaging/event system")

	// Add commands
	newCmd.AddCommand(listCmd)
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

	// Load template customization
	configPath := filepath.Join(os.Getenv("HOME"), ".microlib", "templates.yaml")
	customization, err := LoadTemplateCustomization(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading template customization: %v\n", err)
		os.Exit(1)
	}

	// Validate service type
	if err := customization.ValidateServiceType(serviceType); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create service directory
	serviceDir := filepath.Join(outputDir, serviceName)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating service directory: %v\n", err)
		os.Exit(1)
	}

	// Generate service files
	if err := generateServiceStructure(serviceDir, customization); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating service structure: %v\n", err)
		os.Exit(1)
	}

	// Display success message with service info
	serviceInfo := customization.GetServiceTypeInfo()[serviceType]
	fmt.Printf("✅ Service '%s' created successfully in %s\n", serviceName, serviceDir)
	fmt.Printf("   Type: %s\n", serviceInfo.Description)
	fmt.Printf("   Features: %s\n", strings.Join(serviceInfo.Features, ", "))

	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", serviceName)
	fmt.Println("  make deps     # Install dependencies")
	fmt.Println("  make build    # Build the service")
	fmt.Println("  make test     # Run tests")
	fmt.Println("  make run      # Run the service")

	if serviceType == "api" {
		fmt.Println("\nAPI will be available at:")
		fmt.Println("  http://localhost:8080/health")
		fmt.Println("  http://localhost:8080/api/v1/ping")
	}
}

// generateServiceStructure creates the complete service structure
func generateServiceStructure(serviceDir string, customization *TemplateCustomization) error {
	// Initialize template engine
	engine, err := NewTemplateEngine()
	if err != nil {
		return fmt.Errorf("initializing template engine: %w", err)
	}

	// Get service template based on type
	templates := GetServiceTemplates()
	var selectedTemplate *ServiceTemplate
	for _, tmpl := range templates {
		if string(tmpl.Type) == serviceType {
			selectedTemplate = &tmpl
			break
		}
	}

	if selectedTemplate == nil {
		return fmt.Errorf("unknown service type: %s", serviceType)
	}

	// Create template data
	templateData := TemplateData{
		ServiceName:    serviceName,
		ServiceNameCap: strings.Title(serviceName),
		ServiceType:    ServiceType(serviceType),
		ModuleName:     fmt.Sprintf("%s-service", serviceName),
		Author:         customization.Global.Author,
		License:        customization.Global.License,
		Organization:   customization.Global.Organization,
		GoVersion:      customization.Global.GoVersion,
		WithGRPC:       withGRPC,
		WithJobs:       withJobs,
		WithCache:      withCache,
		WithAuth:       withAuth,
		WithMessaging:  withMessaging,
		WithMetrics:    true,
		WithTracing:    true,
		CustomVars:     make(map[string]string),
	}

	// Apply customization
	customization.ApplyCustomization(&templateData)

	// Create directories for all files
	createdDirs := make(map[string]bool)
	for _, file := range selectedTemplate.Files {
		// Skip conditional files that don't match current configuration
		if file.Condition != nil && !file.Condition(templateData) {
			continue
		}

		dir := filepath.Dir(filepath.Join(serviceDir, file.Path))
		if !createdDirs[dir] {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
			createdDirs[dir] = true
		}
	}

	// Generate files from templates
	for _, file := range selectedTemplate.Files {
		// Skip conditional files that don't match current configuration
		if file.Condition != nil && !file.Condition(templateData) {
			continue
		}

		content, err := engine.RenderTemplate(file.TemplatePath, templateData)
		if err != nil {
			return fmt.Errorf("rendering template %s: %w", file.TemplatePath, err)
		}

		filePath := filepath.Join(serviceDir, file.Path)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", filePath, err)
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

// generateServiceStructureLegacy creates service structure using legacy templates (for testing)
func generateServiceStructureLegacy(serviceDir string) error {
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

	// Generate files using legacy templates
	files := []struct {
		path     string
		template string
		data     interface{}
	}{
		{"go.mod", goModTemplate, getTemplateDataLegacy()},
		{"main.go", mainTemplate, getTemplateDataLegacy()},
		{"cmd/server.go", serverTemplate, getTemplateDataLegacy()},
		{"internal/config/config.go", configTemplate, getTemplateDataLegacy()},
		{"internal/handlers/health.go", healthHandlerTemplate, getTemplateDataLegacy()},
		{"internal/handlers/handlers.go", handlersTemplate, getTemplateDataLegacy()},
		{"Makefile", makefileTemplate, getTemplateDataLegacy()},
		{"Dockerfile", dockerfileTemplate, getTemplateDataLegacy()},
		{"docker-compose.yml", dockerComposeTemplate, getTemplateDataLegacy()},
		{".gitignore", gitignoreTemplate, getTemplateDataLegacy()},
		{"README.md", readmeTemplate, getTemplateDataLegacy()},
		{"config.yaml", configYamlTemplate, getTemplateDataLegacy()},
	}

	// Add optional files based on flags
	if withGRPC {
		files = append(files,
			struct {
				path     string
				template string
				data     interface{}
			}{"internal/handlers/grpc.go", grpcHandlerTemplate, getTemplateDataLegacy()},
		)
	}

	if withJobs {
		files = append(files,
			struct {
				path     string
				template string
				data     interface{}
			}{"internal/jobs/jobs.go", jobsTemplate, getTemplateDataLegacy()},
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

// getTemplateDataLegacy returns data for legacy template execution
func getTemplateDataLegacy() map[string]interface{} {
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

// listServiceTypes lists all available service types
func listServiceTypes(cmd *cobra.Command, args []string) {
	// Load template customization
	configPath := filepath.Join(os.Getenv("HOME"), ".microlib", "templates.yaml")
	customization, err := LoadTemplateCustomization(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading template customization: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Available MicroLib service types:")
	fmt.Println()

	serviceTypes := customization.GetServiceTypeInfo()
	for _, serviceType := range []string{"api", "worker", "scheduler", "gateway", "event"} {
		if info, exists := serviceTypes[serviceType]; exists {
			fmt.Printf("  %s\n", info.Name)
			fmt.Printf("    %s\n", info.Description)
			fmt.Printf("    Features: %s\n", strings.Join(info.Features, ", "))
			fmt.Println()
		}
	}

	fmt.Println("Usage:")
	fmt.Println("  microlib-cli new service <name> --type <type> [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --grpc        Include gRPC server setup")
	fmt.Println("  --jobs        Include job scheduling setup")
	fmt.Println("  --cache       Include Redis cache setup")
	fmt.Println("  --auth        Include JWT authentication setup")
	fmt.Println("  --messaging   Include messaging/event system")
}
