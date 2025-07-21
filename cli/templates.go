package cli

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// ServiceType represents different types of services that can be generated
type ServiceType string

const (
	ServiceTypeAPI       ServiceType = "api"
	ServiceTypeWorker    ServiceType = "worker"
	ServiceTypeScheduler ServiceType = "scheduler"
	ServiceTypeGateway   ServiceType = "gateway"
	ServiceTypeEvent     ServiceType = "event"
)

// TemplateData contains all data needed for template generation
type TemplateData struct {
	ServiceName    string
	ServiceNameCap string
	ServiceType    ServiceType
	ModuleName     string
	Author         string
	License        string
	Organization   string
	GoVersion      string
	WithGRPC       bool
	WithJobs       bool
	WithCache      bool
	WithAuth       bool
	WithMessaging  bool
	WithMetrics    bool
	WithTracing    bool
	CustomVars     map[string]string
}

// ServiceTemplate represents a complete service template
type ServiceTemplate struct {
	Type        ServiceType
	Name        string
	Description string
	Files       []TemplateFile
	Features    []string
}

// TemplateFile represents a file to be generated from a template
type TemplateFile struct {
	Path         string
	TemplatePath string
	Condition    func(TemplateData) bool
}

// GetServiceTemplates returns all available service templates
func GetServiceTemplates() []ServiceTemplate {
	return []ServiceTemplate{
		{
			Type:        ServiceTypeAPI,
			Name:        "REST API Service",
			Description: "A complete REST API service with HTTP endpoints, database, and observability",
			Features:    []string{"HTTP Server", "Database", "Observability", "Health Checks", "OpenAPI"},
			Files:       getAPIServiceFiles(),
		},
		{
			Type:        ServiceTypeWorker,
			Name:        "Background Worker Service",
			Description: "A worker service for processing background jobs and tasks",
			Features:    []string{"Job Queue", "Database", "Observability", "Health Checks"},
			Files:       getWorkerServiceFiles(),
		},
		{
			Type:        ServiceTypeScheduler,
			Name:        "Scheduled Task Service",
			Description: "A service for running scheduled tasks and cron jobs",
			Features:    []string{"Cron Scheduler", "Database", "Observability", "Health Checks"},
			Files:       getSchedulerServiceFiles(),
		},
		{
			Type:        ServiceTypeGateway,
			Name:        "API Gateway Service",
			Description: "An API gateway with routing, authentication, and rate limiting",
			Features:    []string{"HTTP Server", "gRPC Client", "Authentication", "Rate Limiting", "Observability"},
			Files:       getGatewayServiceFiles(),
		},
		{
			Type:        ServiceTypeEvent,
			Name:        "Event-Driven Service",
			Description: "A service for handling events with messaging and outbox pattern",
			Features:    []string{"Messaging", "Outbox Pattern", "Database", "Observability", "Health Checks"},
			Files:       getEventServiceFiles(),
		},
	}
}

// getAPIServiceFiles returns files for API service template
func getAPIServiceFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "go.mod", TemplatePath: "common/go.mod.tmpl"},
		{Path: "main.go", TemplatePath: "common/main.go.tmpl"},
		{Path: "cmd/server.go", TemplatePath: "api/cmd/server.go.tmpl"},
		{Path: "internal/config/config.go", TemplatePath: "api/internal/config/config.go.tmpl"},
		{Path: "internal/handlers/handlers.go", TemplatePath: "api/internal/handlers/handlers.go.tmpl"},
		{Path: "internal/handlers/health.go", TemplatePath: "api/internal/handlers/health.go.tmpl"},
		{Path: "internal/handlers/users.go", TemplatePath: "api/internal/handlers/users.go.tmpl"},
		{Path: "internal/models/user.go", TemplatePath: "api/internal/models/user.go.tmpl"},
		{Path: "internal/services/user_service.go", TemplatePath: "api/internal/services/user_service.go.tmpl"},
		{Path: "internal/repositories/user_repository.go", TemplatePath: "api/internal/repositories/user_repository.go.tmpl"},
		{Path: "migrations/001_create_users_table.sql", TemplatePath: "api/migrations/001_create_users_table.sql.tmpl"},
		{Path: "api/openapi.yaml", TemplatePath: "api/api/openapi.yaml.tmpl"},
		{Path: "Makefile", TemplatePath: "common/Makefile.tmpl"},
		{Path: "Dockerfile", TemplatePath: "common/Dockerfile.tmpl"},
		{Path: "docker-compose.yml", TemplatePath: "common/docker-compose.yml.tmpl"},
		{Path: ".gitignore", TemplatePath: "common/.gitignore.tmpl"},
		{Path: "README.md", TemplatePath: "api/README.md.tmpl"},
		{Path: "config.yaml", TemplatePath: "api/config.yaml.tmpl"},
		{Path: "internal/handlers/grpc.go", TemplatePath: "api/internal/handlers/grpc.go.tmpl", Condition: func(td TemplateData) bool { return td.WithGRPC }},
	}
}

// getWorkerServiceFiles returns files for worker service template
func getWorkerServiceFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "go.mod", TemplatePath: "common/go.mod.tmpl"},
		{Path: "main.go", TemplatePath: "common/main.go.tmpl"},
		{Path: "cmd/worker.go", TemplatePath: "worker/cmd/worker.go.tmpl"},
		{Path: "internal/config/config.go", TemplatePath: "worker/internal/config/config.go.tmpl"},
		{Path: "internal/handlers/health.go", TemplatePath: "common/internal/handlers/health.go.tmpl"},
		{Path: "internal/jobs/email_job.go", TemplatePath: "worker/internal/jobs/email_job.go.tmpl"},
		{Path: "internal/jobs/image_processor.go", TemplatePath: "worker/internal/jobs/image_processor.go.tmpl"},
		{Path: "internal/services/notification_service.go", TemplatePath: "worker/internal/services/notification_service.go.tmpl"},
		{Path: "Makefile", TemplatePath: "common/Makefile.tmpl"},
		{Path: "Dockerfile", TemplatePath: "common/Dockerfile.tmpl"},
		{Path: "docker-compose.yml", TemplatePath: "worker/docker-compose.yml.tmpl"},
		{Path: ".gitignore", TemplatePath: "common/.gitignore.tmpl"},
		{Path: "README.md", TemplatePath: "worker/README.md.tmpl"},
		{Path: "config.yaml", TemplatePath: "worker/config.yaml.tmpl"},
	}
}

// getSchedulerServiceFiles returns files for scheduler service template
func getSchedulerServiceFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "go.mod", TemplatePath: "common/go.mod.tmpl"},
		{Path: "main.go", TemplatePath: "common/main.go.tmpl"},
		{Path: "cmd/scheduler.go", TemplatePath: "scheduler/cmd/scheduler.go.tmpl"},
		{Path: "internal/config/config.go", TemplatePath: "scheduler/internal/config/config.go.tmpl"},
		{Path: "internal/handlers/health.go", TemplatePath: "common/internal/handlers/health.go.tmpl"},
		{Path: "internal/jobs/cleanup_job.go", TemplatePath: "scheduler/internal/jobs/cleanup_job.go.tmpl"},
		{Path: "internal/jobs/report_job.go", TemplatePath: "scheduler/internal/jobs/report_job.go.tmpl"},
		{Path: "internal/jobs/backup_job.go", TemplatePath: "scheduler/internal/jobs/backup_job.go.tmpl"},
		{Path: "Makefile", TemplatePath: "common/Makefile.tmpl"},
		{Path: "Dockerfile", TemplatePath: "common/Dockerfile.tmpl"},
		{Path: "docker-compose.yml", TemplatePath: "scheduler/docker-compose.yml.tmpl"},
		{Path: ".gitignore", TemplatePath: "common/.gitignore.tmpl"},
		{Path: "README.md", TemplatePath: "scheduler/README.md.tmpl"},
		{Path: "config.yaml", TemplatePath: "scheduler/config.yaml.tmpl"},
	}
}

// getGatewayServiceFiles returns files for gateway service template
func getGatewayServiceFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "go.mod", TemplatePath: "common/go.mod.tmpl"},
		{Path: "main.go", TemplatePath: "common/main.go.tmpl"},
		{Path: "cmd/gateway.go", TemplatePath: "gateway/cmd/gateway.go.tmpl"},
		{Path: "internal/config/config.go", TemplatePath: "gateway/internal/config/config.go.tmpl"},
		{Path: "internal/handlers/handlers.go", TemplatePath: "gateway/internal/handlers/handlers.go.tmpl"},
		{Path: "internal/handlers/health.go", TemplatePath: "common/internal/handlers/health.go.tmpl"},
		{Path: "internal/handlers/proxy.go", TemplatePath: "gateway/internal/handlers/proxy.go.tmpl"},
		{Path: "internal/middleware/auth.go", TemplatePath: "gateway/internal/middleware/auth.go.tmpl"},
		{Path: "internal/middleware/rate_limit.go", TemplatePath: "gateway/internal/middleware/rate_limit.go.tmpl"},
		{Path: "internal/services/routing_service.go", TemplatePath: "gateway/internal/services/routing_service.go.tmpl"},
		{Path: "Makefile", TemplatePath: "common/Makefile.tmpl"},
		{Path: "Dockerfile", TemplatePath: "common/Dockerfile.tmpl"},
		{Path: "docker-compose.yml", TemplatePath: "gateway/docker-compose.yml.tmpl"},
		{Path: ".gitignore", TemplatePath: "common/.gitignore.tmpl"},
		{Path: "README.md", TemplatePath: "gateway/README.md.tmpl"},
		{Path: "config.yaml", TemplatePath: "gateway/config.yaml.tmpl"},
	}
}

// getEventServiceFiles returns files for event service template
func getEventServiceFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "go.mod", TemplatePath: "common/go.mod.tmpl"},
		{Path: "main.go", TemplatePath: "common/main.go.tmpl"},
		{Path: "cmd/event.go", TemplatePath: "event/cmd/event.go.tmpl"},
		{Path: "internal/config/config.go", TemplatePath: "event/internal/config/config.go.tmpl"},
		{Path: "internal/handlers/handlers.go", TemplatePath: "event/internal/handlers/handlers.go.tmpl"},
		{Path: "internal/handlers/health.go", TemplatePath: "common/internal/handlers/health.go.tmpl"},
		{Path: "internal/handlers/events.go", TemplatePath: "event/internal/handlers/events.go.tmpl"},
		{Path: "internal/events/user_events.go", TemplatePath: "event/internal/events/user_events.go.tmpl"},
		{Path: "internal/events/order_events.go", TemplatePath: "event/internal/events/order_events.go.tmpl"},
		{Path: "internal/services/event_service.go", TemplatePath: "event/internal/services/event_service.go.tmpl"},
		{Path: "migrations/001_create_outbox_table.sql", TemplatePath: "event/migrations/001_create_outbox_table.sql.tmpl"},
		{Path: "api/asyncapi.yaml", TemplatePath: "event/api/asyncapi.yaml.tmpl"},
		{Path: "Makefile", TemplatePath: "common/Makefile.tmpl"},
		{Path: "Dockerfile", TemplatePath: "common/Dockerfile.tmpl"},
		{Path: "docker-compose.yml", TemplatePath: "event/docker-compose.yml.tmpl"},
		{Path: ".gitignore", TemplatePath: "common/.gitignore.tmpl"},
		{Path: "README.md", TemplatePath: "event/README.md.tmpl"},
		{Path: "config.yaml", TemplatePath: "event/config.yaml.tmpl"},
	}
}

// TemplateEngine handles template processing
type TemplateEngine struct {
	templates map[string]*template.Template
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() (*TemplateEngine, error) {
	engine := &TemplateEngine{
		templates: make(map[string]*template.Template),
	}

	// Load all templates from embedded filesystem
	if err := engine.loadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return engine, nil
}

// loadTemplates loads all templates from the embedded filesystem
func (te *TemplateEngine) loadTemplates() error {
	return fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", path, err)
		}

		// Create template with helper functions
		tmpl, err := template.New(path).Funcs(template.FuncMap{
			"title":   strings.Title,
			"lower":   strings.ToLower,
			"upper":   strings.ToUpper,
			"replace": strings.ReplaceAll,
		}).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", path, err)
		}

		te.templates[path] = tmpl
		return nil
	})
}

// RenderTemplate renders a template with the given data
func (te *TemplateEngine) RenderTemplate(templatePath string, data TemplateData) (string, error) {
	tmpl, exists := te.templates[templatePath]
	if !exists {
		return "", fmt.Errorf("template not found: %s", templatePath)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}

// GetAvailableTemplates returns a list of all available templates
func (te *TemplateEngine) GetAvailableTemplates() []string {
	var templates []string
	for path := range te.templates {
		templates = append(templates, path)
	}
	return templates
}

// Legacy templates for backward compatibility
// goModTemplate generates the go.mod file
const goModTemplate = `module {{.ModuleName}}

go 1.23

require (
	github.com/santif/microlib v0.1.0
)
`

// mainTemplate generates the main.go file
const mainTemplate = `package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"{{.ModuleName}}/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Run(ctx); err != nil {
		log.Fatalf("Service failed: %v", err)
	}
}
`

// serverTemplate generates the cmd/server.go file
const serverTemplate = `package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/santif/microlib/core"
	"github.com/santif/microlib/config"
	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/http"
	{{- if .WithGRPC}}
	"github.com/santif/microlib/grpc"
	{{- end}}
	{{- if .WithCache}}
	"github.com/santif/microlib/data"
	{{- end}}
	{{- if .WithJobs}}
	"github.com/santif/microlib/jobs"
	{{- end}}

	"{{.ModuleName}}/internal/config"
	"{{.ModuleName}}/internal/handlers"
	{{- if .WithJobs}}
	"{{.ModuleName}}/internal/jobs"
	{{- end}}
)

// Run starts the {{.ServiceName}} service
func Run(ctx context.Context) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Initialize logger
	logger := observability.NewLogger(cfg.Logging)
	slog.SetDefault(logger.Slog())

	// Initialize observability
	obs, err := observability.New(cfg.Observability)
	if err != nil {
		return fmt.Errorf("initializing observability: %w", err)
	}
	defer obs.Shutdown(ctx)

	// Initialize database
	db, err := data.NewPostgresDB(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close()

	{{- if .WithCache}}
	// Initialize cache
	cache, err := data.NewRedisCache(ctx, cfg.Cache)
	if err != nil {
		return fmt.Errorf("connecting to cache: %w", err)
	}
	defer cache.Close()
	{{- end}}

	// Create service metadata
	metadata := core.ServiceMetadata{
		Name:      "{{.ServiceName}}",
		Version:   "1.0.0",
		Instance:  cfg.Service.Instance,
		BuildHash: cfg.Service.BuildHash,
	}

	// Create service
	service := core.NewService(metadata)

	// Register dependencies for health checks
	service.RegisterDependency("database", db)
	{{- if .WithCache}}
	service.RegisterDependency("cache", cache)
	{{- end}}

	// Initialize handlers
	h := handlers.New(handlers.Config{
		Logger: logger,
		DB:     db,
		{{- if .WithCache}}
		Cache:  cache,
		{{- end}}
	})

	// Initialize HTTP server
	httpServer := http.NewServer(cfg.HTTP)
	h.RegisterHTTPRoutes(httpServer)

	{{- if .WithGRPC}}
	// Initialize gRPC server
	grpcServer := grpc.NewServer(cfg.GRPC)
	h.RegisterGRPCServices(grpcServer)
	{{- end}}

	{{- if .WithJobs}}
	// Initialize job scheduler
	scheduler := jobs.NewScheduler(cfg.Jobs)
	jobHandlers := jobs.New(jobs.Config{
		Logger: logger,
		DB:     db,
	})
	jobHandlers.RegisterJobs(scheduler)
	{{- end}}

	// Start service
	return service.Run(ctx, func(ctx context.Context) error {
		// Start HTTP server
		go func() {
			if err := httpServer.Start(ctx); err != nil {
				logger.Error("HTTP server failed", "error", err)
			}
		}()

		{{- if .WithGRPC}}
		// Start gRPC server
		go func() {
			if err := grpcServer.Start(ctx); err != nil {
				logger.Error("gRPC server failed", "error", err)
			}
		}()
		{{- end}}

		{{- if .WithJobs}}
		// Start job scheduler
		go func() {
			if err := scheduler.Start(ctx); err != nil {
				logger.Error("Job scheduler failed", "error", err)
			}
		}()
		{{- end}}

		// Wait for context cancellation
		<-ctx.Done()

		// Graceful shutdown
		{{- if .WithJobs}}
		scheduler.Stop(ctx)
		{{- end}}
		{{- if .WithGRPC}}
		grpcServer.Shutdown(ctx)
		{{- end}}
		httpServer.Shutdown(ctx)

		return nil
	})
}

// loadConfig loads the service configuration
func loadConfig() (*config.Config, error) {
	manager := config.NewManager()
	
	// Add configuration sources in priority order
	manager.AddSource(config.NewEnvSource())
	manager.AddSource(config.NewFileSource("config.yaml"))
	
	var cfg config.Config
	if err := manager.Load(&cfg); err != nil {
		return nil, err
	}
	
	if err := manager.Validate(&cfg); err != nil {
		return nil, err
	}
	
	return &cfg, nil
}
`

// configTemplate generates the internal/config/config.go file
const configTemplate = `package config

import (
	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/http"
	{{- if .WithGRPC}}
	"github.com/santif/microlib/grpc"
	{{- end}}
	"github.com/santif/microlib/data"
	{{- if .WithCache}}
	"github.com/santif/microlib/data"
	{{- end}}
	{{- if .WithJobs}}
	"github.com/santif/microlib/jobs"
	{{- end}}
)

// Config represents the service configuration
type Config struct {
	Service       ServiceConfig              ` + "`yaml:\"service\" validate:\"required\"`" + `
	HTTP          http.Config                ` + "`yaml:\"http\" validate:\"required\"`" + `
	{{- if .WithGRPC}}
	GRPC          grpc.Config                ` + "`yaml:\"grpc\" validate:\"required\"`" + `
	{{- end}}
	Database      data.PostgresConfig        ` + "`yaml:\"database\" validate:\"required\"`" + `
	{{- if .WithCache}}
	Cache         data.RedisConfig           ` + "`yaml:\"cache\" validate:\"required\"`" + `
	{{- end}}
	{{- if .WithJobs}}
	Jobs          jobs.Config                ` + "`yaml:\"jobs\" validate:\"required\"`" + `
	{{- end}}
	Logging       observability.LogConfig    ` + "`yaml:\"logging\" validate:\"required\"`" + `
	Observability observability.Config       ` + "`yaml:\"observability\" validate:\"required\"`" + `
}

// ServiceConfig contains service-specific configuration
type ServiceConfig struct {
	Instance  string ` + "`yaml:\"instance\" validate:\"required\"`" + `
	BuildHash string ` + "`yaml:\"build_hash\"`" + `
}
`

// healthHandlerTemplate generates the health handler
const healthHandlerTemplate = `package handlers

import (
	"net/http"
	"encoding/json"
)

// HealthResponse represents a health check response
type HealthResponse struct {
	Status  string            ` + "`json:\"status\"`" + `
	Service string            ` + "`json:\"service\"`" + `
	Version string            ` + "`json:\"version\"`" + `
	Checks  map[string]string ` + "`json:\"checks,omitempty\"`" + `
}

// HealthHandler handles health check requests
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:  "healthy",
		Service: "{{.ServiceName}}",
		Version: "1.0.0",
		Checks:  make(map[string]string),
	}

	// Add dependency checks
	if h.DB != nil {
		if err := h.DB.Ping(r.Context()); err != nil {
			response.Status = "unhealthy"
			response.Checks["database"] = "unhealthy: " + err.Error()
		} else {
			response.Checks["database"] = "healthy"
		}
	}

	{{- if .WithCache}}
	if h.Cache != nil {
		if err := h.Cache.Ping(r.Context()); err != nil {
			response.Status = "unhealthy"
			response.Checks["cache"] = "unhealthy: " + err.Error()
		} else {
			response.Checks["cache"] = "healthy"
		}
	}
	{{- end}}

	w.Header().Set("Content-Type", "application/json")
	if response.Status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(response)
}
`

// handlersTemplate generates the main handlers file
const handlersTemplate = `package handlers

import (
	"log/slog"
	"net/http"

	"github.com/santif/microlib/data"
	httplib "github.com/santif/microlib/http"
	{{- if .WithGRPC}}
	grpclib "github.com/santif/microlib/grpc"
	{{- end}}
)

// Handlers contains all HTTP and gRPC handlers
type Handlers struct {
	logger *slog.Logger
	DB     data.Database
	{{- if .WithCache}}
	Cache  data.Cache
	{{- end}}
}

// Config contains handler dependencies
type Config struct {
	Logger *slog.Logger
	DB     data.Database
	{{- if .WithCache}}
	Cache  data.Cache
	{{- end}}
}

// New creates a new handlers instance
func New(cfg Config) *Handlers {
	return &Handlers{
		logger: cfg.Logger,
		DB:     cfg.DB,
		{{- if .WithCache}}
		Cache:  cfg.Cache,
		{{- end}}
	}
}

// RegisterHTTPRoutes registers all HTTP routes
func (h *Handlers) RegisterHTTPRoutes(server httplib.Server) {
	// Health endpoints
	server.RegisterHandler("/health", http.HandlerFunc(h.HealthHandler))
	server.RegisterHandler("/health/live", http.HandlerFunc(h.LivenessHandler))
	server.RegisterHandler("/health/ready", http.HandlerFunc(h.ReadinessHandler))

	// API routes
	server.RegisterHandler("/api/v1/ping", http.HandlerFunc(h.PingHandler))
	
	// Add your custom routes here
}

{{- if .WithGRPC}}
// RegisterGRPCServices registers all gRPC services
func (h *Handlers) RegisterGRPCServices(server grpclib.Server) {
	// Register your gRPC services here
}
{{- end}}

// LivenessHandler handles liveness probe requests
func (h *Handlers) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ReadinessHandler handles readiness probe requests
func (h *Handlers) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	// Check if service is ready to accept traffic
	// This could include checking database connections, etc.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

// PingHandler handles ping requests
func (h *Handlers) PingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(` + "`" + `{"message": "pong", "service": "{{.ServiceName}}"}` + "`" + `))
}
`

// grpcHandlerTemplate generates gRPC handler when --grpc flag is used
const grpcHandlerTemplate = `package handlers

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Example gRPC service implementation
// Replace this with your actual gRPC service definitions

// PingGRPC handles gRPC ping requests
func (h *Handlers) PingGRPC(ctx context.Context, req *PingRequest) (*PingResponse, error) {
	h.logger.Info("Received gRPC ping request")
	
	return &PingResponse{
		Message: "pong",
		Service: "{{.ServiceName}}",
	}, nil
}

// Add your gRPC service methods here
`

// jobsTemplate generates job handlers when --jobs flag is used
const jobsTemplate = `package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/santif/microlib/data"
	"github.com/santif/microlib/jobs"
)

// JobHandlers contains all job handlers
type JobHandlers struct {
	logger *slog.Logger
	DB     data.Database
}

// Config contains job handler dependencies
type Config struct {
	Logger *slog.Logger
	DB     data.Database
}

// New creates a new job handlers instance
func New(cfg Config) *JobHandlers {
	return &JobHandlers{
		logger: cfg.Logger,
		DB:     cfg.DB,
	}
}

// RegisterJobs registers all scheduled jobs
func (j *JobHandlers) RegisterJobs(scheduler jobs.Scheduler) {
	// Example: Run cleanup job every hour
	scheduler.Schedule("0 * * * *", jobs.JobFunc(j.CleanupJob))
	
	// Example: Run daily report job at midnight
	scheduler.Schedule("0 0 * * *", jobs.JobFunc(j.DailyReportJob))
	
	// Add your scheduled jobs here
}

// CleanupJob performs cleanup tasks
func (j *JobHandlers) CleanupJob(ctx context.Context) error {
	j.logger.Info("Running cleanup job")
	
	// Add your cleanup logic here
	// Example: Delete old records, clean up temporary files, etc.
	
	return nil
}

// DailyReportJob generates daily reports
func (j *JobHandlers) DailyReportJob(ctx context.Context) error {
	j.logger.Info("Running daily report job")
	
	// Add your report generation logic here
	
	return nil
}
`

// makefileTemplate generates the Makefile
const makefileTemplate = `.PHONY: help build test run clean deps lint format docker-build docker-run

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build the application
build: ## Build the application
	@echo "Building {{.ServiceName}}..."
	@go build -o bin/{{.ServiceName}} .

# Install dependencies
deps: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Run tests
test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

# Run the application
run: ## Run the application
	@echo "Running {{.ServiceName}}..."
	@go run .

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

# Lint the code
lint: ## Lint the code
	@echo "Linting..."
	@golangci-lint run

# Format the code
format: ## Format the code
	@echo "Formatting..."
	@go fmt ./...
	@goimports -w .

# Build Docker image
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t {{.ServiceName}}:latest .

# Run with Docker Compose
docker-run: ## Run with Docker Compose
	@echo "Starting with Docker Compose..."
	@docker-compose up --build

# Stop Docker Compose
docker-stop: ## Stop Docker Compose
	@echo "Stopping Docker Compose..."
	@docker-compose down

# Database migrations
migrate-up: ## Apply database migrations
	@echo "Applying migrations..."
	@./bin/{{.ServiceName}} migration apply

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	@./bin/{{.ServiceName}} migration rollback

# Database seeding
seed: ## Apply database seeds
	@echo "Applying seeds..."
	@./bin/{{.ServiceName}} seed apply
`

// dockerfileTemplate generates the Dockerfile
const dockerfileTemplate = `# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/config.yaml .

# Change ownership to non-root user
RUN chown -R appuser:appuser /root/
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
`

// dockerComposeTemplate generates the docker-compose.yml
const dockerComposeTemplate = `version: '3.8'

services:
  {{.ServiceName}}:
    build: .
    ports:
      - "8080:8080"
      {{- if .WithGRPC}}
      - "9090:9090"
      {{- end}}
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=postgres
      - DB_PASSWORD=postgres
      - DB_NAME={{.ServiceName}}_db
      {{- if .WithCache}}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      {{- end}}
    depends_on:
      - postgres
      {{- if .WithCache}}
      - redis
      {{- end}}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB={{.ServiceName}}_db
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  {{- if .WithCache}}
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3
  {{- end}}

volumes:
  postgres_data:
  {{- if .WithCache}}
  redis_data:
  {{- end}}
`

// gitignoreTemplate generates the .gitignore file
const gitignoreTemplate = `# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/
dist/

# Test binary, built with ` + "`go test -c`" + `
*.test

# Output of the go coverage tool
*.out
coverage.html

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS generated files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# Environment files
.env
.env.local
.env.*.local

# Log files
*.log
logs/

# Temporary files
tmp/
temp/

# Database files
*.db
*.sqlite
*.sqlite3

# Configuration files with secrets
config.local.yaml
config.prod.yaml
secrets.yaml
`

// readmeTemplate generates the README.md file
const readmeTemplate = `# {{.ServiceNameCap}} Service

A MicroLib-based microservice for {{.ServiceName}}.

## Features

- ✅ HTTP REST API with middleware stack
{{- if .WithGRPC}}
- ✅ gRPC server with interceptors
{{- end}}
- ✅ Structured logging with slog
- ✅ Prometheus metrics
- ✅ OpenTelemetry tracing
- ✅ Health checks (liveness, readiness, startup)
- ✅ Graceful shutdown
- ✅ Database integration (PostgreSQL)
{{- if .WithCache}}
- ✅ Redis caching
{{- end}}
{{- if .WithJobs}}
- ✅ Job scheduling and processing
{{- end}}
{{- if .WithAuth}}
- ✅ JWT authentication
{{- end}}
- ✅ Configuration management
- ✅ Docker support
- ✅ Database migrations and seeding

## Quick Start

### Prerequisites

- Go 1.23 or later
- PostgreSQL
{{- if .WithCache}}
- Redis
{{- end}}
- Docker (optional)

### Development Setup

1. **Clone and setup**
   ` + "```bash" + `
   git clone <your-repo>
   cd {{.ServiceName}}
   make deps
   ` + "```" + `

2. **Start dependencies**
   ` + "```bash" + `
   docker-compose up postgres{{if .WithCache}} redis{{end}} -d
   ` + "```" + `

3. **Run migrations**
   ` + "```bash" + `
   make build
   make migrate-up
   ` + "```" + `

4. **Start the service**
   ` + "```bash" + `
   make run
   ` + "```" + `

### Docker Setup

` + "```bash" + `
# Build and run everything
make docker-run

# Or manually
docker-compose up --build
` + "```" + `

## API Endpoints

### Health Checks
- ` + "`GET /health`" + ` - Overall health status
- ` + "`GET /health/live`" + ` - Liveness probe
- ` + "`GET /health/ready`" + ` - Readiness probe

### API
- ` + "`GET /api/v1/ping`" + ` - Simple ping endpoint

### Observability
- ` + "`GET /metrics`" + ` - Prometheus metrics

## Configuration

Configuration is loaded from multiple sources in order of priority:
1. Environment variables
2. ` + "`config.yaml`" + ` file
3. Command line flags

### Environment Variables

` + "```bash" + `
# Service
SERVICE_INSTANCE=instance-1
SERVICE_BUILD_HASH=abc123

# HTTP Server
HTTP_PORT=8080
HTTP_HOST=0.0.0.0

{{- if .WithGRPC}}
# gRPC Server
GRPC_PORT=9090
GRPC_HOST=0.0.0.0
{{- end}}

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME={{.ServiceName}}_db

{{- if .WithCache}}
# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
{{- end}}

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
` + "```" + `

## Development

### Available Commands

` + "```bash" + `
make help          # Show available commands
make build         # Build the application
make test          # Run tests with coverage
make run           # Run the application
make lint          # Lint the code
make format        # Format the code
make clean         # Clean build artifacts
make docker-build  # Build Docker image
make docker-run    # Run with Docker Compose
` + "```" + `

### Project Structure

` + "```" + `
{{.ServiceName}}/
├── cmd/                 # Application commands
├── internal/
│   ├── config/         # Configuration structures
│   ├── handlers/       # HTTP/gRPC handlers
│   ├── models/         # Data models
│   ├── services/       # Business logic
│   └── repositories/   # Data access layer
├── migrations/         # Database migrations
├── seeds/             # Database seeds
├── api/               # API specifications
├── docs/              # Documentation
├── scripts/           # Build and deployment scripts
├── deployments/       # Kubernetes/Docker configs
├── main.go           # Application entry point
├── config.yaml       # Default configuration
├── Dockerfile        # Docker image definition
├── docker-compose.yml # Local development setup
└── Makefile          # Build automation
` + "```" + `

### Adding New Features

1. **HTTP Endpoints**: Add handlers in ` + "`internal/handlers/`" + `
2. **Business Logic**: Add services in ` + "`internal/services/`" + `
3. **Data Access**: Add repositories in ` + "`internal/repositories/`" + `
4. **Database Changes**: Create migrations in ` + "`migrations/`" + `
{{- if .WithJobs}}
5. **Background Jobs**: Add jobs in ` + "`internal/jobs/`" + `
{{- end}}

## Testing

` + "```bash" + `
# Run all tests
make test

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test -v ./internal/handlers

# Run tests with race detection
go test -race ./...
` + "```" + `

## Deployment

### Docker

` + "```bash" + `
# Build image
make docker-build

# Run with compose
make docker-run
` + "```" + `

### Kubernetes

See ` + "`deployments/`" + ` directory for Kubernetes manifests.

## Monitoring

- **Metrics**: Available at ` + "`/metrics`" + ` endpoint (Prometheus format)
- **Health**: Available at ` + "`/health`" + ` endpoints
- **Tracing**: OpenTelemetry traces sent to configured endpoint
- **Logs**: Structured JSON logs with trace correlation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run ` + "`make test lint`" + `
6. Submit a pull request

## License

[Your License Here]
`

// configYamlTemplate generates the config.yaml file
const configYamlTemplate = `# {{.ServiceNameCap}} Service Configuration

service:
  instance: "{{.ServiceName}}-1"
  build_hash: "dev"

http:
  port: 8080
  host: "0.0.0.0"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"

{{- if .WithGRPC}}
grpc:
  port: 9090
  host: "0.0.0.0"
  max_recv_msg_size: 4194304  # 4MB
  max_send_msg_size: 4194304  # 4MB
{{- end}}

database:
  host: "localhost"
  port: 5432
  username: "postgres"
  password: "postgres"
  database: "{{.ServiceName}}_db"
  ssl_mode: "disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "5m"

{{- if .WithCache}}
cache:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  max_retries: 3
  pool_size: 10
{{- end}}

{{- if .WithJobs}}
jobs:
  enabled: true
  worker_count: 5
  queue_size: 1000
  retry_attempts: 3
  retry_delay: "1s"
{{- end}}

logging:
  level: "info"
  format: "json"
  add_source: false

observability:
  service_name: "{{.ServiceName}}"
  service_version: "1.0.0"
  metrics:
    enabled: true
    port: 8080
    path: "/metrics"
  tracing:
    enabled: true
    endpoint: "http://localhost:14268/api/traces"
    sampler_ratio: 1.0
  health:
    enabled: true
    startup_timeout: "30s"
    readiness_timeout: "5s"
    liveness_timeout: "5s"
`
