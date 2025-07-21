package security

import (
	"context"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemorySecretStore tests the in-memory secret store implementation
func TestMemorySecretStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySecretStore()

	// Test setting and getting a secret
	err := store.SetSecret(ctx, "test-key", []byte("test-value"), 0)
	require.NoError(t, err)

	value, err := store.GetSecret(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), value)

	// Test getting a non-existent secret
	_, err = store.GetSecret(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrSecretNotFound)

	// Test listing secrets
	keys, err := store.ListSecrets(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, keys, "test-key")

	// Test listing with prefix
	err = store.SetSecret(ctx, "prefix-key", []byte("prefix-value"), 0)
	require.NoError(t, err)

	keys, err = store.ListSecrets(ctx, "prefix")
	require.NoError(t, err)
	assert.Contains(t, keys, "prefix-key")
	assert.NotContains(t, keys, "test-key")

	// Test deleting a secret
	err = store.DeleteSecret(ctx, "test-key")
	require.NoError(t, err)

	_, err = store.GetSecret(ctx, "test-key")
	assert.ErrorIs(t, err, ErrSecretNotFound)

	// Test health check
	err = store.Health(ctx)
	assert.NoError(t, err)

	// Test close
	err = store.Close()
	assert.NoError(t, err)
}

// TestCachedSecretStore tests the cached secret store implementation
func TestCachedSecretStore(t *testing.T) {
	ctx := context.Background()
	memStore := NewMemorySecretStore()
	logger := observability.NewNoOpLogger()

	// Create cached store with short TTL for testing
	store := NewCachedSecretStore(memStore, 100*time.Millisecond, 50*time.Millisecond, logger)

	// Test setting and getting a secret
	err := store.SetSecret(ctx, "test-key", []byte("test-value"), 0)
	require.NoError(t, err)

	// Get from cache
	value, err := store.GetSecret(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), value)

	// Modify underlying store directly
	err = memStore.SetSecret(ctx, "test-key", []byte("modified-value"), 0)
	require.NoError(t, err)

	// Should still get cached value
	value, err = store.GetSecret(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), value)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Should get updated value
	value, err = store.GetSecret(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("modified-value"), value)

	// Test deleting a secret
	err = store.DeleteSecret(ctx, "test-key")
	require.NoError(t, err)

	// Should be removed from cache immediately
	_, err = store.GetSecret(ctx, "test-key")
	assert.ErrorIs(t, err, ErrSecretNotFound)

	// Test close
	err = store.Close()
	assert.NoError(t, err)
}

// TestNewSecretStore tests the factory function for creating secret stores
func TestNewSecretStore(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewNoOpLogger()

	// Test creating memory store
	config := DefaultSecretConfig()
	store, err := NewSecretStore(ctx, config, logger)
	require.NoError(t, err)
	assert.IsType(t, &CachedSecretStore{}, store)

	// Test with cache disabled
	config.CacheTTL = 0
	store, err = NewSecretStore(ctx, config, logger)
	require.NoError(t, err)
	assert.IsType(t, &MemorySecretStore{}, store)

	// Test with unsupported provider
	config.Provider = "unsupported"
	_, err = NewSecretStore(ctx, config, logger)
	assert.Error(t, err)
}

// TestSecretStoreWithRefresh tests the automatic refresh of cached secrets
func TestSecretStoreWithRefresh(t *testing.T) {
	ctx := context.Background()
	memStore := NewMemorySecretStore()
	logger := observability.NewNoOpLogger()

	// Create cached store with short refresh interval
	store := NewCachedSecretStore(memStore, 500*time.Millisecond, 100*time.Millisecond, logger)

	// Set initial value
	err := store.SetSecret(ctx, "refresh-key", []byte("initial-value"), 0)
	require.NoError(t, err)

	// Get from cache
	value, err := store.GetSecret(ctx, "refresh-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("initial-value"), value)

	// Modify underlying store directly
	err = memStore.SetSecret(ctx, "refresh-key", []byte("refreshed-value"), 0)
	require.NoError(t, err)

	// Wait for refresh to happen
	time.Sleep(150 * time.Millisecond)

	// Should get refreshed value
	value, err = store.GetSecret(ctx, "refresh-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("refreshed-value"), value)

	// Test close
	err = store.Close()
	assert.NoError(t, err)
}
