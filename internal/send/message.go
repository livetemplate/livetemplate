// Package send provides message formatting and serialization for LiveTemplate.
package send

import (
	"encoding/json"
	"fmt"
	"net/http"

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
