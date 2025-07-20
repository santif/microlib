package messaging

import (
	"testing"
)

func TestDefaultMessage(t *testing.T) {
	// Test data
	id := "msg-123"
	body := []byte("test message body")
	headers := map[string]string{
		"content-type":   "application/json",
		"correlation-id": "corr-456",
	}

	// Create a new message
	msg := NewMessage(id, body, headers)

	// Test ID
	if msg.ID() != id {
		t.Errorf("Expected ID %s, got %s", id, msg.ID())
	}

	// Test Body
	if string(msg.Body()) != string(body) {
		t.Errorf("Expected body %s, got %s", string(body), string(msg.Body()))
	}

	// Test Headers
	if len(msg.Headers()) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(headers), len(msg.Headers()))
	}

	for k, v := range headers {
		if msg.Headers()[k] != v {
			t.Errorf("Expected header %s=%s, got %s=%s", k, v, k, msg.Headers()[k])
		}
	}
}

func TestNewMessageWithNilHeaders(t *testing.T) {
	// Test creating a message with nil headers
	msg := NewMessage("id-1", []byte("content"), nil)

	// Headers should be initialized as an empty map, not nil
	if msg.Headers() == nil {
		t.Error("Headers should not be nil when initialized with nil")
	}

	if len(msg.Headers()) != 0 {
		t.Errorf("Expected empty headers, got %d items", len(msg.Headers()))
	}
}
