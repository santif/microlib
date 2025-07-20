package messaging

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/santif/microlib/data"
)

// SaveMessageToOutbox saves a message to the outbox within a transaction
func SaveMessageToOutbox(ctx context.Context, tx data.Transaction, store OutboxStore, topic string, message Message) error {
	outboxMessage := OutboxMessage{
		ID:         message.ID(),
		Topic:      topic,
		Payload:    message.Body(),
		Headers:    message.Headers(),
		RetryCount: 0,
	}

	return store.SaveMessage(ctx, tx, outboxMessage)
}

// SaveToOutbox is a helper function to save a message to the outbox within a transaction
func SaveToOutbox(ctx context.Context, tx data.Transaction, store OutboxStore, topic string, payload []byte, headers map[string]string) error {
	id := uuid.New().String()
	message := NewOutboxMessage(id, topic, payload, headers)
	return store.SaveMessage(ctx, tx, message)
}

// PublishFromOutbox publishes pending messages from the outbox to the broker
func PublishFromOutbox(ctx context.Context, store OutboxStore, broker Broker, batchSize int) error {
	messages, err := store.GetPendingMessages(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	for _, msg := range messages {
		err := broker.Publish(ctx, msg.Topic, msg.ToMessage())
		if err != nil {
			if markErr := store.MarkFailed(ctx, msg.ID, err); markErr != nil {
				// Log this error but continue with other messages
				fmt.Printf("Failed to mark message as failed: %v\n", markErr)
			}
			continue
		}

		if markErr := store.MarkProcessed(ctx, msg.ID); markErr != nil {
			// Log this error but continue with other messages
			fmt.Printf("Failed to mark message as processed: %v\n", markErr)
		}
	}

	return nil
}
