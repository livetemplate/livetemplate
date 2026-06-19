package parse

import (
	"strings"
	"testing"
)

func TestBuiltinSlice_ThreeIndex(t *testing.T) {
	s := []int{0, 1, 2, 3, 4, 5}
	got, err := builtinSlice(s, 1, 3, 5)
	if err != nil {
		t.Fatalf("builtinSlice 3-index failed: %v", err)
	}
	gs, ok := got.([]int)
	if !ok {
		t.Fatalf("expected []int, got %T", got)
	}
	// s[1:3:5] → elements {1,2}, len 2, cap 4
	if len(gs) != 2 || cap(gs) != 4 {
		t.Errorf("s[1:3:5]: got len=%d cap=%d, want len=2 cap=4", len(gs), cap(gs))
	}
	if gs[0] != 1 || gs[1] != 2 {
		t.Errorf("s[1:3:5]: got %v, want [1 2]", gs)
	}
}

func TestBuiltinSlice_ThreeIndexStringErrors(t *testing.T) {
	_, err := builtinSlice("hello", 1, 3, 4)
	if err == nil {
		t.Fatal("expected error when 3-indexing a string")
	}
	if !strings.Contains(err.Error(), "cannot 3-index slice a string") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuiltinSlice_TooManyIndices(t *testing.T) {
	if _, err := builtinSlice([]int{1, 2, 3}, 0, 1, 2, 3); err == nil {
		t.Fatal("expected error for more than 3 indices")
	}
}

// TestSliceThreeIndexString_ErrorsViaTemplate confirms the string guard is
// reachable through the real template path (Parse + BuildTree), not just a
// direct builtinSlice call.
func TestSliceThreeIndexString_ErrorsViaTemplate(t *testing.T) {
	tmpl, err := Parse("{{slice .S 1 3 4}}", nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	_, err = BuildTree(tmpl, map[string]interface{}{"S": "hello"}, &Context{IncludeStatics: true})
	if err == nil {
		t.Fatal("expected error when 3-indexing a string via template")
	}
	if !strings.Contains(err.Error(), "cannot 3-index slice a string") {
		t.Errorf("unexpected error: %v", err)
	}
}
