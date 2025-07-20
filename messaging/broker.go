package messaging

import (
	"context"
)

// Broker defines the interface for a message broker that can publish and subscribe to topics
type Broker interface {
	// Publish sends a message to the specified topic
	Publish(ctx context.Context, topic string, message Message) error

	// Subscribe registers a handler function to be called when messages are received on the specified topic
	Subscribe(topic string, handler MessageHandler) error

	// Close shuts down the broker connection and cleans up resources
	Close() error
}

// MessageHandler is a function that processes received messages
type MessageHandler func(ctx context.Context, message Message) error
