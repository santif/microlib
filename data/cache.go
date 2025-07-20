package data

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Common cache errors
var (
	// ErrCacheMiss is returned when a key is not found in the cache
	ErrCacheMiss = errors.New("cache: key not found")

	// ErrCacheInvalidKey is returned when a key is invalid
	ErrCacheInvalidKey = errors.New("cache: invalid key")

	// ErrCacheConnectionFailed is returned when the connection to the cache fails
	ErrCacheConnectionFailed = errors.New("cache: connection failed")
)

// CacheConfig defines the configuration for a cache store
type CacheConfig struct {
	// Type specifies the cache implementation to use (e.g., "redis", "memory")
	Type string `validate:"required,oneof=redis memory"`

	// Address is the address of the cache server (e.g., "localhost:6379" for Redis)
	Address string `validate:"required_if=Type redis"`

	// Password is the password for the cache server
	Password string

	// Database is the database number to use (for Redis)
	Database int `validate:"gte=0"`

	// PoolSize is the maximum number of connections in the pool
	PoolSize int `validate:"gte=0"`

	// MinIdleConns is the minimum number of idle connections to maintain in the pool
	MinIdleConns int `validate:"gte=0"`

	// MaxConnAge is the maximum age of a connection in the pool
	MaxConnAge time.Duration

	// PoolTimeout is the timeout for getting a connection from the pool
	PoolTimeout time.Duration

	// IdleTimeout is the timeout for idle connections in the pool
	IdleTimeout time.Duration

	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL time.Duration `validate:"required"`

	// Namespace is the prefix to add to all cache keys
	Namespace string
}

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from the cache
	// Returns ErrCacheMiss if the key is not found
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with the default TTL
	Set(ctx context.Context, key string, value []byte) error

	// SetWithTTL stores a value in the cache with a specific TTL
	SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Clear removes all values from the cache with the current namespace
	Clear(ctx context.Context) error

	// GetMulti retrieves multiple values from the cache
	// The returned map will only contain entries for keys that were found
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)

	// SetMulti stores multiple values in the cache with the default TTL
	SetMulti(ctx context.Context, items map[string][]byte) error

	// SetMultiWithTTL stores multiple values in the cache with a specific TTL
	SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// DeleteMulti removes multiple values from the cache
	DeleteMulti(ctx context.Context, keys []string) error

	// Increment atomically increments a numeric value by the given amount
	// The value is treated as an int64, and is created if it doesn't exist
	Increment(ctx context.Context, key string, amount int64) (int64, error)

	// Decrement atomically decrements a numeric value by the given amount
	// The value is treated as an int64, and is created if it doesn't exist
	Decrement(ctx context.Context, key string, amount int64) (int64, error)

	// Ping checks if the cache is reachable
	Ping(ctx context.Context) error

	// Close closes the cache connection
	Close() error
}

// CacheProvider is an interface for creating cache instances
type CacheProvider interface {
	// NewCache creates a new cache instance with the given configuration
	NewCache(config CacheConfig) (Cache, error)
}

// validateKey checks if a cache key is valid
func validateKey(key string) error {
	if key == "" {
		return ErrCacheInvalidKey
	}
	return nil
}

// formatKey formats a key with the namespace
func formatKey(namespace, key string) string {
	if namespace == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", namespace, key)
}

// formatKeys formats multiple keys with the namespace
func formatKeys(namespace string, keys []string) []string {
	if namespace == "" {
		return keys
	}

	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		formattedKeys[i] = formatKey(namespace, key)
	}
	return formattedKeys
}
