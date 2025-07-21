package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CLIConfig represents the CLI configuration
type CLIConfig struct {
	DefaultOutputDir string         `yaml:"default_output_dir"`
	Templates        TemplateConfig `yaml:"templates"`
	Plugins          []PluginConfig `yaml:"plugins"`
	Defaults         DefaultsConfig `yaml:"defaults"`
}

// TemplateConfig contains template-related configuration
type TemplateConfig struct {
	CustomPath string            `yaml:"custom_path"`
	Variables  map[string]string `yaml:"variables"`
}

// PluginConfig represents a CLI plugin configuration
type PluginConfig struct {
	Name        string            `yaml:"name"`
	Command     string            `yaml:"command"`
	Description string            `yaml:"description"`
	Enabled     bool              `yaml:"enabled"`
	Config      map[string]string `yaml:"config"`
}

// DefaultsConfig contains default values for service generation
type DefaultsConfig struct {
	GoVersion    string `yaml:"go_version"`
	Author       string `yaml:"author"`
	License      string `yaml:"license"`
	Organization string `yaml:"organization"`
	WithGRPC     bool   `yaml:"with_grpc"`
	WithJobs     bool   `yaml:"with_jobs"`
	WithCache    bool   `yaml:"with_cache"`
	WithAuth     bool   `yaml:"with_auth"`
}

var (
	cliConfig *CLIConfig
	configDir string
)

// initConfig initializes the CLI configuration
func initConfig() error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir = filepath.Join(homeDir, ".microlib")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load existing config or create default
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		cliConfig = getDefaultConfig()
		if err := saveConfig(); err != nil {
			return fmt.Errorf("saving default config: %w", err)
		}
	} else {
		if err := loadConfig(configFile); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	}

	return nil
}

// loadConfig loads configuration from file
func loadConfig(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	cliConfig = &CLIConfig{}
	if err := yaml.Unmarshal(data, cliConfig); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	return nil
}

// saveConfig saves the current configuration to file
func saveConfig() error {
	configFile := filepath.Join(configDir, "config.yaml")

	data, err := yaml.Marshal(cliConfig)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// getDefaultConfig returns the default CLI configuration
func getDefaultConfig() *CLIConfig {
	return &CLIConfig{
		DefaultOutputDir: ".",
		Templates: TemplateConfig{
			CustomPath: "",
			Variables:  make(map[string]string),
		},
		Plugins: []PluginConfig{},
		Defaults: DefaultsConfig{
			GoVersion:    "1.23",
			Author:       "",
			License:      "MIT",
			Organization: "",
			WithGRPC:     false,
			WithJobs:     false,
			WithCache:    false,
			WithAuth:     false,
		},
	}
}

// GetConfig returns the current CLI configuration
func GetConfig() *CLIConfig {
	if cliConfig == nil {
		if err := initConfig(); err != nil {
			// Return default config if initialization fails
			return getDefaultConfig()
		}
	}
	return cliConfig
}

// UpdateConfig updates the CLI configuration
func UpdateConfig(updater func(*CLIConfig)) error {
	config := GetConfig()
	updater(config)
	return saveConfig()
}

// GetConfigDir returns the CLI configuration directory
func GetConfigDir() string {
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".microlib")
	}
	return configDir
}

// init adds config management commands
func init() {
	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  `Commands for viewing and modifying CLI configuration.`,
	}

	// Show config command
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  `Display the current CLI configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config := GetConfig()

			fmt.Println("MicroLib CLI Configuration:")
			fmt.Println("==========================")
			fmt.Printf("Config Directory: %s\n", GetConfigDir())
			fmt.Printf("Default Output Dir: %s\n", config.DefaultOutputDir)
			fmt.Printf("Go Version: %s\n", config.Defaults.GoVersion)
			fmt.Printf("Author: %s\n", config.Defaults.Author)
			fmt.Printf("License: %s\n", config.Defaults.License)
			fmt.Printf("Organization: %s\n", config.Defaults.Organization)
			fmt.Printf("Default gRPC: %t\n", config.Defaults.WithGRPC)
			fmt.Printf("Default Jobs: %t\n", config.Defaults.WithJobs)
			fmt.Printf("Default Cache: %t\n", config.Defaults.WithCache)
			fmt.Printf("Default Auth: %t\n", config.Defaults.WithAuth)

			if len(config.Templates.Variables) > 0 {
				fmt.Println("\nTemplate Variables:")
				for key, value := range config.Templates.Variables {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}

			return nil
		},
	}

	// Set config command
	setCmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long:  `Set a configuration value. Available keys: author, license, organization, go_version, output_dir`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			return UpdateConfig(func(config *CLIConfig) {
				switch key {
				case "author":
					config.Defaults.Author = value
				case "license":
					config.Defaults.License = value
				case "organization":
					config.Defaults.Organization = value
				case "go_version":
					config.Defaults.GoVersion = value
				case "output_dir":
					config.DefaultOutputDir = value
				default:
					fmt.Fprintf(os.Stderr, "Unknown configuration key: %s\n", key)
					return
				}
				fmt.Printf("Set %s = %s\n", key, value)
			})
		},
	}

	// Reset config command
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		Long:  `Reset the CLI configuration to default values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return UpdateConfig(func(config *CLIConfig) {
				defaults := getDefaultConfig()
				*config = *defaults
				fmt.Println("Configuration reset to defaults")
			})
		},
	}

	// Add subcommands
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(resetCmd)

	rootCmd.AddCommand(configCmd)
}
