package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TemplateCustomization represents customization options for service templates
type TemplateCustomization struct {
	ServiceTypes map[string]ServiceTypeCustomization `yaml:"service_types"`
	Global       GlobalCustomization                 `yaml:"global"`
}

// ServiceTypeCustomization contains customization for a specific service type
type ServiceTypeCustomization struct {
	Description    string               `yaml:"description"`
	Features       []string             `yaml:"features"`
	DefaultOptions map[string]bool      `yaml:"default_options"`
	ExtraFiles     []CustomTemplateFile `yaml:"extra_files"`
	Variables      map[string]string    `yaml:"variables"`
}

// CustomTemplateFile represents an additional template file to generate
type CustomTemplateFile struct {
	Path         string            `yaml:"path"`
	TemplatePath string            `yaml:"template_path"`
	Conditions   map[string]bool   `yaml:"conditions"`
	Variables    map[string]string `yaml:"variables"`
}

// GlobalCustomization contains global template customization options
type GlobalCustomization struct {
	Author       string            `yaml:"author"`
	License      string            `yaml:"license"`
	Organization string            `yaml:"organization"`
	GoVersion    string            `yaml:"go_version"`
	Variables    map[string]string `yaml:"variables"`
}

// LoadTemplateCustomization loads template customization from a file
func LoadTemplateCustomization(configPath string) (*TemplateCustomization, error) {
	// Use default configuration if no file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return getDefaultTemplateCustomization(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading template config file: %w", err)
	}

	var config TemplateCustomization
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing template config: %w", err)
	}

	// Merge with defaults
	defaults := getDefaultTemplateCustomization()
	mergeTemplateCustomization(&config, defaults)

	return &config, nil
}

// SaveTemplateCustomization saves template customization to a file
func SaveTemplateCustomization(config *TemplateCustomization, configPath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling template config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing template config file: %w", err)
	}

	return nil
}

// getDefaultTemplateCustomization returns the default template customization
func getDefaultTemplateCustomization() *TemplateCustomization {
	return &TemplateCustomization{
		Global: GlobalCustomization{
			Author:       "Developer",
			License:      "MIT",
			Organization: "Your Organization",
			GoVersion:    "1.23",
			Variables:    make(map[string]string),
		},
		ServiceTypes: map[string]ServiceTypeCustomization{
			"api": {
				Description: "REST API service with HTTP endpoints, database, and observability",
				Features:    []string{"HTTP Server", "Database", "Observability", "Health Checks", "OpenAPI"},
				DefaultOptions: map[string]bool{
					"WithGRPC":      false,
					"WithJobs":      false,
					"WithCache":     true,
					"WithAuth":      true,
					"WithMessaging": false,
				},
				Variables: make(map[string]string),
			},
			"worker": {
				Description: "Background worker service for processing jobs and tasks",
				Features:    []string{"Job Queue", "Database", "Observability", "Health Checks"},
				DefaultOptions: map[string]bool{
					"WithGRPC":      false,
					"WithJobs":      true,
					"WithCache":     false,
					"WithAuth":      false,
					"WithMessaging": true,
				},
				Variables: make(map[string]string),
			},
			"scheduler": {
				Description: "Scheduled task service for running cron jobs",
				Features:    []string{"Cron Scheduler", "Database", "Observability", "Health Checks"},
				DefaultOptions: map[string]bool{
					"WithGRPC":      false,
					"WithJobs":      true,
					"WithCache":     false,
					"WithAuth":      false,
					"WithMessaging": false,
				},
				Variables: make(map[string]string),
			},
			"gateway": {
				Description: "API gateway with routing, authentication, and rate limiting",
				Features:    []string{"HTTP Server", "gRPC Client", "Authentication", "Rate Limiting", "Observability"},
				DefaultOptions: map[string]bool{
					"WithGRPC":      false,
					"WithJobs":      false,
					"WithCache":     true,
					"WithAuth":      true,
					"WithMessaging": false,
				},
				Variables: make(map[string]string),
			},
			"event": {
				Description: "Event-driven service with messaging and outbox pattern",
				Features:    []string{"Messaging", "Outbox Pattern", "Database", "Observability", "Health Checks"},
				DefaultOptions: map[string]bool{
					"WithGRPC":      false,
					"WithJobs":      false,
					"WithCache":     false,
					"WithAuth":      false,
					"WithMessaging": true,
				},
				Variables: make(map[string]string),
			},
		},
	}
}

// mergeTemplateCustomization merges user config with defaults
func mergeTemplateCustomization(user, defaults *TemplateCustomization) {
	// Merge global settings
	if user.Global.Author == "" {
		user.Global.Author = defaults.Global.Author
	}
	if user.Global.License == "" {
		user.Global.License = defaults.Global.License
	}
	if user.Global.Organization == "" {
		user.Global.Organization = defaults.Global.Organization
	}
	if user.Global.GoVersion == "" {
		user.Global.GoVersion = defaults.Global.GoVersion
	}

	// Merge global variables
	if user.Global.Variables == nil {
		user.Global.Variables = make(map[string]string)
	}
	for k, v := range defaults.Global.Variables {
		if _, exists := user.Global.Variables[k]; !exists {
			user.Global.Variables[k] = v
		}
	}

	// Merge service types
	if user.ServiceTypes == nil {
		user.ServiceTypes = make(map[string]ServiceTypeCustomization)
	}
	for serviceType, defaultConfig := range defaults.ServiceTypes {
		userConfig, exists := user.ServiceTypes[serviceType]
		if !exists {
			user.ServiceTypes[serviceType] = defaultConfig
			continue
		}

		// Merge individual service type settings
		if userConfig.Description == "" {
			userConfig.Description = defaultConfig.Description
		}
		if userConfig.Features == nil {
			userConfig.Features = defaultConfig.Features
		}
		if userConfig.DefaultOptions == nil {
			userConfig.DefaultOptions = defaultConfig.DefaultOptions
		} else {
			// Merge default options
			for k, v := range defaultConfig.DefaultOptions {
				if _, exists := userConfig.DefaultOptions[k]; !exists {
					userConfig.DefaultOptions[k] = v
				}
			}
		}
		if userConfig.Variables == nil {
			userConfig.Variables = make(map[string]string)
		}
		for k, v := range defaultConfig.Variables {
			if _, exists := userConfig.Variables[k]; !exists {
				userConfig.Variables[k] = v
			}
		}

		user.ServiceTypes[serviceType] = userConfig
	}
}

// ApplyCustomization applies customization to template data
func (tc *TemplateCustomization) ApplyCustomization(data *TemplateData) {
	// Apply global customization
	if tc.Global.Author != "" {
		data.Author = tc.Global.Author
	}
	if tc.Global.License != "" {
		data.License = tc.Global.License
	}
	if tc.Global.Organization != "" {
		data.Organization = tc.Global.Organization
	}
	if tc.Global.GoVersion != "" {
		data.GoVersion = tc.Global.GoVersion
	}

	// Apply global variables
	if data.CustomVars == nil {
		data.CustomVars = make(map[string]string)
	}
	for k, v := range tc.Global.Variables {
		data.CustomVars[k] = v
	}

	// Apply service type specific customization
	if serviceConfig, exists := tc.ServiceTypes[string(data.ServiceType)]; exists {
		// Apply default options if not explicitly set
		if !data.WithGRPC && serviceConfig.DefaultOptions["WithGRPC"] {
			data.WithGRPC = true
		}
		if !data.WithJobs && serviceConfig.DefaultOptions["WithJobs"] {
			data.WithJobs = true
		}
		if !data.WithCache && serviceConfig.DefaultOptions["WithCache"] {
			data.WithCache = true
		}
		if !data.WithAuth && serviceConfig.DefaultOptions["WithAuth"] {
			data.WithAuth = true
		}
		if !data.WithMessaging && serviceConfig.DefaultOptions["WithMessaging"] {
			data.WithMessaging = true
		}

		// Apply service-specific variables
		for k, v := range serviceConfig.Variables {
			data.CustomVars[k] = v
		}
	}
}

// GetServiceTypeInfo returns information about available service types
func (tc *TemplateCustomization) GetServiceTypeInfo() map[string]ServiceTypeInfo {
	info := make(map[string]ServiceTypeInfo)

	for serviceType, config := range tc.ServiceTypes {
		info[serviceType] = ServiceTypeInfo{
			Name:        serviceType,
			Description: config.Description,
			Features:    config.Features,
		}
	}

	return info
}

// ServiceTypeInfo contains display information about a service type
type ServiceTypeInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
}

// ValidateServiceType checks if a service type is valid
func (tc *TemplateCustomization) ValidateServiceType(serviceType string) error {
	if _, exists := tc.ServiceTypes[serviceType]; !exists {
		available := make([]string, 0, len(tc.ServiceTypes))
		for st := range tc.ServiceTypes {
			available = append(available, st)
		}
		return fmt.Errorf("invalid service type '%s'. Available types: %s",
			serviceType, strings.Join(available, ", "))
	}
	return nil
}
