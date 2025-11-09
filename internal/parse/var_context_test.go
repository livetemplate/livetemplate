package parse

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestOrderedVars_Empty tests empty orderedVars behavior
func TestOrderedVars_Empty(t *testing.T) {
	ov := newOrderedVars()

	if got := ov.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}

	val, ok := ov.Get("missing")
	if ok {
		t.Error("Get(missing) = true, want false")
	}
	if val != nil {
		t.Errorf("Get(missing) value = %v, want nil", val)
	}

	called := false
	ov.Range(func(key string, value interface{}) {
		called = true
	})
	if called {
		t.Error("Range() was called on empty orderedVars")
	}
}

// TestOrderedVars_SetAndGet tests basic set and get operations
func TestOrderedVars_SetAndGet(t *testing.T) {
	ov := newOrderedVars()

	// Set first value
	ov.Set("index", 42)
	if got := ov.Len(); got != 1 {
		t.Errorf("Len() after first Set = %d, want 1", got)
	}

	val, ok := ov.Get("index")
	if !ok {
		t.Error("Get(index) = false, want true")
	}
	if val != 42 {
		t.Errorf("Get(index) = %v, want 42", val)
	}

	// Set second value
	ov.Set("value", "hello")
	if got := ov.Len(); got != 2 {
		t.Errorf("Len() after second Set = %d, want 2", got)
	}

	val, ok = ov.Get("value")
	if !ok {
		t.Error("Get(value) = false, want true")
	}
	if val != "hello" {
		t.Errorf("Get(value) = %v, want hello", val)
	}

	// First value still accessible
	val, ok = ov.Get("index")
	if !ok {
		t.Error("Get(index) after second Set = false, want true")
	}
	if val != 42 {
		t.Errorf("Get(index) after second Set = %v, want 42", val)
	}
}

// TestOrderedVars_UpdateExisting tests updating existing keys
func TestOrderedVars_UpdateExisting(t *testing.T) {
	ov := newOrderedVars()

	ov.Set("x", 1)
	ov.Set("y", 2)
	ov.Set("z", 3)

	if got := ov.Len(); got != 3 {
		t.Errorf("Len() before update = %d, want 3", got)
	}

	// Update middle value
	ov.Set("y", 200)

	if got := ov.Len(); got != 3 {
		t.Errorf("Len() after update = %d, want 3 (should not add new entry)", got)
	}

	val, ok := ov.Get("y")
	if !ok {
		t.Error("Get(y) after update = false, want true")
	}
	if val != 200 {
		t.Errorf("Get(y) after update = %v, want 200", val)
	}

	// Other values unchanged
	val, _ = ov.Get("x")
	if val != 1 {
		t.Errorf("Get(x) after update to y = %v, want 1", val)
	}
	val, _ = ov.Get("z")
	if val != 3 {
		t.Errorf("Get(z) after update to y = %v, want 3", val)
	}
}

// TestOrderedVars_InsertionOrder tests that Range preserves insertion order
func TestOrderedVars_InsertionOrder(t *testing.T) {
	ov := newOrderedVars()

	// Insert in specific order
	ov.Set("first", 1)
	ov.Set("second", 2)
	ov.Set("third", 3)

	expected := []struct {
		key   string
		value interface{}
	}{
		{"first", 1},
		{"second", 2},
		{"third", 3},
	}

	i := 0
	ov.Range(func(key string, value interface{}) {
		if i >= len(expected) {
			t.Fatalf("Range() called more than %d times", len(expected))
		}
		if key != expected[i].key {
			t.Errorf("Range()[%d] key = %q, want %q", i, key, expected[i].key)
		}
		if value != expected[i].value {
			t.Errorf("Range()[%d] value = %v, want %v", i, value, expected[i].value)
		}
		i++
	})

	if i != len(expected) {
		t.Errorf("Range() called %d times, want %d", i, len(expected))
	}
}

// TestOrderedVars_OrderPreservedAfterUpdate tests insertion order after updates
func TestOrderedVars_OrderPreservedAfterUpdate(t *testing.T) {
	ov := newOrderedVars()

	ov.Set("a", 1)
	ov.Set("b", 2)
	ov.Set("c", 3)

	// Update middle value
	ov.Set("b", 200)

	// Order should be preserved: a, b, c (not a, c, b)
	expected := []string{"a", "b", "c"}
	i := 0
	ov.Range(func(key string, value interface{}) {
		if i >= len(expected) {
			t.Fatalf("Range() called more than %d times", len(expected))
		}
		if key != expected[i] {
			t.Errorf("Range()[%d] key = %q, want %q", i, key, expected[i])
		}
		i++
	})
}

// TestOrderedVars_EmptyKey tests handling of empty string keys
func TestOrderedVars_EmptyKey(t *testing.T) {
	ov := newOrderedVars()

	// Set with empty key should be ignored
	ov.Set("", "value")

	if got := ov.Len(); got != 0 {
		t.Errorf("Len() after Set with empty key = %d, want 0", got)
	}

	val, ok := ov.Get("")
	if ok {
		t.Error("Get(\"\") = true, want false")
	}
	if val != nil {
		t.Errorf("Get(\"\") value = %v, want nil", val)
	}
}

// TestOrderedVars_NilValue tests storing nil values
func TestOrderedVars_NilValue(t *testing.T) {
	ov := newOrderedVars()

	ov.Set("null", nil)

	if got := ov.Len(); got != 1 {
		t.Errorf("Len() after Set with nil = %d, want 1", got)
	}

	val, ok := ov.Get("null")
	if !ok {
		t.Error("Get(null) = false, want true")
	}
	if val != nil {
		t.Errorf("Get(null) = %v, want nil", val)
	}
}

// TestOrderedVars_ManyVariables tests performance with many variables
func TestOrderedVars_ManyVariables(t *testing.T) {
	ov := newOrderedVars()

	// Add 100 variables
	for i := 0; i < 100; i++ {
		key := "var" + string(rune('0'+i%10))
		ov.Set(key, i)
	}

	if got := ov.Len(); got != 10 { // Only 10 unique keys (var0-var9)
		t.Errorf("Len() after 100 Sets = %d, want 10", got)
	}

	// Verify last values
	for i := 0; i < 10; i++ {
		key := "var" + string(rune('0'+i))
		val, ok := ov.Get(key)
		if !ok {
			t.Errorf("Get(%s) = false, want true", key)
		}
		// Each key was set 10 times, last value should be 90+i
		expectedVal := 90 + i
		if val != expectedVal {
			t.Errorf("Get(%s) = %v, want %v", key, val, expectedVal)
		}
	}
}

// TestOrderedVars_DifferentTypes tests storing different value types
func TestOrderedVars_DifferentTypes(t *testing.T) {
	ov := newOrderedVars()

	testCases := []struct {
		key   string
		value interface{}
	}{
		{"int", 42},
		{"string", "hello"},
		{"bool", true},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
		{"struct", struct{ X int }{X: 10}},
		{"nil", nil},
	}

	for _, tc := range testCases {
		ov.Set(tc.key, tc.value)
	}

	if got := ov.Len(); got != len(testCases) {
		t.Errorf("Len() = %d, want %d", got, len(testCases))
	}

	for _, tc := range testCases {
		val, ok := ov.Get(tc.key)
		if !ok {
			t.Errorf("Get(%s) = false, want true", tc.key)
			continue
		}
		// For nil, check explicitly
		if tc.value == nil {
			if val != nil {
				t.Errorf("Get(%s) = %#v, want nil", tc.key, val)
			}
			continue
		}
		// For non-nil, just verify it exists (avoid comparing uncomparable types)
		if val == nil {
			t.Errorf("Get(%s) = nil, want non-nil value", tc.key)
		}
	}
}

// TestOrderedVars_RangePanic tests that Range propagates panics
func TestOrderedVars_RangePanic(t *testing.T) {
	ov := newOrderedVars()
	ov.Set("a", 1)
	ov.Set("b", 2)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Range() did not propagate panic")
		}
	}()

	ov.Range(func(key string, value interface{}) {
		panic("test panic")
	})
}

// TestCreateEmptyTree tests the empty tree helper function
func TestCreateEmptyTree(t *testing.T) {
	t.Run("with statics", func(t *testing.T) {
		ctx := build.NewContext()
		tree := createEmptyTree(ctx)

		if tree == nil {
			t.Fatal("createEmptyTree returned nil")
		}

		if len(tree.Statics) != 1 {
			t.Errorf("tree.Statics length = %d, want 1", len(tree.Statics))
		}

		if len(tree.Statics) > 0 && tree.Statics[0] != "" {
			t.Errorf("tree.Statics[0] = %q, want empty string", tree.Statics[0])
		}
	})

	t.Run("without statics", func(t *testing.T) {
		ctx := build.NewUpdateContext(nil)
		tree := createEmptyTree(ctx)

		if tree == nil {
			t.Fatal("createEmptyTree returned nil")
		}

		if len(tree.Statics) != 0 {
			t.Errorf("tree.Statics length = %d, want 0", len(tree.Statics))
		}
	})
}

// TestVarContext_FieldAccess tests accessing varContext fields
func TestVarContext_FieldAccess(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	vc := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    data,
	}

	vc.vars.Set("index", 0)
	vc.vars.Set("value", "item")

	if vc.parent == nil {
		t.Error("varContext.parent is nil")
	}
	if vc.dot == nil {
		t.Error("varContext.dot is nil")
	}
	if vc.vars.Len() != 2 {
		t.Errorf("varContext.vars.Len() = %d, want 2", vc.vars.Len())
	}

	val, ok := vc.vars.Get("index")
	if !ok || val != 0 {
		t.Errorf("vars.Get(index) = (%v, %v), want (0, true)", val, ok)
	}

	val, ok = vc.vars.Get("value")
	if !ok || val != "item" {
		t.Errorf("vars.Get(value) = (%v, %v), want (item, true)", val, ok)
	}
}
