package security

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/santif/microlib/observability"
)

// Common errors for secrets management
var (
	ErrSecretNotFound      = errors.New("secret not found")
	ErrSecretStoreNotReady = errors.New("secret store not ready")
	ErrInvalidSecretValue  = errors.New("invalid secret value")
	ErrSecretExpired       = errors.New("secret expired")
)

// SecretStore is the interface for external secret stores
type SecretStore interface {
	// GetSecret retrieves a secret by key
	GetSecret(ctx context.Context, key string) ([]byte, error)

	// SetSecret stores a secret with optional TTL
	SetSecret(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// DeleteSecret removes a secret
	DeleteSecret(ctx context.Context, key string) error

	// ListSecrets lists all secret keys with optional prefix
	ListSecrets(ctx context.Context, prefix string) ([]string, error)

	// Health checks if the secret store is available
	Health(ctx context.Context) error

	// Close closes the connection to the secret store
	Close() error
}

// SecretConfig contains configuration for secret stores
type SecretConfig struct {
	// Provider is the secret store provider (vault, aws, azure)
	Provider string `json:"provider" yaml:"provider" validate:"required,oneof=vault aws azure memory"`

	// Address is the address of the secret store
	Address string `json:"address" yaml:"address"`

	// Namespace is the namespace for secrets (e.g., vault namespace)
	Namespace string `json:"namespace" yaml:"namespace"`

	// Path is the base path for secrets
	Path string `json:"path" yaml:"path"`

	// CacheTTL is the TTL for the secret cache
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// RefreshInterval is the interval for refreshing cached secrets
	RefreshInterval time.Duration `json:"refresh_interval" yaml:"refresh_interval"`

	// Auth contains authentication configuration
	Auth SecretAuthConfig `json:"auth" yaml:"auth"`
}

// SecretAuthConfig contains authentication configuration for secret stores
type SecretAuthConfig struct {
	// Method is the authentication method (token, approle, iam, etc.)
	Method string `json:"method" yaml:"method"`

	// Token is the authentication token
	Token string `json:"token" yaml:"token"`

	// RoleID is the AppRole role ID (for Vault)
	RoleID string `json:"role_id" yaml:"role_id"`

	// SecretID is the AppRole secret ID (for Vault)
	SecretID string `json:"secret_id" yaml:"secret_id"`

	// ClientID is the client ID (for Azure)
	ClientID string `json:"client_id" yaml:"client_id"`

	// ClientSecret is the client secret (for Azure)
	ClientSecret string `json:"client_secret" yaml:"client_secret"`

	// ManagedIdentity indicates whether to use managed identity (for Azure)
	ManagedIdentity bool `json:"managed_identity" yaml:"managed_identity"`
}

// DefaultSecretConfig returns the default secret configuration
func DefaultSecretConfig() SecretConfig {
	return SecretConfig{
		Provider:        "memory",
		CacheTTL:        15 * time.Minute,
		RefreshInterval: 5 * time.Minute,
		Path:            "secrets",
	}
}

// cachedSecret represents a cached secret with metadata
type cachedSecret struct {
	value      []byte
	expiration time.Time
}

// CachedSecretStore wraps a SecretStore with caching capabilities
type CachedSecretStore struct {
	store          SecretStore
	cache          map[string]cachedSecret
	cacheTTL       time.Duration
	refreshTicker  *time.Ticker
	refreshKeys    map[string]bool
	mu             sync.RWMutex
	logger         observability.Logger
	refreshRunning bool
	stopCh         chan struct{}
}

// NewCachedSecretStore creates a new cached secret store
func NewCachedSecretStore(store SecretStore, cacheTTL, refreshInterval time.Duration, logger observability.Logger) *CachedSecretStore {
	css := &CachedSecretStore{
		store:       store,
		cache:       make(map[string]cachedSecret),
		cacheTTL:    cacheTTL,
		refreshKeys: make(map[string]bool),
		logger:      logger,
		stopCh:      make(chan struct{}),
	}

	// Start refresh ticker if interval is positive
	if refreshInterval > 0 {
		css.refreshTicker = time.NewTicker(refreshInterval)
		css.refreshRunning = true
		go css.refreshLoop()
	}

	return css
}

// refreshLoop periodically refreshes cached secrets
func (c *CachedSecretStore) refreshLoop() {
	for {
		select {
		case <-c.refreshTicker.C:
			c.refreshCachedSecrets()
		case <-c.stopCh:
			if c.refreshTicker != nil {
				c.refreshTicker.Stop()
			}
			return
		}
	}
}

// refreshCachedSecrets refreshes all cached secrets
func (c *CachedSecretStore) refreshCachedSecrets() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.mu.RLock()
	keysToRefresh := make([]string, 0, len(c.refreshKeys))
	for key := range c.refreshKeys {
		keysToRefresh = append(keysToRefresh, key)
	}
	c.mu.RUnlock()

	for _, key := range keysToRefresh {
		// Skip if the key is no longer in the cache
		c.mu.RLock()
		_, exists := c.cache[key]
		c.mu.RUnlock()
		if !exists {
			continue
		}

		// Refresh the secret
		value, err := c.store.GetSecret(ctx, key)
		if err != nil {
			c.logger.ErrorContext(ctx, "Failed to refresh secret", err,
				observability.NewField("key", key),
			)
			continue
		}

		// Update the cache
		c.mu.Lock()
		c.cache[key] = cachedSecret{
			value:      value,
			expiration: time.Now().Add(c.cacheTTL),
		}
		c.mu.Unlock()

		c.logger.DebugContext(ctx, "Secret refreshed",
			observability.NewField("key", key),
		)
	}
}

// GetSecret retrieves a secret by key, using cache if available
func (c *CachedSecretStore) GetSecret(ctx context.Context, key string) ([]byte, error) {
	// Check cache first
	c.mu.RLock()
	cached, found := c.cache[key]
	c.mu.RUnlock()

	if found && time.Now().Before(cached.expiration) {
		return cached.value, nil
	}

	// Cache miss or expired, get from store
	value, err := c.store.GetSecret(ctx, key)
	if err != nil {
		return nil, err
	}

	// Update cache
	c.mu.Lock()
	c.cache[key] = cachedSecret{
		value:      value,
		expiration: time.Now().Add(c.cacheTTL),
	}
	c.refreshKeys[key] = true
	c.mu.Unlock()

	return value, nil
}

// SetSecret stores a secret with optional TTL
func (c *CachedSecretStore) SetSecret(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.store.SetSecret(ctx, key, value, ttl); err != nil {
		return err
	}

	// Update cache
	c.mu.Lock()
	c.cache[key] = cachedSecret{
		value:      value,
		expiration: time.Now().Add(c.cacheTTL),
	}
	c.refreshKeys[key] = true
	c.mu.Unlock()

	return nil
}

// DeleteSecret removes a secret
func (c *CachedSecretStore) DeleteSecret(ctx context.Context, key string) error {
	if err := c.store.DeleteSecret(ctx, key); err != nil {
		return err
	}

	// Remove from cache
	c.mu.Lock()
	delete(c.cache, key)
	delete(c.refreshKeys, key)
	c.mu.Unlock()

	return nil
}

// ListSecrets lists all secret keys with optional prefix
func (c *CachedSecretStore) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	// This operation is not cached
	return c.store.ListSecrets(ctx, prefix)
}

// Health checks if the secret store is available
func (c *CachedSecretStore) Health(ctx context.Context) error {
	return c.store.Health(ctx)
}

// Close closes the connection to the secret store
func (c *CachedSecretStore) Close() error {
	// Stop the refresh loop
	if c.refreshRunning {
		close(c.stopCh)
		c.refreshRunning = false
	}

	// Clear the cache
	c.mu.Lock()
	c.cache = make(map[string]cachedSecret)
	c.refreshKeys = make(map[string]bool)
	c.mu.Unlock()

	// Close the underlying store
	return c.store.Close()
}

// NewSecretStore creates a new secret store based on the configuration
func NewSecretStore(ctx context.Context, config SecretConfig, logger observability.Logger) (SecretStore, error) {
	var store SecretStore
	var err error

	switch config.Provider {
	case "vault":
		store, err = NewVaultSecretStore(ctx, config, logger)
	case "aws":
		store, err = NewAWSSecretStore(ctx, config, logger)
	case "azure":
		store, err = NewAzureSecretStore(ctx, config, logger)
	case "memory":
		store = NewMemorySecretStore()
	default:
		return nil, errors.New("unsupported secret store provider")
	}

	if err != nil {
		return nil, err
	}

	// Wrap with cache if TTL is positive
	if config.CacheTTL > 0 {
		return NewCachedSecretStore(store, config.CacheTTL, config.RefreshInterval, logger), nil
	}

	return store, nil
}

// MemorySecretStore is an in-memory implementation of SecretStore for testing
type MemorySecretStore struct {
	secrets map[string][]byte
	mu      sync.RWMutex
}

// NewMemorySecretStore creates a new in-memory secret store
func NewMemorySecretStore() *MemorySecretStore {
	return &MemorySecretStore{
		secrets: make(map[string][]byte),
	}
}

// GetSecret retrieves a secret by key
func (m *MemorySecretStore) GetSecret(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.secrets[key]
	if !ok {
		return nil, ErrSecretNotFound
	}

	return value, nil
}

// SetSecret stores a secret with optional TTL
func (m *MemorySecretStore) SetSecret(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.secrets[key] = value
	return nil
}

// DeleteSecret removes a secret
func (m *MemorySecretStore) DeleteSecret(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.secrets, key)
	return nil
}

// ListSecrets lists all secret keys with optional prefix
func (m *MemorySecretStore) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.secrets))
	for key := range m.secrets {
		if prefix == "" || (len(key) >= len(prefix) && key[:len(prefix)] == prefix) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Health checks if the secret store is available
func (m *MemorySecretStore) Health(ctx context.Context) error {
	return nil // Always healthy
}

// Close closes the connection to the secret store
func (m *MemorySecretStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.secrets = make(map[string][]byte)
	return nil
}
