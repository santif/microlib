package messaging

import (
	"context"
	"time"

	"github.com/santif/microlib/data"
)

// OutboxMessage represents a message stored in the outbox table
type OutboxMessage struct {
	ID          string            `db:"id"`
	Topic       string            `db:"topic"`
	Payload     []byte            `db:"payload"`
	Headers     map[string]string `db:"headers"`
	CreatedAt   time.Time         `db:"created_at"`
	ProcessedAt *time.Time        `db:"processed_at"`
	RetryCount  int               `db:"retry_count"`
	Error       string            `db:"error"`
}

// OutboxStore defines the interface for persisting messages in an outbox
type OutboxStore interface {
	// SaveMessage saves a message to the outbox within a transaction
	SaveMessage(ctx context.Context, tx data.Transaction, message OutboxMessage) error

	// GetPendingMessages retrieves pending messages that need to be processed
	GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error)

	// MarkProcessed marks a message as processed
	MarkProcessed(ctx context.Context, messageID string) error

	// MarkFailed marks a message as failed with an error and increments the retry count
	MarkFailed(ctx context.Context, messageID string, err error) error

	// CreateOutboxTable creates the outbox table if it doesn't exist
	CreateOutboxTable(ctx context.Context) error
}

// NewOutboxMessage creates a new OutboxMessage
func NewOutboxMessage(id, topic string, payload []byte, headers map[string]string) OutboxMessage {
	if headers == nil {
		headers = make(map[string]string)
	}

	return OutboxMessage{
		ID:         id,
		Topic:      topic,
		Payload:    payload,
		Headers:    headers,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}
}

// ToMessage converts an OutboxMessage to a Message
func (m OutboxMessage) ToMessage() Message {
	return NewMessage(m.ID, m.Payload, m.Headers)
}
