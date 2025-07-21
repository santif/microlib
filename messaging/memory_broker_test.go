package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var ErrMessageProcessingFailed = errors.New("message processing failed")

func TestNewMemoryBroker(t *testing.T) {
	broker := NewMemoryBroker()
	if broker == nil {
		t.Fatal("Expected non-nil broker")
	}

	// Broker should be properly initialized (we can't check private fields)
}

func TestMemoryBroker_Publish(t *testing.T) {
	broker := NewMemoryBroker()
	ctx := context.Background()

	// Test publishing to a topic with no subscribers
	msg := NewMessage("test-id", []byte("test message"), map[string]string{"key": "value"})
	err := broker.Publish(ctx, "test-topic", msg)
	if err != nil {
		t.Errorf("Expected no error when publishing to topic with no subscribers, got: %v", err)
	}

	// Test publishing with nil message - memory broker doesn't validate this
	err = broker.Publish(ctx, "test-topic", nil)
	// Memory broker doesn't validate nil messages, it will just pass them to handlers
	if err != nil {
		t.Errorf("Expected no error when publishing nil message to memory broker, got: %v", err)
	}

	// Test publishing with empty topic - memory broker allows this
	err = broker.Publish(ctx, "", msg)
	if err != nil {
		t.Errorf("Expected no error when publishing to empty topic in memory broker, got: %v", err)
	}
}

func TestMemoryBroker_Subscribe(t *testing.T) {
	broker := NewMemoryBroker()
	ctx := context.Background()

	// Test subscribing to a topic
	messageReceived := make(chan Message, 1)
	handler := func(ctx context.Context, msg Message) error {
		messageReceived <- msg
		return nil
	}

	err := broker.Subscribe("test-topic", handler)
	if err != nil {
		t.Fatalf("Expected no error when subscribing, got: %v", err)
	}

	// Publish a message and verify it's received
	msg := NewMessage("test-id", []byte("test message"), map[string]string{"key": "value"})
	err = broker.Publish(ctx, "test-topic", msg)
	if err != nil {
		t.Fatalf("Expected no error when publishing, got: %v", err)
	}

	// Wait for message to be received
	select {
	case receivedMsg := <-messageReceived:
		if receivedMsg.ID() != msg.ID() {
			t.Errorf("Expected message ID %s, got %s", msg.ID(), receivedMsg.ID())
		}
		if string(receivedMsg.Body()) != string(msg.Body()) {
			t.Errorf("Expected message body %s, got %s", string(msg.Body()), string(receivedMsg.Body()))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive message within timeout")
	}
}

func TestMemoryBroker_SubscribeWithError(t *testing.T) {
	broker := NewMemoryBroker()
	ctx := context.Background()

	// Test subscribing with handler that returns error
	handler := func(ctx context.Context, msg Message) error {
		return ErrMessageProcessingFailed
	}

	err := broker.Subscribe("test-topic", handler)
	if err != nil {
		t.Fatalf("Expected no error when subscribing, got: %v", err)
	}

	// Publish a message - handler error should not affect publishing
	msg := NewMessage("test-id", []byte("test message"), nil)
	err = broker.Publish(ctx, "test-topic", msg)
	if err != nil {
		t.Errorf("Expected no error when publishing, got: %v", err)
	}
}

func TestMemoryBroker_MultipleSubscribers(t *testing.T) {
	broker := NewMemoryBroker()
	ctx := context.Background()

	// Create multiple subscribers for the same topic
	messageReceived1 := make(chan Message, 1)
	messageReceived2 := make(chan Message, 1)

	handler1 := func(ctx context.Context, msg Message) error {
		messageReceived1 <- msg
		return nil
	}

	handler2 := func(ctx context.Context, msg Message) error {
		messageReceived2 <- msg
		return nil
	}

	err := broker.Subscribe("test-topic", handler1)
	if err != nil {
		t.Fatalf("Expected no error when subscribing handler1, got: %v", err)
	}

	err = broker.Subscribe("test-topic", handler2)
	if err != nil {
		t.Fatalf("Expected no error when subscribing handler2, got: %v", err)
	}

	// Publish a message
	msg := NewMessage("test-id", []byte("test message"), nil)
	err = broker.Publish(ctx, "test-topic", msg)
	if err != nil {
		t.Fatalf("Expected no error when publishing, got: %v", err)
	}

	// Both handlers should receive the message
	select {
	case <-messageReceived1:
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected handler1 to receive message within timeout")
	}

	select {
	case <-messageReceived2:
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected handler2 to receive message within timeout")
	}
}

func TestMemoryBroker_ConcurrentPublish(t *testing.T) {
	broker := NewMemoryBroker()
	ctx := context.Background()

	// Subscribe to topic
	messagesReceived := make(chan Message, 100)
	handler := func(ctx context.Context, msg Message) error {
		messagesReceived <- msg
		return nil
	}

	err := broker.Subscribe("test-topic", handler)
	if err != nil {
		t.Fatalf("Expected no error when subscribing, got: %v", err)
	}

	// Publish messages concurrently
	numMessages := 50
	var wg sync.WaitGroup
	wg.Add(numMessages)

	for i := 0; i < numMessages; i++ {
		go func(id int) {
			defer wg.Done()
			msg := NewMessage(fmt.Sprintf("msg-%d", id), []byte(fmt.Sprintf("message %d", id)), nil)
			err := broker.Publish(ctx, "test-topic", msg)
			if err != nil {
				t.Errorf("Expected no error when publishing message %d, got: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were received
	receivedCount := 0
	timeout := time.After(1 * time.Second)
	for receivedCount < numMessages {
		select {
		case <-messagesReceived:
			receivedCount++
		case <-timeout:
			t.Errorf("Expected to receive %d messages, got %d", numMessages, receivedCount)
			return
		}
	}
}

func TestMemoryBroker_Close(t *testing.T) {
	broker := NewMemoryBroker()

	// Test closing the broker
	err := broker.Close()
	if err != nil {
		t.Errorf("Expected no error when closing broker, got: %v", err)
	}

	// Test that publishing after close returns error
	ctx := context.Background()
	msg := NewMessage("test-id", []byte("test message"), nil)
	err = broker.Publish(ctx, "test-topic", msg)
	if err == nil {
		t.Error("Expected error when publishing after close")
	}
}

func TestMemoryBroker_SubscribeEmptyTopic(t *testing.T) {
	broker := NewMemoryBroker()

	handler := func(ctx context.Context, msg Message) error {
		return nil
	}

	// Test subscribing to empty topic - memory broker allows this
	err := broker.Subscribe("", handler)
	if err != nil {
		t.Errorf("Expected no error when subscribing to empty topic in memory broker, got: %v", err)
	}
}

func TestMemoryBroker_SubscribeNilHandler(t *testing.T) {
	broker := NewMemoryBroker()

	// Test subscribing with nil handler - memory broker allows this
	err := broker.Subscribe("test-topic", nil)
	if err != nil {
		t.Errorf("Expected no error when subscribing with nil handler in memory broker, got: %v", err)
	}
}
