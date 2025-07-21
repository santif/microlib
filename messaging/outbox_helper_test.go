package messaging

import (
	"context"
	"testing"

	"github.com/santif/microlib/data"
)

// mockTransaction implements the data.Transaction interface for testing
type mockTransaction struct {
	committed  bool
	rolledBack bool
}

func (m *mockTransaction) Query(ctx context.Context, query string, args ...interface{}) (data.Rows, error) {
	return nil, nil
}

func (m *mockTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) data.Row {
	return nil
}

func (m *mockTransaction) Exec(ctx context.Context, query string, args ...interface{}) (data.Result, error) {
	return nil, nil
}

// mockOutboxStore implements the OutboxStore interface for testing
type mockOutboxStore struct {
	savedMessages []OutboxMessage
	saveError     error
}

func (m *mockOutboxStore) CreateOutboxTable(ctx context.Context) error {
	return nil
}

func (m *mockOutboxStore) SaveMessage(ctx context.Context, tx data.Transaction, message OutboxMessage) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.savedMessages = append(m.savedMessages, message)
	return nil
}

func (m *mockOutboxStore) GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error) {
	return m.savedMessages, nil
}

func (m *mockOutboxStore) MarkProcessed(ctx context.Context, messageID string) error {
	return nil
}

func (m *mockOutboxStore) MarkFailed(ctx context.Context, messageID string, err error) error {
	return nil
}

// mockBroker implements the Broker interface for testing
type mockBroker struct {
	publishedMessages []Message
	publishError      error
}

func (m *mockBroker) Publish(ctx context.Context, topic string, message Message) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.publishedMessages = append(m.publishedMessages, message)
	return nil
}

func (m *mockBroker) Subscribe(topic string, handler MessageHandler) error {
	return nil
}

func (m *mockBroker) Close() error {
	return nil
}

func TestSaveMessageToOutbox(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	tx := &mockTransaction{}

	// Test saving a message to outbox
	message := NewMessage("test-id", []byte("test message"), map[string]string{"key": "value"})
	err := SaveMessageToOutbox(ctx, tx, store, "test-topic", message)
	if err != nil {
		t.Fatalf("Expected no error when saving message to outbox, got: %v", err)
	}

	// Verify the message was saved
	if len(store.savedMessages) != 1 {
		t.Errorf("Expected 1 saved message, got %d", len(store.savedMessages))
	}

	savedMsg := store.savedMessages[0]
	if savedMsg.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", savedMsg.Topic)
	}
	if string(savedMsg.Payload) != string(message.Body()) {
		t.Errorf("Expected payload '%s', got '%s'", string(message.Body()), string(savedMsg.Payload))
	}
}

func TestSaveMessageToOutboxWithError(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{saveError: ErrMessageProcessingFailed}
	tx := &mockTransaction{}

	// Test saving a message when store returns error
	message := NewMessage("test-id", []byte("test message"), nil)
	err := SaveMessageToOutbox(ctx, tx, store, "test-topic", message)
	if err != ErrMessageProcessingFailed {
		t.Errorf("Expected ErrMessageProcessingFailed, got: %v", err)
	}
}

func TestSaveToOutbox(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	tx := &mockTransaction{}

	// Test saving raw data to outbox
	payload := []byte("test payload")
	headers := map[string]string{"content-type": "application/json"}
	err := SaveToOutbox(ctx, tx, store, "test-topic", payload, headers)
	if err != nil {
		t.Fatalf("Expected no error when saving to outbox, got: %v", err)
	}

	// Verify the message was saved
	if len(store.savedMessages) != 1 {
		t.Errorf("Expected 1 saved message, got %d", len(store.savedMessages))
	}

	savedMsg := store.savedMessages[0]
	if savedMsg.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", savedMsg.Topic)
	}
	if string(savedMsg.Payload) != string(payload) {
		t.Errorf("Expected payload '%s', got '%s'", string(payload), string(savedMsg.Payload))
	}
	if savedMsg.Headers["content-type"] != "application/json" {
		t.Errorf("Expected header 'application/json', got '%s'", savedMsg.Headers["content-type"])
	}
}

func TestSaveToOutboxWithNilHeaders(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	tx := &mockTransaction{}

	// Test saving with nil headers
	payload := []byte("test payload")
	err := SaveToOutbox(ctx, tx, store, "test-topic", payload, nil)
	if err != nil {
		t.Fatalf("Expected no error when saving to outbox with nil headers, got: %v", err)
	}

	// Verify the message was saved
	if len(store.savedMessages) != 1 {
		t.Errorf("Expected 1 saved message, got %d", len(store.savedMessages))
	}

	savedMsg := store.savedMessages[0]
	if savedMsg.Headers == nil {
		t.Error("Expected headers to be initialized as empty map, got nil")
	}
}

func TestPublishFromOutboxHelper(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	broker := &mockBroker{}

	// Add some messages to the outbox store
	msg1 := NewOutboxMessage("msg1", "topic1", []byte("message 1"), map[string]string{"key": "value1"})
	msg2 := NewOutboxMessage("msg2", "topic2", []byte("message 2"), map[string]string{"key": "value2"})
	store.savedMessages = []OutboxMessage{msg1, msg2}

	// Test publishing from outbox
	err := PublishFromOutbox(ctx, store, broker, 10)
	if err != nil {
		t.Fatalf("Expected no error when publishing from outbox, got: %v", err)
	}

	// Verify messages were published
	if len(broker.publishedMessages) != 2 {
		t.Errorf("Expected 2 published messages, got %d", len(broker.publishedMessages))
	}

	// Verify message content
	publishedMsg1 := broker.publishedMessages[0]
	if string(publishedMsg1.Body()) != "message 1" {
		t.Errorf("Expected message body 'message 1', got '%s'", string(publishedMsg1.Body()))
	}
	if publishedMsg1.Headers()["key"] != "value1" {
		t.Errorf("Expected header value 'value1', got '%s'", publishedMsg1.Headers()["key"])
	}
}

func TestPublishFromOutboxHelperWithBrokerError(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	broker := &mockBroker{publishError: ErrMessageProcessingFailed}

	// Add a message to the outbox store
	msg := NewOutboxMessage("msg1", "topic1", []byte("message 1"), nil)
	store.savedMessages = []OutboxMessage{msg}

	// Test publishing from outbox when broker returns error
	// The function continues processing even when broker fails, so no error is returned
	err := PublishFromOutbox(ctx, store, broker, 10)
	if err != nil {
		t.Errorf("Expected no error when broker fails to publish (function continues), got: %v", err)
	}
}

func TestPublishFromOutboxHelperEmptyStore(t *testing.T) {
	ctx := context.Background()
	store := &mockOutboxStore{}
	broker := &mockBroker{}

	// Test publishing from empty outbox
	err := PublishFromOutbox(ctx, store, broker, 10)
	if err != nil {
		t.Errorf("Expected no error when publishing from empty outbox, got: %v", err)
	}

	// Verify no messages were published
	if len(broker.publishedMessages) != 0 {
		t.Errorf("Expected 0 published messages, got %d", len(broker.publishedMessages))
	}
}
