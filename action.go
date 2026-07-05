package livetemplate

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

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
	ErrInvalidRedirectURL = errors.New("invalid redirect URL (must be a path or relative reference with no scheme or host)")
)

// message is an alias for internal/send.ActionMessage for backward compatibility
type message = send.ActionMessage

// defaultFormAction is the conventional action name used when a form submits
// without explicit routing (no button name="action" or form name).
// Maps to the Submit() method on the controller via methodNameToActions().
const defaultFormAction = "submit"

// actionNavigate is a reserved WebSocket-only action name. The event loop
// re-runs Mount with msg.Data as query params instead of routing through
// DispatchWithState, so no controller method named "__navigate__" is required.
const actionNavigate = "__navigate__"

// actionPing is a reserved WebSocket-only action name for the client liveness
// heartbeat. The event loop replies with a tiny pong and skips the
// action/render pipeline entirely — it exists so a client can detect a
// dead/zombie socket (one that still reports OPEN but whose TCP is gone, a
// documented mobile behaviour) by the ABSENCE of a pong within its timeout, and
// reconnect. No controller method named "__ping__" is required.
const actionPing = "__ping__"

// pongMessage is the fixed reply to an actionPing. Distinct top-level `pong`
// key so the client recognises it before the normal tree-update path (it has no
// Tree to morph). Kept as a package var so the write path allocates nothing.
var pongMessage = []byte(`{"pong":true}`)

// applyDefaultAction sets the action to defaultFormAction for forms that
// submitted without explicit action routing. Called only for browser form
// submissions (not JSON action requests).
func applyDefaultAction(msg *message) {
	if msg.Action == "" {
		msg.Action = defaultFormAction
	}
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
		// Check if it's an integer value within safe bounds.
		// JavaScript MAX_SAFE_INTEGER is 2^53-1; values beyond this may have precision issues.
		// We also guard against int64 overflow for very large float64 values.
		const maxSafeFloat = float64(1<<53 - 1) // JavaScript MAX_SAFE_INTEGER
		if v >= -maxSafeFloat && v <= maxSafeFloat && v == float64(int64(v)) {
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
// Accepts native Go numeric types (int, int32, int64, float32, float64, etc.)
// and numeric strings from form fields and data-* attributes. Native numeric
// support matters for Session.TriggerAction, where callers pass Go-native
// values rather than JSON-unmarshaled ones.
//
// Overflow and bounds safety:
//
//   - Unsigned integers that exceed math.MaxInt on the current platform
//     return (0, false) rather than silently wrapping to a negative int.
//     This matters on 64-bit platforms for values in (math.MaxInt64,
//     math.MaxUint64] and on 32-bit platforms for values in (math.MaxInt32,
//     math.MaxUint32].
//   - int64 values outside [math.MinInt, math.MaxInt] return (0, false);
//     this only matters on 32-bit platforms.
//   - float values that are NaN, infinite, out of the int range, or
//     non-integer (e.g. 1.7) return (0, false). Silently truncating a
//     float to an int would hide caller mistakes when a map entry meant
//     for a string field or a float-typed field gets routed to GetInt.
func (a *ActionData) GetIntOk(key string) (int, bool) {
	switch v := a.raw[key].(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		if v > math.MaxInt || v < math.MinInt {
			return 0, false
		}
		return int(v), true
	case uint:
		if v > math.MaxInt {
			return 0, false
		}
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		// On 64-bit platforms math.MaxInt == math.MaxInt64, and uint32's
		// max (4_294_967_295) is always within range — this check only
		// triggers on 32-bit builds, where uint32 can exceed math.MaxInt32.
		if uint64(v) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case uint64:
		if v > math.MaxInt {
			return 0, false
		}
		return int(v), true
	case float32:
		return floatToIntOk(float64(v))
	case float64:
		return floatToIntOk(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

// floatToIntOk converts a float to int with strict bounds and integer
// checks. Rejects NaN, ±Inf, values outside the platform int range, and
// non-integer values (e.g. 1.7). See GetIntOk's doc comment for rationale.
func floatToIntOk(v float64) (int, bool) {
	// NaN takes the v != math.Trunc(v) branch (NaN != anything, including
	// NaN itself) and is rejected. ±Inf is rejected by the bounds checks.
	if v < math.MinInt || v > math.MaxInt || v != math.Trunc(v) {
		return 0, false
	}
	return int(v), true
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
// Accepts native Go numeric types (int, int32, int64, float32, float64, etc.)
// and numeric strings from form fields and data-* attributes. Native numeric
// support matters for Session.TriggerAction, where callers pass Go-native
// values rather than JSON-unmarshaled ones.
//
// Precision note: float64 has a 53-bit mantissa, so integer values larger
// than 2^53 (int64 and uint64) lose precision during conversion. Callers
// needing exact large integer round-trips should use GetInt instead.
func (a *ActionData) GetFloatOk(key string) (float64, bool) {
	switch v := a.raw[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		// Values outside [-2^53, 2^53] lose precision. See doc comment.
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		// Values above 2^53 lose precision. See doc comment.
		return float64(v), true
	case string:
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
// String comparison is case-insensitive to handle variations like "True", "TRUE", etc.
func (a *ActionData) GetBoolOk(key string) (bool, bool) {
	// Handle bool values directly (from WebSocket + parseValue)
	if v, ok := a.raw[key].(bool); ok {
		return v, true
	}
	// Handle string values from HTTP form submissions (case-insensitive)
	if v, ok := a.raw[key].(string); ok {
		lowerV := strings.ToLower(v)
		if lowerV == "true" {
			return true, true
		}
		if lowerV == "false" {
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

// isValidRedirectURL reports whether rawURL is safe as a redirect target.
// It permits absolute-path references ("/dashboard") and relative references
// ("", ".", "./settings", "../list") so recipes mounted behind
// http.StripPrefix can redirect to their own mount via a relative Location the
// browser resolves against the un-stripped request URL.
//
// It rejects anything that could escape the current origin: protocol-relative
// URLs ("//evil.com"), backslash variants browsers fold to "//" ("/\evil.com",
// "\\evil.com"), and any URL carrying a scheme or host ("https://evil.com",
// "javascript:...", "mailto:..."). url.Parse treats a leading "scheme:" in a
// relative path (e.g. "foo:bar") as a scheme, so such targets are rejected;
// write "./foo:bar" to keep the colon in a path segment.
func isValidRedirectURL(rawURL string) bool {
	// Guard "//host" and the backslash variants ("/\host", "\\host", "\/host")
	// before parsing — url.Parse alone treats them as host-less relative paths,
	// but browsers fold the backslashes to "//" and follow them cross-origin.
	if len(rawURL) >= 2 {
		if c0, c1 := rawURL[0], rawURL[1]; (c0 == '/' || c0 == '\\') && (c1 == '/' || c1 == '\\') {
			return false
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "" && u.Host == ""
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

// formatFieldName converts PascalCase struct field names to human-readable display names.
// e.g., "PhoneNumber" → "Phone Number", "URL" → "URL", "Email" → "Email"
func formatFieldName(name string) string {
	if name == "" {
		return name
	}
	var result []rune
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			// Insert space before uppercase if preceded by lowercase,
			// or if preceded by uppercase followed by lowercase (e.g., "URLField" → "URL Field")
			prev := runes[i-1]
			if unicode.IsLower(prev) {
				result = append(result, ' ')
			} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				result = append(result, ' ')
			}
		}
		result = append(result, r)
	}
	return string(result)
}

// ValidationToMultiError converts go-playground/validator errors to MultiError
func ValidationToMultiError(err error) MultiError {
	var fieldErrors MultiError

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return fieldErrors
	}

	for _, e := range validationErrs {
		// Convert struct field name (e.g., "PasswordConfirmation") to snake_case
		// to match HTML form input names (e.g., "password_confirmation")
		structFieldName := e.Field()
		formFieldName := toSnakeCase(structFieldName)
		displayName := formatFieldName(structFieldName)

		var message string
		switch e.Tag() {
		case "required":
			message = fmt.Sprintf("%s is required", displayName)
		case "min":
			message = fmt.Sprintf("%s must be at least %s characters", displayName, e.Param())
		case "max":
			message = fmt.Sprintf("%s must be at most %s characters", displayName, e.Param())
		case "email":
			message = fmt.Sprintf("%s must be a valid email address", displayName)
		case "url":
			message = fmt.Sprintf("%s must be a valid URL", displayName)
		case "eqfield":
			message = fmt.Sprintf("%s must match %s", displayName, formatFieldName(e.Param()))
		default:
			message = fmt.Sprintf("%s is invalid", displayName)
		}

		fieldErrors = append(fieldErrors, FieldError{
			Field:   formFieldName, // Use snake_case to match HTML input names
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
