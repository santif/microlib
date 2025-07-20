package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santif/microlib/observability"
	"github.com/stretchr/testify/assert"
)

func TestKafkaBrokerConfig(t *testing.T) {
	// This test only verifies the configuration structure, not actual connectivity
	config := DefaultKafkaConfig()

	assert.Equal(t, []string{"localhost:9092"}, config.Brokers)
	assert.Equal(t, "microlib", config.ClientID)
	assert.Equal(t, "microlib-consumer", config.ConsumerGroup)
	assert.Equal(t, "plaintext", config.SecurityProtocol)
	assert.Equal(t, "plain", config.SASLMechanism)
	assert.False(t, config.EnableTLS)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryBackoff)
}

func TestKafkaBrokerIntegration(t *testing.T) {
	// Skip this test unless explicitly enabled for integration testing
	if testing.Short() {
		t.Skip("Skipping Kafka integration test in short mode")
	}

	// This test requires a running Kafka instance
	// It's marked as an integration test and should be run separately

	config := DefaultKafkaConfig()
	logger := observability.NewLogger()
	metrics := observability.NewMetrics()

	broker, err := NewKafkaBroker(config, logger, metrics)
	if err != nil {
		t.Skipf("Skipping test, could not connect to Kafka: %v", err)
		return
	}
	defer broker.Close()

	// Test topic - use a unique name to avoid conflicts
	topic := "test.kafka." + uuid.New().String()

	// Message content
	messageID := uuid.New().String()
	messageBody := []byte(`{"test":"Kafka Integration Test"}`)
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
	time.Sleep(2 * time.Second)

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
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}
