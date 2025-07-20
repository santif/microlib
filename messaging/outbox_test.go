package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/santif/microlib/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of data.Database
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Query(ctx context.Context, query string, args ...interface{}) (data.Rows, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Rows), ret.Error(1)
}

func (m *MockDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) data.Row {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Row)
}

func (m *MockDatabase) Exec(ctx context.Context, query string, args ...interface{}) (data.Result, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Result), ret.Error(1)
}

func (m *MockDatabase) Transaction(ctx context.Context, fn func(data.Transaction) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockTransaction is a mock implementation of data.Transaction
type MockTransaction struct {
	mock.Mock
}

func (m *MockTransaction) Query(ctx context.Context, query string, args ...interface{}) (data.Rows, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Rows), ret.Error(1)
}

func (m *MockTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) data.Row {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Row)
}

func (m *MockTransaction) Exec(ctx context.Context, query string, args ...interface{}) (data.Result, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	ret := m.Called(mockArgs...)
	return ret.Get(0).(data.Result), ret.Error(1)
}

// MockResult is a mock implementation of data.Result
type MockResult struct {
	mock.Mock
}

func (m *MockResult) LastInsertId() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockResult) RowsAffected() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

// MockRows is a mock implementation of data.Rows
type MockRows struct {
	mock.Mock
	data [][]interface{}
	idx  int
}

func NewMockRows(data [][]interface{}) *MockRows {
	return &MockRows{
		data: data,
		idx:  -1,
	}
}

func (m *MockRows) Next() bool {
	m.idx++
	return m.idx < len(m.data)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	if m.idx < 0 || m.idx >= len(m.data) {
		return errors.New("invalid row index")
	}

	row := m.data[m.idx]
	if len(dest) != len(row) {
		return errors.New("column count mismatch")
	}

	for i, val := range row {
		switch d := dest[i].(type) {
		case *string:
			if v, ok := val.(string); ok {
				*d = v
			}
		case *[]byte:
			if v, ok := val.([]byte); ok {
				*d = v
			}
		case *int:
			if v, ok := val.(int); ok {
				*d = v
			}
		case *time.Time:
			if v, ok := val.(time.Time); ok {
				*d = v
			}
		case **time.Time:
			if v, ok := val.(*time.Time); ok {
				*d = v
			}
		}
	}

	return nil
}

func (m *MockRows) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRows) Err() error {
	args := m.Called()
	return args.Error(0)
}

// Tests

func TestNewOutboxMessage(t *testing.T) {
	id := "msg-123"
	topic := "test-topic"
	payload := []byte("test payload")
	headers := map[string]string{"key": "value"}

	msg := NewOutboxMessage(id, topic, payload, headers)

	assert.Equal(t, id, msg.ID)
	assert.Equal(t, topic, msg.Topic)
	assert.Equal(t, payload, msg.Payload)
	assert.Equal(t, headers, msg.Headers)
	assert.NotZero(t, msg.CreatedAt)
	assert.Equal(t, 0, msg.RetryCount)
	assert.Nil(t, msg.ProcessedAt)
	assert.Empty(t, msg.Error)
}

func TestOutboxMessageToMessage(t *testing.T) {
	id := "msg-123"
	topic := "test-topic"
	payload := []byte("test payload")
	headers := map[string]string{"key": "value"}

	outboxMsg := NewOutboxMessage(id, topic, payload, headers)
	msg := outboxMsg.ToMessage()

	assert.Equal(t, id, msg.ID())
	assert.Equal(t, payload, msg.Body())
	assert.Equal(t, headers, msg.Headers())
}

func TestPostgresOutboxStore_CreateOutboxTable(t *testing.T) {
	mockDB := new(MockDatabase)
	mockResult := new(MockResult)

	ctx := context.Background()
	store := NewPostgresOutboxStore(mockDB)

	mockDB.On("Exec", ctx, mock.AnythingOfType("string")).Return(mockResult, nil)

	err := store.CreateOutboxTable(ctx)
	assert.NoError(t, err)

	mockDB.AssertExpectations(t)
}

func TestPostgresOutboxStore_SaveMessage(t *testing.T) {
	mockDB := new(MockDatabase)
	mockTx := new(MockTransaction)
	mockResult := new(MockResult)

	ctx := context.Background()
	store := NewPostgresOutboxStore(mockDB)

	msg := NewOutboxMessage("msg-123", "test-topic", []byte("test payload"), map[string]string{"key": "value"})

	mockTx.On("Exec", ctx, mock.AnythingOfType("string"),
		msg.ID, msg.Topic, msg.Payload, mock.AnythingOfType("[]uint8"), msg.CreatedAt).Return(mockResult, nil)

	err := store.SaveMessage(ctx, mockTx, msg)
	assert.NoError(t, err)

	mockTx.AssertExpectations(t)
}

func TestPostgresOutboxStore_GetPendingMessages(t *testing.T) {
	mockDB := new(MockDatabase)

	ctx := context.Background()
	store := NewPostgresOutboxStore(mockDB)

	now := time.Now()
	headersJSON := []byte(`{"key":"value"}`)

	mockRows := NewMockRows([][]interface{}{
		{"msg-123", "test-topic", []byte("test payload"), headersJSON, now, nil, 0, ""},
		{"msg-456", "test-topic", []byte("another payload"), headersJSON, now, nil, 1, "error message"},
	})

	mockRows.On("Close").Return(nil)
	mockRows.On("Err").Return(nil)

	mockDB.On("Query", ctx, mock.AnythingOfType("string"), 10).Return(mockRows, nil)

	messages, err := store.GetPendingMessages(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)

	assert.Equal(t, "msg-123", messages[0].ID)
	assert.Equal(t, "test-topic", messages[0].Topic)
	assert.Equal(t, []byte("test payload"), messages[0].Payload)
	assert.Equal(t, map[string]string{"key": "value"}, messages[0].Headers)
	assert.Equal(t, 0, messages[0].RetryCount)
	assert.Empty(t, messages[0].Error)

	assert.Equal(t, "msg-456", messages[1].ID)
	assert.Equal(t, 1, messages[1].RetryCount)
	assert.Equal(t, "error message", messages[1].Error)

	mockDB.AssertExpectations(t)
}

func TestPostgresOutboxStore_MarkProcessed(t *testing.T) {
	mockDB := new(MockDatabase)
	mockResult := new(MockResult)

	ctx := context.Background()
	store := NewPostgresOutboxStore(mockDB)

	mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Time"), "msg-123").Return(mockResult, nil)

	err := store.MarkProcessed(ctx, "msg-123")
	assert.NoError(t, err)

	mockDB.AssertExpectations(t)
}

func TestPostgresOutboxStore_MarkFailed(t *testing.T) {
	mockDB := new(MockDatabase)
	mockResult := new(MockResult)

	ctx := context.Background()
	store := NewPostgresOutboxStore(mockDB)
	testErr := errors.New("test error")

	mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
		testErr.Error(), "msg-123").Return(mockResult, nil)

	err := store.MarkFailed(ctx, "msg-123", testErr)
	assert.NoError(t, err)

	mockDB.AssertExpectations(t)
}
