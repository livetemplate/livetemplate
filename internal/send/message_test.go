package send

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseActionFromHTTP_ValidInput tests parsing valid HTTP action messages.
func TestParseActionFromHTTP_ValidInput(t *testing.T) {
	body := strings.NewReader(`{"action":"increment","data":{"amount":"5"}}`)
	req := httptest.NewRequest("POST", "/", body)

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Action != "increment" {
		t.Errorf("Expected action 'increment', got: %q", msg.Action)
	}

	if msg.Data["amount"] != "5" {
		t.Errorf("Expected data amount '5', got: %v", msg.Data["amount"])
	}
}

// TestParseActionFromHTTP_EmptyData tests that empty data is initialized.
func TestParseActionFromHTTP_EmptyData(t *testing.T) {
	body := strings.NewReader(`{"action":"test"}`)
	req := httptest.NewRequest("POST", "/", body)

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Data == nil {
		t.Error("Expected data map to be initialized, got nil")
	}

	if len(msg.Data) != 0 {
		t.Errorf("Expected empty data map, got: %v", msg.Data)
	}
}

// TestParseActionFromHTTP_NullData tests that null data is initialized.
func TestParseActionFromHTTP_NullData(t *testing.T) {
	body := strings.NewReader(`{"action":"test","data":null}`)
	req := httptest.NewRequest("POST", "/", body)

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Data == nil {
		t.Error("Expected data map to be initialized, got nil")
	}
}

// TestParseActionFromHTTP_MalformedJSON tests handling of malformed JSON.
func TestParseActionFromHTTP_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{"action": invalid}`},
		{"unclosed brace", `{"action":"test"`},
		{"trailing comma", `{"action":"test",}`},
		{"empty string", ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.body)
			req := httptest.NewRequest("POST", "/", body)

			_, err := ParseActionFromHTTP(req)
			if err == nil {
				t.Error("Expected error for malformed JSON, got nil")
			}

			if !strings.Contains(err.Error(), "failed to parse action") {
				t.Errorf("Expected 'failed to parse action' error, got: %v", err)
			}
		})
	}
}

// TestParseActionFromHTTP_EmptyBody tests handling of empty request body.
func TestParseActionFromHTTP_EmptyBody(t *testing.T) {
	body := bytes.NewReader([]byte{})
	req := httptest.NewRequest("POST", "/", body)

	_, err := ParseActionFromHTTP(req)
	if err == nil {
		t.Error("Expected error for empty body, got nil")
	}
}

// TestParseActionFromWebSocket_ValidInput tests parsing valid WebSocket messages.
func TestParseActionFromWebSocket_ValidInput(t *testing.T) {
	data := []byte(`{"action":"decrement","data":{"value":"10"}}`)

	msg, err := ParseActionFromWebSocket(data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Action != "decrement" {
		t.Errorf("Expected action 'decrement', got: %q", msg.Action)
	}

	if msg.Data["value"] != "10" {
		t.Errorf("Expected data value '10', got: %v", msg.Data["value"])
	}
}

// TestParseActionFromWebSocket_EmptyData tests that empty data is initialized.
func TestParseActionFromWebSocket_EmptyData(t *testing.T) {
	data := []byte(`{"action":"reset"}`)

	msg, err := ParseActionFromWebSocket(data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Data == nil {
		t.Error("Expected data map to be initialized, got nil")
	}

	if len(msg.Data) != 0 {
		t.Errorf("Expected empty data map, got: %v", msg.Data)
	}
}

// TestParseActionFromWebSocket_NullData tests that null data is initialized.
func TestParseActionFromWebSocket_NullData(t *testing.T) {
	data := []byte(`{"action":"test","data":null}`)

	msg, err := ParseActionFromWebSocket(data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Data == nil {
		t.Error("Expected data map to be initialized, got nil")
	}
}

// TestParseActionFromWebSocket_MalformedJSON tests handling of malformed JSON.
func TestParseActionFromWebSocket_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"invalid json", `{"action": invalid}`},
		{"unclosed brace", `{"action":"test"`},
		{"trailing comma", `{"action":"test",}`},
		{"empty bytes", ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseActionFromWebSocket([]byte(tt.data))
			if err == nil {
				t.Error("Expected error for malformed JSON, got nil")
			}

			if !strings.Contains(err.Error(), "failed to parse action") {
				t.Errorf("Expected 'failed to parse action' error, got: %v", err)
			}
		})
	}
}

// TestParseActionFromWebSocket_ComplexData tests parsing complex data structures.
func TestParseActionFromWebSocket_ComplexData(t *testing.T) {
	data := []byte(`{
		"action":"update",
		"data":{
			"user":{"name":"John","age":30},
			"items":["a","b","c"],
			"count":42
		}
	}`)

	msg, err := ParseActionFromWebSocket(data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if msg.Action != "update" {
		t.Errorf("Expected action 'update', got: %q", msg.Action)
	}

	// Verify nested object
	user, ok := msg.Data["user"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'user' to be a map")
	} else {
		if user["name"] != "John" {
			t.Errorf("Expected user.name 'John', got: %v", user["name"])
		}
	}

	// Verify array
	items, ok := msg.Data["items"].([]interface{})
	if !ok {
		t.Error("Expected 'items' to be an array")
	} else {
		if len(items) != 3 {
			t.Errorf("Expected 3 items, got: %d", len(items))
		}
	}

	// Verify number
	count, ok := msg.Data["count"].(float64)
	if !ok {
		t.Error("Expected 'count' to be a number")
	} else {
		if count != 42 {
			t.Errorf("Expected count 42, got: %v", count)
		}
	}
}

// mockConnectionSender is a mock implementation of ConnectionSender for testing.
type mockConnectionSender struct {
	sendCalled bool
	lastData   []byte
	sendError  error
}

func (m *mockConnectionSender) Send(messageType int, data []byte) error {
	m.sendCalled = true
	m.lastData = data
	return m.sendError
}

// TestWriteUpdateToWebSocket_Success tests successful write to WebSocket.
func TestWriteUpdateToWebSocket_Success(t *testing.T) {
	update := []byte(`{"tree":{"s":["<div>","</div>"],"0":"test"}}`)

	mock := &mockConnectionSender{}
	err := WriteUpdateToWebSocket(mock, update)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !mock.sendCalled {
		t.Error("Expected Send to be called")
	}

	if string(mock.lastData) != string(update) {
		t.Errorf("Expected data %q, got %q", update, mock.lastData)
	}
}

// TestActionMessage_DataInitialization tests that data is always initialized.
func TestActionMessage_DataInitialization(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		parse    func([]byte) (ActionMessage, error)
	}{
		{
			name:     "websocket null data",
			jsonData: `{"action":"test","data":null}`,
			parse: func(data []byte) (ActionMessage, error) {
				return ParseActionFromWebSocket(data)
			},
		},
		{
			name:     "websocket missing data",
			jsonData: `{"action":"test"}`,
			parse: func(data []byte) (ActionMessage, error) {
				return ParseActionFromWebSocket(data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tt.parse([]byte(tt.jsonData))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if msg.Data == nil {
				t.Error("Expected data map to be initialized, got nil")
			}
		})
	}
}
