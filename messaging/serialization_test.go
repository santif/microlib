package messaging

import (
	"testing"
)

type testStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestNewJSONSerializer(t *testing.T) {
	serializer := NewJSONSerializer()
	if serializer == nil {
		t.Fatal("Expected non-nil serializer")
	}
}

func TestJSONSerializer_Serialize(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test serializing a struct
	data := testStruct{
		Name:  "test",
		Value: 42,
	}

	result, err := serializer.Serialize(data)
	if err != nil {
		t.Fatalf("Expected no error when serializing struct, got: %v", err)
	}

	expected := `{"name":"test","value":42}`
	if string(result) != expected {
		t.Errorf("Expected serialized data %s, got %s", expected, string(result))
	}

	// Test serializing a string
	stringData := "hello world"
	result, err = serializer.Serialize(stringData)
	if err != nil {
		t.Fatalf("Expected no error when serializing string, got: %v", err)
	}

	expected = `"hello world"`
	if string(result) != expected {
		t.Errorf("Expected serialized data %s, got %s", expected, string(result))
	}

	// Test serializing nil
	result, err = serializer.Serialize(nil)
	if err != nil {
		t.Fatalf("Expected no error when serializing nil, got: %v", err)
	}

	expected = "null"
	if string(result) != expected {
		t.Errorf("Expected serialized data %s, got %s", expected, string(result))
	}
}

func TestJSONSerializer_SerializeInvalidData(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test serializing a channel (which can't be serialized to JSON)
	ch := make(chan int)
	_, err := serializer.Serialize(ch)
	if err == nil {
		t.Error("Expected error when serializing channel")
	}
}

func TestJSONSerializer_Deserialize(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test deserializing to struct
	data := []byte(`{"name":"test","value":42}`)
	var result testStruct
	err := serializer.Deserialize(data, &result)
	if err != nil {
		t.Fatalf("Expected no error when deserializing to struct, got: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("Expected value 42, got %d", result.Value)
	}

	// Test deserializing to string
	data = []byte(`"hello world"`)
	var stringResult string
	err = serializer.Deserialize(data, &stringResult)
	if err != nil {
		t.Fatalf("Expected no error when deserializing to string, got: %v", err)
	}

	if stringResult != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", stringResult)
	}

	// Test deserializing to interface{}
	data = []byte(`{"key":"value"}`)
	var interfaceResult interface{}
	err = serializer.Deserialize(data, &interfaceResult)
	if err != nil {
		t.Fatalf("Expected no error when deserializing to interface{}, got: %v", err)
	}

	resultMap, ok := interfaceResult.(map[string]interface{})
	if !ok {
		t.Error("Expected result to be a map")
	} else if resultMap["key"] != "value" {
		t.Errorf("Expected key 'value', got '%v'", resultMap["key"])
	}
}

func TestJSONSerializer_DeserializeInvalidJSON(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test deserializing invalid JSON
	data := []byte(`{"invalid": json}`)
	var result testStruct
	err := serializer.Deserialize(data, &result)
	if err == nil {
		t.Error("Expected error when deserializing invalid JSON")
	}
}

func TestJSONSerializer_DeserializeNilDestination(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test deserializing to nil destination
	data := []byte(`{"name":"test","value":42}`)
	err := serializer.Deserialize(data, nil)
	if err == nil {
		t.Error("Expected error when deserializing to nil destination")
	}
}

func TestJSONSerializer_DeserializeEmptyData(t *testing.T) {
	serializer := NewJSONSerializer()

	// Test deserializing empty data
	var result testStruct
	err := serializer.Deserialize([]byte{}, &result)
	if err == nil {
		t.Error("Expected error when deserializing empty data")
	}

	// Test deserializing nil data
	err = serializer.Deserialize(nil, &result)
	if err == nil {
		t.Error("Expected error when deserializing nil data")
	}
}
