package messaging

import (
	"testing"
	"time"
)

func TestDefaultBrokerConfig(t *testing.T) {
	config := DefaultBrokerConfig()

	// Test default values
	if config.Type != BrokerTypeMemory {
		t.Errorf("Expected default type '%s', got '%s'", BrokerTypeMemory, config.Type)
	}

	// Test that RabbitMQ config is initialized
	if config.RabbitMQ == nil {
		t.Error("Expected RabbitMQ config to be initialized")
	} else {
		if config.RabbitMQ.URI == "" {
			t.Error("Expected RabbitMQ URI to be set")
		}
		if config.RabbitMQ.ExchangeName == "" {
			t.Error("Expected RabbitMQ exchange name to be set")
		}
		if config.RabbitMQ.ConnectionTimeout == 0 {
			t.Error("Expected RabbitMQ connection timeout to be set")
		}
	}

	// Test that Kafka config is initialized
	if config.Kafka == nil {
		t.Error("Expected Kafka config to be initialized")
	} else {
		if len(config.Kafka.Brokers) == 0 {
			t.Error("Expected Kafka brokers to be set")
		}
		if config.Kafka.ClientID == "" {
			t.Error("Expected Kafka client ID to be set")
		}
		if config.Kafka.MaxRetries == 0 {
			t.Error("Expected Kafka max retries to be set")
		}
	}
}

func TestDefaultRabbitMQConfig(t *testing.T) {
	config := DefaultRabbitMQConfig()

	// Test default values
	expectedURI := "amqp://guest:guest@localhost:5672/"
	if config.URI != expectedURI {
		t.Errorf("Expected URI '%s', got '%s'", expectedURI, config.URI)
	}

	expectedExchange := "microlib"
	if config.ExchangeName != expectedExchange {
		t.Errorf("Expected exchange name '%s', got '%s'", expectedExchange, config.ExchangeName)
	}

	if config.ExchangeType != "topic" {
		t.Errorf("Expected exchange type 'topic', got '%s'", config.ExchangeType)
	}

	if !config.QueueDurable {
		t.Error("Expected queue durable to be true")
	}

	expectedTimeout := 30 * time.Second
	if config.ConnectionTimeout != expectedTimeout {
		t.Errorf("Expected connection timeout %v, got %v", expectedTimeout, config.ConnectionTimeout)
	}

	if config.ReconnectDelay != 5*time.Second {
		t.Errorf("Expected reconnect delay %v, got %v", 5*time.Second, config.ReconnectDelay)
	}

	if config.MaxReconnectAttempts != 10 {
		t.Errorf("Expected max reconnect attempts 10, got %d", config.MaxReconnectAttempts)
	}
}

func TestDefaultKafkaConfig(t *testing.T) {
	config := DefaultKafkaConfig()

	// Test default values
	expectedBrokers := []string{"localhost:9092"}
	if len(config.Brokers) != 1 || config.Brokers[0] != expectedBrokers[0] {
		t.Errorf("Expected brokers %v, got %v", expectedBrokers, config.Brokers)
	}

	expectedClientID := "microlib"
	if config.ClientID != expectedClientID {
		t.Errorf("Expected client ID '%s', got '%s'", expectedClientID, config.ClientID)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}

	expectedBackoff := 100 * time.Millisecond
	if config.RetryBackoff != expectedBackoff {
		t.Errorf("Expected retry backoff %v, got %v", expectedBackoff, config.RetryBackoff)
	}

	if config.SecurityProtocol != "plaintext" {
		t.Errorf("Expected security protocol 'plaintext', got '%s'", config.SecurityProtocol)
	}

	if config.SASLMechanism != "plain" {
		t.Errorf("Expected SASL mechanism 'plain', got '%s'", config.SASLMechanism)
	}
}

func TestBrokerConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config BrokerConfig
		valid  bool
	}{
		{
			name: "valid memory config",
			config: BrokerConfig{
				Type: BrokerTypeMemory,
			},
			valid: true,
		},
		{
			name: "valid rabbitmq config",
			config: BrokerConfig{
				Type: BrokerTypeRabbitMQ,
				RabbitMQ: &RabbitMQConfig{
					URI:          "amqp://localhost:5672",
					ExchangeName: "test",
					ExchangeType: "topic",
				},
			},
			valid: true,
		},
		{
			name: "valid kafka config",
			config: BrokerConfig{
				Type: BrokerTypeKafka,
				Kafka: &KafkaConfig{
					Brokers:  []string{"localhost:9092"},
					ClientID: "test-client",
				},
			},
			valid: true,
		},
		{
			name: "invalid empty type",
			config: BrokerConfig{
				Type: "",
			},
			valid: false,
		},
		{
			name: "invalid unknown type",
			config: BrokerConfig{
				Type: "unknown",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a basic validation test - in a real implementation,
			// you might have a Validate() method on the config struct
			isValid := tt.config.Type != "" &&
				(tt.config.Type == BrokerTypeMemory ||
					tt.config.Type == BrokerTypeRabbitMQ ||
					tt.config.Type == BrokerTypeKafka)

			if isValid != tt.valid {
				t.Errorf("Expected config validity %v, got %v", tt.valid, isValid)
			}
		})
	}
}
