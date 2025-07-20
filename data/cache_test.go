package data

import (
	"context"
	"testing"
	"time"
)

// MockCache is a mock implementation of the Cache interface for testing
type MockCache struct {
	data       map[string][]byte
	defaultTTL time.Duration
	namespace  string
}

// NewMockCache creates a new mock cache
func NewMockCache(config CacheConfig) *MockCache {
	return &MockCache{
		data:       make(map[string][]byte),
		defaultTTL: config.DefaultTTL,
		namespace:  config.Namespace,
	}
}

// Get retrieves a value from the mock cache
func (c *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	formattedKey := formatKey(c.namespace, key)
	value, ok := c.data[formattedKey]
	if !ok {
		return nil, ErrCacheMiss
	}

	return value, nil
}

// Set stores a value in the mock cache with the default TTL
func (c *MockCache) Set(ctx context.Context, key string, value []byte) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the mock cache with a specific TTL
func (c *MockCache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := validateKey(key); err != nil {
		return err
	}

	formattedKey := formatKey(c.namespace, key)
	c.data[formattedKey] = value
	return nil
}

// Delete removes a value from the mock cache
func (c *MockCache) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	formattedKey := formatKey(c.namespace, key)
	delete(c.data, formattedKey)
	return nil
}

// Clear removes all values from the mock cache with the current namespace
func (c *MockCache) Clear(ctx context.Context) error {
	if c.namespace == "" {
		return nil
	}

	// Remove all keys with the namespace prefix
	for key := range c.data {
		if len(key) >= len(c.namespace)+1 && key[:len(c.namespace)+1] == c.namespace+":" {
			delete(c.data, key)
		}
	}

	return nil
}

// GetMulti retrieves multiple values from the mock cache
func (c *MockCache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for _, key := range keys {
		if err := validateKey(key); err != nil {
			return nil, err
		}

		formattedKey := formatKey(c.namespace, key)
		if value, ok := c.data[formattedKey]; ok {
			result[key] = value
		}
	}

	return result, nil
}

// SetMulti stores multiple values in the mock cache with the default TTL
func (c *MockCache) SetMulti(ctx context.Context, items map[string][]byte) error {
	return c.SetMultiWithTTL(ctx, items, c.defaultTTL)
}

// SetMultiWithTTL stores multiple values in the mock cache with a specific TTL
func (c *MockCache) SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	for key, value := range items {
		if err := c.SetWithTTL(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMulti removes multiple values from the mock cache
func (c *MockCache) DeleteMulti(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// Increment atomically increments a numeric value in the mock cache
func (c *MockCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	if err := validateKey(key); err != nil {
		return 0, err
	}

	formattedKey := formatKey(c.namespace, key)
	var value int64 = 0

	// Get existing value if it exists
	if existingValue, ok := c.data[formattedKey]; ok {
		// Try to parse the existing value as an int64
		// For simplicity in this mock, we'll just assume it's valid
		value = int64(existingValue[0])
	}

	// Increment the value
	value += amount

	// Store the new value
	c.data[formattedKey] = []byte{byte(value)}

	return value, nil
}

// Decrement atomically decrements a numeric value in the mock cache
func (c *MockCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	return c.Increment(ctx, key, -amount)
}

// Ping checks if the mock cache is reachable
func (c *MockCache) Ping(ctx context.Context) error {
	return nil
}

// Close closes the mock cache connection
func (c *MockCache) Close() error {
	return nil
}

// MockCacheProvider implements the CacheProvider interface for testing
type MockCacheProvider struct{}

// NewCache creates a new mock cache instance
func (p *MockCacheProvider) NewCache(config CacheConfig) (Cache, error) {
	return NewMockCache(config), nil
}

// TestFormatKey tests the formatKey function
func TestFormatKey(t *testing.T) {
	tests := []struct {
		namespace  string
		key        string
		expected   string
		shouldFail bool
	}{
		{namespace: "app", key: "user:123", expected: "app:user:123"},
		{namespace: "", key: "user:123", expected: "user:123"},
		{namespace: "test", key: "", shouldFail: true},
	}

	for _, test := range tests {
		if test.shouldFail {
			if err := validateKey(test.key); err == nil {
				t.Errorf("Expected validateKey to fail for key %q", test.key)
			}
			continue
		}

		result := formatKey(test.namespace, test.key)
		if result != test.expected {
			t.Errorf("formatKey(%q, %q) = %q, expected %q", test.namespace, test.key, result, test.expected)
		}
	}
}

// TestFormatKeys tests the formatKeys function
func TestFormatKeys(t *testing.T) {
	tests := []struct {
		namespace string
		keys      []string
		expected  []string
	}{
		{
			namespace: "app",
			keys:      []string{"user:123", "post:456"},
			expected:  []string{"app:user:123", "app:post:456"},
		},
		{
			namespace: "",
			keys:      []string{"user:123", "post:456"},
			expected:  []string{"user:123", "post:456"},
		},
	}

	for _, test := range tests {
		result := formatKeys(test.namespace, test.keys)
		if len(result) != len(test.expected) {
			t.Errorf("formatKeys(%q, %v) returned %d keys, expected %d", test.namespace, test.keys, len(result), len(test.expected))
			continue
		}

		for i, key := range result {
			if key != test.expected[i] {
				t.Errorf("formatKeys(%q, %v)[%d] = %q, expected %q", test.namespace, test.keys, i, key, test.expected[i])
			}
		}
	}
}

// TestMockCache tests the mock cache implementation
func TestMockCache(t *testing.T) {
	ctx := context.Background()
	config := CacheConfig{
		Type:       "memory",
		DefaultTTL: 1 * time.Hour,
		Namespace:  "test",
	}

	cache := NewMockCache(config)

	// Test Set and Get
	err := cache.Set(ctx, "key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("Get returned %q, expected %q", string(value), "value1")
	}

	// Test Delete
	err = cache.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = cache.Get(ctx, "key1")
	if err != ErrCacheMiss {
		t.Errorf("Get after Delete returned error %v, expected ErrCacheMiss", err)
	}

	// Test SetMulti and GetMulti
	items := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}
	err = cache.SetMulti(ctx, items)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	values, err := cache.GetMulti(ctx, []string{"key1", "key2", "key3"})
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("GetMulti returned %d values, expected 2", len(values))
	}
	if string(values["key1"]) != "value1" {
		t.Errorf("GetMulti[key1] = %q, expected %q", string(values["key1"]), "value1")
	}
	if string(values["key2"]) != "value2" {
		t.Errorf("GetMulti[key2] = %q, expected %q", string(values["key2"]), "value2")
	}

	// Test DeleteMulti
	err = cache.DeleteMulti(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	values, err = cache.GetMulti(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Fatalf("GetMulti after DeleteMulti failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("GetMulti after DeleteMulti returned %d values, expected 0", len(values))
	}

	// Test Clear
	err = cache.SetMulti(ctx, items)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	values, err = cache.GetMulti(ctx, []string{"key1", "key2"})
	if err != nil {
		t.Fatalf("GetMulti after Clear failed: %v", err)
	}
	if len(values) != 0 {
		t.Errorf("GetMulti after Clear returned %d values, expected 0", len(values))
	}

	// Test Increment and Decrement
	val, err := cache.Increment(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 5 {
		t.Errorf("Increment returned %d, expected 5", val)
	}

	val, err = cache.Decrement(ctx, "counter", 2)
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}
	if val != 3 {
		t.Errorf("Decrement returned %d, expected 3", val)
	}
}
