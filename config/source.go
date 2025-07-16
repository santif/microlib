package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Source is the interface for configuration sources
type Source interface {
	// Name returns the name of the source
	Name() string
	
	// Load loads configuration from the source
	Load(ctx context.Context) (map[string]interface{}, error)
	
	// Priority returns the priority of the source (higher values have higher priority)
	Priority() int
	
	// Watch starts watching for changes in the source (if supported)
	Watch(ctx context.Context, callback func()) error
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
		
		// Special handling for boolean values
		if strings.EqualFold(value, "true") {
			result[key] = true
			continue
		}
		if strings.EqualFold(value, "false") {
			result[key] = false
			continue
		}
		
		// Handle comma-separated lists for array values
		if strings.Contains(value, ",") && !strings.Contains(value, "\\,") {
			// Split by comma and trim spaces
			items := strings.Split(value, ",")
			for i := range items {
				items[i] = strings.TrimSpace(items[i])
			}
			
			// Store as array
			result[key] = items
			
			// Also store as individual items for array indexing
			for i, item := range items {
				arrayKey := fmt.Sprintf("%s.%d", key, i)
				result[arrayKey] = item
			}
		} else {
			// Try to parse the value as a number
			if i, err := parseInt(value); err == nil {
				result[key] = i
				continue
			}
			
			if f, err := parseFloat(value); err == nil {
				result[key] = f
				continue
			}
			
			// If not a number or boolean, store as string
			result[key] = value
		}
	}
	
	return result, nil
}

// Priority returns the priority of the source
func (s *EnvSource) Priority() int {
	return PriorityEnv
}

// Watch starts watching for changes in environment variables
// Note: Environment variables don't support watching, so this is a no-op
func (s *EnvSource) Watch(ctx context.Context, callback func()) error {
	// Environment variables don't support watching
	return nil
}

// FileSource is a configuration source that loads from a file
type FileSource struct {
	path     string
	format   string
	priority int
	watcher  bool
	mu       sync.RWMutex
	fsWatcher *fsnotify.Watcher
	callback func()
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
	// If format is not specified, try to infer it from the file extension
	if format == "" {
		format = strings.TrimPrefix(filepath.Ext(path), ".")
	}
	
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if the file exists
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file %s does not exist", s.path)
	}
	
	// Read the file
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", s.path, err)
	}
	
	// Parse the file based on format
	result := make(map[string]interface{})
	
	switch strings.ToLower(s.format) {
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse YAML configuration file %s: %w", s.path, err)
		}
	case "json":
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON configuration file %s: %w", s.path, err)
		}
	case "toml":
		if err := toml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse TOML configuration file %s: %w", s.path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported configuration format: %s", s.format)
	}
	
	// Flatten nested maps to dot notation
	flattenedResult := make(map[string]interface{})
	flattenMap(result, "", flattenedResult)
	
	return flattenedResult, nil
}

// flattenMap flattens a nested map to dot notation
func flattenMap(input map[string]interface{}, prefix string, output map[string]interface{}) {
	for k, v := range input {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		
		switch value := v.(type) {
		case map[string]interface{}:
			flattenMap(value, key, output)
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{}
			strMap := make(map[string]interface{})
			for mk, mv := range value {
				if mkStr, ok := mk.(string); ok {
					strMap[mkStr] = mv
				}
			}
			flattenMap(strMap, key, output)
		case []interface{}:
			// Handle arrays by creating indexed keys
			for i, item := range value {
				arrayKey := fmt.Sprintf("%s.%d", key, i)
				output[arrayKey] = item
			}
			// Also store the original array
			output[key] = value
		default:
			output[key] = v
		}
	}
}

// Priority returns the priority of the source
func (s *FileSource) Priority() int {
	return s.priority
}

// Watch starts watching for changes in the file
func (s *FileSource) Watch(ctx context.Context, callback func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.watcher {
		return nil
	}
	
	// Create a new file watcher if it doesn't exist
	if s.fsWatcher == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("failed to create file watcher: %w", err)
		}
		
		s.fsWatcher = watcher
		s.callback = callback
		
		// Watch the directory containing the file
		dir := filepath.Dir(s.path)
		if err := s.fsWatcher.Add(dir); err != nil {
			s.fsWatcher.Close()
			s.fsWatcher = nil
			return fmt.Errorf("failed to watch directory %s: %w", dir, err)
		}
		
		// Start watching for events
		go s.watchLoop(ctx)
	}
	
	return nil
}

// watchLoop watches for file changes
func (s *FileSource) watchLoop(ctx context.Context) {
	fileName := filepath.Base(s.path)
	var lastEventTime time.Time
	debounceInterval := 100 * time.Millisecond
	
	for {
		select {
		case event, ok := <-s.fsWatcher.Events:
			if !ok {
				return
			}
			
			// Check if the event is for our file
			if filepath.Base(event.Name) != fileName {
				continue
			}
			
			// Check if the event is a write or create event
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Debounce events
				now := time.Now()
				if now.Sub(lastEventTime) < debounceInterval {
					continue
				}
				lastEventTime = now
				
				// Notify callback
				s.mu.RLock()
				callback := s.callback
				s.mu.RUnlock()
				
				if callback != nil {
					callback()
				}
			}
		case err, ok := <-s.fsWatcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Error watching file %s: %v\n", s.path, err)
		case <-ctx.Done():
			s.mu.Lock()
			if s.fsWatcher != nil {
				s.fsWatcher.Close()
				s.fsWatcher = nil
			}
			s.mu.Unlock()
			return
		}
	}
}

// Close closes the file watcher
func (s *FileSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.fsWatcher != nil {
		err := s.fsWatcher.Close()
		s.fsWatcher = nil
		return err
	}
	
	return nil
}

// FlagSource is a configuration source that loads from command line flags
type FlagSource struct {
	flagSet *pflag.FlagSet
	values  map[string]interface{}
	mu      sync.RWMutex
}

// FlagSourceOption is a function that configures a FlagSource
type FlagSourceOption func(*FlagSource)

// WithFlagSet sets the flag set for the flag source
func WithFlagSet(flagSet *pflag.FlagSet) FlagSourceOption {
	return func(s *FlagSource) {
		s.flagSet = flagSet
	}
}

// NewFlagSource creates a new command line flag configuration source
func NewFlagSource(opts ...FlagSourceOption) *FlagSource {
	s := &FlagSource{
		flagSet: pflag.NewFlagSet("config", pflag.ContinueOnError),
		values:  make(map[string]interface{}),
	}
	
	for _, opt := range opts {
		opt(s)
	}
	
	return s
}

// Name returns the name of the source
func (s *FlagSource) Name() string {
	return "flags"
}

// Load loads configuration from command line flags
func (s *FlagSource) Load(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]interface{})
	
	// Copy values from the map
	for k, v := range s.values {
		result[k] = v
	}
	
	// If we have a flag set, extract values from it
	if s.flagSet != nil {
		s.flagSet.VisitAll(func(flag *pflag.Flag) {
			if flag.Changed {
				// Convert flag name to lowercase and replace hyphens with dots
				key := strings.ToLower(flag.Name)
				key = strings.Replace(key, "-", ".", -1)
				
				// Get the flag value
				value := flag.Value.String()
				
				// Try to parse the value as a number or boolean
				parsedValue, err := parseValue(value)
				if err == nil {
					result[key] = parsedValue
				} else {
					result[key] = value
				}
			}
		})
	}
	
	return result, nil
}

// Priority returns the priority of the source
func (s *FlagSource) Priority() int {
	return PriorityFlag
}

// Watch starts watching for changes in command line flags
// Note: Flags don't support watching, so this is a no-op
func (s *FlagSource) Watch(ctx context.Context, callback func()) error {
	// Flags don't support watching
	return nil
}

// SetValue sets a value in the flag source (for testing)
func (s *FlagSource) SetValue(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

// AddToCommand adds the flag source to a cobra command
func (s *FlagSource) AddToCommand(cmd *cobra.Command) {
	// Use the command's flag set
	s.flagSet = cmd.PersistentFlags()
}

// AddFlag adds a flag to the flag source
func (s *FlagSource) AddFlag(name string, value interface{}, usage string) {
	if s.flagSet == nil {
		return
	}
	
	switch v := value.(type) {
	case string:
		s.flagSet.String(name, v, usage)
	case bool:
		s.flagSet.Bool(name, v, usage)
	case int:
		s.flagSet.Int(name, v, usage)
	case int64:
		s.flagSet.Int64(name, v, usage)
	case uint:
		s.flagSet.Uint(name, v, usage)
	case uint64:
		s.flagSet.Uint64(name, v, usage)
	case float64:
		s.flagSet.Float64(name, v, usage)
	case []string:
		s.flagSet.StringSlice(name, v, usage)
	}
}

// parseValue tries to parse a string value as a number or boolean
func parseValue(value string) (interface{}, error) {
	// Try to parse as boolean
	if strings.EqualFold(value, "true") {
		return true, nil
	}
	if strings.EqualFold(value, "false") {
		return false, nil
	}
	
	// Try to parse as integer
	if i, err := parseInt(value); err == nil {
		return i, nil
	}
	
	// Try to parse as float
	if f, err := parseFloat(value); err == nil {
		return f, nil
	}
	
	// Return error to indicate that the value is a string
	return nil, fmt.Errorf("value is a string")
}

// parseInt tries to parse a string as an integer
func parseInt(value string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}

// parseFloat tries to parse a string as a float
func parseFloat(value string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(value, "%f", &result)
	return result, err
}