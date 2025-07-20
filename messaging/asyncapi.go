package messaging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AsyncAPIVersion is the version of AsyncAPI specification used
const AsyncAPIVersion = "2.6.0"

// AsyncAPIDocument represents an AsyncAPI specification document
type AsyncAPIDocument struct {
	AsyncAPI           string                     `json:"asyncapi"`
	Info               AsyncAPIInfo               `json:"info"`
	Servers            map[string]AsyncAPIServer  `json:"servers,omitempty"`
	Channels           map[string]AsyncAPIChannel `json:"channels"`
	Components         *AsyncAPIComponents        `json:"components,omitempty"`
	DefaultContentType string                     `json:"defaultContentType,omitempty"`
}

// AsyncAPIInfo contains metadata about the API
type AsyncAPIInfo struct {
	Title          string           `json:"title"`
	Version        string           `json:"version"`
	Description    string           `json:"description,omitempty"`
	TermsOfService string           `json:"termsOfService,omitempty"`
	Contact        *AsyncAPIContact `json:"contact,omitempty"`
	License        *AsyncAPILicense `json:"license,omitempty"`
}

// AsyncAPIContact information for the exposed API
type AsyncAPIContact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// AsyncAPILicense information for the exposed API
type AsyncAPILicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// AsyncAPIServer represents a server where the API is available
type AsyncAPIServer struct {
	URL         string                 `json:"url"`
	Protocol    string                 `json:"protocol"`
	Description string                 `json:"description,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Security    []map[string][]string  `json:"security,omitempty"`
}

// AsyncAPIChannel represents a channel of communication
type AsyncAPIChannel struct {
	Description string                 `json:"description,omitempty"`
	Subscribe   *AsyncAPIOperation     `json:"subscribe,omitempty"`
	Publish     *AsyncAPIOperation     `json:"publish,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// AsyncAPIOperation represents an operation on a channel
type AsyncAPIOperation struct {
	Summary     string           `json:"summary,omitempty"`
	Description string           `json:"description,omitempty"`
	OperationID string           `json:"operationId,omitempty"`
	Tags        []AsyncAPITag    `json:"tags,omitempty"`
	Message     *AsyncAPIMessage `json:"message,omitempty"`
}

// AsyncAPITag represents a tag for an operation
type AsyncAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// AsyncAPIMessage represents a message being sent or received
type AsyncAPIMessage struct {
	Headers       *AsyncAPISchema        `json:"headers,omitempty"`
	Payload       *AsyncAPISchema        `json:"payload,omitempty"`
	CorrelationID *AsyncAPICorrelationID `json:"correlationId,omitempty"`
	ContentType   string                 `json:"contentType,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Title         string                 `json:"title,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Examples      []interface{}          `json:"examples,omitempty"`
	Ref           string                 `json:"$ref,omitempty"`
}

// AsyncAPICorrelationID represents a correlation ID for a message
type AsyncAPICorrelationID struct {
	Location    string `json:"location"`
	Description string `json:"description,omitempty"`
}

// AsyncAPISchema represents a JSON Schema for message validation
type AsyncAPISchema struct {
	Type                 string                     `json:"type,omitempty"`
	Format               string                     `json:"format,omitempty"`
	Properties           map[string]*AsyncAPISchema `json:"properties,omitempty"`
	Required             []string                   `json:"required,omitempty"`
	Description          string                     `json:"description,omitempty"`
	Items                *AsyncAPISchema            `json:"items,omitempty"`
	AdditionalProperties interface{}                `json:"additionalProperties,omitempty"`
	Ref                  string                     `json:"$ref,omitempty"`
}

// AsyncAPIComponents holds reusable objects for the specification
type AsyncAPIComponents struct {
	Schemas         map[string]*AsyncAPISchema  `json:"schemas,omitempty"`
	Messages        map[string]*AsyncAPIMessage `json:"messages,omitempty"`
	SecuritySchemes map[string]interface{}      `json:"securitySchemes,omitempty"`
}

// AsyncAPIGenerator generates AsyncAPI specifications for messaging systems
type AsyncAPIGenerator struct {
	document AsyncAPIDocument
}

// NewAsyncAPIGenerator creates a new AsyncAPI specification generator
func NewAsyncAPIGenerator(title, version, description string) *AsyncAPIGenerator {
	return &AsyncAPIGenerator{
		document: AsyncAPIDocument{
			AsyncAPI: AsyncAPIVersion,
			Info: AsyncAPIInfo{
				Title:       title,
				Version:     version,
				Description: description,
			},
			Servers:            make(map[string]AsyncAPIServer),
			Channels:           make(map[string]AsyncAPIChannel),
			DefaultContentType: "application/json",
			Components: &AsyncAPIComponents{
				Schemas:  make(map[string]*AsyncAPISchema),
				Messages: make(map[string]*AsyncAPIMessage),
			},
		},
	}
}

// AddServer adds a server to the AsyncAPI specification
func (g *AsyncAPIGenerator) AddServer(name, url, protocol, description string) {
	g.document.Servers[name] = AsyncAPIServer{
		URL:         url,
		Protocol:    protocol,
		Description: description,
	}
}

// AddRabbitMQServer adds a RabbitMQ server to the AsyncAPI specification
func (g *AsyncAPIGenerator) AddRabbitMQServer(name, host, virtualHost, description string) {
	url := fmt.Sprintf("amqp://{username}:{password}@%s/{virtualHost}", host)
	if virtualHost == "" {
		virtualHost = "/"
	}

	server := AsyncAPIServer{
		URL:         url,
		Protocol:    "amqp",
		Description: description,
		Variables: map[string]interface{}{
			"username": map[string]string{
				"description": "RabbitMQ username",
				"default":     "guest",
			},
			"password": map[string]string{
				"description": "RabbitMQ password",
				"default":     "guest",
			},
			"virtualHost": map[string]string{
				"description": "RabbitMQ virtual host",
				"default":     virtualHost,
			},
		},
	}

	g.document.Servers[name] = server
}

// AddKafkaServer adds a Kafka server to the AsyncAPI specification
func (g *AsyncAPIGenerator) AddKafkaServer(name, host, description string) {
	url := fmt.Sprintf("kafka://%s", host)

	server := AsyncAPIServer{
		URL:         url,
		Protocol:    "kafka",
		Description: description,
	}

	g.document.Servers[name] = server
}

// AddChannel adds a channel to the AsyncAPI specification
func (g *AsyncAPIGenerator) AddChannel(name, description string) {
	g.document.Channels[name] = AsyncAPIChannel{
		Description: description,
	}
}

// AddPublishOperation adds a publish operation to a channel
func (g *AsyncAPIGenerator) AddPublishOperation(
	channel, summary, description, operationId string,
	messageRef string,
) {
	ch, exists := g.document.Channels[channel]
	if !exists {
		ch = AsyncAPIChannel{}
	}

	ch.Publish = &AsyncAPIOperation{
		Summary:     summary,
		Description: description,
		OperationID: operationId,
		Message: &AsyncAPIMessage{
			Ref: fmt.Sprintf("#/components/messages/%s", messageRef),
		},
	}

	g.document.Channels[channel] = ch
}

// AddSubscribeOperation adds a subscribe operation to a channel
func (g *AsyncAPIGenerator) AddSubscribeOperation(
	channel, summary, description, operationId string,
	messageRef string,
) {
	ch, exists := g.document.Channels[channel]
	if !exists {
		ch = AsyncAPIChannel{}
	}

	ch.Subscribe = &AsyncAPIOperation{
		Summary:     summary,
		Description: description,
		OperationID: operationId,
		Message: &AsyncAPIMessage{
			Ref: fmt.Sprintf("#/components/messages/%s", messageRef),
		},
	}

	g.document.Channels[channel] = ch
}

// AddMessage adds a message definition to the components section
func (g *AsyncAPIGenerator) AddMessage(
	name, title, summary, description string,
	headers, payload *AsyncAPISchema,
) {
	g.document.Components.Messages[name] = &AsyncAPIMessage{
		Name:        name,
		Title:       title,
		Summary:     summary,
		Description: description,
		Headers:     headers,
		Payload:     payload,
		CorrelationID: &AsyncAPICorrelationID{
			Location:    "$message.headers#/correlation-id",
			Description: "Correlation ID for message tracing",
		},
	}
}

// AddSchema adds a schema definition to the components section
func (g *AsyncAPIGenerator) AddSchema(name string, schema *AsyncAPISchema) {
	g.document.Components.Schemas[name] = schema
}

// CreateObjectSchema creates a simple object schema with properties
func (g *AsyncAPIGenerator) CreateObjectSchema(properties map[string]*AsyncAPISchema, required []string) *AsyncAPISchema {
	return &AsyncAPISchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

// CreateStringSchema creates a string schema with optional format
func (g *AsyncAPIGenerator) CreateStringSchema(description, format string) *AsyncAPISchema {
	schema := &AsyncAPISchema{
		Type:        "string",
		Description: description,
	}

	if format != "" {
		schema.Format = format
	}

	return schema
}

// CreateNumberSchema creates a number schema
func (g *AsyncAPIGenerator) CreateNumberSchema(description string) *AsyncAPISchema {
	return &AsyncAPISchema{
		Type:        "number",
		Description: description,
	}
}

// CreateArraySchema creates an array schema with item type
func (g *AsyncAPIGenerator) CreateArraySchema(description string, items *AsyncAPISchema) *AsyncAPISchema {
	return &AsyncAPISchema{
		Type:        "array",
		Description: description,
		Items:       items,
	}
}

// CreateSchemaRef creates a reference to a schema in the components section
func (g *AsyncAPIGenerator) CreateSchemaRef(schemaName string) *AsyncAPISchema {
	return &AsyncAPISchema{
		Ref: fmt.Sprintf("#/components/schemas/%s", schemaName),
	}
}

// ToJSON converts the AsyncAPI document to JSON
func (g *AsyncAPIGenerator) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g.document, "", "  ")
}

// SaveToFile saves the AsyncAPI document to a file
func (g *AsyncAPIGenerator) SaveToFile(filePath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Convert to JSON
	data, err := g.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to convert to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadFromFile loads an AsyncAPI document from a file
func LoadAsyncAPIFromFile(filePath string) (*AsyncAPIGenerator, error) {
	// Read file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var doc AsyncAPIDocument
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create generator with loaded document
	generator := &AsyncAPIGenerator{
		document: doc,
	}

	return generator, nil
}

// GenerateFromTopics creates an AsyncAPI specification from a list of topics
func GenerateFromTopics(
	title, version, description string,
	serverName, serverURL, protocol string,
	topics []string,
) *AsyncAPIGenerator {
	generator := NewAsyncAPIGenerator(title, version, description)

	// Add server
	generator.AddServer(serverName, serverURL, protocol, "Default server")

	// Add default message schema
	defaultPayloadSchema := &AsyncAPISchema{
		Type:                 "object",
		AdditionalProperties: true,
	}

	defaultHeadersSchema := &AsyncAPISchema{
		Type: "object",
		Properties: map[string]*AsyncAPISchema{
			"correlation-id": {
				Type:        "string",
				Description: "Correlation ID for message tracing",
			},
			"timestamp": {
				Type:        "string",
				Format:      "date-time",
				Description: "Message timestamp",
			},
		},
	}

	// Add channels for each topic
	for _, topic := range topics {
		// Clean topic name for use as identifier
		cleanTopic := strings.ReplaceAll(topic, ".", "-")
		cleanTopic = strings.ReplaceAll(cleanTopic, "/", "-")

		// Add channel
		generator.AddChannel(topic, fmt.Sprintf("Channel for %s messages", topic))

		// Add message
		messageName := fmt.Sprintf("%sMessage", cleanTopic)
		generator.AddMessage(
			messageName,
			fmt.Sprintf("%s Message", topic),
			fmt.Sprintf("Message for %s topic", topic),
			fmt.Sprintf("Message published/subscribed on the %s topic", topic),
			defaultHeadersSchema,
			defaultPayloadSchema,
		)

		// Add operations
		generator.AddPublishOperation(
			topic,
			fmt.Sprintf("Publish to %s", topic),
			fmt.Sprintf("Publish a message to the %s topic", topic),
			fmt.Sprintf("publish%s", strings.Title(cleanTopic)),
			messageName,
		)

		generator.AddSubscribeOperation(
			topic,
			fmt.Sprintf("Subscribe to %s", topic),
			fmt.Sprintf("Receive messages from the %s topic", topic),
			fmt.Sprintf("subscribe%s", strings.Title(cleanTopic)),
			messageName,
		)
	}

	return generator
}
