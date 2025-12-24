// Package send provides message formatting and serialization for LiveTemplate.
package send

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// ActionMessage represents an action message from the client (internal protocol).
type ActionMessage struct {
	Action string                 `json:"action"` // Action name, may include store prefix (e.g., "counter.increment")
	Data   map[string]interface{} `json:"data"`   // All values from forms, inputs, data attributes, etc.
}

// ParseActionFromHTTP parses an action message from HTTP POST request body (internal protocol).
// Supports three content types:
//   - application/json: {"action": "...", "data": {...}}
//   - application/x-www-form-urlencoded: lvt-action=login&username=...&password=...
//   - multipart/form-data: File uploads with optional action and data fields
func ParseActionFromHTTP(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	contentType := r.Header.Get("Content-Type")

	// Handle multipart form data (file uploads)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartForm(r)
	}

	// Handle URL-encoded form data (standard HTML forms)
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		return parseURLEncodedForm(r)
	}

	// Default: JSON request body
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// parseMultipartForm parses action from multipart/form-data (file uploads).
func parseMultipartForm(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// If parse fails, default to empty action (upload-only request)
		msg.Action = ""
		msg.Data = make(map[string]interface{})
		return msg, nil
	}

	// Try to get action from form value (supports both "action" and "lvt-action")
	msg.Action = r.FormValue("lvt-action")
	if msg.Action == "" {
		msg.Action = r.FormValue("action")
	}

	// Try to get data from JSON-encoded form field
	if dataStr := r.FormValue("data"); dataStr != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
			msg.Data = data
		}
	}

	// Initialize data map if not set
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// parseURLEncodedForm parses action from application/x-www-form-urlencoded (standard HTML forms).
// Action is specified via "lvt-action" field, all other fields become data.
func parseURLEncodedForm(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	if err := r.ParseForm(); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse form: %w", err)
	}

	// Get action from lvt-action field (or fallback to action)
	msg.Action = r.FormValue("lvt-action")
	if msg.Action == "" {
		msg.Action = r.FormValue("action")
	}

	// Convert all form fields to data map (except lvt-action and action)
	msg.Data = make(map[string]interface{})
	for key, values := range r.Form {
		if key == "lvt-action" || key == "action" {
			continue // Skip action fields
		}
		// Use first value (forms typically have single values)
		if len(values) == 1 {
			msg.Data[key] = values[0]
		} else if len(values) > 1 {
			// Convert to interface slice for multiple values
			interfaceSlice := make([]interface{}, len(values))
			for i, v := range values {
				interfaceSlice[i] = v
			}
			msg.Data[key] = interfaceSlice
		}
	}

	return msg, nil
}

// ParseActionFromWebSocket parses an action message from WebSocket message bytes (internal protocol).
func ParseActionFromWebSocket(data []byte) (ActionMessage, error) {
	var msg ActionMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// WriteUpdateToWebSocket writes a tree update to WebSocket connection (internal protocol).
// Uses async Send() method to avoid blocking on slow clients.
func WriteUpdateToWebSocket(conn ConnectionSender, update []byte) error {
	return conn.Send(websocket.TextMessage, update)
}

// ConnectionSender is an interface for sending WebSocket messages asynchronously.
// Implemented by *session.Connection.
type ConnectionSender interface {
	Send(messageType int, data []byte) error
}

// QueryParamsToData converts URL query parameters to action data map.
// Single values are stored as strings, multiple values as []interface{}.
func QueryParamsToData(r *http.Request) map[string]interface{} {
	data := make(map[string]interface{})
	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			data[key] = values[0]
		} else if len(values) > 1 {
			interfaceSlice := make([]interface{}, len(values))
			for i, v := range values {
				interfaceSlice[i] = v
			}
			data[key] = interfaceSlice
		}
	}
	return data
}

// MergeData merges base data with override data.
// Override values take precedence over base values.
func MergeData(base, override map[string]interface{}) map[string]interface{} {
	if len(base) == 0 {
		return override
	}
	if len(override) == 0 {
		return base
	}

	merged := make(map[string]interface{}, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
