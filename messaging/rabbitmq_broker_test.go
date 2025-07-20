package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santif/microlib/observability"
	"github.com/stretchr/testify/assert"
)

func TestRabbitMQBrokerConfig(t *testing.T) {
	// This test only verifies the configuration structure, not actual connectivity
	config := DefaultRabbitMQConfig()

	assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URI)
	assert.Equal(t, "microlib", config.ExchangeName)
	assert.Equal(t, "topic", config.ExchangeType)
	assert.True(t, config.QueueDurable)
	assert.False(t, config.QueueAutoDelete)
	assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
	assert.Equal(t, 5*time.Second, config.ReconnectDelay)
	assert.Equal(t, 10, config.MaxReconnectAttempts)
}

func TestRabbitMQBrokerIntegration(t *testing.T) {
	// Skip this test unless explicitly enabled for integration testing
	if testing.Short() {
		t.Skip("Skipping RabbitMQ integration test in short mode")
	}

	// This test requires a running RabbitMQ instance
	// It's marked as an integration test and should be run separately

	config := DefaultRabbitMQConfig()
	logger := observability.NewLogger()
	metrics := observability.NewMetrics()

	broker, err := NewRabbitMQBroker(config, logger, metrics)
	if err != nil {
		t.Skipf("Skipping test, could not connect to RabbitMQ: %v", err)
		return
	}
	defer broker.Close()

	// Test topic
	topic := "test.rabbitmq." + uuid.New().String()

	// Message content
	messageID := uuid.New().String()
	messageBody := []byte(`{"test":"RabbitMQ Integration Test"}`)
	messageHeaders := map[string]string{
		"content-type": "application/json",
		"test-header":  "test-value",
	}

	// Create message
	message := NewMessage(messageID, messageBody, messageHeaders)

	// Channel to receive the message
	received := make(chan Message, 1)

	// Subscribe to the topic
	err = broker.Subscribe(topic, func(ctx context.Context, msg Message) error {
		received <- msg
		return nil
	})
	assert.NoError(t, err)

	// Wait a moment for the subscription to be established
	time.Sleep(1 * time.Second)

	// Publish the message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = broker.Publish(ctx, topic, message)
	assert.NoError(t, err)

	// Wait for the message to be received
	select {
	case msg := <-received:
		assert.Equal(t, messageID, msg.ID())
		assert.Equal(t, messageBody, msg.Body())
		assert.Equal(t, messageHeaders["content-type"], msg.Headers()["content-type"])
		assert.Equal(t, messageHeaders["test-header"], msg.Headers()["test-header"])
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}
