package messaging

import (
	"context"
	"fmt"
	"sync"
)

// MemoryBroker is an in-memory implementation of the Broker interface
// primarily used for testing and simple use cases
type MemoryBroker struct {
	subscribers map[string][]MessageHandler
	mu          sync.RWMutex
	closed      bool
}

// NewMemoryBroker creates a new in-memory message broker
func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{
		subscribers: make(map[string][]MessageHandler),
	}
}

// Publish sends a message to all subscribers of the specified topic
func (b *MemoryBroker) Publish(ctx context.Context, topic string, message Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return fmt.Errorf("broker is closed")
	}

	handlers, exists := b.subscribers[topic]
	if !exists {
		// No subscribers for this topic, message is effectively dropped
		return nil
	}

	// Call each handler with the message
	for _, handler := range handlers {
		// We're intentionally calling handlers synchronously in the memory implementation
		// A real broker would likely handle this asynchronously
		err := handler(ctx, message)
		if err != nil {
			// In a real implementation, we might want to handle errors differently
			// For now, we'll just continue with other handlers
			continue
		}
	}

	return nil
}

// Subscribe registers a handler function for the specified topic
func (b *MemoryBroker) Subscribe(topic string, handler MessageHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("broker is closed")
	}

	b.subscribers[topic] = append(b.subscribers[topic], handler)
	return nil
}

// Close cleans up resources and prevents further message publishing/subscription
func (b *MemoryBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.subscribers = make(map[string][]MessageHandler)
	return nil
}
