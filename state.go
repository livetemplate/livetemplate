package livetemplate

import (
	"encoding"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
)

// =============================================================================
// Controller+State Pattern Types
// =============================================================================

// State is the interface for session state that can be persisted.
// The serialization requirement ensures state contains only pure data.
// Use AsState[T]() for zero-boilerplate implementation.
//
// Controllers hold dependencies (DB, Logger, etc.) and are never cloned.
// State holds pure data and is cloned per session via serialization.
type State interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	Inner() any // Returns the underlying value for framework use
}

// AsState wraps a plain struct pointer to satisfy the State interface.
// Panics if the state type contains dependency fields (e.g., *sql.DB,
// *slog.Logger) that belong in the controller. Uses JSON serialization
// by default. For custom serialization, implement the State interface
// directly on your type.
//
// Example:
//
//	state := AsState(&TodoState{})
//	handler := tmpl.Handle(&TodoController{DB: db}, state)
func AsState[T any](s *T) State {
	if err := validatePureState[T](); err != nil {
		panic(fmt.Sprintf("livetemplate.AsState: %v", err))
	}
	return &jsonState[T]{value: s}
}

func validatePureState[T any]() error {
	var zero T
	typ := reflect.TypeOf(zero)
	return validatePureStateType(typ, "")
}

func validatePureStateType(typ reflect.Type, path string) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}
		if isDependencyType(field.Type) {
			return fmt.Errorf("field %s appears to be a dependency (%s) - move to controller",
				fieldPath, field.Type.String())
		}
		if field.Type.Kind() == reflect.Struct {
			if err := validatePureStateType(field.Type, fieldPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func isDependencyType(typ reflect.Type) bool {
	if typ.Kind() != reflect.Ptr && typ.Kind() != reflect.Interface {
		return false
	}
	name := typ.String()
	patterns := []string{
		"*sql.DB", "*sql.Tx", "*sql.Conn",
		"*slog.Logger", "*log.Logger",
		"*http.Client",
		"*redis.Client",
		"io.Writer", "io.Reader",
	}
	for _, p := range patterns {
		if name == p {
			return true
		}
	}
	return false
}

// jsonState is the generic wrapper implementing State with JSON serialization
type jsonState[T any] struct {
	value *T
}

func (s *jsonState[T]) MarshalBinary() ([]byte, error) {
	return json.Marshal(s.value)
}

func (s *jsonState[T]) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, s.value)
}

func (s *jsonState[T]) Inner() any {
	return s.value
}

// =============================================================================
// Legacy State Tag Handling (to be removed in Task 9)
// =============================================================================

// stateTag is the struct tag used to mark fields for persistence.
// Fields tagged with `lvt:"state"` are serialized/deserialized by the framework.
// Fields without this tag are NOT persisted (e.g., dependencies like DB, Logger).
//
// Example:
//
//	type UserController struct {
//	    Profile  *UserProfile  `lvt:"state"`  // Persisted
//	    Settings *UserSettings `lvt:"state"`  // Persisted
//	    DB       *sql.DB                      // NOT persisted
//	    Logger   *slog.Logger                 // NOT persisted
//	}
const stateTag = "lvt"
const stateTagValue = "state"
const transientTagValue = "transient"

// stateFieldCache caches reflection info for state fields by type.
// Key: reflect.Type, Value: []stateFieldInfo
var stateFieldCache sync.Map

// stateFieldInfo holds metadata about a state field.
type stateFieldInfo struct {
	Name  string       // Field name
	Index int          // Field index in struct
	Type  reflect.Type // Field type
}

// HasStateFields checks if a store has any fields tagged with `lvt:"state"`.
// This is used to determine if selective serialization should be used.
func HasStateFields(store interface{}) bool {
	fields := getStateFieldInfo(store)
	return len(fields) > 0
}

// getStateFieldInfo returns metadata about state-tagged fields for a store type.
// Results are cached by type for performance.
func getStateFieldInfo(store interface{}) []stateFieldInfo {
	storeType := reflect.TypeOf(store)
	if storeType.Kind() == reflect.Ptr {
		storeType = storeType.Elem()
	}

	// Check cache
	if cached, ok := stateFieldCache.Load(storeType); ok {
		return cached.([]stateFieldInfo)
	}

	// Build field info
	var fields []stateFieldInfo
	for i := 0; i < storeType.NumField(); i++ {
		field := storeType.Field(i)
		tag := field.Tag.Get(stateTag)

		if tag == stateTagValue {
			fields = append(fields, stateFieldInfo{
				Name:  field.Name,
				Index: i,
				Type:  field.Type,
			})
		} else if tag != "" {
			// Warn on potential typos: case variations or common mistakes
			validateStateTag(tag, field.Name, storeType.Name())
		}
	}

	// Cache and return
	stateFieldCache.Store(storeType, fields)
	return fields
}

// validateStateTag warns if a tag value looks like a typo or invalid combination.
// Valid values: "state" (for persistence in Controller+State pattern), "transient" (cleared on reload)
func validateStateTag(tag, fieldName, typeName string) {
	// Check for comma-separated values (invalid - can only use one)
	if strings.Contains(tag, ",") {
		slog.Warn("Invalid lvt tag: cannot combine multiple values",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag),
			slog.String("valid_values", "state, transient (use one, not both)"))
		return
	}

	// Check if it's a known valid value
	validValues := []string{stateTagValue, transientTagValue}
	for _, valid := range validValues {
		if tag == valid {
			return // Valid, no warning needed
		}
	}

	lowerTag := strings.ToLower(tag)

	// Case variation of "state"
	if lowerTag == stateTagValue && tag != stateTagValue {
		slog.Warn("Possible typo in lvt tag: use lowercase 'state'",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag),
			slog.String("expected", stateTagValue))
		return
	}

	// Case variation of "transient"
	if lowerTag == transientTagValue && tag != transientTagValue {
		slog.Warn("Possible typo in lvt tag: use lowercase 'transient'",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag),
			slog.String("expected", transientTagValue))
		return
	}

	// Common typos of "state"
	typos := []string{"states", "stae", "satte", "stat", "staet"}
	for _, typo := range typos {
		if lowerTag == typo {
			slog.Warn("Possible typo in lvt tag",
				slog.String("field", fieldName),
				slog.String("type", typeName),
				slog.String("got", tag),
				slog.String("expected", stateTagValue))
			return
		}
	}

	// Common typos of "transient"
	transientTypos := []string{"transiant", "transent", "trasient", "tranisent"}
	for _, typo := range transientTypos {
		if lowerTag == typo {
			slog.Warn("Possible typo in lvt tag",
				slog.String("field", fieldName),
				slog.String("type", typeName),
				slog.String("got", tag),
				slog.String("expected", transientTagValue))
			return
		}
	}

	// Unknown tag value - just log at debug level
	slog.Debug("Unknown lvt tag value",
		slog.String("field", fieldName),
		slog.String("type", typeName),
		slog.String("got", tag))
}

// ExtractState extracts state-tagged fields from a store into a serializable map.
// Returns nil if the store has no state-tagged fields.
//
// The returned map has field names as keys and field values as values.
// This map can be serialized with SerializeState.
func ExtractState(store interface{}) map[string]interface{} {
	fields := getStateFieldInfo(store)
	if len(fields) == 0 {
		return nil
	}

	storeValue := reflect.ValueOf(store)
	if storeValue.Kind() == reflect.Ptr {
		storeValue = storeValue.Elem()
	}

	result := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		fieldValue := storeValue.Field(field.Index)
		if fieldValue.CanInterface() {
			result[field.Name] = fieldValue.Interface()
		}
	}

	return result
}

// InjectState injects state from a map back into a store's state-tagged fields.
// This is used during deserialization to restore state into a cloned controller.
//
// The map should have field names as keys (matching the struct field names).
func InjectState(store interface{}, state map[string]interface{}) error {
	fields := getStateFieldInfo(store)
	if len(fields) == 0 {
		return nil
	}

	storeValue := reflect.ValueOf(store)
	if storeValue.Kind() == reflect.Ptr {
		storeValue = storeValue.Elem()
	}

	for _, field := range fields {
		if value, ok := state[field.Name]; ok {
			fieldValue := storeValue.Field(field.Index)
			if fieldValue.CanSet() {
				// Handle type conversion
				valueReflect := reflect.ValueOf(value)
				if valueReflect.Type().AssignableTo(field.Type) {
					fieldValue.Set(valueReflect)
				} else if valueReflect.Type().ConvertibleTo(field.Type) {
					fieldValue.Set(valueReflect.Convert(field.Type))
				} else {
					return fmt.Errorf("cannot assign %T to field %s of type %s", value, field.Name, field.Type)
				}
			}
		}
	}

	return nil
}

// ClearTransientFields zeros out fields tagged with `lvt:"transient"`.
// This is called when restoring state after a page reload/reconnect to ensure
// transient UI state (like which modal is open) doesn't persist across page loads.
//
// Example usage in state struct:
//
//	type PostsState struct {
//	    SearchQuery  string     `json:"search_query"`                    // Persisted
//	    EditingID    string     `json:"editing_id" lvt:"transient"`      // Cleared on reload
//	    EditingPost  *PostItem  `json:"editing_post" lvt:"transient"`    // Cleared on reload
//	}
func ClearTransientFields(state interface{}) interface{} {
	// If state implements the State interface (e.g., jsonState wrapper),
	// unwrap it to get the actual state struct
	if s, ok := state.(State); ok {
		state = s.Inner()
	}

	v := reflect.ValueOf(state)

	// Track whether input was a pointer for correct return type
	wasPointer := false
	var elem reflect.Value

	// Handle pointer vs value - we need an addressable struct to modify fields
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return state
		}
		wasPointer = true
		elem = v.Elem()
	} else if v.Kind() == reflect.Struct {
		// Value type - create an addressable copy
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		elem = ptr.Elem()
	} else {
		return state
	}

	if elem.Kind() != reflect.Struct {
		return state
	}

	t := elem.Type()
	clearedCount := 0
	for i := 0; i < elem.NumField(); i++ {
		field := t.Field(i)
		lvtTag := field.Tag.Get(stateTag)
		if lvtTag == transientTagValue {
			if elem.Field(i).CanSet() {
				elem.Field(i).Set(reflect.Zero(field.Type))
				clearedCount++
			}
		}
	}

	if clearedCount > 0 {
		slog.Debug("ClearTransientFields: cleared transient fields",
			slog.String("type", t.Name()),
			slog.Int("count", clearedCount))
	}

	// Return matching type: pointer if input was pointer, value otherwise
	if wasPointer {
		return elem.Addr().Interface()
	}
	return elem.Interface()
}

// stateEnvelope is the serialization format for state-tagged fields.
// Each field is serialized independently (either via BinaryMarshaler or JSON).
type stateEnvelope struct {
	Version int               `json:"v"`      // Envelope version for future compatibility
	Fields  map[string][]byte `json:"fields"` // Field name -> serialized bytes
}

// SerializeState serializes state fields into bytes.
// Each field is serialized using BinaryMarshaler if available, otherwise JSON.
//
// The envelope format allows individual fields to use different serialization methods.
func SerializeState(state map[string]interface{}) ([]byte, error) {
	if len(state) == 0 {
		return nil, nil
	}

	envelope := stateEnvelope{
		Version: 1,
		Fields:  make(map[string][]byte, len(state)),
	}

	for name, value := range state {
		var data []byte
		var err error

		// Check if value implements BinaryMarshaler
		if marshaler, ok := value.(encoding.BinaryMarshaler); ok {
			data, err = marshaler.MarshalBinary()
		} else {
			// Default to JSON
			data, err = json.Marshal(value)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to serialize field %q: %w", name, err)
		}

		envelope.Fields[name] = data
	}

	return json.Marshal(envelope)
}

// DeserializeState deserializes state bytes into a map of field values.
// The store parameter provides type information for proper deserialization.
//
// For BinaryUnmarshaler fields, the method creates new instances and calls UnmarshalBinary.
// For other fields, JSON is used.
func DeserializeState(data []byte, store interface{}) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var envelope stateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state envelope: %w", err)
	}

	if envelope.Version != 1 {
		return nil, fmt.Errorf("unsupported state envelope version: %d", envelope.Version)
	}

	fields := getStateFieldInfo(store)
	fieldTypes := make(map[string]reflect.Type, len(fields))
	for _, f := range fields {
		fieldTypes[f.Name] = f.Type
	}

	result := make(map[string]interface{}, len(envelope.Fields))

	for name, fieldData := range envelope.Fields {
		fieldType, ok := fieldTypes[name]
		if !ok {
			// Field no longer exists in struct - skip
			continue
		}

		// Create a new instance of the field type
		var fieldPtr reflect.Value
		if fieldType.Kind() == reflect.Ptr {
			fieldPtr = reflect.New(fieldType.Elem())
		} else {
			fieldPtr = reflect.New(fieldType)
		}

		fieldValue := fieldPtr.Interface()

		// Check if field implements BinaryUnmarshaler
		if unmarshaler, ok := fieldValue.(encoding.BinaryUnmarshaler); ok {
			if err := unmarshaler.UnmarshalBinary(fieldData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal field %q: %w", name, err)
			}
			if fieldType.Kind() == reflect.Ptr {
				result[name] = fieldValue
			} else {
				result[name] = fieldPtr.Elem().Interface()
			}
		} else {
			// Default to JSON
			if err := json.Unmarshal(fieldData, fieldValue); err != nil {
				return nil, fmt.Errorf("failed to JSON unmarshal field %q: %w", name, err)
			}
			if fieldType.Kind() == reflect.Ptr {
				result[name] = fieldValue
			} else {
				result[name] = fieldPtr.Elem().Interface()
			}
		}
	}

	return result, nil
}
