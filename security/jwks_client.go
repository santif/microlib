package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/santif/microlib/observability"
)

// JWKSClient is a client for fetching and caching JSON Web Key Sets (JWKS)
type JWKSClient struct {
	endpoint      string
	keys          map[string]interface{}
	mu            sync.RWMutex
	httpClient    *http.Client
	logger        observability.Logger
	refreshTicker *time.Ticker
	lastRefresh   time.Time
	stopCh        chan struct{}
}

// JWKSClientOption is a function that configures a JWKSClient
type JWKSClientOption func(*JWKSClient)

// WithHTTPClient sets the HTTP client for the JWKS client
func WithHTTPClient(client *http.Client) JWKSClientOption {
	return func(c *JWKSClient) {
		c.httpClient = client
	}
}

// WithRefreshInterval sets the refresh interval for the JWKS client
func WithRefreshInterval(interval time.Duration) JWKSClientOption {
	return func(c *JWKSClient) {
		if interval > 0 {
			c.refreshTicker = time.NewTicker(interval)
		}
	}
}

// NewJWKSClient creates a new JWKS client
func NewJWKSClient(endpoint string, logger observability.Logger, opts ...JWKSClientOption) (*JWKSClient, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("JWKS endpoint is required")
	}

	client := &JWKSClient{
		endpoint: endpoint,
		keys:     make(map[string]interface{}),
		logger:   logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Fetch keys initially
	if err := client.refreshKeys(); err != nil {
		return nil, fmt.Errorf("failed to fetch initial JWKS: %w", err)
	}

	// Start background refresh if ticker is set
	if client.refreshTicker != nil {
		go client.startKeyRefresher()
	}

	return client, nil
}

// startKeyRefresher starts a background goroutine to refresh the JWKS keys
func (c *JWKSClient) startKeyRefresher() {
	for {
		select {
		case <-c.refreshTicker.C:
			if err := c.refreshKeys(); err != nil {
				c.logger.Error("Failed to refresh JWKS keys", err,
					observability.NewField("endpoint", c.endpoint))
			}
		case <-c.stopCh:
			c.refreshTicker.Stop()
			return
		}
	}
}

// Stop stops the background key refresher
func (c *JWKSClient) Stop() {
	if c.refreshTicker != nil {
		close(c.stopCh)
	}
}

// refreshKeys fetches the latest keys from the JWKS endpoint
func (c *JWKSClient) refreshKeys() error {
	c.logger.Info("Refreshing JWKS keys",
		observability.NewField("endpoint", c.endpoint))

	// Make HTTP request to JWKS endpoint
	resp, err := c.httpClient.Get(c.endpoint)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status code %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: failed to read response body: %v", ErrJWKSFetchFailed, err)
	}

	// Parse JWKS
	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("%w: failed to parse JWKS: %v", ErrJWKSFetchFailed, err)
	}

	// Process keys
	newKeys := make(map[string]interface{})
	for _, key := range jwks.Keys {
		if key.Use == "sig" {
			var publicKey interface{}
			var err error

			switch key.Kty {
			case "RSA":
				if len(key.X5c) > 0 {
					// Convert X5c (X.509 certificate chain) to PEM format
					certData := "-----BEGIN CERTIFICATE-----\n" + key.X5c[0] + "\n-----END CERTIFICATE-----"
					publicKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(certData))
				} else if key.N != "" && key.E != "" {
					// Parse from modulus and exponent
					publicKey, err = parseRSAPublicKeyFromJWK(key.N, key.E)
				} else {
					c.logger.Error("RSA key missing required parameters", nil,
						observability.NewField("kid", key.Kid))
					continue
				}
			case "EC":
				// Support for EC keys could be added here
				c.logger.Error("EC keys not yet supported", nil,
					observability.NewField("kid", key.Kid))
				continue
			default:
				c.logger.Error("Unsupported key type", nil,
					observability.NewField("kid", key.Kid),
					observability.NewField("kty", key.Kty))
				continue
			}

			if err != nil {
				c.logger.Error("Failed to parse public key", err,
					observability.NewField("kid", key.Kid),
					observability.NewField("kty", key.Kty))
				continue
			}

			newKeys[key.Kid] = publicKey
		}
	}

	// Update keys
	c.mu.Lock()
	c.keys = newKeys
	c.lastRefresh = time.Now()
	c.mu.Unlock()

	c.logger.Info("JWKS keys refreshed successfully",
		observability.NewField("key_count", len(newKeys)))

	return nil
}

// GetKey returns the key for the given key ID
func (c *JWKSClient) GetKey(kid string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key, ok := c.keys[kid]
	if !ok {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

// ForceRefresh forces a refresh of the JWKS keys
func (c *JWKSClient) ForceRefresh(ctx context.Context) error {
	return c.refreshKeys()
}

// LastRefresh returns the time of the last successful refresh
func (c *JWKSClient) LastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}

// KeyCount returns the number of keys in the JWKS
func (c *JWKSClient) KeyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.keys)
}
