package livetemplate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate/internal/send"
)

// HTTP context errors for ActionContext methods
var (
	// ErrNoHTTPContext is returned when HTTP methods (SetCookie, Redirect, etc.)
	// are called from a WebSocket action. These methods require an HTTP response
	// writer which is not available in WebSocket contexts.
	//
	// To set cookies or redirect, use HTTP POST forms instead of WebSocket actions.
	// This is consistent with security best practices - session cookies should be
	// HttpOnly and can only be set via HTTP responses, not JavaScript/WebSocket.
	ErrNoHTTPContext = errors.New("HTTP methods require HTTP context (not available in WebSocket actions)")

	// ErrInvalidRedirectCode is returned when Redirect is called with a non-3xx status code.
	ErrInvalidRedirectCode = errors.New("invalid redirect status code (must be 3xx)")

	// ErrInvalidRedirectURL is returned when Redirect is called with a potentially
	// unsafe URL that could lead to open redirect vulnerabilities.
	ErrInvalidRedirectURL = errors.New("invalid redirect URL (must be relative path starting with /)")
)

// message is an alias for internal/send.ActionMessage for backward compatibility
type message = send.ActionMessage

// ActionData wraps action data with utilities for binding and validation
type ActionData struct {
	raw   map[string]interface{}
	bytes []byte // Cached JSON for efficient binding
}

// newActionData creates ActionData from a map (internal use only)
func newActionData(data map[string]interface{}) *ActionData {
	return &ActionData{raw: data}
}

// NewActionData creates ActionData from a map
// This is the public version for use by external packages like livepage
func NewActionData(data map[string]interface{}) *ActionData {
	return newActionData(data)
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
//
// DEPRECATED: Use GetStringOk for explicit error handling to distinguish
// between missing keys, type errors, and actual empty strings.
// This method will be removed in v0.3.0.
func (a *ActionData) GetString(key string) string {
	v, _ := a.GetStringOk(key)
	return v
}

// GetStringOk extracts a string value with explicit success indicator.
// Returns (value, true) if key exists and value is a string or number.
// Returns ("", false) if key doesn't exist or value cannot be converted to string.
//
// This method handles both string values and JSON numbers (float64), since
// the client-side parseValue() may convert numeric strings like "1" to numbers.
func (a *ActionData) GetStringOk(key string) (string, bool) {
	// Handle string values directly
	if v, ok := a.raw[key].(string); ok {
		return v, true
	}
	// Handle float64 (JSON numbers) - convert to string
	if v, ok := a.raw[key].(float64); ok {
		// Use FormatFloat to avoid scientific notation and preserve precision
		// Check if it's an integer value
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), true
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	}
	return "", false
}

// GetInt extracts an int value (JSON numbers are float64).
// Returns 0 if key doesn't exist or value is not a number.
//
// DEPRECATED: Use GetIntOk for explicit error handling to distinguish
// between missing keys, type errors, and actual zero values.
// This method will be removed in v0.3.0.
func (a *ActionData) GetInt(key string) int {
	v, _ := a.GetIntOk(key)
	return v
}

// GetIntOk extracts an int value with explicit success indicator.
// Returns (value, true) if key exists and value is a number or numeric string.
// Returns (0, false) if key doesn't exist or value cannot be parsed as int.
//
// This method handles both JSON numbers (float64) and string values from
// lvt-data-* attributes, which are always transmitted as strings.
func (a *ActionData) GetIntOk(key string) (int, bool) {
	// Handle float64 (JSON numbers)
	if v, ok := a.raw[key].(float64); ok {
		return int(v), true
	}
	// Handle string values from lvt-data-* attributes
	if v, ok := a.raw[key].(string); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

// GetFloat extracts a float64 value.
// Returns 0 if key doesn't exist or value is not a number.
//
// DEPRECATED: Use GetFloatOk for explicit error handling to distinguish
// between missing keys, type errors, and actual zero values.
// This method will be removed in v0.3.0.
func (a *ActionData) GetFloat(key string) float64 {
	v, _ := a.GetFloatOk(key)
	return v
}

// GetFloatOk extracts a float64 value with explicit success indicator.
// Returns (value, true) if key exists and value is a number or numeric string.
// Returns (0, false) if key doesn't exist or value cannot be parsed as float.
//
// This method handles both JSON numbers (float64) and string values from
// lvt-data-* attributes, which are always transmitted as strings.
func (a *ActionData) GetFloatOk(key string) (float64, bool) {
	// Handle float64 (JSON numbers)
	if v, ok := a.raw[key].(float64); ok {
		return v, true
	}
	// Handle string values from lvt-data-* attributes
	if v, ok := a.raw[key].(string); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// GetBool extracts a bool value.
// Returns false if key doesn't exist or value is not a bool.
//
// DEPRECATED: Use GetBoolOk for explicit error handling to distinguish
// between missing keys, type errors, and actual false values.
// This method will be removed in v0.3.0.
func (a *ActionData) GetBool(key string) bool {
	v, _ := a.GetBoolOk(key)
	return v
}

// GetBoolOk extracts a bool value with explicit success indicator.
// Returns (value, true) if key exists and value is a bool or boolean string.
// Returns (false, false) if key doesn't exist or value cannot be parsed as bool.
//
// This method handles both boolean values and string values "true"/"false"
// from HTML form submissions (HTTP path uses strings, WebSocket uses booleans).
func (a *ActionData) GetBoolOk(key string) (bool, bool) {
	// Handle bool values directly (from WebSocket + parseValue)
	if v, ok := a.raw[key].(bool); ok {
		return v, true
	}
	// Handle string values from HTTP form submissions
	if v, ok := a.raw[key].(string); ok {
		if v == "true" {
			return true, true
		}
		if v == "false" {
			return false, true
		}
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

// isValidRedirectURL checks if a URL is safe for redirects.
// Only allows relative paths starting with "/" to prevent open redirects.
func isValidRedirectURL(url string) bool {
	// Must start with / (relative path)
	if !strings.HasPrefix(url, "/") {
		return false
	}
	// Reject protocol-relative URLs like "//evil.com"
	if strings.HasPrefix(url, "//") {
		return false
	}
	return true
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
		// Convert struct field name (e.g., "Title") to lowercase to match HTML form input names (e.g., "title")
		// HTML input names are typically lowercase, but struct fields are PascalCase
		structFieldName := e.Field()
		formFieldName := strings.ToLower(structFieldName)

		var message string
		switch e.Tag() {
		case "required":
			message = fmt.Sprintf("%s is required", structFieldName)
		case "min":
			message = fmt.Sprintf("%s must be at least %s characters", structFieldName, e.Param())
		case "max":
			message = fmt.Sprintf("%s must be at most %s characters", structFieldName, e.Param())
		case "email":
			message = fmt.Sprintf("%s must be a valid email", structFieldName)
		default:
			message = fmt.Sprintf("%s is invalid", structFieldName)
		}

		fieldErrors = append(fieldErrors, FieldError{
			Field:   formFieldName, // Use lowercase to match HTML input names
			Message: message,
		})
	}

	return fieldErrors
}

// parseActionFromHTTP wraps internal/send.ParseActionFromHTTP for backward compatibility
func parseActionFromHTTP(r *http.Request) (message, error) {
	return send.ParseActionFromHTTP(r)
}

// parseActionFromWebSocket wraps internal/send.ParseActionFromWebSocket for backward compatibility
func parseActionFromWebSocket(data []byte) (message, error) {
	return send.ParseActionFromWebSocket(data)
}

// writeUpdateWebSocket wraps internal/send.WriteUpdateToWebSocket for backward compatibility
func writeUpdateWebSocket(conn send.ConnectionSender, update []byte) error {
	return send.WriteUpdateToWebSocket(conn, update)
}

// Removed: Generic helper functions (getString, getInt, etc.)
// Users should use ActionData/ActionContext methods instead
