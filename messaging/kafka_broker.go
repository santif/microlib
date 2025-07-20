package messaging

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/santif/microlib/observability"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// KafkaBroker implements the Broker interface using Kafka
type KafkaBroker struct {
	config  *KafkaConfig
	logger  observability.Logger
	metrics observability.Metrics

	// For connection management
	dialer    *kafka.Dialer
	writers   map[string]*kafka.Writer
	writersMu sync.RWMutex

	// For subscription management
	readers   map[string]*kafka.Reader
	readersMu sync.RWMutex

	// For shutdown management
	closed   bool
	closedMu sync.RWMutex
	wg       sync.WaitGroup
}

// NewKafkaBroker creates a new Kafka broker
func NewKafkaBroker(
	config *KafkaConfig,
	logger observability.Logger,
	metrics observability.Metrics,
) (*KafkaBroker, error) {
	if logger == nil {
		logger = observability.NewLogger()
	}

	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	// Create the dialer with authentication and TLS if configured
	dialer, err := createKafkaDialer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka dialer: %w", err)
	}

	broker := &KafkaBroker{
		config:  config,
		logger:  logger,
		metrics: metrics,
		dialer:  dialer,
		writers: make(map[string]*kafka.Writer),
		readers: make(map[string]*kafka.Reader),
	}

	// Verify connection to Kafka
	if err := broker.verifyConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}

	return broker, nil
}

// verifyConnection checks if we can connect to at least one Kafka broker
func (b *KafkaBroker) verifyConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to connect to the first broker
	conn, err := b.dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	b.logger.Info("Connected to Kafka",
		observability.NewField("brokers", strings.Join(b.config.Brokers, ",")),
		observability.NewField("clientID", b.config.ClientID))

	return nil
}

// createKafkaDialer creates a Kafka dialer with the appropriate authentication and TLS settings
func createKafkaDialer(config *KafkaConfig) (*kafka.Dialer, error) {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	// Configure TLS if enabled
	if config.EnableTLS {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Load client certificate if provided
		if config.TLSCertFile != "" && config.TLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// Load CA certificate if provided
		if config.TLSCAFile != "" {
			caCert, err := os.ReadFile(config.TLSCAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsConfig.RootCAs = caCertPool
		}

		dialer.TLS = tlsConfig
	}

	// Configure SASL if needed
	if strings.HasPrefix(config.SecurityProtocol, "sasl") {
		var mechanism sasl.Mechanism
		var err error

		switch config.SASLMechanism {
		case "plain":
			mechanism = plain.Mechanism{
				Username: config.SASLUsername,
				Password: config.SASLPassword,
			}
		case "scram-sha-256":
			mechanism, err = scram.Mechanism(scram.SHA256, config.SASLUsername, config.SASLPassword)
		case "scram-sha-512":
			mechanism, err = scram.Mechanism(scram.SHA512, config.SASLUsername, config.SASLPassword)
		default:
			return nil, fmt.Errorf("unsupported SASL mechanism: %s", config.SASLMechanism)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
		}

		dialer.SASLMechanism = mechanism
	}

	return dialer, nil
}

// getOrCreateWriter gets an existing writer for a topic or creates a new one
func (b *KafkaBroker) getOrCreateWriter(topic string) *kafka.Writer {
	b.writersMu.RLock()
	writer, exists := b.writers[topic]
	b.writersMu.RUnlock()

	if exists {
		return writer
	}

	// Create a new writer
	b.writersMu.Lock()
	defer b.writersMu.Unlock()

	// Check again in case another goroutine created it while we were waiting
	if writer, exists = b.writers[topic]; exists {
		return writer
	}

	writer = &kafka.Writer{
		Addr:         kafka.TCP(b.config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll, // Wait for all replicas
		Async:        false,            // Synchronous writes for reliability
		Logger:       kafka.LoggerFunc(b.kafkaLogAdapter),
		ErrorLogger:  kafka.LoggerFunc(b.kafkaErrorLogAdapter),
		Transport: &kafka.Transport{
			Dial: b.dialer.DialFunc,
			TLS:  b.dialer.TLS,
			SASL: b.dialer.SASLMechanism,
		},
		BatchTimeout: 10 * time.Millisecond,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	b.writers[topic] = writer
	return writer
}

// Publish sends a message to the specified topic
func (b *KafkaBroker) Publish(ctx context.Context, topic string, message Message) error {
	b.closedMu.RLock()
	if b.closed {
		b.closedMu.RUnlock()
		return fmt.Errorf("broker is closed")
	}
	b.closedMu.RUnlock()

	// Get or create writer for this topic
	writer := b.getOrCreateWriter(topic)

	// Create Kafka message
	headers := make([]kafka.Header, 0, len(message.Headers()))
	for k, v := range message.Headers() {
		headers = append(headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	kafkaMsg := kafka.Message{
		Key:     []byte(message.ID()),
		Value:   message.Body(),
		Headers: headers,
		Time:    time.Now(),
	}

	// Write message with retries
	var err error
	for attempt := 0; attempt <= b.config.MaxRetries; attempt++ {
		// If this is a retry, create a new context with timeout
		msgCtx := ctx
		if attempt > 0 {
			var cancel context.CancelFunc
			msgCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Wait before retrying
			backoff := time.Duration(attempt) * b.config.RetryBackoff
			time.Sleep(backoff)

			b.logger.Info("Retrying message publish",
				observability.NewField("topic", topic),
				observability.NewField("messageId", message.ID()),
				observability.NewField("attempt", attempt))
		}

		err = writer.WriteMessages(msgCtx, kafkaMsg)
		if err == nil {
			// Success
			return nil
		}

		b.logger.Error("Failed to publish message", err,
			observability.NewField("topic", topic),
			observability.NewField("messageId", message.ID()),
			observability.NewField("attempt", attempt))
	}

	return fmt.Errorf("failed to publish message after %d attempts: %w", b.config.MaxRetries+1, err)
}

// Subscribe registers a handler function for the specified topic
func (b *KafkaBroker) Subscribe(topic string, handler MessageHandler) error {
	b.closedMu.RLock()
	if b.closed {
		b.closedMu.RUnlock()
		return fmt.Errorf("broker is closed")
	}
	b.closedMu.RUnlock()

	// Create a unique consumer group ID for this subscription if needed
	groupID := fmt.Sprintf("%s-%s", b.config.ConsumerGroup, topic)

	// Create reader configuration
	readerConfig := kafka.ReaderConfig{
		Brokers:        b.config.Brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.FirstOffset,
		RetentionTime:  7 * 24 * time.Hour, // 1 week
		Logger:         kafka.LoggerFunc(b.kafkaLogAdapter),
		ErrorLogger:    kafka.LoggerFunc(b.kafkaErrorLogAdapter),
		IsolationLevel: kafka.ReadCommitted,
		Dialer:         b.dialer,
	}

	// Create reader
	reader := kafka.NewReader(readerConfig)

	// Store reader for cleanup
	b.readersMu.Lock()
	b.readers[topic] = reader
	b.readersMu.Unlock()

	// Start consumer goroutine
	b.wg.Add(1)
	go b.consumeMessages(reader, topic, handler)

	b.logger.Info("Subscribed to Kafka topic",
		observability.NewField("topic", topic),
		observability.NewField("groupId", groupID))

	return nil
}

// consumeMessages continuously reads messages from Kafka and processes them
func (b *KafkaBroker) consumeMessages(reader *kafka.Reader, topic string, handler MessageHandler) {
	defer b.wg.Done()

	for {
		// Check if broker is closed
		b.closedMu.RLock()
		if b.closed {
			b.closedMu.RUnlock()
			return
		}
		b.closedMu.RUnlock()

		// Create context with timeout for reading
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		msg, err := reader.FetchMessage(ctx)
		cancel()

		if err != nil {
			if err == context.DeadlineExceeded || err == context.Canceled {
				// This is normal when waiting for messages or during shutdown
				continue
			}

			if err == io.EOF {
				// Reader was closed
				return
			}

			b.logger.Error("Error fetching message from Kafka", err,
				observability.NewField("topic", topic))

			// Wait a bit before retrying
			time.Sleep(1 * time.Second)
			continue
		}

		// Convert Kafka message to our Message interface
		message := &kafkaMessage{
			id:      string(msg.Key),
			body:    msg.Value,
			headers: convertKafkaHeaders(msg.Headers),
			msg:     msg,
		}

		// If message ID is empty, generate one
		if message.id == "" {
			message.id = uuid.New().String()
		}

		// Process message
		processCtx, processCancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = handler(processCtx, message)
		processCancel()

		if err != nil {
			b.logger.Error("Failed to process message", err,
				observability.NewField("topic", topic),
				observability.NewField("messageId", message.ID()))

			// In Kafka, we commit the message even if processing fails
			// In a production system, you might want to implement a dead letter queue
		}

		// Commit the message
		commitCtx, commitCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := reader.CommitMessages(commitCtx, msg); err != nil {
			b.logger.Error("Failed to commit message", err,
				observability.NewField("topic", topic),
				observability.NewField("messageId", message.ID()))
		}
		commitCancel()
	}
}

// Close shuts down the broker and cleans up resources
func (b *KafkaBroker) Close() error {
	b.closedMu.Lock()
	if b.closed {
		b.closedMu.Unlock()
		return nil
	}
	b.closed = true
	b.closedMu.Unlock()

	// Close all writers
	b.writersMu.Lock()
	for topic, writer := range b.writers {
		if err := writer.Close(); err != nil {
			b.logger.Error("Error closing Kafka writer", err,
				observability.NewField("topic", topic))
		}
	}
	b.writers = make(map[string]*kafka.Writer)
	b.writersMu.Unlock()

	// Close all readers
	b.readersMu.Lock()
	for topic, reader := range b.readers {
		if err := reader.Close(); err != nil {
			b.logger.Error("Error closing Kafka reader", err,
				observability.NewField("topic", topic))
		}
	}
	b.readers = make(map[string]*kafka.Reader)
	b.readersMu.Unlock()

	// Wait for all consumer goroutines to finish
	waitCh := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(waitCh)
	}()

	// Wait with timeout
	select {
	case <-waitCh:
		// All goroutines finished
	case <-time.After(10 * time.Second):
		b.logger.Error("Timeout waiting for Kafka consumers to stop", nil)
	}

	b.logger.Info("Kafka broker closed")
	return nil
}

// kafkaMessage implements the Message interface for Kafka messages
type kafkaMessage struct {
	id      string
	body    []byte
	headers map[string]string
	msg     kafka.Message
}

func (m *kafkaMessage) ID() string {
	return m.id
}

func (m *kafkaMessage) Body() []byte {
	return m.body
}

func (m *kafkaMessage) Headers() map[string]string {
	return m.headers
}

// Helper functions

// convertKafkaHeaders converts Kafka headers to a string map
func convertKafkaHeaders(headers []kafka.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string)
	for _, h := range headers {
		result[h.Key] = string(h.Value)
	}
	return result
}

// Log adapters for Kafka

func (b *KafkaBroker) kafkaLogAdapter(msg string, args ...interface{}) {
	b.logger.Debug(fmt.Sprintf(msg, args...),
		observability.NewField("component", "kafka"))
}

func (b *KafkaBroker) kafkaErrorLogAdapter(msg string, args ...interface{}) {
	b.logger.Error(fmt.Sprintf(msg, args...), nil,
		observability.NewField("component", "kafka"))
}
