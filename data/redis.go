package data

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client     *redis.Client
	config     CacheConfig
	defaultTTL time.Duration
	namespace  string
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config CacheConfig) (*RedisCache, error) {
	if config.Type != "redis" {
		return nil, errors.New("cache: config type must be 'redis'")
	}

	if config.Address == "" {
		return nil, errors.New("cache: Redis address is required")
	}

	// Set default values if not provided
	if config.PoolSize <= 0 {
		config.PoolSize = 10
	}

	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 1 * time.Hour
	}

	// Create Redis client options
	options := &redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.Database,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
	}

	if config.MaxConnAge > 0 {
		options.ConnMaxLifetime = config.MaxConnAge
	}

	if config.PoolTimeout > 0 {
		options.PoolTimeout = config.PoolTimeout
	}

	if config.IdleTimeout > 0 {
		options.ConnMaxIdleTime = config.IdleTimeout
	}

	// Create Redis client
	client := redis.NewClient(options)

	// Create cache instance
	cache := &RedisCache{
		client:     client,
		config:     config,
		defaultTTL: config.DefaultTTL,
		namespace:  config.Namespace,
	}

	return cache, nil
}

// Get retrieves a value from the cache
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	formattedKey := formatKey(c.namespace, key)
	val, err := c.client.Get(ctx, formattedKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	return val, nil
}

// Set stores a value in the cache with the default TTL
func (c *RedisCache) Set(ctx context.Context, key string, value []byte) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL
func (c *RedisCache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := validateKey(key); err != nil {
		return err
	}

	formattedKey := formatKey(c.namespace, key)
	return c.client.Set(ctx, formattedKey, value, ttl).Err()
}

// Delete removes a value from the cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	formattedKey := formatKey(c.namespace, key)
	return c.client.Del(ctx, formattedKey).Err()
}

// Clear removes all values from the cache with the current namespace
func (c *RedisCache) Clear(ctx context.Context) error {
	if c.namespace == "" {
		return errors.New("cache: cannot clear cache without namespace")
	}

	// Use scan to find all keys with the namespace prefix
	pattern := c.namespace + ":*"
	var cursor uint64
	var keys []string

	for {
		var batch []string
		var err error
		batch, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		keys = append(keys, batch...)

		if cursor == 0 {
			break
		}
	}

	// Delete all found keys
	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}

	return nil
}

// GetMulti retrieves multiple values from the cache
func (c *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// Format keys with namespace
	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		if err := validateKey(key); err != nil {
			return nil, err
		}
		formattedKeys[i] = formatKey(c.namespace, key)
	}

	// Get values in a pipeline
	pipe := c.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(formattedKeys))
	for i, key := range formattedKeys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Process results
	result := make(map[string][]byte)
	for i, cmd := range cmds {
		val, err := cmd.Bytes()
		if err == nil {
			// Remove namespace from key when adding to result
			originalKey := keys[i]
			result[originalKey] = val
		}
	}

	return result, nil
}

// SetMulti stores multiple values in the cache with the default TTL
func (c *RedisCache) SetMulti(ctx context.Context, items map[string][]byte) error {
	return c.SetMultiWithTTL(ctx, items, c.defaultTTL)
}

// SetMultiWithTTL stores multiple values in the cache with a specific TTL
func (c *RedisCache) SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for key, value := range items {
		if err := validateKey(key); err != nil {
			return err
		}
		formattedKey := formatKey(c.namespace, key)
		pipe.Set(ctx, formattedKey, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// DeleteMulti removes multiple values from the cache
func (c *RedisCache) DeleteMulti(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// Format keys with namespace
	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		if err := validateKey(key); err != nil {
			return err
		}
		formattedKeys[i] = formatKey(c.namespace, key)
	}

	return c.client.Del(ctx, formattedKeys...).Err()
}

// Increment atomically increments a numeric value by the given amount
func (c *RedisCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	if err := validateKey(key); err != nil {
		return 0, err
	}

	formattedKey := formatKey(c.namespace, key)
	return c.client.IncrBy(ctx, formattedKey, amount).Result()
}

// Decrement atomically decrements a numeric value by the given amount
func (c *RedisCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	if err := validateKey(key); err != nil {
		return 0, err
	}

	formattedKey := formatKey(c.namespace, key)
	return c.client.DecrBy(ctx, formattedKey, amount).Result()
}

// Ping checks if the cache is reachable
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the cache connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// RedisCacheProvider implements the CacheProvider interface for Redis
type RedisCacheProvider struct{}

// NewCache creates a new Redis cache instance
func (p *RedisCacheProvider) NewCache(config CacheConfig) (Cache, error) {
	return NewRedisCache(config)
}

// NewRedisCacheProvider creates a new Redis cache provider
func NewRedisCacheProvider() *RedisCacheProvider {
	return &RedisCacheProvider{}
}
