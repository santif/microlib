package data

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestNewRedisCache tests the NewRedisCache function
func TestNewRedisCache(t *testing.T) {
	tests := []struct {
		name        string
		config      CacheConfig
		shouldError bool
	}{
		{
			name: "Valid config",
			config: CacheConfig{
				Type:       "redis",
				Address:    "localhost:6379",
				DefaultTTL: 1 * time.Hour,
			},
			shouldError: false,
		},
		{
			name: "Invalid type",
			config: CacheConfig{
				Type:       "invalid",
				Address:    "localhost:6379",
				DefaultTTL: 1 * time.Hour,
			},
			shouldError: true,
		},
		{
			name: "Missing address",
			config: CacheConfig{
				Type:       "redis",
				DefaultTTL: 1 * time.Hour,
			},
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRedisCache(test.config)
			if test.shouldError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !test.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestRedisCache_Integration tests the Redis cache implementation with a real Redis server
// This test is skipped unless the REDIS_TEST_ADDR environment variable is set
func TestRedisCache_Integration(t *testing.T) {
	redisAddr := os.Getenv("REDIS_TEST_ADDR")
	if redisAddr == "" {
		t.Skip("Skipping integration test: REDIS_TEST_ADDR not set")
	}

	ctx := context.Background()
	config := CacheConfig{
		Type:       "redis",
		Address:    redisAddr,
		DefaultTTL: 1 * time.Hour,
		Namespace:  "test",
	}

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer cache.Close()

	// Test connection
	err = cache.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// Test Set and Get
	err = cache.Set(ctx, "key1", []byte("value1"))
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

	// Clean up
	err = cache.Delete(ctx, "counter")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

// TestRedisNamespaceHandling tests the namespace handling in the Redis cache
func TestRedisNamespaceHandling(t *testing.T) {
	// Test namespace handling
	namespace := "test"
	key := "key1"
	formattedKey := formatKey(namespace, key)
	if formattedKey != "test:key1" {
		t.Errorf("formatKey returned %q, expected %q", formattedKey, "test:key1")
	}

	// Test with empty namespace
	formattedKey = formatKey("", key)
	if formattedKey != "key1" {
		t.Errorf("formatKey with empty namespace returned %q, expected %q", formattedKey, "key1")
	}
}

// TestRedisConfigValidation tests the configuration validation in the Redis cache
func TestRedisConfigValidation(t *testing.T) {
	// Test with invalid type
	_, err := NewRedisCache(CacheConfig{
		Type:       "invalid",
		Address:    "localhost:6379",
		DefaultTTL: 1 * time.Hour,
	})
	if err == nil {
		t.Errorf("Expected error for invalid type, got nil")
	}

	// Test with empty address
	_, err = NewRedisCache(CacheConfig{
		Type:       "redis",
		DefaultTTL: 1 * time.Hour,
	})
	if err == nil {
		t.Errorf("Expected error for empty address, got nil")
	}

	// Test with valid config
	_, err = NewRedisCache(CacheConfig{
		Type:       "redis",
		Address:    "localhost:6379",
		DefaultTTL: 1 * time.Hour,
	})
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}
}
