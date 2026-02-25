package livetemplate

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// AssertPureState validates that a state type contains only serializable data.
// Use in tests to catch accidental dependency inclusion:
//
//	func TestMyState_IsPure(t *testing.T) {
//	    AssertPureState[MyState](t)
//	}
func AssertPureState[T any](t *testing.T) {
	t.Helper()
	if err := validatePureState[T](); err != nil {
		t.Error(err)
	}
}

func validatePureState[T any]() error {
	var zero T
	typ := reflect.TypeOf(zero)

	return validatePureStateType(typ, "")
}

func validatePureStateType(typ reflect.Type, path string) error {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil // Non-struct types are OK
	}

	for field := range typ.Fields() {
		fieldPath := path + "." + field.Name
		if path == "" {
			fieldPath = field.Name
		}

		fieldType := field.Type

		// Check for common dependency patterns
		if isDependencyType(fieldType) {
			return fmt.Errorf("field %s appears to be a dependency (%s) - move to controller",
				fieldPath, fieldType.String())
		}

		// Recursively check embedded structs
		if fieldType.Kind() == reflect.Struct {
			if err := validatePureStateType(fieldType, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// isDependencyType checks if a type looks like a dependency
func isDependencyType(typ reflect.Type) bool {
	if typ.Kind() != reflect.Pointer && typ.Kind() != reflect.Interface {
		return false
	}

	name := typ.String()
	// Common dependency patterns
	patterns := []string{
		"*sql.DB", "*sql.Tx", "*sql.Conn",
		"*slog.Logger", "*log.Logger",
		"*http.Client",
		"*redis.Client",
		"io.Writer", "io.Reader",
	}

	return slices.Contains(patterns, name)
}
