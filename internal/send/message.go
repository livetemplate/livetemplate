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
func ParseActionFromHTTP(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	// Check if this is multipart form data
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// For multipart requests, try to get action from form field
		// This allows file uploads to include an action
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			// If parse fails, default to empty action (upload-only request)
			msg.Action = ""
			msg.Data = make(map[string]interface{})
			return msg, nil
		}

		// Try to get action from form value
		if actionStr := r.FormValue("action"); actionStr != "" {
			msg.Action = actionStr
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

	// For regular JSON requests
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
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
func WriteUpdateToWebSocket(conn *websocket.Conn, update []byte) error {
	return conn.WriteMessage(websocket.TextMessage, update)
}
