package messaging

import (
	"time"
)

// BrokerType defines the type of message broker to use
type BrokerType string

const (
	// BrokerTypeMemory is an in-memory broker for testing
	BrokerTypeMemory BrokerType = "memory"
	// BrokerTypeRabbitMQ is a RabbitMQ broker
	BrokerTypeRabbitMQ BrokerType = "rabbitmq"
	// BrokerTypeKafka is a Kafka broker
	BrokerTypeKafka BrokerType = "kafka"
)

// BrokerConfig holds common configuration for message brokers
type BrokerConfig struct {
	// Type specifies which broker implementation to use
	Type BrokerType `yaml:"type" validate:"required,oneof=memory rabbitmq kafka"`

	// RabbitMQ holds RabbitMQ-specific configuration
	RabbitMQ *RabbitMQConfig `yaml:"rabbitmq,omitempty"`

	// Kafka holds Kafka-specific configuration
	Kafka *KafkaConfig `yaml:"kafka,omitempty"`
}

// RabbitMQConfig holds configuration for RabbitMQ broker
type RabbitMQConfig struct {
	// URI is the connection URI for RabbitMQ
	URI string `yaml:"uri" validate:"required"`

	// ExchangeName is the default exchange to use
	ExchangeName string `yaml:"exchangeName" validate:"required"`

	// ExchangeType is the type of exchange (direct, fanout, topic, headers)
	ExchangeType string `yaml:"exchangeType" validate:"required,oneof=direct fanout topic headers"`

	// QueueDurable determines if queues should survive broker restarts
	QueueDurable bool `yaml:"queueDurable" default:"true"`

	// QueueAutoDelete determines if queues should be deleted when no longer used
	QueueAutoDelete bool `yaml:"queueAutoDelete" default:"false"`

	// ConnectionTimeout is the timeout for establishing a connection
	ConnectionTimeout time.Duration `yaml:"connectionTimeout" default:"30s"`

	// ReconnectDelay is the delay between reconnection attempts
	ReconnectDelay time.Duration `yaml:"reconnectDelay" default:"5s"`

	// MaxReconnectAttempts is the maximum number of reconnection attempts (0 = infinite)
	MaxReconnectAttempts int `yaml:"maxReconnectAttempts" default:"10"`
}

// KafkaConfig holds configuration for Kafka broker
type KafkaConfig struct {
	// Brokers is a list of Kafka broker addresses
	Brokers []string `yaml:"brokers" validate:"required,min=1"`

	// ClientID is the client identifier
	ClientID string `yaml:"clientID" validate:"required"`

	// ConsumerGroup is the consumer group ID for subscribers
	ConsumerGroup string `yaml:"consumerGroup" validate:"required"`

	// SecurityProtocol defines the protocol used to communicate with brokers
	// (plaintext, ssl, sasl_plaintext, sasl_ssl)
	SecurityProtocol string `yaml:"securityProtocol" default:"plaintext"`

	// SASL mechanism if using SASL (plain, scram-sha-256, scram-sha-512)
	SASLMechanism string `yaml:"saslMechanism" default:"plain"`

	// SASL username for authentication
	SASLUsername string `yaml:"saslUsername"`

	// SASL password for authentication
	SASLPassword string `yaml:"saslPassword"`

	// EnableTLS enables TLS for secure connections
	EnableTLS bool `yaml:"enableTLS" default:"false"`

	// TLSCertFile is the path to the client certificate file
	TLSCertFile string `yaml:"tlsCertFile"`

	// TLSKeyFile is the path to the client key file
	TLSKeyFile string `yaml:"tlsKeyFile"`

	// TLSCAFile is the path to the CA certificate file
	TLSCAFile string `yaml:"tlsCAFile"`

	// MaxRetries is the maximum number of retries for producing messages
	MaxRetries int `yaml:"maxRetries" default:"3"`

	// RetryBackoff is the backoff time between retries
	RetryBackoff time.Duration `yaml:"retryBackoff" default:"100ms"`
}

// DefaultRabbitMQConfig returns a default configuration for RabbitMQ
func DefaultRabbitMQConfig() *RabbitMQConfig {
	return &RabbitMQConfig{
		URI:                  "amqp://guest:guest@localhost:5672/",
		ExchangeName:         "microlib",
		ExchangeType:         "topic",
		QueueDurable:         true,
		QueueAutoDelete:      false,
		ConnectionTimeout:    30 * time.Second,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 10,
	}
}

// DefaultKafkaConfig returns a default configuration for Kafka
func DefaultKafkaConfig() *KafkaConfig {
	return &KafkaConfig{
		Brokers:          []string{"localhost:9092"},
		ClientID:         "microlib",
		ConsumerGroup:    "microlib-consumer",
		SecurityProtocol: "plaintext",
		SASLMechanism:    "plain",
		EnableTLS:        false,
		MaxRetries:       3,
		RetryBackoff:     100 * time.Millisecond,
	}
}

// DefaultBrokerConfig returns a default broker configuration using memory broker
func DefaultBrokerConfig() *BrokerConfig {
	return &BrokerConfig{
		Type:     BrokerTypeMemory,
		RabbitMQ: DefaultRabbitMQConfig(),
		Kafka:    DefaultKafkaConfig(),
	}
}
