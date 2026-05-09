package send

import (
	"bytes"
	"fmt"
	"mime/multipart"
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

// TestParseActionFromHTTP_URLEncoded tests parsing URL-encoded form data.
func TestParseActionFromHTTP_URLEncoded(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantAction string
		wantData   map[string]interface{}
		wantErr    bool
	}{
		{
			name:       "basic form with lvt-action",
			body:       "lvt-action=login&username=testuser&password=secret",
			wantAction: "login",
			wantData: map[string]interface{}{
				"username": "testuser",
				"password": "secret",
			},
		},
		{
			name:       "action field is normal data (not routing)",
			body:       "action=logout",
			wantAction: "",
			wantData: map[string]interface{}{
				"action": "logout",
			},
		},
		{
			name:       "lvt-action routes, action is data",
			body:       "lvt-action=real&action=fallback&data=test",
			wantAction: "real",
			wantData: map[string]interface{}{
				"action": "fallback",
				"data":   "test",
			},
		},
		{
			name:       "empty form",
			body:       "",
			wantAction: "",
			wantData:   map[string]interface{}{},
		},
		{
			name:       "form with special characters",
			body:       "lvt-action=update&email=test%40example.com&name=John+Doe",
			wantAction: "update",
			wantData: map[string]interface{}{
				"email": "test@example.com",
				"name":  "John Doe",
			},
		},
		{
			name:       "button name as action (empty value)",
			body:       "increment=&title=test",
			wantAction: "increment",
			wantData: map[string]interface{}{
				"title": "test",
			},
		},
		{
			name:       "button name as action (standalone)",
			body:       "delete=",
			wantAction: "delete",
			wantData:   map[string]interface{}{},
		},
		{
			// "action" with a value is normal data; "draft=" is the button-name
			// candidate because it's the only empty-value field and "action" is
			// excluded from the button-name scan to avoid routing ambiguity.
			name:       "action is data, button name is routing",
			body:       "action=save&draft=",
			wantAction: "draft",
			wantData: map[string]interface{}{
				"action": "save",
			},
		},
		{
			// Regression: empty action= must NOT be interpreted as button-name
			// routing. The "action" key is excluded from the empty-value scan.
			name:       "empty action field is normal data (not routing)",
			body:       "action=",
			wantAction: "",
			wantData: map[string]interface{}{
				"action": "",
			},
		},
		{
			name:       "ambiguous empty values skips button detection",
			body:       "delete=&archive=&title=test",
			wantAction: "",
			wantData: map[string]interface{}{
				"delete":  "",
				"archive": "",
				"title":   "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			msg, err := ParseActionFromHTTP(req)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if msg.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", msg.Action, tt.wantAction)
			}

			for key, want := range tt.wantData {
				got := msg.Data[key]
				if got != want {
					t.Errorf("Data[%q] = %v, want %v", key, got, want)
				}
			}

			// Check no extra data fields
			for key := range msg.Data {
				if _, ok := tt.wantData[key]; !ok {
					t.Errorf("Unexpected data field: %q = %v", key, msg.Data[key])
				}
			}
		})
	}
}

// multipartField writes a field to a multipart writer, failing the test on error.
func multipartField(t *testing.T, w *multipart.Writer, key, value string) {
	t.Helper()
	if err := w.WriteField(key, value); err != nil {
		t.Fatalf("Failed to write multipart field %q: %v", key, err)
	}
}

// closeMultipart closes the multipart writer, failing the test on error.
func closeMultipart(t *testing.T, w *multipart.Writer) {
	t.Helper()
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}
}

// TestParseActionFromHTTP_Multipart_IndividualFields tests that multipart forms
// with individual form fields (not a JSON "data" blob) are parsed correctly.
func TestParseActionFromHTTP_Multipart_IndividualFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "lvt-action", "updateProfile")
	multipartField(t, writer, "name", "Jane Doe")
	multipartField(t, writer, "email", "jane@example.com")
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "updateProfile" {
		t.Errorf("Action = %q, want %q", msg.Action, "updateProfile")
	}
	if msg.Data["name"] != "Jane Doe" {
		t.Errorf("Data[name] = %q, want %q", msg.Data["name"], "Jane Doe")
	}
	if msg.Data["email"] != "jane@example.com" {
		t.Errorf("Data[email] = %q, want %q", msg.Data["email"], "jane@example.com")
	}
	if _, ok := msg.Data["lvt-action"]; ok {
		t.Error("lvt-action should not appear in data map")
	}
}

// TestParseActionFromHTTP_Multipart_ButtonNameAction tests button-name-as-action
// detection in multipart forms (e.g., <button name="save"> with empty value).
func TestParseActionFromHTTP_Multipart_ButtonNameAction(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "updateProfile", "") // button name with empty value
	multipartField(t, writer, "name", "Jane Doe")
	multipartField(t, writer, "email", "jane@example.com")
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "updateProfile" {
		t.Errorf("Action = %q, want %q", msg.Action, "updateProfile")
	}
	if msg.Data["name"] != "Jane Doe" {
		t.Errorf("Data[name] = %q, want %q", msg.Data["name"], "Jane Doe")
	}
	if _, ok := msg.Data["updateProfile"]; ok {
		t.Error("Button name field should not appear in data map")
	}
}

// TestParseActionFromHTTP_Multipart_WithFileAndFields tests that text fields
// are parsed alongside file uploads in multipart forms.
func TestParseActionFromHTTP_Multipart_WithFileAndFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "updateProfile", "") // button name
	multipartField(t, writer, "name", "Jane Doe")
	multipartField(t, writer, "email", "jane@example.com")
	filePart, err := writer.CreateFormFile("avatar", "photo.png")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := fmt.Fprint(filePart, "fake-png-data"); err != nil {
		t.Fatalf("Failed to write file data: %v", err)
	}
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "updateProfile" {
		t.Errorf("Action = %q, want %q", msg.Action, "updateProfile")
	}
	if msg.Data["name"] != "Jane Doe" {
		t.Errorf("Data[name] = %q, want %q", msg.Data["name"], "Jane Doe")
	}
	if msg.Data["email"] != "jane@example.com" {
		t.Errorf("Data[email] = %q, want %q", msg.Data["email"], "jane@example.com")
	}
}

// TestParseActionFromHTTP_Multipart_JSONDataTakesPrecedence tests that
// when a JSON "data" blob is present, it takes precedence over individual fields.
func TestParseActionFromHTTP_Multipart_JSONDataTakesPrecedence(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "lvt-action", "updateProfile")
	multipartField(t, writer, "data", `{"name":"FromJSON","extra":"value"}`)
	multipartField(t, writer, "name", "FromField") // should be ignored
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Data["name"] != "FromJSON" {
		t.Errorf("Data[name] = %q, want %q (JSON should take precedence)", msg.Data["name"], "FromJSON")
	}
	if msg.Data["extra"] != "value" {
		t.Errorf("Data[extra] = %q, want %q", msg.Data["extra"], "value")
	}
}

// TestParseActionFromHTTP_Multipart_EmptyJSONDataNoFallback tests that when
// data={} is sent (valid but empty JSON), the fallback to individual fields
// does NOT activate — JSON data takes precedence even when empty.
func TestParseActionFromHTTP_Multipart_EmptyJSONDataNoFallback(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "lvt-action", "updateProfile")
	multipartField(t, writer, "data", `{}`)              // empty JSON object
	multipartField(t, writer, "name", "ShouldBeIgnored") // individual field
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "updateProfile" {
		t.Errorf("Action = %q, want %q", msg.Action, "updateProfile")
	}
	// data={} was parsed successfully, so individual fields should NOT be read
	if _, ok := msg.Data["name"]; ok {
		t.Error("Individual field 'name' should not appear when JSON data={} was parsed")
	}
	if len(msg.Data) != 0 {
		t.Errorf("Data should be empty (from JSON {}), got %v", msg.Data)
	}
}

// TestParseActionFromHTTP_Multipart_InvalidJSONFallback documents that invalid JSON
// in the "data" field causes a silent fallback to individual field parsing. This is
// the intended behavior: the "data" field is treated as a client library convention,
// not a hard contract. Browser-native submissions may include a field named "data"
// that isn't JSON.
func TestParseActionFromHTTP_Multipart_InvalidJSONFallback(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "lvt-action", "save")
	multipartField(t, writer, "data", "not-valid-json") // invalid JSON
	multipartField(t, writer, "name", "Jane")
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "save" {
		t.Errorf("Action = %q, want %q", msg.Action, "save")
	}
	// Fallback activates: individual fields are read (excluding "data" itself)
	if msg.Data["name"] != "Jane" {
		t.Errorf("Data[name] = %q, want %q (fallback should read individual fields)", msg.Data["name"], "Jane")
	}
	if _, ok := msg.Data["data"]; ok {
		t.Error("'data' field should be excluded from data map")
	}
}

// TestParseActionFromHTTP_Multipart_MultiValueFields tests repeated form fields
// (e.g., <select multiple> or repeated checkboxes) in multipart submissions.
func TestParseActionFromHTTP_Multipart_MultiValueFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	multipartField(t, writer, "lvt-action", "save")
	multipartField(t, writer, "tags", "go")
	multipartField(t, writer, "tags", "web")
	multipartField(t, writer, "tags", "test")
	multipartField(t, writer, "name", "Jane")
	closeMultipart(t, writer)

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Data["name"] != "Jane" {
		t.Errorf("Data[name] = %q, want %q", msg.Data["name"], "Jane")
	}

	tags, ok := msg.Data["tags"].([]interface{})
	if !ok {
		t.Fatalf("Expected tags to be []interface{}, got %T", msg.Data["tags"])
	}
	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}
	expected := []string{"go", "web", "test"}
	for i, want := range expected {
		if got, ok := tags[i].(string); !ok || got != want {
			t.Errorf("tags[%d] = %v, want %q", i, tags[i], want)
		}
	}
}

// TestParseActionFromHTTP_URLEncoded_MultipleValues tests multiple values for same field.
func TestParseActionFromHTTP_URLEncoded_MultipleValues(t *testing.T) {
	body := "lvt-action=update&tags=go&tags=web&tags=test"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	msg, err := ParseActionFromHTTP(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if msg.Action != "update" {
		t.Errorf("Action = %q, want %q", msg.Action, "update")
	}

	tags, ok := msg.Data["tags"].([]interface{})
	if !ok {
		t.Fatalf("Expected tags to be []interface{}, got %T", msg.Data["tags"])
	}

	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}

	expected := []string{"go", "web", "test"}
	for i, want := range expected {
		if got, ok := tags[i].(string); !ok || got != want {
			t.Errorf("tags[%d] = %v, want %q", i, tags[i], want)
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

// TestQueryParamsToData tests conversion of URL query parameters to data map.
func TestQueryParamsToData(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected map[string]interface{}
	}{
		{
			name:     "empty query string",
			url:      "http://example.com/",
			expected: map[string]interface{}{},
		},
		{
			name: "single param",
			url:  "http://example.com/?error=invalid",
			expected: map[string]interface{}{
				"error": "invalid",
			},
		},
		{
			name: "empty value",
			url:  "http://example.com/?error=",
			expected: map[string]interface{}{
				"error": "",
			},
		},
		{
			name: "multiple params",
			url:  "http://example.com/?error=invalid&success=created",
			expected: map[string]interface{}{
				"error":   "invalid",
				"success": "created",
			},
		},
		{
			name: "repeated param becomes slice",
			url:  "http://example.com/?tags=a&tags=b&tags=c",
			expected: map[string]interface{}{
				"tags": []interface{}{"a", "b", "c"},
			},
		},
		{
			name: "mixed single and repeated",
			url:  "http://example.com/?error=bad&items=1&items=2",
			expected: map[string]interface{}{
				"error": "bad",
				"items": []interface{}{"1", "2"},
			},
		},
		{
			name: "url encoded values",
			url:  "http://example.com/?email=test%40example.com&name=John+Doe",
			expected: map[string]interface{}{
				"email": "test@example.com",
				"name":  "John Doe",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			got := QueryParamsToData(req)

			if len(got) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			for key, want := range tt.expected {
				gotVal := got[key]

				// Handle slice comparison
				if wantSlice, ok := want.([]interface{}); ok {
					gotSlice, ok := gotVal.([]interface{})
					if !ok {
						t.Errorf("key %q: expected slice, got %T", key, gotVal)
						continue
					}
					if len(gotSlice) != len(wantSlice) {
						t.Errorf("key %q: slice length mismatch: got %d, want %d", key, len(gotSlice), len(wantSlice))
						continue
					}
					for i, v := range wantSlice {
						if gotSlice[i] != v {
							t.Errorf("key %q[%d]: got %v, want %v", key, i, gotSlice[i], v)
						}
					}
				} else if gotVal != want {
					t.Errorf("key %q: got %v, want %v", key, gotVal, want)
				}
			}
		})
	}
}

// TestParseActionFromHTTP_URLEncoded_ExplicitSubmitter covers Phase 1 of the
// explicit-submitter proposal (docs/proposals/explicit-submitter.md): the
// "lvt-submitter" form field is the explicit, client-emitted SubmitEvent.
// submitter.name. It populates msg.Action when no lvt-action is provided,
// is preserved on msg.Submitter for diagnostics, is stripped from msg.Data,
// and takes precedence over the empty-value heuristic in collision cases
// the heuristic gets wrong today.
func TestParseActionFromHTTP_URLEncoded_ExplicitSubmitter(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantAction    string
		wantSubmitter string
		wantData      map[string]interface{}
	}{
		{
			name:          "explicit submitter populates action",
			body:          "lvt-submitter=save&title=hello",
			wantAction:    "save",
			wantSubmitter: "save",
			wantData: map[string]interface{}{
				"title": "hello",
			},
		},
		{
			name:          "lvt-action takes precedence over lvt-submitter",
			body:          "lvt-action=foo&lvt-submitter=bar&title=hello",
			wantAction:    "foo",
			wantSubmitter: "bar",
			wantData: map[string]interface{}{
				"title": "hello",
			},
		},
		{
			// Heuristic alone would misroute to "search" (sole empty-value field).
			// Explicit submitter wins and routes to "save". The empty "search"
			// input is preserved as user data.
			name:          "explicit submitter beats empty user input collision",
			body:          "search=&lvt-submitter=save&title=hello",
			wantAction:    "save",
			wantSubmitter: "save",
			wantData: map[string]interface{}{
				"search": "",
				"title":  "hello",
			},
		},
		{
			// Heuristic returns "" (ambiguous) for two empty-value fields.
			// Explicit submitter resolves the ambiguity.
			name:          "explicit submitter resolves ambiguous heuristic",
			body:          "delete=&archive=&lvt-submitter=delete&title=hello",
			wantAction:    "delete",
			wantSubmitter: "delete",
			wantData: map[string]interface{}{
				"delete":  "",
				"archive": "",
				"title":   "hello",
			},
		},
		{
			// Documented heuristic workaround: <button name="action"> is excluded
			// from the empty-value scan to avoid <form action> ambiguity. An
			// explicit submitter routes correctly even when the clicked button
			// is named "action".
			name:          "explicit submitter resolves <button name=action> collision",
			body:          "action=&lvt-submitter=action&title=hello",
			wantAction:    "action",
			wantSubmitter: "action",
			wantData: map[string]interface{}{
				"action": "",
				"title":  "hello",
			},
		},
		{
			// lvt-submitter must not be echoed back into msg.Data.
			name:          "lvt-submitter is consumed, not surfaced as user data",
			body:          "lvt-submitter=save&name=Jane",
			wantAction:    "save",
			wantSubmitter: "save",
			wantData: map[string]interface{}{
				"name": "Jane",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			msg, err := ParseActionFromHTTP(req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if msg.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", msg.Action, tt.wantAction)
			}
			if msg.Submitter != tt.wantSubmitter {
				t.Errorf("Submitter = %q, want %q", msg.Submitter, tt.wantSubmitter)
			}
			if _, ok := msg.Data["lvt-submitter"]; ok {
				t.Errorf("lvt-submitter must not appear in msg.Data, got %v", msg.Data["lvt-submitter"])
			}

			for key, want := range tt.wantData {
				got := msg.Data[key]
				if got != want {
					t.Errorf("Data[%q] = %v, want %v", key, got, want)
				}
			}
			for key := range msg.Data {
				if _, ok := tt.wantData[key]; !ok {
					t.Errorf("Unexpected data field: %q = %v", key, msg.Data[key])
				}
			}
		})
	}
}

// TestParseActionFromHTTP_Multipart_ExplicitSubmitter mirrors the URL-encoded
// explicit-submitter coverage for multipart form bodies. It exercises both the
// individual-fields branch and the JSON-data-envelope branch (the proposal
// requires lvt-submitter to take precedence even when a JSON "data" envelope
// is present).
func TestParseActionFromHTTP_Multipart_ExplicitSubmitter(t *testing.T) {
	t.Run("submitter populates action with individual fields", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		multipartField(t, writer, "lvt-submitter", "save")
		multipartField(t, writer, "title", "hello")
		closeMultipart(t, writer)

		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		msg, err := ParseActionFromHTTP(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if msg.Action != "save" {
			t.Errorf("Action = %q, want %q", msg.Action, "save")
		}
		if msg.Submitter != "save" {
			t.Errorf("Submitter = %q, want %q", msg.Submitter, "save")
		}
		if _, ok := msg.Data["lvt-submitter"]; ok {
			t.Error("lvt-submitter must not appear in msg.Data")
		}
		if msg.Data["title"] != "hello" {
			t.Errorf("Data[title] = %v, want %q", msg.Data["title"], "hello")
		}
	})

	t.Run("lvt-action takes precedence over lvt-submitter", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		multipartField(t, writer, "lvt-action", "foo")
		multipartField(t, writer, "lvt-submitter", "bar")
		multipartField(t, writer, "title", "hello")
		closeMultipart(t, writer)

		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		msg, err := ParseActionFromHTTP(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if msg.Action != "foo" {
			t.Errorf("Action = %q, want %q", msg.Action, "foo")
		}
		// Submitter is preserved as diagnostic context even when lvt-action wins.
		if msg.Submitter != "bar" {
			t.Errorf("Submitter = %q, want %q (preserved as diagnostic)", msg.Submitter, "bar")
		}
	})

	t.Run("submitter beats empty user input collision", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		multipartField(t, writer, "search", "") // legitimate empty user input
		multipartField(t, writer, "lvt-submitter", "save")
		multipartField(t, writer, "title", "hello")
		closeMultipart(t, writer)

		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		msg, err := ParseActionFromHTTP(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if msg.Action != "save" {
			t.Errorf("Action = %q, want %q (heuristic would have misrouted to \"search\")", msg.Action, "save")
		}
		if msg.Data["search"] != "" {
			t.Errorf("Data[search] = %v, want empty string preserved", msg.Data["search"])
		}
	})

	t.Run("submitter resolves ambiguous heuristic", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		multipartField(t, writer, "delete", "")
		multipartField(t, writer, "archive", "")
		multipartField(t, writer, "lvt-submitter", "delete")
		multipartField(t, writer, "title", "hello")
		closeMultipart(t, writer)

		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		msg, err := ParseActionFromHTTP(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if msg.Action != "delete" {
			t.Errorf("Action = %q, want %q (heuristic ambiguous)", msg.Action, "delete")
		}
	})

	t.Run("submitter takes precedence with JSON data envelope", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		multipartField(t, writer, "lvt-submitter", "save")
		multipartField(t, writer, "data", `{"title":"hello"}`)
		closeMultipart(t, writer)

		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		msg, err := ParseActionFromHTTP(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if msg.Action != "save" {
			t.Errorf("Action = %q, want %q (lvt-submitter must win even with JSON data envelope)", msg.Action, "save")
		}
		if msg.Data["title"] != "hello" {
			t.Errorf("Data[title] = %v, want %q", msg.Data["title"], "hello")
		}
		if _, ok := msg.Data["lvt-submitter"]; ok {
			t.Error("lvt-submitter must not appear in msg.Data")
		}
	})
}

// TestParseActionFromWebSocket_Submitter exercises the WS path's submitter
// fallback: when action="" but submitter is set, the server promotes
// submitter to action; when action is set, submitter is preserved as
// diagnostic context but never overrides.
func TestParseActionFromWebSocket_Submitter(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		wantAction    string
		wantSubmitter string
	}{
		{
			name:          "empty action falls back to submitter",
			data:          `{"action":"","submitter":"X","data":{}}`,
			wantAction:    "X",
			wantSubmitter: "X",
		},
		{
			name:          "missing action falls back to submitter",
			data:          `{"submitter":"X","data":{}}`,
			wantAction:    "X",
			wantSubmitter: "X",
		},
		{
			name:          "explicit action wins over submitter",
			data:          `{"action":"Y","submitter":"X","data":{}}`,
			wantAction:    "Y",
			wantSubmitter: "X",
		},
		{
			name:          "no submitter leaves action empty",
			data:          `{"action":"","data":{}}`,
			wantAction:    "",
			wantSubmitter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseActionFromWebSocket([]byte(tt.data))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if msg.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", msg.Action, tt.wantAction)
			}
			if msg.Submitter != tt.wantSubmitter {
				t.Errorf("Submitter = %q, want %q", msg.Submitter, tt.wantSubmitter)
			}
		})
	}
}

// TestParseActionFromHTTP_JSON_Submitter exercises the JSON HTTP content-type
// path: the JSON decoder populates ActionMessage.Submitter, and
// resolveSubmitterFallback fills Action when it would otherwise be empty.
func TestParseActionFromHTTP_JSON_Submitter(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantAction    string
		wantSubmitter string
	}{
		{
			name:          "empty action falls back to submitter (JSON HTTP)",
			body:          `{"action":"","submitter":"X","data":{}}`,
			wantAction:    "X",
			wantSubmitter: "X",
		},
		{
			name:          "explicit action wins over submitter (JSON HTTP)",
			body:          `{"action":"Y","submitter":"X","data":{}}`,
			wantAction:    "Y",
			wantSubmitter: "X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			msg, err := ParseActionFromHTTP(req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if msg.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", msg.Action, tt.wantAction)
			}
			if msg.Submitter != tt.wantSubmitter {
				t.Errorf("Submitter = %q, want %q", msg.Submitter, tt.wantSubmitter)
			}
		})
	}
}

// TestMergeData tests merging of data maps with precedence.
func TestMergeData(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]interface{}
		override map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "both nil",
			base:     nil,
			override: nil,
			expected: nil,
		},
		{
			name:     "base nil",
			base:     nil,
			override: map[string]interface{}{"a": "1"},
			expected: map[string]interface{}{"a": "1"},
		},
		{
			name:     "override nil",
			base:     map[string]interface{}{"a": "1"},
			override: nil,
			expected: map[string]interface{}{"a": "1"},
		},
		{
			name:     "both empty",
			base:     map[string]interface{}{},
			override: map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name:     "override wins on conflict",
			base:     map[string]interface{}{"a": "base", "b": "base"},
			override: map[string]interface{}{"a": "override"},
			expected: map[string]interface{}{"a": "override", "b": "base"},
		},
		{
			name:     "no conflict - both preserved",
			base:     map[string]interface{}{"a": "1"},
			override: map[string]interface{}{"b": "2"},
			expected: map[string]interface{}{"a": "1", "b": "2"},
		},
		{
			name:     "form data over query params",
			base:     map[string]interface{}{"error": "query_error", "page": "1"},
			override: map[string]interface{}{"error": "form_error", "username": "test"},
			expected: map[string]interface{}{"error": "form_error", "page": "1", "username": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeData(tt.base, tt.override)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}

			if len(got) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			for key, want := range tt.expected {
				if got[key] != want {
					t.Errorf("key %q: got %v, want %v", key, got[key], want)
				}
			}
		})
	}
}
