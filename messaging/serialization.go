package messaging

import (
	"encoding/json"
	"fmt"
)

// Serializer defines the interface for message serialization/deserialization
type Serializer interface {
	// Serialize converts an object to bytes
	Serialize(v interface{}) ([]byte, error)

	// Deserialize converts bytes to an object
	Deserialize(data []byte, v interface{}) error
}

// JSONSerializer implements Serializer using JSON encoding
type JSONSerializer struct{}

// NewJSONSerializer creates a new JSON serializer
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize converts an object to JSON bytes
func (s *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize to JSON: %w", err)
	}
	return data, nil
}

// Deserialize converts JSON bytes to an object
func (s *JSONSerializer) Deserialize(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to deserialize from JSON: %w", err)
	}
	return nil
}
