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
// Wire-format note: HTTP form paths use the form key "lvt-submitter"; JSON
// (HTTP and WS) paths use the top-level key "submitter". Different conventions
// for different envelopes — form fields take the lvt- prefix, JSON keys do not.
type ActionMessage struct {
	Action    string                 `json:"action"`              // Action name, may include store prefix (e.g., "counter.increment")
	Submitter string                 `json:"submitter,omitempty"` // Optional explicit submitter name (e.g. SubmitEvent.submitter.name); used as Action when Action is empty.
	Data      map[string]interface{} `json:"data"`                // All values from forms, inputs, data attributes, etc.
}

// resolveSubmitterFallback fills msg.Action from msg.Submitter when Action
// is empty, so an explicit `submitter` field on the wire is treated as the
// action of last resort. Idempotent; safe to call from any parser.
func resolveSubmitterFallback(msg *ActionMessage) {
	if msg.Action == "" && msg.Submitter != "" {
		msg.Action = msg.Submitter
	}
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

	resolveSubmitterFallback(&msg)

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// detectSubmitButtonName scans form values for a single field with one empty
// string value and returns its name. This is how a standard HTML submit button
// with name="X" surfaces (the browser sends "X=" with no value). Returns "" if
// zero or multiple such candidates exist (ambiguous).
//
// "action" is excluded because it's a common HTML form attribute (<form
// action="/path">) that browsers don't submit; an empty action= field on the
// wire would be ambiguous between "name='action' button" and accidental data.
// Names already in actionFields (e.g. "lvt-action", "data") are also skipped.
func detectSubmitButtonName(values map[string][]string, actionFields map[string]bool) string {
	var candidate string
	for key, vs := range values {
		if key == "" || key == "action" || actionFields[key] {
			continue
		}
		if len(vs) == 1 && vs[0] == "" {
			if candidate != "" {
				return ""
			}
			candidate = key
		}
	}
	return candidate
}

// BuildActionFromValues reconstructs an ActionMessage from already-collected
// multipart value parts (form key -> values). It mirrors parseMultipartForm's
// value handling and is used by the streaming-upload path, which reads the body
// via MultipartReader (so r.MultipartForm is never populated) and must rebuild
// the action message from the value parts it buffered during iteration.
func BuildActionFromValues(values map[string][]string) ActionMessage {
	var msg ActionMessage

	first := func(key string) string {
		if vs, ok := values[key]; ok && len(vs) > 0 {
			return vs[0]
		}
		return ""
	}

	msg.Action = first("lvt-action")
	msg.Submitter = first("lvt-submitter")
	resolveSubmitterFallback(&msg)

	if dataStr := first("data"); dataStr != "" {
		var data map[string]interface{}
		if err := jsonutil.API.Unmarshal([]byte(dataStr), &data); err == nil {
			msg.Data = data
			return msg
		}
	}

	actionFields := map[string]bool{"lvt-action": true, "lvt-submitter": true, "data": true}
	if msg.Action == "" {
		if name := detectSubmitButtonName(values, actionFields); name != "" {
			msg.Action = name
			actionFields[name] = true
		}
	}

	msg.Data = make(map[string]interface{})
	for key, vs := range values {
		if actionFields[key] {
			continue
		}
		switch len(vs) {
		case 0:
			// skip
		case 1:
			msg.Data[key] = vs[0]
		default:
			interfaceSlice := make([]interface{}, len(vs))
			for i, v := range vs {
				interfaceSlice[i] = v
			}
			msg.Data[key] = interfaceSlice
		}
	}

	return msg
}

// parseMultipartForm parses action from multipart/form-data (file uploads).
// Supports two data formats:
//  1. JSON-encoded "data" form field (client library sends this)
//  2. Individual form fields (browser-native multipart submission)
//
// When a JSON "data" blob is present, it takes precedence. Otherwise,
// individual form fields are read, matching parseURLEncodedForm behavior.
//
// Action resolution order (first match wins):
//  1. "lvt-action" form field (legacy explicit routing for progressive enhancement)
//  2. "lvt-submitter" form field (explicit client-emitted SubmitEvent.submitter.name)
//  3. Button name routing: a form field with an empty string value is treated as a
//     submit button whose name is the action (standard HTML: <button name="increment">)
//  4. Empty string (server defaults to "submit" via applyDefaultAction)
//
// Both lvt-action and lvt-submitter are read BEFORE the JSON "data" branch so
// they take precedence over the heuristic regardless of whether a JSON "data"
// envelope is present.
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
	// Capture explicit submitter (client-emitted SubmitEvent.submitter.name);
	// resolveSubmitterFallback (called immediately after) promotes this to msg.Action only when
	// msg.Action is empty, so lvt-action always wins. Read here (before the
	// jsonDataParsed branch) so submitter routing applies whether or not a
	// JSON "data" envelope is present.
	msg.Submitter = r.FormValue("lvt-submitter")
	resolveSubmitterFallback(&msg)

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
		actionFields := map[string]bool{"lvt-action": true, "lvt-submitter": true, "data": true}

		if msg.Action == "" && r.MultipartForm != nil {
			if name := detectSubmitButtonName(r.MultipartForm.Value, actionFields); name != "" {
				msg.Action = name
				actionFields[name] = true
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
//  2. "lvt-submitter" form field (explicit client-emitted SubmitEvent.submitter.name)
//  3. Button name routing: a form field with an empty string value is treated as a
//     submit button whose name is the action (standard HTML: <button name="increment">)
//  4. Empty string (server defaults to "submit" via applyDefaultAction)
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
	// Capture explicit submitter (client-emitted SubmitEvent.submitter.name);
	// resolveSubmitterFallback (called immediately after) promotes this to msg.Action only when
	// msg.Action is empty, so lvt-action always wins.
	msg.Submitter = r.FormValue("lvt-submitter")
	resolveSubmitterFallback(&msg)

	// Action routing fields to exclude from data
	actionFields := map[string]bool{"lvt-action": true, "lvt-submitter": true}

	if msg.Action == "" {
		if name := detectSubmitButtonName(r.Form, actionFields); name != "" {
			msg.Action = name
			// The detected empty-value field IS the clicked submit button, so it
			// is also the submitter — record it so ctx.Submitter() is correct on
			// the no-JS tier, not just ctx.Action().
			msg.Submitter = name
			actionFields[name] = true
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

	resolveSubmitterFallback(&msg)

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
