package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/santif/microlib/observability"
)

// RabbitMQBroker implements the Broker interface using RabbitMQ
type RabbitMQBroker struct {
	config     *RabbitMQConfig
	connection *amqp.Connection
	channel    *amqp.Channel
	logger     observability.Logger
	metrics    observability.Metrics

	// For connection management
	connMu       sync.Mutex
	reconnecting bool
	closed       bool

	// For subscription management
	subscribers   map[string][]subscriberInfo
	subscribersMu sync.RWMutex

	// Notification channels
	connCloseChan chan *amqp.Error
	chanCloseChan chan *amqp.Error
}

type subscriberInfo struct {
	topic   string
	queue   string
	handler MessageHandler
}

// NewRabbitMQBroker creates a new RabbitMQ broker
func NewRabbitMQBroker(
	config *RabbitMQConfig,
	logger observability.Logger,
	metrics observability.Metrics,
) (*RabbitMQBroker, error) {
	if logger == nil {
		logger = observability.NewLogger()
	}

	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	broker := &RabbitMQBroker{
		config:      config,
		logger:      logger,
		metrics:     metrics,
		subscribers: make(map[string][]subscriberInfo),
	}

	// Connect to RabbitMQ
	if err := broker.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return broker, nil
}

// connect establishes a connection to RabbitMQ and sets up the channel
func (b *RabbitMQBroker) connect() error {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	if b.closed {
		return fmt.Errorf("broker is closed")
	}

	// Close existing connection if any
	if b.connection != nil {
		_ = b.connection.Close()
		b.connection = nil
	}

	// Connect to RabbitMQ with timeout
	ctx, cancel := context.WithTimeout(context.Background(), b.config.ConnectionTimeout)
	defer cancel()

	var err error
	connChan := make(chan *amqp.Connection, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := amqp.Dial(b.config.URI)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	select {
	case conn := <-connChan:
		b.connection = conn
	case err = <-errChan:
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("connection timeout: %w", ctx.Err())
	}

	// Create channel
	b.channel, err = b.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Declare exchange
	err = b.channel.ExchangeDeclare(
		b.config.ExchangeName, // name
		b.config.ExchangeType, // type
		b.config.QueueDurable, // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Set up notification channels for connection and channel close events
	b.connCloseChan = make(chan *amqp.Error, 1)
	b.chanCloseChan = make(chan *amqp.Error, 1)
	b.connection.NotifyClose(b.connCloseChan)
	b.channel.NotifyClose(b.chanCloseChan)

	// Start a goroutine to handle reconnection
	go b.handleReconnection()

	// Resubscribe all existing subscribers
	if err := b.resubscribeAll(); err != nil {
		b.logger.Error("Failed to resubscribe all handlers", err)
		// Continue anyway, as we'll retry on next reconnection
	}

	b.logger.Info("Connected to RabbitMQ",
		observability.NewField("uri", maskURI(b.config.URI)),
		observability.NewField("exchange", b.config.ExchangeName))

	return nil
}

// handleReconnection monitors connection status and reconnects when necessary
func (b *RabbitMQBroker) handleReconnection() {
	for {
		select {
		case err := <-b.connCloseChan:
			if b.closed {
				return
			}
			b.logger.Error("RabbitMQ connection closed", fmt.Errorf("connection closed: %v", err),
				observability.NewField("will_retry", true))
			b.reconnect()

		case err := <-b.chanCloseChan:
			if b.closed {
				return
			}
			b.logger.Error("RabbitMQ channel closed", fmt.Errorf("channel closed: %v", err),
				observability.NewField("will_retry", true))
			b.reconnect()

		}
	}
}

// reconnect attempts to reconnect to RabbitMQ with exponential backoff
func (b *RabbitMQBroker) reconnect() {
	b.connMu.Lock()
	if b.reconnecting || b.closed {
		b.connMu.Unlock()
		return
	}
	b.reconnecting = true
	b.connMu.Unlock()

	defer func() {
		b.connMu.Lock()
		b.reconnecting = false
		b.connMu.Unlock()
	}()

	attempts := 0
	for {
		if b.closed {
			return
		}

		if b.config.MaxReconnectAttempts > 0 && attempts >= b.config.MaxReconnectAttempts {
			b.logger.Error("Max reconnection attempts reached", fmt.Errorf("failed after %d attempts", attempts))
			return
		}

		attempts++
		b.logger.Info("Attempting to reconnect to RabbitMQ",
			observability.NewField("attempt", attempts),
			observability.NewField("max_attempts", b.config.MaxReconnectAttempts))

		if err := b.connect(); err != nil {
			b.logger.Error("Failed to reconnect", err,
				observability.NewField("attempt", attempts),
				observability.NewField("will_retry", true))

			// Wait before next attempt
			time.Sleep(b.config.ReconnectDelay)
			continue
		}

		b.logger.Info("Successfully reconnected to RabbitMQ",
			observability.NewField("attempt", attempts))
		return
	}
}

// resubscribeAll reestablishes all subscriptions after a reconnection
func (b *RabbitMQBroker) resubscribeAll() error {
	b.subscribersMu.RLock()
	defer b.subscribersMu.RUnlock()

	for _, subscribers := range b.subscribers {
		for _, sub := range subscribers {
			if err := b.setupSubscription(sub.topic, sub.queue, sub.handler); err != nil {
				return err
			}
		}
	}

	return nil
}

// setupSubscription creates a queue, binds it to the exchange, and starts consuming
func (b *RabbitMQBroker) setupSubscription(topic, queueName string, handler MessageHandler) error {
	// Declare queue
	queue, err := b.channel.QueueDeclare(
		queueName,                // name
		b.config.QueueDurable,    // durable
		b.config.QueueAutoDelete, // delete when unused
		false,                    // exclusive
		false,                    // no-wait
		nil,                      // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = b.channel.QueueBind(
		queue.Name,            // queue name
		topic,                 // routing key
		b.config.ExchangeName, // exchange
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Start consuming
	deliveries, err := b.channel.Consume(
		queue.Name, // queue
		"",         // consumer
		false,      // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume from queue: %w", err)
	}

	// Handle deliveries in a separate goroutine
	go b.handleDeliveries(deliveries, handler)

	return nil
}

// handleDeliveries processes incoming messages from RabbitMQ
func (b *RabbitMQBroker) handleDeliveries(deliveries <-chan amqp.Delivery, handler MessageHandler) {
	for d := range deliveries {
		// Create message from delivery
		msg := &rabbitMQMessage{
			id:       d.MessageId,
			body:     d.Body,
			headers:  convertAMQPHeaders(d.Headers),
			delivery: d,
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Process message
		err := handler(ctx, msg)

		// Handle acknowledgment based on processing result
		if err != nil {
			b.logger.Error("Failed to process message", err,
				observability.NewField("messageId", msg.ID()),
				observability.NewField("topic", d.RoutingKey))

			// Negative acknowledgment, requeue the message
			if err := d.Nack(false, true); err != nil {
				b.logger.Error("Failed to nack message", err,
					observability.NewField("messageId", msg.ID()))
			}
		} else {
			// Positive acknowledgment
			if err := d.Ack(false); err != nil {
				b.logger.Error("Failed to ack message", err,
					observability.NewField("messageId", msg.ID()))
			}
		}

		cancel() // Clean up the context
	}
}

// Publish sends a message to the specified topic
func (b *RabbitMQBroker) Publish(ctx context.Context, topic string, message Message) error {
	b.connMu.Lock()
	if b.closed {
		b.connMu.Unlock()
		return fmt.Errorf("broker is closed")
	}

	// Check if channel is available
	if b.channel == nil {
		b.connMu.Unlock()
		return fmt.Errorf("not connected to RabbitMQ")
	}
	channel := b.channel
	b.connMu.Unlock()

	// Create publishing
	publishing := amqp.Publishing{
		MessageId:    message.ID(),
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/octet-stream",
		Body:         message.Body(),
		Headers:      convertToAMQPHeaders(message.Headers()),
	}

	// Publish with context timeout
	err := channel.PublishWithContext(
		ctx,
		b.config.ExchangeName, // exchange
		topic,                 // routing key
		false,                 // mandatory
		false,                 // immediate
		publishing,            // message
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe registers a handler function for the specified topic
func (b *RabbitMQBroker) Subscribe(topic string, handler MessageHandler) error {
	b.connMu.Lock()
	if b.closed {
		b.connMu.Unlock()
		return fmt.Errorf("broker is closed")
	}

	// Check if channel is available
	if b.channel == nil {
		b.connMu.Unlock()
		return fmt.Errorf("not connected to RabbitMQ")
	}
	b.connMu.Unlock()

	// Generate a queue name based on the topic
	queueName := fmt.Sprintf("%s.%s", b.config.ExchangeName, topic)

	// Set up the subscription
	if err := b.setupSubscription(topic, queueName, handler); err != nil {
		return err
	}

	// Store the subscription for reconnection handling
	b.subscribersMu.Lock()
	defer b.subscribersMu.Unlock()

	sub := subscriberInfo{
		topic:   topic,
		queue:   queueName,
		handler: handler,
	}
	b.subscribers[topic] = append(b.subscribers[topic], sub)

	return nil
}

// Close shuts down the broker connection and cleans up resources
func (b *RabbitMQBroker) Close() error {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Close channel if open
	if b.channel != nil {
		_ = b.channel.Close()
		b.channel = nil
	}

	// Close connection if open
	if b.connection != nil {
		_ = b.connection.Close()
		b.connection = nil
	}

	b.logger.Info("RabbitMQ broker closed")
	return nil
}

// rabbitMQMessage implements the Message interface for RabbitMQ messages
type rabbitMQMessage struct {
	id       string
	body     []byte
	headers  map[string]string
	delivery amqp.Delivery
}

func (m *rabbitMQMessage) ID() string {
	return m.id
}

func (m *rabbitMQMessage) Body() []byte {
	return m.body
}

func (m *rabbitMQMessage) Headers() map[string]string {
	return m.headers
}

// Helper functions

// convertToAMQPHeaders converts a string map to AMQP headers
func convertToAMQPHeaders(headers map[string]string) amqp.Table {
	if headers == nil {
		return nil
	}

	amqpHeaders := make(amqp.Table)
	for k, v := range headers {
		amqpHeaders[k] = v
	}
	return amqpHeaders
}

// convertAMQPHeaders converts AMQP headers to a string map
func convertAMQPHeaders(headers amqp.Table) map[string]string {
	if headers == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range headers {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			// Convert non-string values to string
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// maskURI masks sensitive information in the URI
func maskURI(uri string) string {
	// Simple implementation - in a real system you might want a more robust solution
	return uri // For now, just return the URI as is
}
