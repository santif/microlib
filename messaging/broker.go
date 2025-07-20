package messaging

import (
	"context"
	"fmt"

	"github.com/santif/microlib/observability"
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

// NewBroker creates a new message broker based on the provided configuration
func NewBroker(
	config *BrokerConfig,
	logger observability.Logger,
	metrics observability.Metrics,
) (Broker, error) {
	if logger == nil {
		logger = observability.NewLogger()
	}

	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	switch config.Type {
	case BrokerTypeMemory:
		return NewMemoryBroker(), nil
	case BrokerTypeRabbitMQ:
		if config.RabbitMQ == nil {
			return nil, fmt.Errorf("RabbitMQ configuration is required for RabbitMQ broker")
		}
		return NewRabbitMQBroker(config.RabbitMQ, logger, metrics)
	case BrokerTypeKafka:
		if config.Kafka == nil {
			return nil, fmt.Errorf("Kafka configuration is required for Kafka broker")
		}
		return NewKafkaBroker(config.Kafka, logger, metrics)
	default:
		return nil, fmt.Errorf("unsupported broker type: %s", config.Type)
	}
}
