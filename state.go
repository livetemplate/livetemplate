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
// *slog.Logger) that belong in the controller. Checks direct fields,
// nested structs, pointer-to-struct fields, and slice/array/map element
// types. Uses JSON serialization by default. For custom serialization,
// implement the State interface directly on your type.
//
// The check is best-effort: it matches a fixed set of known stdlib
// dependency types. Custom wrappers (e.g., type AppDB struct{ *sql.DB })
// or third-party types (e.g., *pgxpool.Pool) are not caught.
// Use AssertPureState[T]() in tests for stricter validation.
//
// Example:
//
//	state := AsState(&TodoState{})
//	handler := tmpl.Handle(&TodoController{DB: db}, state)
func AsState[T any](s *T) State {
	if err := validatePureState[T](); err != nil {
		panic(fmt.Sprintf("livetemplate.AsState: %v", err))
	}
	return &jsonState[T]{
		value:         s,
		persistFields: detectPersistFields[T](),
	}
}

var pureStateCache sync.Map // reflect.Type → error (nil for pure)

func validatePureState[T any]() error {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return nil
	}
	if cached, ok := pureStateCache.Load(typ); ok {
		// A nil error stored as any produces a nil interface value,
		// so the nil check correctly handles the "type is pure" case.
		if cached == nil {
			return nil
		}
		return cached.(error)
	}
	err := validatePureStateType(typ, "", make(map[reflect.Type]bool))
	pureStateCache.Store(typ, err)
	return err
}

func validatePureStateType(typ reflect.Type, path string, visited map[reflect.Type]bool) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	if visited[typ] {
		return nil
	}
	visited[typ] = true
	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}
		if err := checkFieldType(field.Type, fieldPath, visited); err != nil {
			return err
		}
	}
	return nil
}

func checkFieldType(ft reflect.Type, fieldPath string, visited map[reflect.Type]bool) error {
	// isDependencyType only matches pointer/interface kinds; value-type structs
	// fall through to recursive descent below.
	if isDependencyType(ft) {
		return fmt.Errorf("field %s appears to be a dependency (%s) - move to controller",
			fieldPath, ft.String())
	}
	switch ft.Kind() {
	case reflect.Struct:
		return validatePureStateType(ft, fieldPath, visited)
	case reflect.Ptr:
		if ft.Elem().Kind() == reflect.Struct {
			return validatePureStateType(ft.Elem(), fieldPath, visited)
		}
	case reflect.Slice, reflect.Array:
		return checkFieldType(ft.Elem(), fieldPath+"[]", visited)
	case reflect.Map:
		if err := checkFieldType(ft.Key(), fieldPath+"[key]", visited); err != nil {
			return err
		}
		return checkFieldType(ft.Elem(), fieldPath+"[value]", visited)
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

// persistableState is an internal interface for selective state persistence.
// Implemented by jsonState[T] when the state type has lvt:"persist" tagged fields.
// Used by mount.go to determine persistence behavior.
type persistableState interface {
	HasPersistFields() bool
	ExtractPersistFields(state interface{}) ([]byte, error)
	InjectPersistFields(data []byte) (interface{}, error)
}

// jsonState is the generic wrapper implementing State with JSON serialization.
// It also detects lvt:"persist" tags at construction time for selective persistence.
type jsonState[T any] struct {
	value         *T
	persistFields []persistFieldInfo // cached at AsState() time; nil if no persist tags
}

// persistFieldInfo holds metadata for a field tagged with lvt:"persist".
type persistFieldInfo struct {
	jsonName string // JSON tag name used for serialization
	index    int    // Field index in struct
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

// HasPersistFields returns true if the state type has any lvt:"persist" tagged fields.
func (s *jsonState[T]) HasPersistFields() bool {
	return len(s.persistFields) > 0
}

// ExtractPersistFields serializes only lvt:"persist" tagged fields from a raw state value.
// Returns JSON bytes containing only the persist fields, suitable for SessionStore storage.
func (s *jsonState[T]) ExtractPersistFields(state interface{}) ([]byte, error) {
	if len(s.persistFields) == 0 {
		return nil, nil
	}
	v := reflect.ValueOf(state)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	m := make(map[string]any, len(s.persistFields))
	for _, f := range s.persistFields {
		m[f.jsonName] = v.Field(f.index).Interface()
	}
	return json.Marshal(m)
}

// InjectPersistFields creates a zero-value state and deserializes persist field data into it.
// Only persist-tagged fields are populated; all other fields remain at zero values.
// Returns the state as a value type (not pointer).
//
// Safety invariant: data must contain only persist-field keys (as produced by
// ExtractPersistFields). The function uses json.Unmarshal which would populate
// any matching field — the caller contract, not this function, ensures selectivity.
func (s *jsonState[T]) InjectPersistFields(data []byte) (interface{}, error) {
	if len(s.persistFields) == 0 || len(data) == 0 {
		var zero T
		return zero, nil
	}
	var state T
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to deserialize persist fields: %w", err)
	}
	return state, nil
}

// detectPersistFields inspects a struct type for lvt:"persist" tagged fields.
func detectPersistFields[T any]() []persistFieldInfo {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var fields []persistFieldInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get(stateTag)
		if tag != "" && tag != persistTagValue {
			validatePersistTag(tag, f.Name, t.Name())
		}
		if tag == persistTagValue {
			jsonName := f.Name
			if jt := f.Tag.Get("json"); jt != "" {
				parts := strings.Split(jt, ",")
				if parts[0] != "" && parts[0] != "-" {
					jsonName = parts[0]
				}
			}
			fields = append(fields, persistFieldInfo{
				jsonName: jsonName,
				index:    i,
			})
		}
	}
	return fields
}

// =============================================================================
// Selective Persistence via lvt:"persist" Tag
// =============================================================================

// stateTag is the struct tag key for LiveTemplate field annotations.
const stateTag = "lvt"

// persistTagValue marks a field for persistence to SessionStore.
// Fields with this tag survive page refresh; fields without it are ephemeral
// (zero value on reload, loaded by Mount() from DB or URL params).
const persistTagValue = "persist"

// validatePersistTag warns if a tag value looks like a typo.
func validatePersistTag(tag, fieldName, typeName string) {
	if tag == persistTagValue {
		return
	}

	// Warn about removed tag values from previous versions
	if tag == "state" || tag == "transient" {
		slog.Warn("lvt tag value is no longer supported, use 'persist' instead",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag))
		return
	}

	if strings.Contains(tag, ",") {
		slog.Warn("Invalid lvt tag: only single values are allowed",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag),
			slog.String("valid_values", "persist"))
		return
	}

	lowerTag := strings.ToLower(tag)

	if lowerTag == persistTagValue && tag != persistTagValue {
		slog.Warn("Possible typo in lvt tag: use lowercase 'persist'",
			slog.String("field", fieldName),
			slog.String("type", typeName),
			slog.String("got", tag),
			slog.String("expected", persistTagValue))
		return
	}

	typos := []string{"persit", "persits", "presist", "persistant", "persistent"}
	for _, typo := range typos {
		if lowerTag == typo {
			slog.Warn("Possible typo in lvt tag",
				slog.String("field", fieldName),
				slog.String("type", typeName),
				slog.String("got", tag),
				slog.String("expected", persistTagValue))
			return
		}
	}

	slog.Debug("Unknown lvt tag value",
		slog.String("field", fieldName),
		slog.String("type", typeName),
		slog.String("got", tag))
}
