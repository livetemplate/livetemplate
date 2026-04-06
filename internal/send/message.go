// Package send provides message formatting and serialization for LiveTemplate.
package send

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/livetemplate/livetemplate/internal/jsonutil"
)

const wsTextMessage = 1 // RFC 6455 text message type (mirrors WSTextMessage in root package)

// ActionMessage represents an action message from the client (internal protocol).
type ActionMessage struct {
	Action string                 `json:"action"` // Action name, may include store prefix (e.g., "counter.increment")
	Data   map[string]interface{} `json:"data"`   // All values from forms, inputs, data attributes, etc.
}

// ParseActionFromHTTP parses an action message from HTTP POST request body (internal protocol).
// Supports three content types:
//   - application/json: {"action": "...", "data": {...}}
//   - application/x-www-form-urlencoded: lvt-action=login&username=...&password=...
//   - multipart/form-data: File uploads with optional lvt-action and data fields
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
	if err := jsonutil.API.NewDecoder(r.Body).Decode(&msg); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// parseMultipartForm parses action from multipart/form-data (file uploads).
// Supports two data formats:
//  1. JSON-encoded "data" form field (client library sends this)
//  2. Individual form fields (browser-native multipart submission)
//
// When a JSON "data" blob is present, it takes precedence. Otherwise,
// individual form fields are read, matching parseURLEncodedForm behavior.
func parseMultipartForm(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// If parse fails, default to empty action (upload-only request)
		msg.Action = ""
		msg.Data = make(map[string]interface{})
		return msg, nil
	}

	// Get action from lvt-action field (explicit routing for progressive enhancement)
	msg.Action = r.FormValue("lvt-action")

	// Try to get data from JSON-encoded form field (client library format).
	// When present, JSON data takes precedence over individual form fields.
	jsonDataParsed := false
	if dataStr := r.FormValue("data"); dataStr != "" {
		var data map[string]interface{}
		if err := jsonutil.API.Unmarshal([]byte(dataStr), &data); err == nil {
			msg.Data = data
			jsonDataParsed = true
		}
	}

	// Fallback: read individual form fields when no JSON "data" blob was parsed.
	// This handles browser-native multipart submissions where text fields are
	// sent as separate form fields alongside file uploads.
	if !jsonDataParsed {
		actionFields := map[string]bool{"lvt-action": true, "data": true}

		// Button-name-as-action detection (same logic as parseURLEncodedForm)
		if msg.Action == "" && r.MultipartForm != nil {
			var candidate string
			ambiguous := false
			for key, values := range r.MultipartForm.Value {
				// Skip "action" — it's a common HTML attribute (<form action="/path">)
				// that browsers don't submit, but could cause false-positive routing.
				if key == "" || key == "action" || actionFields[key] {
					continue
				}
				if len(values) == 1 && values[0] == "" {
					if candidate != "" {
						ambiguous = true
						break
					}
					candidate = key
				}
			}
			if candidate != "" && !ambiguous {
				msg.Action = candidate
				actionFields[candidate] = true
			}
		}

		// Read individual form fields into data map
		msg.Data = make(map[string]interface{})
		if r.MultipartForm != nil {
			for key, values := range r.MultipartForm.Value {
				if actionFields[key] {
					continue
				}
				if len(values) == 1 {
					msg.Data[key] = values[0]
				} else if len(values) > 1 {
					interfaceSlice := make([]interface{}, len(values))
					for i, v := range values {
						interfaceSlice[i] = v
					}
					msg.Data[key] = interfaceSlice
				}
			}
		}
	}

	return msg, nil
}

// parseURLEncodedForm parses action from application/x-www-form-urlencoded (standard HTML forms).
//
// Action resolution order (first match wins):
//  1. "lvt-action" form field (legacy explicit routing for progressive enhancement)
//  2. Button name routing: a form field with an empty string value is treated as a
//     submit button whose name is the action (standard HTML: <button name="increment">)
//  3. Empty string (server defaults to "submit" via applyDefaultAction)
//
// Note: "action" is NOT reserved — it flows through as normal form data.
// For explicit routing, use lvt-form:action attribute (client-side) or
// lvt-action hidden field (server-side progressive enhancement).
func parseURLEncodedForm(r *http.Request) (ActionMessage, error) {
	var msg ActionMessage

	if err := r.ParseForm(); err != nil {
		return ActionMessage{}, fmt.Errorf("failed to parse form: %w", err)
	}

	// Get action from lvt-action field (explicit routing for progressive enhancement)
	msg.Action = r.FormValue("lvt-action")

	// Action routing fields to exclude from data
	actionFields := map[string]bool{"lvt-action": true}

	// If no explicit action, detect button-name-as-action:
	// A submit button with name="X" and no value submits "X=" in form data.
	// We detect this as the unique single-value field with an empty string.
	// If multiple empty-value fields exist, we skip (ambiguous — can't determine which button).
	// Note: "action" is explicitly excluded from this scan. While it's a normal
	// data field, an empty action= would be ambiguous with the common HTML pattern
	// <form action="/path"> which browsers don't submit, but could confuse routing.
	if msg.Action == "" {
		var candidate string
		ambiguous := false
		for key, values := range r.Form {
			if key == "" || key == "action" || actionFields[key] {
				continue
			}
			if len(values) == 1 && values[0] == "" {
				if candidate != "" {
					ambiguous = true
					break
				}
				candidate = key
			}
		}
		if candidate != "" && !ambiguous {
			msg.Action = candidate
			actionFields[candidate] = true
		}
	}

	// Convert all form fields to data map (except action routing fields)
	msg.Data = make(map[string]interface{})
	for key, values := range r.Form {
		if actionFields[key] {
			continue
		}
		if len(values) == 1 {
			msg.Data[key] = values[0]
		} else if len(values) > 1 {
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
	if err := jsonutil.API.Unmarshal(data, &msg); err != nil {
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
	return conn.Send(wsTextMessage, update)
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
