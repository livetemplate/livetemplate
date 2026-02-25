package livetemplate

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Increment", "increment"},
		{"AddItem", "add_item"},
		{"UpdateUserProfile", "update_user_profile"},
		{"HTTPHandler", "h_t_t_p_handler"}, // edge case - consecutive caps
		{"", ""},
		{"a", "a"},
		{"A", "a"},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestMethodNameToActions(t *testing.T) {
	tests := []struct {
		methodName string
		expected   []string
	}{
		{"Increment", []string{"increment", "Increment"}},
		{"AddItem", []string{"addItem", "add_item", "AddItem"}},
	}

	for _, tt := range tests {
		result := methodNameToActions(tt.methodName)
		if len(result) != len(tt.expected) {
			t.Errorf("methodNameToActions(%q) = %v, expected %v", tt.methodName, result, tt.expected)
			continue
		}
		for i, exp := range tt.expected {
			found := slices.Contains(result, exp)
			if !found {
				t.Errorf("methodNameToActions(%q)[%d] missing %q, got %v", tt.methodName, i, exp, result)
			}
		}
	}
}

// ============================================================================
// Controller+State Pattern Tests
// ============================================================================

// Test state for dispatch pattern
type testCounterState struct {
	Count int
}

// Test controller for dispatch pattern
type testCounterController struct {
	IncrementCalled bool
	AddCalled       bool
}

// Method signature: (state, ctx) -> (state, error)
func (c *testCounterController) Increment(state testCounterState, ctx *Context) (testCounterState, error) {
	c.IncrementCalled = true
	state.Count++
	return state, nil
}

func (c *testCounterController) Add(state testCounterState, ctx *Context) (testCounterState, error) {
	c.AddCalled = true
	amount := ctx.GetInt("amount")
	if amount == 0 {
		amount = 1
	}
	state.Count += amount
	return state, nil
}

func (c *testCounterController) FailingAction(state testCounterState, ctx *Context) (testCounterState, error) {
	return state, errors.New("action failed")
}

// Invalid signature - should be ignored
func (c *testCounterController) WrongSig(state testCounterState) (testCounterState, error) {
	return state, nil
}

func TestDispatchWithState_Increment(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 5}
	ctx := NewContext(context.TODO(), "increment", nil)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result, ok := newState.(testCounterState)
	if !ok {
		t.Fatalf("Expected testCounterState, got %T", newState)
	}

	if result.Count != 6 {
		t.Errorf("Count = %d, want 6", result.Count)
	}

	if !ctrl.IncrementCalled {
		t.Error("Increment was not called on controller")
	}
}

func TestDispatchWithState_WithData(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 10}
	data := map[string]any{"amount": float64(5)} // JSON numbers are float64
	ctx := NewContext(context.TODO(), "add", data)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result := newState.(testCounterState)
	if result.Count != 15 {
		t.Errorf("Count = %d, want 15", result.Count)
	}
}

func TestDispatchWithState_MethodNotFound(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "nonexistent", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound, got %v", err)
	}
}

func TestDispatchWithState_EmptyAction(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for empty action, got %v", err)
	}
}

func TestDispatchWithState_NilContext(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}

	_, err := DispatchWithState(ctrl, state, nil)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for nil context, got %v", err)
	}
}

func TestDispatchWithState_MethodReturnsError(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "failingAction", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if err == nil {
		t.Fatal("Expected error from method")
	}
	if err.Error() != "action failed" {
		t.Errorf("Expected 'action failed', got %q", err.Error())
	}
}

func TestDispatchWithState_SnakeCase(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 0}
	ctx := NewContext(context.TODO(), "failing_action", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if err == nil || err.Error() != "action failed" {
		t.Errorf("Expected snake_case action to work, got %v", err)
	}
}

func TestDispatchWithState_StateUnchangedOnError(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 100}
	ctx := NewContext(context.TODO(), "failingAction", nil)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	// State should be returned as-is even on error
	result := newState.(testCounterState)
	if result.Count != 100 {
		t.Errorf("State should be unchanged, Count = %d", result.Count)
	}
}

func BenchmarkDispatchWithState_Cached(b *testing.B) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 0}
	ctx := NewContext(context.TODO(), "increment", nil)

	// Warm up cache
	_, _ = DispatchWithState(ctrl, state, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DispatchWithState(ctrl, state, ctx)
	}
}
