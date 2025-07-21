package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/santif/microlib/observability"
)

// VaultSecretStore implements SecretStore using HashiCorp Vault
type VaultSecretStore struct {
	config     SecretConfig
	client     *http.Client
	token      string
	logger     observability.Logger
	baseURL    string
	basePath   string
	namespace  string
	expiration time.Time
}

// vaultAuthResponse represents the response from Vault's auth endpoints
type vaultAuthResponse struct {
	Auth struct {
		ClientToken   string            `json:"client_token"`
		Accessor      string            `json:"accessor"`
		Policies      []string          `json:"policies"`
		TokenPolicies []string          `json:"token_policies"`
		Metadata      map[string]string `json:"metadata"`
		LeaseDuration int               `json:"lease_duration"`
		Renewable     bool              `json:"renewable"`
	} `json:"auth"`
}

// vaultSecretResponse represents the response from Vault's secret endpoints
type vaultSecretResponse struct {
	Data struct {
		Data     map[string]interface{} `json:"data"`
		Metadata struct {
			CreatedTime  string `json:"created_time"`
			DeletionTime string `json:"deletion_time"`
			Destroyed    bool   `json:"destroyed"`
			Version      int    `json:"version"`
		} `json:"metadata"`
	} `json:"data"`
}

// vaultListResponse represents the response from Vault's list endpoints
type vaultListResponse struct {
	Data struct {
		Keys []string `json:"keys"`
	} `json:"data"`
}

// NewVaultSecretStore creates a new Vault secret store
func NewVaultSecretStore(ctx context.Context, config SecretConfig, logger observability.Logger) (*VaultSecretStore, error) {
	if config.Address == "" {
		return nil, errors.New("vault address is required")
	}

	// Ensure address has scheme
	address := config.Address
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "https://" + address
	}

	// Create the store
	store := &VaultSecretStore{
		config:    config,
		client:    &http.Client{Timeout: 10 * time.Second},
		logger:    logger,
		baseURL:   address,
		basePath:  config.Path,
		namespace: config.Namespace,
	}

	// Authenticate
	if err := store.authenticate(ctx); err != nil {
		return nil, fmt.Errorf("vault authentication failed: %w", err)
	}

	return store, nil
}

// authenticate authenticates with Vault using the configured method
func (v *VaultSecretStore) authenticate(ctx context.Context) error {
	var err error
	var token string

	switch v.config.Auth.Method {
	case "token":
		token = v.config.Auth.Token
		if token == "" {
			return errors.New("token authentication requires a token")
		}
	case "approle":
		token, err = v.authenticateAppRole(ctx)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported authentication method: %s", v.config.Auth.Method)
	}

	v.token = token
	// Set token expiration to 80% of lease duration or 1 hour if not renewable
	v.expiration = time.Now().Add(1 * time.Hour)

	return nil
}

// authenticateAppRole authenticates with Vault using AppRole
func (v *VaultSecretStore) authenticateAppRole(ctx context.Context) (string, error) {
	if v.config.Auth.RoleID == "" || v.config.Auth.SecretID == "" {
		return "", errors.New("approle authentication requires role_id and secret_id")
	}

	// Prepare request body
	reqBody := map[string]string{
		"role_id":   v.config.Auth.RoleID,
		"secret_id": v.config.Auth.SecretID,
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/v1/auth/approle/login", v.baseURL),
		strings.NewReader(string(reqData)),
	)
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault authentication failed with status: %s", resp.Status)
	}

	// Parse response
	var authResp vaultAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	// Set token expiration based on lease duration
	if authResp.Auth.LeaseDuration > 0 {
		// Set to 80% of lease duration to allow for renewal
		v.expiration = time.Now().Add(time.Duration(authResp.Auth.LeaseDuration*8/10) * time.Second)
	}

	return authResp.Auth.ClientToken, nil
}

// ensureAuthenticated ensures the store has a valid token
func (v *VaultSecretStore) ensureAuthenticated(ctx context.Context) error {
	if v.token == "" || time.Now().After(v.expiration) {
		return v.authenticate(ctx)
	}
	return nil
}

// GetSecret retrieves a secret by key
func (v *VaultSecretStore) GetSecret(ctx context.Context, key string) ([]byte, error) {
	if err := v.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/v1/%s/data/%s", v.baseURL, v.basePath, key),
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("X-Vault-Token", v.token)
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrSecretNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault request failed with status: %s", resp.Status)
	}

	// Parse response
	var secretResp vaultSecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
		return nil, err
	}

	// Extract the value
	value, ok := secretResp.Data.Data["value"]
	if !ok {
		return nil, ErrInvalidSecretValue
	}

	// Convert to string then bytes
	strValue, ok := value.(string)
	if !ok {
		return nil, ErrInvalidSecretValue
	}

	return []byte(strValue), nil
}

// SetSecret stores a secret with optional TTL
func (v *VaultSecretStore) SetSecret(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := v.ensureAuthenticated(ctx); err != nil {
		return err
	}

	// Prepare request body
	reqBody := map[string]interface{}{
		"data": map[string]string{
			"value": string(value),
		},
	}

	// Add options if TTL is specified
	if ttl > 0 {
		reqBody["options"] = map[string]interface{}{
			"ttl": ttl.String(),
		}
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/v1/%s/data/%s", v.baseURL, v.basePath, key),
		strings.NewReader(string(reqData)),
	)
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", v.token)
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault request failed with status: %s", resp.Status)
	}

	return nil
}

// DeleteSecret removes a secret
func (v *VaultSecretStore) DeleteSecret(ctx context.Context, key string) error {
	if err := v.ensureAuthenticated(ctx); err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"DELETE",
		fmt.Sprintf("%s/v1/%s/data/%s", v.baseURL, v.basePath, key),
		nil,
	)
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("X-Vault-Token", v.token)
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault request failed with status: %s", resp.Status)
	}

	return nil
}

// ListSecrets lists all secret keys with optional prefix
func (v *VaultSecretStore) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	if err := v.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	// Create request
	listPath := ""
	if prefix != "" {
		listPath = prefix
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"LIST", // Vault uses LIST method for listing
		fmt.Sprintf("%s/v1/%s/metadata/%s", v.baseURL, v.basePath, listPath),
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("X-Vault-Token", v.token)
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault request failed with status: %s", resp.Status)
	}

	// Parse response
	var listResp vaultListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	// Process keys
	keys := make([]string, 0, len(listResp.Data.Keys))
	for _, key := range listResp.Data.Keys {
		// If it ends with /, it's a directory
		if strings.HasSuffix(key, "/") {
			// Recursively list keys in this directory
			subKeys, err := v.ListSecrets(ctx, path.Join(prefix, key))
			if err != nil {
				return nil, err
			}
			keys = append(keys, subKeys...)
		} else {
			// It's a key
			if prefix != "" {
				keys = append(keys, path.Join(prefix, key))
			} else {
				keys = append(keys, key)
			}
		}
	}

	return keys, nil
}

// Health checks if the secret store is available
func (v *VaultSecretStore) Health(ctx context.Context) error {
	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/v1/sys/health", v.baseURL),
		nil,
	)
	if err != nil {
		return err
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vault health check failed with status: %s", resp.Status)
	}

	return nil
}

// Close closes the connection to the secret store
func (v *VaultSecretStore) Close() error {
	// Nothing to close for HTTP client
	return nil
}
