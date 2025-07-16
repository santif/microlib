package config

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	
	// StartWatching starts watching for configuration changes
	StartWatching(ctx context.Context, config interface{}) error
	
	// StopWatching stops watching for configuration changes
	StopWatching() error
}

// ValidationError represents a validation error with detailed information
type ValidationError struct {
	Field   string
	Value   interface{}
	Tag     string
	Message string
}

// Error returns the error message
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// DefaultManager is the default implementation of the Manager interface
type DefaultManager struct {
	sources  []Source
	validate *validator.Validate
	mu       sync.RWMutex
	watchers []func(interface{})
	ctx      context.Context
	cancel   context.CancelFunc
	config   interface{}
}

// ManagerOption is a function that configures a DefaultManager
type ManagerOption func(*DefaultManager)

// WithSource adds a configuration source to the manager
func WithSource(source Source) ManagerOption {
	return func(m *DefaultManager) {
		m.sources = append(m.sources, source)
	}
}

// WithCustomValidator adds custom validation functions to the validator
func WithCustomValidator(tag string, fn validator.Func) ManagerOption {
	return func(m *DefaultManager) {
		m.validate.RegisterValidation(tag, fn)
	}
}

// NewManager creates a new configuration manager with the provided options
func NewManager(opts ...ManagerOption) *DefaultManager {
	validate := validator.New()
	
	// Register custom error formatter
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return fld.Name
		}
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("yaml"), ",", 2)[0]
			if name == "-" {
				return fld.Name
			}
		}
		if name == "" {
			return fld.Name
		}
		return name
	})
	
	m := &DefaultManager{
		sources:  make([]Source, 0),
		validate: validate,
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

	// Create a merged configuration map to handle priority correctly
	mergedConfig := make(map[string]interface{})
	
	// Load configuration from each source in priority order
	ctx := context.Background()
	for _, source := range sources {
		data, err := source.Load(ctx)
		if err != nil {
			// Log the error but continue with other sources
			fmt.Printf("Error loading configuration from source %s: %v\n", source.Name(), err)
			continue
		}
		
		// Merge data from this source into the merged config
		// Lower priority sources won't overwrite values from higher priority sources
		for k, v := range data {
			if _, exists := mergedConfig[k]; !exists {
				mergedConfig[k] = v
			}
		}
	}
	
	// Debug output for testing
	// fmt.Printf("Merged config: %+v\n", mergedConfig)
	
	// Apply the merged configuration to the destination struct
	if err := mergeConfig(mergedConfig, dest); err != nil {
		return fmt.Errorf("failed to merge configuration: %w", err)
	}

	// Validate the configuration
	if err := m.Validate(dest); err != nil {
		return err
	}

	return nil
}

// Validate validates the configuration struct against defined rules
func (m *DefaultManager) Validate(config interface{}) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	err := m.validate.Struct(config)
	if err == nil {
		return nil
	}

	// Convert validation errors to our custom format
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		errors := make([]ValidationError, 0, len(validationErrors))
		
		for _, err := range validationErrors {
			var message string
			
			switch err.Tag() {
			case "required":
				message = "this field is required"
			case "min":
				message = fmt.Sprintf("value must be greater than or equal to %s", err.Param())
			case "max":
				message = fmt.Sprintf("value must be less than or equal to %s", err.Param())
			case "email":
				message = "invalid email format"
			case "url":
				message = "invalid URL format"
			case "semver":
				message = "invalid semantic version format (must be X.Y.Z)"
			default:
				message = fmt.Sprintf("failed validation for tag '%s'", err.Tag())
			}
			
			// Extract the field path from the struct namespace
			// Convert "TestConfig.Service.Name" to "service.name"
			parts := strings.Split(err.Namespace(), ".")
			if len(parts) > 1 {
				// Skip the struct name (first part)
				parts = parts[1:]
				// Convert to lowercase
				for i := range parts {
					parts[i] = strings.ToLower(parts[i])
				}
				fieldPath := strings.Join(parts, ".")
				
				errors = append(errors, ValidationError{
					Field:   fieldPath,
					Value:   err.Value(),
					Tag:     err.Tag(),
					Message: message,
				})
			} else {
				// Fallback if we can't parse the namespace
				errors = append(errors, ValidationError{
					Field:   err.Field(),
					Value:   err.Value(),
					Tag:     err.Tag(),
					Message: message,
				})
			}
		}
		
		// Create a detailed error message
		var errMsg strings.Builder
		errMsg.WriteString("Configuration validation failed:\n")
		
		for _, e := range errors {
			errMsg.WriteString(fmt.Sprintf("  - validation failed for field '%s': %s\n", e.Field, e.Message))
		}
		
		return fmt.Errorf(errMsg.String())
	}
	
	return err
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

// StartWatching starts watching for configuration changes
func (m *DefaultManager) StartWatching(ctx context.Context, config interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// If already watching, stop first
	if m.cancel != nil {
		m.cancel()
	}
	
	// Create a new context with cancel function
	watchCtx, cancel := context.WithCancel(ctx)
	m.ctx = watchCtx
	m.cancel = cancel
	m.config = config
	
	// Start watching each source that supports it
	for _, source := range m.sources {
		err := source.Watch(watchCtx, func() {
			// Reload configuration when a change is detected
			if err := m.reloadConfig(); err != nil {
				fmt.Printf("Error reloading configuration: %v\n", err)
			}
		})
		
		if err != nil {
			fmt.Printf("Error setting up watch for source %s: %v\n", source.Name(), err)
		}
	}
	
	return nil
}

// StopWatching stops watching for configuration changes
func (m *DefaultManager) StopWatching() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
		m.ctx = nil
	}
	
	return nil
}

// reloadConfig reloads the configuration from all sources
func (m *DefaultManager) reloadConfig() error {
	if m.config == nil {
		return fmt.Errorf("no configuration to reload")
	}
	
	// Create a copy of the configuration
	configType := reflect.TypeOf(m.config)
	if configType.Kind() != reflect.Ptr {
		return fmt.Errorf("configuration must be a pointer")
	}
	
	// Create a new instance of the configuration
	newConfig := reflect.New(configType.Elem()).Interface()
	
	// Load configuration into the new instance
	if err := m.Load(newConfig); err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}
	
	// Copy the new configuration to the original
	reflect.ValueOf(m.config).Elem().Set(reflect.ValueOf(newConfig).Elem())
	
	// Notify watchers
	m.notifyWatchers(m.config)
	
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
		go func(w func(interface{})) {
			w(config)
		}(watcher)
	}
}

// mergeConfig merges configuration data into the destination struct
func mergeConfig(data map[string]interface{}, dest interface{}) error {
	// Implementation of merging configuration data into the destination struct
	// This handles flat key paths (e.g., "service.name") and array indices (e.g., "http.hosts.0")
	
	destVal := reflect.ValueOf(dest).Elem()
	
	// First, collect all array keys to process them together
	arrayKeys := make(map[string][]string)
	arrayValues := make(map[string][]interface{})
	
	for key := range data {
		// Check if this is an array index key (ends with .0, .1, etc.)
		parts := strings.Split(key, ".")
		if len(parts) >= 2 {
			lastPart := parts[len(parts)-1]
			if _, err := strconv.Atoi(lastPart); err == nil {
				// This is an array index key
				baseKey := strings.Join(parts[:len(parts)-1], ".")
				arrayKeys[baseKey] = append(arrayKeys[baseKey], key)
			}
		}
	}
	
	// Process array keys
	for baseKey, keys := range arrayKeys {
		// Sort keys by index
		sort.Strings(keys)
		
		// Collect values
		values := make([]interface{}, len(keys))
		for i, key := range keys {
			values[i] = data[key]
		}
		
		// Store for later processing
		arrayValues[baseKey] = values
		
		// Add the array to the data map
		baseParts := strings.Split(baseKey, ".")
		if len(baseParts) > 0 {
			// Try to find the field
			fieldVal, err := findField(destVal, baseParts)
			if err == nil && fieldVal.Kind() == reflect.Slice {
				// This is a slice field, set it directly
				if err := setSliceValue(fieldVal, values); err != nil {
					fmt.Printf("Error setting slice field %s: %v\n", baseKey, err)
				}
			}
		}
	}
	
	// Process regular keys
	for key, value := range data {
		// Skip array index keys, they were processed above
		parts := strings.Split(key, ".")
		if len(parts) >= 2 {
			lastPart := parts[len(parts)-1]
			if _, err := strconv.Atoi(lastPart); err == nil {
				continue
			}
		}
		
		// Skip array base keys that we've already processed
		if _, ok := arrayValues[key]; ok {
			continue
		}
		
		// Skip empty keys
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

// setSliceValue sets a slice field value from an array of values
func setSliceValue(field reflect.Value, values []interface{}) error {
	if field.Kind() != reflect.Slice {
		return fmt.Errorf("field is not a slice")
	}
	
	// Create a new slice of the correct type
	sliceType := field.Type()
	elemType := sliceType.Elem()
	slice := reflect.MakeSlice(sliceType, len(values), len(values))
	
	// Set each element
	for i, value := range values {
		if value == nil {
			continue
		}
		
		elemValue := slice.Index(i)
		valueVal := reflect.ValueOf(value)
		
		// Handle string conversion for various types
		if valueVal.Kind() == reflect.String {
			strValue := value.(string)
			
			switch elemType.Kind() {
			case reflect.String:
				elemValue.SetString(strValue)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intVal, err := strconv.ParseInt(strValue, 10, 64)
				if err != nil {
					return err
				}
				elemValue.SetInt(intVal)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				uintVal, err := strconv.ParseUint(strValue, 10, 64)
				if err != nil {
					return err
				}
				elemValue.SetUint(uintVal)
			case reflect.Float32, reflect.Float64:
				floatVal, err := strconv.ParseFloat(strValue, 64)
				if err != nil {
					return err
				}
				elemValue.SetFloat(floatVal)
			case reflect.Bool:
				boolVal, err := strconv.ParseBool(strValue)
				if err != nil {
					return err
				}
				elemValue.SetBool(boolVal)
			default:
				return fmt.Errorf("unsupported conversion from string to %s", elemType.Kind())
			}
		} else if valueVal.Type().AssignableTo(elemType) {
			// Direct assignment
			elemValue.Set(valueVal)
		} else if valueVal.Type().ConvertibleTo(elemType) {
			// Type conversion
			elemValue.Set(valueVal.Convert(elemType))
		} else {
			return fmt.Errorf("cannot convert %s to %s", valueVal.Type(), elemType)
		}
	}
	
	// Set the slice
	field.Set(slice)
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
		case reflect.Slice:
			// Handle string slices
			if field.Type().Elem().Kind() == reflect.String {
				// Split by comma
				parts := strings.Split(strValue, ",")
				slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))
				for i, part := range parts {
					slice.Index(i).SetString(strings.TrimSpace(part))
				}
				field.Set(slice)
			} else {
				return fmt.Errorf("unsupported conversion from string to %s", field.Type())
			}
		case reflect.Map:
			// Handle string maps (not implemented yet)
			return fmt.Errorf("unsupported conversion from string to %s", field.Type())
		case reflect.Struct:
			// Handle special types like time.Time
			if field.Type() == reflect.TypeOf(time.Time{}) {
				t, err := time.Parse(time.RFC3339, strValue)
				if err != nil {
					return fmt.Errorf("invalid time format: %w", err)
				}
				field.Set(reflect.ValueOf(t))
			} else {
				return fmt.Errorf("unsupported conversion from string to %s", field.Type())
			}
		default:
			return fmt.Errorf("unsupported conversion from string to %s", field.Kind())
		}
		return nil
	}
	
	// Handle slice conversion
	if valueVal.Kind() == reflect.Slice && field.Kind() == reflect.Slice {
		// Create a new slice of the correct type
		slice := reflect.MakeSlice(field.Type(), valueVal.Len(), valueVal.Len())
		
		// Convert each element
		for i := 0; i < valueVal.Len(); i++ {
			elem := valueVal.Index(i)
			if elem.Type().ConvertibleTo(field.Type().Elem()) {
				slice.Index(i).Set(elem.Convert(field.Type().Elem()))
			} else {
				return fmt.Errorf("cannot convert slice element %v to %s", elem.Interface(), field.Type().Elem())
			}
		}
		
		field.Set(slice)
		return nil
	}
	
	// Handle map conversion
	if valueVal.Kind() == reflect.Map && field.Kind() == reflect.Map {
		// Create a new map of the correct type
		mapValue := reflect.MakeMap(field.Type())
		
		// Convert each key-value pair
		iter := valueVal.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			
			if k.Type().ConvertibleTo(field.Type().Key()) && v.Type().ConvertibleTo(field.Type().Elem()) {
				mapValue.SetMapIndex(k.Convert(field.Type().Key()), v.Convert(field.Type().Elem()))
			} else {
				return fmt.Errorf("cannot convert map entry %v:%v to %s:%s", 
					k.Interface(), v.Interface(), field.Type().Key(), field.Type().Elem())
			}
		}
		
		field.Set(mapValue)
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