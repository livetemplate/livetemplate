package livetemplate

import (
	"errors"
	"testing"
)

// Test store with various action methods
type testDispatchStore struct {
	Count       int
	LastAction  string
	ShouldError bool
}

func (s *testDispatchStore) Increment(ctx *ActionContext) error {
	s.LastAction = "increment"
	s.Count++
	return nil
}

func (s *testDispatchStore) Decrement(ctx *ActionContext) error {
	s.LastAction = "decrement"
	s.Count--
	return nil
}

func (s *testDispatchStore) AddItem(ctx *ActionContext) error {
	s.LastAction = "add_item"
	s.Count++
	return nil
}

func (s *testDispatchStore) UpdateUserProfile(ctx *ActionContext) error {
	s.LastAction = "update_user_profile"
	return nil
}

func (s *testDispatchStore) ErrorMethod(ctx *ActionContext) error {
	if s.ShouldError {
		return errors.New("intentional error")
	}
	return nil
}

// Method with wrong signature (should be ignored)
func (s *testDispatchStore) WrongSignature() error {
	return nil
}

// Method with wrong return type (should be ignored)
func (s *testDispatchStore) WrongReturn(ctx *ActionContext) int {
	return 0
}

func TestDispatch_SimpleAction(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "increment"}

	err := Dispatch(store, ctx)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if store.Count != 1 {
		t.Errorf("Expected Count=1, got %d", store.Count)
	}
	if store.LastAction != "increment" {
		t.Errorf("Expected LastAction='increment', got %q", store.LastAction)
	}
}

func TestDispatch_SnakeCaseAction(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "add_item"}

	err := Dispatch(store, ctx)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if store.LastAction != "add_item" {
		t.Errorf("Expected LastAction='add_item', got %q", store.LastAction)
	}
}

func TestDispatch_CamelCaseAction(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "addItem"}

	err := Dispatch(store, ctx)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if store.LastAction != "add_item" {
		t.Errorf("Expected LastAction='add_item', got %q", store.LastAction)
	}
}

func TestDispatch_LongSnakeCaseAction(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "update_user_profile"}

	err := Dispatch(store, ctx)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if store.LastAction != "update_user_profile" {
		t.Errorf("Expected LastAction='update_user_profile', got %q", store.LastAction)
	}
}

func TestDispatch_MethodNotFound(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "nonexistent"}

	err := Dispatch(store, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound, got %v", err)
	}
}

func TestDispatch_EmptyAction(t *testing.T) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: ""}

	err := Dispatch(store, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for empty action, got %v", err)
	}
}

func TestDispatch_NilContext(t *testing.T) {
	store := &testDispatchStore{}

	err := Dispatch(store, nil)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for nil context, got %v", err)
	}
}

func TestDispatch_MethodReturnsError(t *testing.T) {
	store := &testDispatchStore{ShouldError: true}
	ctx := &ActionContext{Action: "errorMethod"}

	err := Dispatch(store, ctx)
	if err == nil {
		t.Fatal("Expected error from method")
	}
	if err.Error() != "intentional error" {
		t.Errorf("Expected 'intentional error', got %q", err.Error())
	}
}

func TestDispatch_CacheReuse(t *testing.T) {
	store1 := &testDispatchStore{}
	store2 := &testDispatchStore{}

	// First call builds cache
	ctx := &ActionContext{Action: "increment"}
	_ = Dispatch(store1, ctx)

	// Second call should use cached method lookup
	_ = Dispatch(store2, ctx)

	if store1.Count != 1 || store2.Count != 1 {
		t.Errorf("Expected both stores to have Count=1, got %d and %d", store1.Count, store2.Count)
	}
}

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
			found := false
			for _, r := range result {
				if r == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("methodNameToActions(%q)[%d] missing %q, got %v", tt.methodName, i, exp, result)
			}
		}
	}
}

// Benchmark to verify cache effectiveness
func BenchmarkDispatch_Cached(b *testing.B) {
	store := &testDispatchStore{}
	ctx := &ActionContext{Action: "increment"}

	// Warm up cache
	_ = Dispatch(store, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Dispatch(store, ctx)
	}
}

// BenchmarkDispatch_WithAllocation measures dispatch including store allocation.
// Note: The method cache is type-based, so after the first call the cache is warm.
// This benchmark measures realistic per-request overhead where a new store instance
// is created for each request but the type's methods are already cached.
func BenchmarkDispatch_WithAllocation(b *testing.B) {
	// First call warms up the cache for this type
	warmup := &testDispatchStore{}
	_ = Dispatch(warmup, &ActionContext{Action: "increment"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store := &testDispatchStore{}
		ctx := &ActionContext{Action: "increment"}
		_ = Dispatch(store, ctx)
	}
}
