package messaging

// Message defines the interface for a message that can be published or received
type Message interface {
	// ID returns the unique identifier of the message
	ID() string

	// Body returns the payload of the message
	Body() []byte

	// Headers returns the metadata associated with the message
	Headers() map[string]string
}

// DefaultMessage is a basic implementation of the Message interface
type DefaultMessage struct {
	id      string
	body    []byte
	headers map[string]string
}

// NewMessage creates a new DefaultMessage with the provided parameters
func NewMessage(id string, body []byte, headers map[string]string) *DefaultMessage {
	if headers == nil {
		headers = make(map[string]string)
	}

	return &DefaultMessage{
		id:      id,
		body:    body,
		headers: headers,
	}
}

// ID returns the unique identifier of the message
func (m *DefaultMessage) ID() string {
	return m.id
}

// Body returns the payload of the message
func (m *DefaultMessage) Body() []byte {
	return m.body
}

// Headers returns the metadata associated with the message
func (m *DefaultMessage) Headers() map[string]string {
	return m.headers
}
