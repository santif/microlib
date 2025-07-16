package config

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// Manager is the interface for configuration management
type Manager interface {
	// Load loads configuration into the provided struct
	Load(dest interface{}) error
	
	// Validate validates the configuration struct against defined rules
	Validate(config interface{}) error
	
	// Watch registers a callback to be called when configuration changes
	Watch(callback func(interface{})) error
}

// DefaultManager is the default implementation of the Manager interface
type DefaultManager struct {
	sources []Source
	validate *validator.Validate
	mu       sync.RWMutex
	watchers []func(interface{})
}

// ManagerOption is a function that configures a DefaultManager
type ManagerOption func(*DefaultManager)

// WithSource adds a configuration source to the manager
func WithSource(source Source) ManagerOption {
	return func(m *DefaultManager) {
		m.sources = append(m.sources, source)
	}
}

// NewManager creates a new configuration manager with the provided options
func NewManager(opts ...ManagerOption) *DefaultManager {
	m := &DefaultManager{
		sources:  make([]Source, 0),
		validate: validator.New(),
		watchers: make([]func(interface{}), 0),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Load loads configuration from all sources into the provided struct
func (m *DefaultManager) Load(dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}

	// Check if dest is a pointer to a struct
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("destination must be a pointer to a struct")
	}

	// Sort sources by priority (higher priority first)
	m.mu.RLock()
	sources := make([]Source, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Priority() > sources[j].Priority()
	})

	// Load configuration from each source
	ctx := context.Background()
	for _, source := range sources {
		data, err := source.Load(ctx)
		if err != nil {
			// Log the error but continue with other sources
			fmt.Printf("Error loading configuration from source %s: %v\n", source.Name(), err)
			continue
		}

		if err := mergeConfig(data, dest); err != nil {
			return fmt.Errorf("failed to merge configuration from source %s: %w", source.Name(), err)
		}
	}

	// Validate the configuration
	if err := m.Validate(dest); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// Validate validates the configuration struct against defined rules
func (m *DefaultManager) Validate(config interface{}) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	return m.validate.Struct(config)
}

// Watch registers a callback to be called when configuration changes
func (m *DefaultManager) Watch(callback func(interface{})) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers = append(m.watchers, callback)
	return nil
}

// AddSource adds a configuration source to the manager
func (m *DefaultManager) AddSource(source Source) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, source)
}

// notifyWatchers notifies all registered watchers about configuration changes
func (m *DefaultManager) notifyWatchers(config interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, watcher := range m.watchers {
		watcher(config)
	}
}

// mergeConfig merges configuration data into the destination struct
func mergeConfig(data map[string]interface{}, dest interface{}) error {
	// Implementation of merging configuration data into the destination struct
	// This is a simplified version that handles flat key paths (e.g., "service.name")
	// A real implementation would handle more complex nested structures
	
	destVal := reflect.ValueOf(dest).Elem()
	
	// Process each key in the data map
	for key, value := range data {
		// Split the key by dots to handle nested fields
		parts := strings.Split(key, ".")
		if len(parts) == 0 {
			continue
		}
		
		// Navigate to the correct struct field
		fieldVal, err := findField(destVal, parts)
		if err != nil {
			// Skip fields that don't exist in the struct
			continue
		}
		
		// Set the field value if possible
		if fieldVal.CanSet() {
			err := setFieldValue(fieldVal, value)
			if err != nil {
				// Log the error but continue with other fields
				fmt.Printf("Error setting field %s: %v\n", key, err)
			}
		}
	}
	
	return nil
}

// findField navigates to a nested field in a struct using the key parts
func findField(val reflect.Value, parts []string) (reflect.Value, error) {
	current := val
	
	for i, part := range parts {
		// If current is not a struct, we can't go deeper
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("not a struct at %s", strings.Join(parts[:i], "."))
		}
		
		// Find the field by name
		field := findFieldByName(current, part)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("field not found: %s", part)
		}
		
		current = field
	}
	
	return current, nil
}

// findFieldByName finds a field in a struct by name, checking json and yaml tags
func findFieldByName(val reflect.Value, name string) reflect.Value {
	typ := val.Type()
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Check the field name
		if strings.EqualFold(field.Name, name) {
			return val.Field(i)
		}
		
		// Check json tag
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == name {
				return val.Field(i)
			}
		}
		
		// Check yaml tag
		if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
			parts := strings.Split(yamlTag, ",")
			if parts[0] == name {
				return val.Field(i)
			}
		}
	}
	
	return reflect.Value{}
}

// setFieldValue sets a field value with appropriate type conversion
func setFieldValue(field reflect.Value, value interface{}) error {
	// Handle nil values
	if value == nil {
		return nil
	}
	
	valueVal := reflect.ValueOf(value)
	
	// Handle string conversion for various types
	if valueVal.Kind() == reflect.String {
		strValue := value.(string)
		
		switch field.Kind() {
		case reflect.String:
			field.SetString(strValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(strValue, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(intVal)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			uintVal, err := strconv.ParseUint(strValue, 10, 64)
			if err != nil {
				return err
			}
			field.SetUint(uintVal)
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(strValue, 64)
			if err != nil {
				return err
			}
			field.SetFloat(floatVal)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(strValue)
			if err != nil {
				return err
			}
			field.SetBool(boolVal)
		default:
			return fmt.Errorf("unsupported conversion from string to %s", field.Kind())
		}
		return nil
	}
	
	// Direct assignment for compatible types
	if valueVal.Type().AssignableTo(field.Type()) {
		field.Set(valueVal)
		return nil
	}
	
	// Try type conversion
	if valueVal.Type().ConvertibleTo(field.Type()) {
		field.Set(valueVal.Convert(field.Type()))
		return nil
	}
	
	return fmt.Errorf("cannot convert %s to %s", valueVal.Type(), field.Type())
}