package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
)

func TestNewJWKSClient(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Test with empty endpoint
	_, err := NewJWKSClient("", logger)
	if err == nil {
		t.Errorf("Expected error for empty endpoint, got nil")
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Test with valid endpoint
	client, err := NewJWKSClient(server.URL, logger)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if client == nil {
		t.Errorf("Expected client to be non-nil")
	}

	// Test with refresh interval
	client, err = NewJWKSClient(
		server.URL,
		logger,
		WithRefreshInterval(1*time.Hour),
	)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if client == nil {
		t.Errorf("Expected client to be non-nil")
	}
	if client.refreshTicker == nil {
		t.Errorf("Expected refreshTicker to be non-nil")
	}

	// Clean up
	client.Stop()
}

func TestJWKSClient_GetKey(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Create client
	client, err := NewJWKSClient(server.URL, logger)
	if err != nil {
		t.Fatalf("Failed to create JWKS client: %v", err)
	}

	// Test getting existing key
	key, err := client.GetKey("test-key-1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if key == nil {
		t.Errorf("Expected key to be non-nil")
	}

	// Test getting non-existent key
	key, err = client.GetKey("non-existent-key")
	if err == nil {
		t.Errorf("Expected error for non-existent key, got nil")
	}
	if key != nil {
		t.Errorf("Expected key to be nil, got %v", key)
	}
}

func TestJWKSClient_ForceRefresh(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create a test server
	refreshCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount++
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Create client
	client, err := NewJWKSClient(server.URL, logger)
	if err != nil {
		t.Fatalf("Failed to create JWKS client: %v", err)
	}

	// Initial refresh should have happened
	if refreshCount != 1 {
		t.Errorf("Expected refresh count to be 1, got %d", refreshCount)
	}

	// Force refresh
	err = client.ForceRefresh(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Refresh count should be 2
	if refreshCount != 2 {
		t.Errorf("Expected refresh count to be 2, got %d", refreshCount)
	}
}

func TestJWKSClient_LastRefresh(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Create client
	client, err := NewJWKSClient(server.URL, logger)
	if err != nil {
		t.Fatalf("Failed to create JWKS client: %v", err)
	}

	// Last refresh should be recent
	lastRefresh := client.LastRefresh()
	if lastRefresh.IsZero() {
		t.Errorf("Expected last refresh to be non-zero")
	}
	if time.Since(lastRefresh) > 5*time.Second {
		t.Errorf("Expected last refresh to be recent, got %v", lastRefresh)
	}
}

func TestJWKSClient_KeyCount(t *testing.T) {
	// Create a mock logger
	logger := observability.NewNoOpLogger()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid JWKS response
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-key-1",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
				{
					Kid: "test-key-2",
					Kty: "RSA",
					Use: "sig",
					N:   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E:   "AQAB",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Create client
	client, err := NewJWKSClient(server.URL, logger)
	if err != nil {
		t.Fatalf("Failed to create JWKS client: %v", err)
	}

	// Key count should be 2
	if client.KeyCount() != 2 {
		t.Errorf("Expected key count to be 2, got %d", client.KeyCount())
	}
}
