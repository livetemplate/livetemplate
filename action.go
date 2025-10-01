package livetemplate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Message represents an action message from the client
type Message struct {
	Action string                 `json:"action"` // Action name, may include store prefix (e.g., "counter.increment")
	Data   map[string]interface{} `json:"data"`   // All values from forms, inputs, data attributes, etc.
}

// Store is any type that can handle state changes
type Store interface {
	Change(action string, data map[string]interface{})
}

// Stores is a map of named stores
type Stores map[string]Store

// ParseAction splits "counter.increment" into ("counter", "increment")
// For single store actions like "increment", returns ("", "increment")
func ParseAction(action string) (store string, actualAction string) {
	parts := strings.SplitN(action, ".", 2)

	if len(parts) == 2 {
		return parts[0], parts[1] // "counter", "increment"
	}

	return "", parts[0] // "", "increment" (single store)
}

// ParseActionFromHTTP parses an action message from HTTP POST request body
func ParseActionFromHTTP(r *http.Request) (Message, error) {
	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		return Message{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// ParseActionFromWebSocket parses an action message from WebSocket message bytes
func ParseActionFromWebSocket(data []byte) (Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return Message{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// WriteUpdateHTTP writes a tree update as JSON response
func WriteUpdateHTTP(w http.ResponseWriter, update []byte) error {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(update)
	return err
}

// WriteUpdateWebSocket writes a tree update to WebSocket connection
func WriteUpdateWebSocket(conn *websocket.Conn, update []byte) error {
	return conn.WriteMessage(websocket.TextMessage, update)
}

// Helper functions for extracting typed values from data map

// GetString extracts a string value from the data map
func GetString(data map[string]interface{}, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// GetInt extracts an int value from the data map
// JSON numbers are decoded as float64, so we convert
func GetInt(data map[string]interface{}, key string) int {
	if v, ok := data[key].(float64); ok {
		return int(v)
	}
	return 0
}

// GetFloat extracts a float64 value from the data map
func GetFloat(data map[string]interface{}, key string) float64 {
	if v, ok := data[key].(float64); ok {
		return v
	}
	return 0
}

// GetBool extracts a bool value from the data map
func GetBool(data map[string]interface{}, key string) bool {
	if v, ok := data[key].(bool); ok {
		return v
	}
	return false
}

// WriteJSON writes a JSON response
func WriteJSON(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}
