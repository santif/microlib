package config

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Source is the interface for configuration sources
type Source interface {
	// Name returns the name of the source
	Name() string
	
	// Load loads configuration from the source
	Load(ctx context.Context) (map[string]interface{}, error)
	
	// Priority returns the priority of the source (higher values have higher priority)
	Priority() int
}

// SourcePriority defines standard priority levels for configuration sources
const (
	// PriorityDefault is the default priority for sources
	PriorityDefault = 100
	
	// PriorityEnv is the priority for environment variables (highest)
	PriorityEnv = 300
	
	// PriorityFile is the priority for configuration files
	PriorityFile = 200
	
	// PriorityFlag is the priority for command line flags
	PriorityFlag = 400 // Highest priority as it's an explicit override
)

// EnvSource is a configuration source that loads from environment variables
type EnvSource struct {
	prefix string
}

// NewEnvSource creates a new environment variable configuration source
func NewEnvSource(prefix string) *EnvSource {
	return &EnvSource{
		prefix: prefix,
	}
}

// Name returns the name of the source
func (s *EnvSource) Name() string {
	return "environment"
}

// Load loads configuration from environment variables
func (s *EnvSource) Load(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// Get all environment variables
	for _, env := range os.Environ() {
		// Split by first equals sign
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := parts[0]
		value := parts[1]
		
		// Check if the key has the prefix
		if s.prefix != "" && !strings.HasPrefix(key, s.prefix) {
			continue
		}
		
		// Remove the prefix if it exists
		if s.prefix != "" {
			key = strings.TrimPrefix(key, s.prefix)
			// Remove the separator if it exists (usually "_")
			if len(key) > 0 && key[0] == '_' {
				key = key[1:]
			}
		}
		
		// Convert to lowercase and replace underscores with dots for nested keys
		key = strings.ToLower(key)
		key = strings.Replace(key, "_", ".", -1)
		
		// Add to result
		result[key] = value
	}
	
	return result, nil
}

// Priority returns the priority of the source
func (s *EnvSource) Priority() int {
	return PriorityEnv
}

// FileSource is a configuration source that loads from a file
type FileSource struct {
	path     string
	format   string
	priority int
	watcher  bool
}

// FileSourceOption is a function that configures a FileSource
type FileSourceOption func(*FileSource)

// WithWatcher enables file watching for configuration changes
func WithWatcher(enabled bool) FileSourceOption {
	return func(s *FileSource) {
		s.watcher = enabled
	}
}

// WithPriority sets the priority of the file source
func WithPriority(priority int) FileSourceOption {
	return func(s *FileSource) {
		s.priority = priority
	}
}

// NewFileSource creates a new file configuration source
func NewFileSource(path string, format string, opts ...FileSourceOption) *FileSource {
	s := &FileSource{
		path:     path,
		format:   format,
		priority: PriorityFile,
		watcher:  false,
	}
	
	for _, opt := range opts {
		opt(s)
	}
	
	return s
}

// Name returns the name of the source
func (s *FileSource) Name() string {
	return fmt.Sprintf("file(%s)", s.path)
}

// Load loads configuration from a file
func (s *FileSource) Load(ctx context.Context) (map[string]interface{}, error) {
	// Check if the file exists
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file %s does not exist", s.path)
	}
	
	// Read the file
	_, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", s.path, err)
	}
	
	// Parse the file based on format
	result := make(map[string]interface{})
	
	switch s.format {
	case "yaml", "yml":
		// In a real implementation, this would use a YAML parser
		// For now, we'll just return an empty map
		return result, fmt.Errorf("YAML parsing not implemented")
	case "json":
		// In a real implementation, this would use a JSON parser
		// For now, we'll just return an empty map
		return result, fmt.Errorf("JSON parsing not implemented")
	case "toml":
		// In a real implementation, this would use a TOML parser
		// For now, we'll just return an empty map
		return result, fmt.Errorf("TOML parsing not implemented")
	default:
		return nil, fmt.Errorf("unsupported configuration format: %s", s.format)
	}
}

// Priority returns the priority of the source
func (s *FileSource) Priority() int {
	return s.priority
}

// FlagSource is a configuration source that loads from command line flags
type FlagSource struct {
	// In a real implementation, this would use a flag parsing library
	// For now, we'll just use a map for demonstration
	values map[string]interface{}
}

// NewFlagSource creates a new command line flag configuration source
func NewFlagSource() *FlagSource {
	return &FlagSource{
		values: make(map[string]interface{}),
	}
}

// Name returns the name of the source
func (s *FlagSource) Name() string {
	return "flags"
}

// Load loads configuration from command line flags
func (s *FlagSource) Load(ctx context.Context) (map[string]interface{}, error) {
	// In a real implementation, this would parse command line flags
	// For now, we'll just return the values map
	return s.values, nil
}

// Priority returns the priority of the source
func (s *FlagSource) Priority() int {
	return PriorityFlag
}

// SetValue sets a value in the flag source (for testing)
func (s *FlagSource) SetValue(key string, value interface{}) {
	s.values[key] = value
}