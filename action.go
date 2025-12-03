package livetemplate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate/internal/send"
	uploadtypes "github.com/livetemplate/livetemplate/internal/uploadtypes"
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
//
// DEPRECATED: Use GetIntOk for explicit error handling to distinguish
// between missing keys, type errors, and actual zero values.
// This method will be removed in v0.3.0.
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
//
// DEPRECATED: Use GetFloatOk for explicit error handling to distinguish
// between missing keys, type errors, and actual zero values.
// This method will be removed in v0.3.0.
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
//
// DEPRECATED: Use GetBoolOk for explicit error handling to distinguish
// between missing keys, type errors, and actual false values.
// This method will be removed in v0.3.0.
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

// uploadAccessor provides upload access for ActionContext
type uploadAccessor interface {
	HasUploads(name string) bool
	GetUploads(name string) []*uploadtypes.UploadEntry
	GetUpload(name string, entryID string) *uploadtypes.UploadEntry
	GetValidUploads(name string) []*uploadtypes.UploadEntry
	GetCompletedUploads(name string) []*uploadtypes.UploadEntry
}

// ActionContext provides context for a Change action.
//
// The Ctx field contains the request context.Context which can be used for:
// - Timeouts and cancellation
// - Trace ID propagation
// - Request-scoped values (e.g., user_id, group_id)
//
// Upload functionality is available via upload methods when handling
// upload-related actions.
//
// HTTP methods (SetCookie, Redirect, etc.) are available for HTTP POST actions
// but will return ErrNoHTTPContext for WebSocket actions. This is by design:
// session cookies should be HttpOnly and can only be set via HTTP responses.
// Use HTTP POST forms for authentication flows that need to set cookies.
type ActionContext struct {
	Action  string
	Data    *ActionData
	Ctx     context.Context // Request context for timeout/cancellation/values
	uploads uploadAccessor  // Internal: provides upload access

	// HTTP context (nil for WebSocket actions)
	w http.ResponseWriter
	r *http.Request
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

// HasUploads checks if there are any uploads for the given field name
func (c *ActionContext) HasUploads(name string) bool {
	if c.uploads == nil {
		return false
	}
	return c.uploads.HasUploads(name)
}

// GetUploads returns all upload entries for the given field name
func (c *ActionContext) GetUploads(name string) []*uploadtypes.UploadEntry {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.GetUploads(name)
}

// GetUpload returns a specific upload entry by field name and entry ID
func (c *ActionContext) GetUpload(name string, entryID string) *uploadtypes.UploadEntry {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.GetUpload(name, entryID)
}

// GetValidUploads returns all valid (non-error) upload entries for the given field name
func (c *ActionContext) GetValidUploads(name string) []*uploadtypes.UploadEntry {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.GetValidUploads(name)
}

// GetCompletedUploads returns all completed upload entries for the given field name
func (c *ActionContext) GetCompletedUploads(name string) []*uploadtypes.UploadEntry {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.GetCompletedUploads(name)
}

// --- HTTP Methods ---
// These methods are available for HTTP POST actions but return ErrNoHTTPContext
// for WebSocket actions. Use HTTP POST forms for authentication flows.

// IsHTTP returns true if this action was triggered via HTTP POST,
// false if via WebSocket. Use this to check before calling HTTP-only methods.
func (c *ActionContext) IsHTTP() bool {
	return c.w != nil && c.r != nil
}

// SetCookie adds a Set-Cookie header to the HTTP response.
// Returns ErrNoHTTPContext if called from a WebSocket action.
//
// For authentication, use HttpOnly cookies to prevent XSS attacks:
//
//	ctx.SetCookie(&http.Cookie{
//	    Name:     "session_token",
//	    Value:    token,
//	    Path:     "/",
//	    HttpOnly: true,
//	    Secure:   true,
//	    SameSite: http.SameSiteStrictMode,
//	})
func (c *ActionContext) SetCookie(cookie *http.Cookie) error {
	if c.w == nil {
		return ErrNoHTTPContext
	}
	http.SetCookie(c.w, cookie)
	return nil
}

// GetCookie retrieves a cookie from the HTTP request.
// Returns ErrNoHTTPContext if called from a WebSocket action,
// or http.ErrNoCookie if the cookie doesn't exist.
func (c *ActionContext) GetCookie(name string) (*http.Cookie, error) {
	if c.r == nil {
		return nil, ErrNoHTTPContext
	}
	return c.r.Cookie(name)
}

// DeleteCookie removes a cookie by setting MaxAge to -1.
// Returns ErrNoHTTPContext if called from a WebSocket action.
func (c *ActionContext) DeleteCookie(name string) error {
	if c.w == nil {
		return ErrNoHTTPContext
	}
	http.SetCookie(c.w, &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	return nil
}

// Redirect sends an HTTP redirect response.
// Returns ErrNoHTTPContext if called from a WebSocket action.
// Returns ErrInvalidRedirectCode if code is not a 3xx status.
// Returns ErrInvalidRedirectURL if URL could cause an open redirect vulnerability.
//
// Only relative paths starting with "/" are allowed (e.g., "/dashboard").
// Protocol-relative URLs like "//evil.com" are rejected.
//
// Example:
//
//	return ctx.Redirect("/dashboard", http.StatusSeeOther)
func (c *ActionContext) Redirect(url string, code int) error {
	if c.w == nil || c.r == nil {
		return ErrNoHTTPContext
	}

	if code < 300 || code >= 400 {
		return ErrInvalidRedirectCode
	}

	if !isValidRedirectURL(url) {
		return ErrInvalidRedirectURL
	}

	http.Redirect(c.w, c.r, url, code)
	return nil
}

// SetHeader sets an HTTP response header.
// Returns ErrNoHTTPContext if called from a WebSocket action.
func (c *ActionContext) SetHeader(key, value string) error {
	if c.w == nil {
		return ErrNoHTTPContext
	}
	c.w.Header().Set(key, value)
	return nil
}

// GetHeader retrieves an HTTP request header value.
// Returns empty string if called from a WebSocket action or if header doesn't exist.
func (c *ActionContext) GetHeader(key string) string {
	if c.r == nil {
		return ""
	}
	return c.r.Header.Get(key)
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

// Store is an optional interface for stores that want explicit control over action routing.
// If a store implements this interface, Change() will be called for all actions.
// If a store does NOT implement this interface, actions are automatically dispatched
// to methods matching the action name (e.g., "increment" → Increment(ctx *ActionContext) error).
type Store interface {
	Change(ctx *ActionContext) error
}

// StoreInitializer is an optional interface that stores can implement
// to perform initialization after being cloned for a new session.
// This is useful for loading data from external sources like databases.
type StoreInitializer interface {
	Init() error
}

// Stores is a map of named stores.
// Stores can be any type - if they implement the Store interface, Change() is called.
// Otherwise, actions are automatically dispatched to matching methods.
type Stores map[string]interface{}

// parseAction splits "counter.increment" into ("counter", "increment")
// For single store actions like "increment", returns ("", "increment")
func parseAction(action string) (store string, actualAction string) {
	parts := strings.SplitN(action, ".", 2)

	if len(parts) == 2 {
		return parts[0], parts[1] // "counter", "increment"
	}

	return "", parts[0] // "", "increment" (single store)
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
