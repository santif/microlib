package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/santif/microlib/data"
)

// PostgresOutboxStore implements OutboxStore using PostgreSQL
type PostgresOutboxStore struct {
	db data.Database
}

// NewPostgresOutboxStore creates a new PostgreSQL outbox store
func NewPostgresOutboxStore(db data.Database) *PostgresOutboxStore {
	return &PostgresOutboxStore{
		db: db,
	}
}

// CreateOutboxTable creates the outbox table if it doesn't exist
func (s *PostgresOutboxStore) CreateOutboxTable(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS outbox_messages (
		id TEXT PRIMARY KEY,
		topic TEXT NOT NULL,
		payload BYTEA NOT NULL,
		headers JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		processed_at TIMESTAMP WITH TIME ZONE,
		retry_count INTEGER NOT NULL DEFAULT 0,
		error TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_outbox_messages_processed_at ON outbox_messages (processed_at) 
	WHERE processed_at IS NULL;
	`

	_, err := s.db.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create outbox table: %w", err)
	}

	return nil
}

// SaveMessage saves a message to the outbox within a transaction
func (s *PostgresOutboxStore) SaveMessage(ctx context.Context, tx data.Transaction, message OutboxMessage) error {
	headersJSON, err := json.Marshal(message.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal message headers: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO outbox_messages (id, topic, payload, headers, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		message.ID,
		message.Topic,
		message.Payload,
		headersJSON,
		message.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save message to outbox: %w", err)
	}

	return nil
}

// GetPendingMessages retrieves pending messages that need to be processed
func (s *PostgresOutboxStore) GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error) {
	rows, err := s.db.Query(
		ctx,
		`SELECT id, topic, payload, headers, created_at, processed_at, retry_count, error
		FROM outbox_messages
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending messages: %w", err)
	}
	defer rows.Close()

	var messages []OutboxMessage
	for rows.Next() {
		var message OutboxMessage
		var headersJSON []byte

		err := rows.Scan(
			&message.ID,
			&message.Topic,
			&message.Payload,
			&headersJSON,
			&message.CreatedAt,
			&message.ProcessedAt,
			&message.RetryCount,
			&message.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox message: %w", err)
		}

		// Unmarshal headers
		if err := json.Unmarshal(headersJSON, &message.Headers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message headers: %w", err)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outbox messages: %w", err)
	}

	return messages, nil
}

// MarkProcessed marks a message as processed
func (s *PostgresOutboxStore) MarkProcessed(ctx context.Context, messageID string) error {
	now := time.Now()
	_, err := s.db.Exec(
		ctx,
		`UPDATE outbox_messages
		SET processed_at = $1, error = NULL
		WHERE id = $2`,
		now,
		messageID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark message as processed: %w", err)
	}

	return nil
}

// MarkFailed marks a message as failed with an error and increments the retry count
func (s *PostgresOutboxStore) MarkFailed(ctx context.Context, messageID string, err error) error {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	_, dbErr := s.db.Exec(
		ctx,
		`UPDATE outbox_messages
		SET retry_count = retry_count + 1, error = $1
		WHERE id = $2`,
		errorMessage,
		messageID,
	)

	if dbErr != nil {
		return fmt.Errorf("failed to mark message as failed: %w", dbErr)
	}

	return nil
}
