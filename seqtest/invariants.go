package seqtest

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Invariant is a property that must hold after every action
// It receives the state before and after the action, plus the action itself
type Invariant func(before, after StateSnapshot, action Action) error

// InvariantFunc is a convenience type for creating invariants from simple functions
type InvariantFunc func(state interface{}) error

// AsInvariant converts a simple state check function to an Invariant
func (f InvariantFunc) AsInvariant() Invariant {
	return func(before, after StateSnapshot, action Action) error {
		return f(after.State)
	}
}

// DefaultInvariants are the standard invariants applied by default
var DefaultInvariants = []Invariant{
	InvariantSerializable,
	InvariantNoPanic,
}

// InvariantSerializable ensures state can be JSON serialized
// This detects leaked dependencies (DB connections, loggers, etc.)
var InvariantSerializable Invariant = func(before, after StateSnapshot, action Action) error {
	data, err := json.Marshal(after.State)
	if err != nil {
		return fmt.Errorf("state not serializable after %s: %w", action.Name, err)
	}

	// Verify it can be unmarshaled back
	var check interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		return fmt.Errorf("state not deserializable after %s: %w", action.Name, err)
	}

	return nil
}

// InvariantNoPanic wraps execution to catch panics
// Note: This is handled by the executor, included here for documentation
var InvariantNoPanic Invariant = func(before, after StateSnapshot, action Action) error {
	// Panic recovery is handled at executor level
	return nil
}

// InvariantValidTree ensures the tree structure is valid
var InvariantValidTree Invariant = func(before, after StateSnapshot, action Action) error {
	if after.Tree == nil {
		return nil // No tree to validate
	}
	return validateTree(after.Tree, "")
}

// InvariantNonNegativeCounts checks that all int fields are non-negative
// Useful for counters, quantities, and similar fields
var InvariantNonNegativeCounts Invariant = func(before, after StateSnapshot, action Action) error {
	return checkNonNegativeInts(after.State, "")
}

// InvariantNoDataLoss ensures no data is silently lost during state transitions
// Checks that fields present before are either still present or explicitly removed
var InvariantNoDataLoss Invariant = func(before, after StateSnapshot, action Action) error {
	beforeData, err := json.Marshal(before.State)
	if err != nil {
		return nil // Can't check
	}
	afterData, err := json.Marshal(after.State)
	if err != nil {
		return nil
	}

	var beforeMap, afterMap map[string]interface{}
	json.Unmarshal(beforeData, &beforeMap)
	json.Unmarshal(afterData, &afterMap)

	// Check top-level fields
	for key := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			return fmt.Errorf("field %q lost after action %s", key, action.Name)
		}
	}

	return nil
}

// InvariantStateUnchangedOnError ensures state doesn't change when action returns error
var InvariantStateUnchangedOnError = func(actionErr error) Invariant {
	return func(before, after StateSnapshot, action Action) error {
		if actionErr == nil {
			return nil // No error, state can change
		}

		beforeData, _ := json.Marshal(before.State)
		afterData, _ := json.Marshal(after.State)

		if string(beforeData) != string(afterData) {
			return fmt.Errorf("state changed despite error in %s", action.Name)
		}
		return nil
	}
}

// NewInvariantSliceLen creates an invariant checking a slice field's length
func NewInvariantSliceLen(fieldName string, minLen, maxLen int) Invariant {
	return func(before, after StateSnapshot, action Action) error {
		v := reflect.ValueOf(after.State)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		field := v.FieldByName(fieldName)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", fieldName)
		}

		if field.Kind() != reflect.Slice {
			return fmt.Errorf("field %s is not a slice", fieldName)
		}

		length := field.Len()
		if length < minLen || length > maxLen {
			return fmt.Errorf("field %s length %d outside bounds [%d, %d]",
				fieldName, length, minLen, maxLen)
		}

		return nil
	}
}

// NewInvariantFieldRange creates an invariant checking a numeric field is in range
func NewInvariantFieldRange(fieldName string, min, max int) Invariant {
	return func(before, after StateSnapshot, action Action) error {
		v := reflect.ValueOf(after.State)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		field := v.FieldByName(fieldName)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", fieldName)
		}

		var val int
		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			val = int(field.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			val = int(field.Uint())
		default:
			return fmt.Errorf("field %s is not numeric", fieldName)
		}

		if val < min || val > max {
			return fmt.Errorf("field %s value %d outside range [%d, %d]",
				fieldName, val, min, max)
		}

		return nil
	}
}

// validateTree recursively validates tree structure
func validateTree(tree map[string]interface{}, path string) error {
	for key, value := range tree {
		fullPath := path + "/" + key

		switch v := value.(type) {
		case map[string]interface{}:
			if err := validateTree(v, fullPath); err != nil {
				return err
			}
		case []interface{}:
			// Valid for statics ("s") or range operations
			if key == "s" {
				for i, item := range v {
					if _, ok := item.(string); !ok {
						// Could be a map for nested statics
						if m, ok := item.(map[string]interface{}); ok {
							if err := validateTree(m, fmt.Sprintf("%s[%d]", fullPath, i)); err != nil {
								return err
							}
						}
					}
				}
			}
		case string, float64, bool, nil:
			// Valid leaf values
		default:
			return fmt.Errorf("invalid tree value type %T at %s", value, fullPath)
		}
	}
	return nil
}

// checkNonNegativeInts recursively checks all int fields are non-negative
func checkNonNegativeInts(v interface{}, path string) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldName := t.Field(i).Name
		fieldPath := path + "." + fieldName
		if path == "" {
			fieldPath = fieldName
		}

		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Int() < 0 {
				return fmt.Errorf("negative value %d in field %s", field.Int(), fieldPath)
			}
		case reflect.Struct:
			if err := checkNonNegativeInts(field.Interface(), fieldPath); err != nil {
				return err
			}
		case reflect.Ptr:
			if !field.IsNil() {
				if err := checkNonNegativeInts(field.Interface(), fieldPath); err != nil {
					return err
				}
			}
		case reflect.Slice:
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if elem.Kind() == reflect.Struct || (elem.Kind() == reflect.Ptr && !elem.IsNil()) {
					if err := checkNonNegativeInts(elem.Interface(), fmt.Sprintf("%s[%d]", fieldPath, j)); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
