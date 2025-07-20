package messaging

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncAPIGenerator(t *testing.T) {
	// Create a new generator
	generator := NewAsyncAPIGenerator(
		"Test API",
		"1.0.0",
		"Test AsyncAPI specification",
	)

	// Add servers
	generator.AddRabbitMQServer(
		"rabbitmq",
		"localhost:5672",
		"",
		"RabbitMQ server",
	)

	generator.AddKafkaServer(
		"kafka",
		"localhost:9092",
		"Kafka server",
	)

	// Add a channel
	generator.AddChannel(
		"user.created",
		"User creation events",
	)

	// Add schemas
	userSchema := generator.CreateObjectSchema(
		map[string]*AsyncAPISchema{
			"id":        generator.CreateStringSchema("User ID", "uuid"),
			"name":      generator.CreateStringSchema("User name", ""),
			"email":     generator.CreateStringSchema("User email", "email"),
			"createdAt": generator.CreateStringSchema("Creation timestamp", "date-time"),
		},
		[]string{"id", "name", "email"},
	)

	generator.AddSchema("User", userSchema)

	// Add message
	generator.AddMessage(
		"UserCreatedMessage",
		"User Created",
		"Published when a new user is created",
		"Contains information about the newly created user",
		nil,
		generator.CreateSchemaRef("User"),
	)

	// Add operations
	generator.AddPublishOperation(
		"user.created",
		"Publish User Created",
		"Publish a message when a new user is created",
		"publishUserCreated",
		"UserCreatedMessage",
	)

	generator.AddSubscribeOperation(
		"user.created",
		"Subscribe to User Created",
		"Receive notifications when new users are created",
		"subscribeUserCreated",
		"UserCreatedMessage",
	)

	// Convert to JSON
	data, err := generator.ToJSON()
	require.NoError(t, err)

	// Verify JSON is valid
	var jsonObj map[string]interface{}
	err = json.Unmarshal(data, &jsonObj)
	require.NoError(t, err)

	// Verify basic structure
	assert.Equal(t, AsyncAPIVersion, jsonObj["asyncapi"])
	assert.Equal(t, "Test API", jsonObj["info"].(map[string]interface{})["title"])

	// Test GenerateFromTopics
	topicsGenerator := GenerateFromTopics(
		"Topics API",
		"1.0.0",
		"Generated from topics",
		"default",
		"kafka://localhost:9092",
		"kafka",
		[]string{"orders.created", "orders.updated", "orders.deleted"},
	)

	// Convert to JSON
	topicsData, err := topicsGenerator.ToJSON()
	require.NoError(t, err)

	// Verify JSON is valid
	err = json.Unmarshal(topicsData, &jsonObj)
	require.NoError(t, err)

	// Verify channels were created
	channels := jsonObj["channels"].(map[string]interface{})
	assert.Contains(t, channels, "orders.created")
	assert.Contains(t, channels, "orders.updated")
	assert.Contains(t, channels, "orders.deleted")

	// Test file saving and loading (in a temp file)
	tempFile := os.TempDir() + "/asyncapi-test.json"
	defer os.Remove(tempFile)

	err = generator.SaveToFile(tempFile)
	require.NoError(t, err)

	loadedGenerator, err := LoadAsyncAPIFromFile(tempFile)
	require.NoError(t, err)

	loadedData, err := loadedGenerator.ToJSON()
	require.NoError(t, err)

	// Compare original and loaded data
	var originalObj, loadedObj map[string]interface{}
	err = json.Unmarshal(data, &originalObj)
	require.NoError(t, err)
	err = json.Unmarshal(loadedData, &loadedObj)
	require.NoError(t, err)

	assert.Equal(t, originalObj["asyncapi"], loadedObj["asyncapi"])
	assert.Equal(t, originalObj["info"].(map[string]interface{})["title"],
		loadedObj["info"].(map[string]interface{})["title"])
}
