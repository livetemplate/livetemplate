package livetemplate

import "testing"

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
