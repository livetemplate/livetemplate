package livetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
)

// message represents an action message from the client (internal protocol)
type message struct {
	Action string                 `json:"action"` // Action name, may include store prefix (e.g., "counter.increment")
	Data   map[string]interface{} `json:"data"`   // All values from forms, inputs, data attributes, etc.
}

// ActionData wraps action data with utilities for binding and validation
type ActionData struct {
	raw   map[string]interface{}
	bytes []byte // Cached JSON for efficient binding
}

// newActionData creates ActionData from a map (internal use only)
func newActionData(data map[string]interface{}) *ActionData {
	return &ActionData{raw: data}
}

// Bind unmarshals the data into a struct
func (a *ActionData) Bind(v interface{}) error {
	// Lazy marshal to JSON
	if a.bytes == nil {
		var err error
		a.bytes, err = json.Marshal(a.raw)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	}

	return json.Unmarshal(a.bytes, v)
}

// BindAndValidate binds data to struct and validates it in one step
func (a *ActionData) BindAndValidate(v interface{}, validate *validator.Validate) error {
	if err := a.Bind(v); err != nil {
		return err
	}

	if err := validate.Struct(v); err != nil {
		return ValidationToMultiError(err)
	}

	return nil
}

// Raw returns the underlying map for direct access
func (a *ActionData) Raw() map[string]interface{} {
	return a.raw
}

// GetString extracts a string value.
// Returns empty string if key doesn't exist or value is not a string.
// Use GetStringOk for explicit error handling.
func (a *ActionData) GetString(key string) string {
	v, _ := a.GetStringOk(key)
	return v
}

// GetStringOk extracts a string value with explicit success indicator.
// Returns (value, true) if key exists and value is a string.
// Returns ("", false) if key doesn't exist or value is not a string.
func (a *ActionData) GetStringOk(key string) (string, bool) {
	if v, ok := a.raw[key].(string); ok {
		return v, true
	}
	return "", false
}

// GetInt extracts an int value (JSON numbers are float64).
// Returns 0 if key doesn't exist or value is not a number.
// Use GetIntOk for explicit error handling.
func (a *ActionData) GetInt(key string) int {
	v, _ := a.GetIntOk(key)
	return v
}

// GetIntOk extracts an int value with explicit success indicator.
// Returns (value, true) if key exists and value is a number.
// Returns (0, false) if key doesn't exist or value is not a number.
func (a *ActionData) GetIntOk(key string) (int, bool) {
	if v, ok := a.raw[key].(float64); ok {
		return int(v), true
	}
	return 0, false
}

// GetFloat extracts a float64 value.
// Returns 0 if key doesn't exist or value is not a number.
// Use GetFloatOk for explicit error handling.
func (a *ActionData) GetFloat(key string) float64 {
	v, _ := a.GetFloatOk(key)
	return v
}

// GetFloatOk extracts a float64 value with explicit success indicator.
// Returns (value, true) if key exists and value is a number.
// Returns (0, false) if key doesn't exist or value is not a number.
func (a *ActionData) GetFloatOk(key string) (float64, bool) {
	if v, ok := a.raw[key].(float64); ok {
		return v, true
	}
	return 0, false
}

// GetBool extracts a bool value.
// Returns false if key doesn't exist or value is not a bool.
// Use GetBoolOk for explicit error handling.
func (a *ActionData) GetBool(key string) bool {
	v, _ := a.GetBoolOk(key)
	return v
}

// GetBoolOk extracts a bool value with explicit success indicator.
// Returns (value, true) if key exists and value is a bool.
// Returns (false, false) if key doesn't exist or value is not a bool.
func (a *ActionData) GetBoolOk(key string) (bool, bool) {
	if v, ok := a.raw[key].(bool); ok {
		return v, true
	}
	return false, false
}

// Has checks if a key exists
func (a *ActionData) Has(key string) bool {
	_, exists := a.raw[key]
	return exists
}

// Get returns the raw value for a key
func (a *ActionData) Get(key string) interface{} {
	return a.raw[key]
}

// ActionContext provides context for a Change action.
//
// The Ctx field contains the request context.Context which can be used for:
// - Timeouts and cancellation
// - Trace ID propagation
// - Request-scoped values (e.g., user_id, group_id)
type ActionContext struct {
	Action string
	Data   *ActionData
	Ctx    context.Context // Request context for timeout/cancellation/values
}

// Bind is a convenience method that delegates to Data.Bind
func (c *ActionContext) Bind(v interface{}) error {
	return c.Data.Bind(v)
}

// BindAndValidate is a convenience method
func (c *ActionContext) BindAndValidate(v interface{}, validate *validator.Validate) error {
	return c.Data.BindAndValidate(v, validate)
}

// GetString is a convenience method
func (c *ActionContext) GetString(key string) string {
	return c.Data.GetString(key)
}

// GetInt is a convenience method
func (c *ActionContext) GetInt(key string) int {
	return c.Data.GetInt(key)
}

// GetFloat is a convenience method
func (c *ActionContext) GetFloat(key string) float64 {
	return c.Data.GetFloat(key)
}

// GetBool is a convenience method
func (c *ActionContext) GetBool(key string) bool {
	return c.Data.GetBool(key)
}

// Has is a convenience method
func (c *ActionContext) Has(key string) bool {
	return c.Data.Has(key)
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewFieldError creates a field-specific error
func NewFieldError(field string, err error) FieldError {
	return FieldError{Field: field, Message: err.Error()}
}

// MultiError is a collection of field errors (implements error interface)
type MultiError []FieldError

func (m MultiError) Error() string {
	if len(m) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// ValidationToMultiError converts go-playground/validator errors to MultiError
func ValidationToMultiError(err error) MultiError {
	var fieldErrors MultiError

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return fieldErrors
	}

	for _, e := range validationErrs {
		// Use original field name to match form field names (camelCase/PascalCase)
		fieldName := e.Field()

		var message string
		switch e.Tag() {
		case "required":
			message = fmt.Sprintf("%s is required", fieldName)
		case "min":
			message = fmt.Sprintf("%s must be at least %s characters", fieldName, e.Param())
		case "max":
			message = fmt.Sprintf("%s must be at most %s characters", fieldName, e.Param())
		case "email":
			message = fmt.Sprintf("%s must be a valid email", fieldName)
		default:
			message = fmt.Sprintf("%s is invalid", fieldName)
		}

		fieldErrors = append(fieldErrors, FieldError{
			Field:   fieldName,
			Message: message,
		})
	}

	return fieldErrors
}

// Store is any type that can handle state changes
type Store interface {
	Change(ctx *ActionContext) error
}

// StoreInitializer is an optional interface that stores can implement
// to perform initialization after being cloned for a new session.
// This is useful for loading data from external sources like databases.
type StoreInitializer interface {
	Init() error
}

// Stores is a map of named stores
type Stores map[string]Store

// parseAction splits "counter.increment" into ("counter", "increment")
// For single store actions like "increment", returns ("", "increment")
func parseAction(action string) (store string, actualAction string) {
	parts := strings.SplitN(action, ".", 2)

	if len(parts) == 2 {
		return parts[0], parts[1] // "counter", "increment"
	}

	return "", parts[0] // "", "increment" (single store)
}

// parseActionFromHTTP parses an action message from HTTP POST request body (internal protocol)
func parseActionFromHTTP(r *http.Request) (message, error) {
	var msg message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		return message{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// parseActionFromWebSocket parses an action message from WebSocket message bytes (internal protocol)
func parseActionFromWebSocket(data []byte) (message, error) {
	var msg message
	if err := json.Unmarshal(data, &msg); err != nil {
		return message{}, fmt.Errorf("failed to parse action: %w", err)
	}

	// Ensure data map is initialized
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	return msg, nil
}

// writeUpdateWebSocket writes a tree update to WebSocket connection (internal protocol)
func writeUpdateWebSocket(conn *websocket.Conn, update []byte) error {
	return conn.WriteMessage(websocket.TextMessage, update)
}

// Removed: Generic helper functions (getString, getInt, etc.)
// Users should use ActionData/ActionContext methods instead
