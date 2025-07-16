package config

import (
	"fmt"
	"reflect"
	"sync"
)

// Config is a thread-safe configuration container that provides
// safe access to configuration values across multiple goroutines
type Config struct {
	mu     sync.RWMutex
	data   interface{}
	fields map[string]reflect.Value
}

// Reloadable is an interface for components that can reload their configuration
type Reloadable interface {
	// Reload updates the component with new configuration
	// Returns an error if the reload fails
	Reload(newConfig interface{}) error
}

// NewConfig creates a new thread-safe configuration container
func NewConfig(data interface{}) (*Config, error) {
	if data == nil {
		return nil, fmt.Errorf("configuration data cannot be nil")
	}

	// Check if data is a pointer to a struct
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("configuration data must be a pointer to a struct")
	}

	c := &Config{
		data:   data,
		fields: make(map[string]reflect.Value),
	}

	// Index fields for faster access
	c.indexFields(v.Elem(), "")

	return c, nil
}

// indexFields recursively indexes all fields in the struct for faster access
func (c *Config) indexFields(v reflect.Value, prefix string) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Build the field path
		fieldPath := field.Name
		if prefix != "" {
			fieldPath = prefix + "." + fieldPath
		}

		// Store the field
		c.fields[fieldPath] = fieldValue

		// If this is a struct, recursively index its fields
		if fieldValue.Kind() == reflect.Struct {
			c.indexFields(fieldValue, fieldPath)
		}
	}
}

// Get returns the configuration data
// This method is thread-safe and can be called from multiple goroutines
func (c *Config) Get() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// GetField returns a specific field from the configuration
// This method is thread-safe and can be called from multiple goroutines
func (c *Config) GetField(path string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	field, ok := c.fields[path]
	if !ok {
		return nil, fmt.Errorf("field not found: %s", path)
	}

	// Return a copy of the value to avoid concurrent modification
	return field.Interface(), nil
}

// Update updates the configuration with new data
// This method is thread-safe and can be called from multiple goroutines
func (c *Config) Update(newData interface{}) error {
	if newData == nil {
		return fmt.Errorf("new configuration data cannot be nil")
	}

	// Check if newData is a pointer to a struct
	v := reflect.ValueOf(newData)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("new configuration data must be a pointer to a struct")
	}

	// Check if newData is the same type as the original data
	if v.Type() != reflect.ValueOf(c.data).Type() {
		return fmt.Errorf("new configuration data must be the same type as the original data")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Copy the new data to the original data
	reflect.ValueOf(c.data).Elem().Set(v.Elem())

	// Re-index fields
	c.fields = make(map[string]reflect.Value)
	c.indexFields(reflect.ValueOf(c.data).Elem(), "")

	return nil
}

// UpdateField updates a specific field in the configuration
// This method is thread-safe and can be called from multiple goroutines
func (c *Config) UpdateField(path string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	field, ok := c.fields[path]
	if !ok {
		return fmt.Errorf("field not found: %s", path)
	}

	if !field.CanSet() {
		return fmt.Errorf("field cannot be set: %s", path)
	}

	// Convert the value to the field's type if possible
	v := reflect.ValueOf(value)
	if !v.Type().AssignableTo(field.Type()) {
		if !v.Type().ConvertibleTo(field.Type()) {
			return fmt.Errorf("value type %s cannot be converted to field type %s", v.Type(), field.Type())
		}
		v = v.Convert(field.Type())
	}

	// Set the field value
	field.Set(v)

	return nil
}

// ConfigProvider is an interface for components that provide configuration
type ConfigProvider interface {
	// GetConfig returns the configuration
	GetConfig() interface{}
}

// Injectable is a struct that can be embedded in services to provide
// configuration injection capabilities
type Injectable struct {
	config *Config
}

// InjectConfig injects a configuration into the service
func (i *Injectable) InjectConfig(config *Config) {
	i.config = config
}

// GetConfig returns the injected configuration
func (i *Injectable) GetConfig() interface{} {
	if i.config == nil {
		return nil
	}
	return i.config.Get()
}

// GetConfigField returns a specific field from the injected configuration
func (i *Injectable) GetConfigField(path string) (interface{}, error) {
	if i.config == nil {
		return nil, fmt.Errorf("no configuration injected")
	}
	return i.config.GetField(path)
}