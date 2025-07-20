package messaging

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/santif/microlib/data"
	"github.com/santif/microlib/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOutboxStore is a mock implementation of OutboxStore
type MockOutboxStore struct {
	mock.Mock
}

func (m *MockOutboxStore) SaveMessage(ctx context.Context, tx data.Transaction, message OutboxMessage) error {
	args := m.Called(ctx, tx, message)
	return args.Error(0)
}

func (m *MockOutboxStore) GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]OutboxMessage), args.Error(1)
}

func (m *MockOutboxStore) MarkProcessed(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockOutboxStore) MarkFailed(ctx context.Context, messageID string, err error) error {
	args := m.Called(ctx, messageID, err)
	return args.Error(0)
}

func (m *MockOutboxStore) CreateOutboxTable(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockBroker is a mock implementation of Broker
type MockBroker struct {
	mock.Mock
}

func (m *MockBroker) Publish(ctx context.Context, topic string, message Message) error {
	args := m.Called(ctx, topic, message)
	return args.Error(0)
}

func (m *MockBroker) Subscribe(topic string, handler MessageHandler) error {
	args := m.Called(topic, handler)
	return args.Error(0)
}

func (m *MockBroker) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockLogger is a mock implementation of observability.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, fields ...observability.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) InfoContext(ctx context.Context, msg string, fields ...observability.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Error(msg string, err error, fields ...observability.Field) {
	m.Called(msg, err, fields)
}

func (m *MockLogger) ErrorContext(ctx context.Context, msg string, err error, fields ...observability.Field) {
	m.Called(ctx, msg, err, fields)
}

func (m *MockLogger) Debug(msg string, fields ...observability.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) DebugContext(ctx context.Context, msg string, fields ...observability.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...observability.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) WarnContext(ctx context.Context, msg string, fields ...observability.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) WithContext(ctx context.Context) observability.Logger {
	args := m.Called(ctx)
	return args.Get(0).(observability.Logger)
}

// MockMetrics is a mock implementation of observability.Metrics
type MockMetrics struct {
	mock.Mock
}

func (m *MockMetrics) Counter(name string, help string, labels ...string) observability.Counter {
	args := m.Called(name, help, labels)
	return args.Get(0).(observability.Counter)
}

func (m *MockMetrics) Gauge(name string, help string, labels ...string) observability.Gauge {
	args := m.Called(name, help, labels)
	return args.Get(0).(observability.Gauge)
}

func (m *MockMetrics) Histogram(name string, help string, buckets []float64, labels ...string) observability.Histogram {
	args := m.Called(name, help, buckets, labels)
	return args.Get(0).(observability.Histogram)
}

func (m *MockMetrics) Handler() http.Handler {
	args := m.Called()
	return args.Get(0).(http.Handler)
}

func (m *MockMetrics) Registry() *prometheus.Registry {
	args := m.Called()
	return args.Get(0).(*prometheus.Registry)
}

// MockCounter is a mock implementation of observability.Counter
type MockCounter struct {
	mock.Mock
}

func (m *MockCounter) Inc() {
	m.Called()
}

func (m *MockCounter) Add(value float64) {
	m.Called(value)
}

func (m *MockCounter) WithLabels(labels map[string]string) observability.Counter {
	args := m.Called(labels)
	return args.Get(0).(observability.Counter)
}

// MockGauge is a mock implementation of observability.Gauge
type MockGauge struct {
	mock.Mock
}

func (m *MockGauge) Set(value float64) {
	m.Called(value)
}

func (m *MockGauge) Inc() {
	m.Called()
}

func (m *MockGauge) Dec() {
	m.Called()
}

func (m *MockGauge) Add(value float64) {
	m.Called(value)
}

func (m *MockGauge) Sub(value float64) {
	m.Called(value)
}

func (m *MockGauge) WithLabels(labels map[string]string) observability.Gauge {
	args := m.Called(labels)
	return args.Get(0).(observability.Gauge)
}

// MockHistogram is a mock implementation of observability.Histogram
type MockHistogram struct {
	mock.Mock
}

func (m *MockHistogram) Observe(value float64) {
	m.Called(value)
}

func (m *MockHistogram) WithLabels(labels map[string]string) observability.Histogram {
	args := m.Called(labels)
	return args.Get(0).(observability.Histogram)
}

func TestOutboxRelay_BackoffLogic(t *testing.T) {
	relay := NewOutboxRelay(
		nil, nil,
		RelayConfig{
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     1 * time.Second,
			BackoffFactor:  2.0,
		},
		nil, nil,
	)

	// Test backoff calculations
	assert.Equal(t, 100*time.Millisecond, relay.calculateBackoff(1))
	assert.Equal(t, 200*time.Millisecond, relay.calculateBackoff(2))
	assert.Equal(t, 400*time.Millisecond, relay.calculateBackoff(3))
	assert.Equal(t, 800*time.Millisecond, relay.calculateBackoff(4))
	assert.Equal(t, 1*time.Second, relay.calculateBackoff(5)) // Should cap at MaxBackoff
	assert.Equal(t, 1*time.Second, relay.calculateBackoff(6)) // Should cap at MaxBackoff
}

func TestOutboxRelay_ConcurrentAccess(t *testing.T) {
	mockStore := new(MockOutboxStore)
	mockBroker := new(MockBroker)

	config := RelayConfig{
		BatchSize:      10,
		PollInterval:   50 * time.Millisecond,
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
	}

	// Setup store expectations
	mockStore.On("GetPendingMessages", mock.Anything, config.BatchSize).Return([]OutboxMessage{}, nil)

	relay := NewOutboxRelay(mockStore, mockBroker, config, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start the relay
	err := relay.Start(ctx)
	assert.NoError(t, err)

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer stopCancel()
			_ = relay.Stop(stopCtx)
		}()
	}

	wg.Wait()

	// Verify the relay was stopped
	err = relay.Start(ctx)
	assert.Error(t, err) // Should error because relay was stopped
}

func TestDefaultRelayConfig(t *testing.T) {
	config := DefaultRelayConfig()

	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 5*time.Second, config.PollInterval)
	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialBackoff)
	assert.Equal(t, 1*time.Hour, config.MaxBackoff)
	assert.Equal(t, 2.0, config.BackoffFactor)
}
