package diff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// NOTE: Tests in this file use explicit data-key attributes in statics to test key-based diffing.
// For tests that verify hash-based key generation (when no key attribute is present), see:
// - TestRangeDiff_WithoutExplicitKeys
// - TestRangeDiff_RemovalWithoutExplicitKeys
// - TestRangeDiff_ReorderWithoutExplicitKeys
// - TestRangeDiff_SamePosition0DifferentItems

// TestGenerateRangeDifferentialOperations_NoChange tests that no operations are generated when items are identical.
func TestGenerateRangeDifferentialOperations_NoChange(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	item2 := &TreeNode{
		Dynamics: []interface{}{"id2", "Name 2"},
	}

	oldValue := &TreeNode{
		Statics: []string{`<li data-key="`, `">`, `</li>`},
		Range: &RangeData{
			Items: []interface{}{item1, item2},
		},
	}

	newValue := &TreeNode{
		Statics: []string{`<li data-key="`, `">`, `</li>`},
		Range: &RangeData{
			Items: []interface{}{item1, item2},
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	if len(ops) != 0 {
		t.Errorf("Expected no operations, got %d: %v", len(ops), ops)
	}
}

// TestGenerateRangeDifferentialOperations_PureReorder tests pure reordering without changes.
func TestGenerateRangeDifferentialOperations_PureReorder(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	item2 := &TreeNode{
		Dynamics: []interface{}{"id2", "Name 2"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item2},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item2, item1}, // Reordered
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation (reorder), got %d: %v", len(ops), ops)
	}

	// Check for reorder operation ["o", [keys]]
	opArray, ok := ops[0].([]interface{})
	if !ok {
		t.Fatalf("Operation should be array, got %T", ops[0])
	}

	if len(opArray) != 2 || opArray[0] != "o" {
		t.Errorf("Expected reorder operation ['o', keys], got %v", opArray)
	}

	keys, ok := opArray[1].([]string)
	if !ok {
		t.Fatalf("Keys should be []string, got %T", opArray[1])
	}

	expectedKeys := []string{"id2", "id1"}
	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Errorf("Keys = %v, want %v", keys, expectedKeys)
	}
}

// TestGenerateRangeDifferentialOperations_Removal tests item removal operations.
func TestGenerateRangeDifferentialOperations_Removal(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	item2 := &TreeNode{
		Dynamics: []interface{}{"id2", "Name 2"},
	}
	item3 := &TreeNode{
		Dynamics: []interface{}{"id3", "Name 3"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item2, item3},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item3}, // item2 removed
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one removal operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) == 2 && opArray[0] == "r" && opArray[1] == "id2" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected removal operation ['r', 'id2'], operations: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_Insertion tests item insertion (append).
func TestGenerateRangeDifferentialOperations_Insertion(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	item2 := &TreeNode{
		Dynamics: []interface{}{"id2", "Name 2"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item2}, // item2 appended
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one append operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) >= 2 && opArray[0] == "a" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected append operation ['a', ...], operations: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_Mixed verifies that a render combining
// structural changes (id1/id3 removed, id4 added) with a kept-item content change
// (id2: "Old Name" → "New Name") is encoded as differential ops: removes for the
// dropped items, a per-item ["u"] for the kept-but-changed one, and an append for
// the new one. Kept-item content changes are diffed in place (they no longer force
// a full-tree fallback).
func TestGenerateRangeDifferentialOperations_Mixed(t *testing.T) {
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2Old := &TreeNode{Dynamics: []interface{}{"id2", "Old Name"}}
	item2New := &TreeNode{Dynamics: []interface{}{"id2", "New Name"}}
	item3 := &TreeNode{Dynamics: []interface{}{"id3", "Name 3"}}
	item4 := &TreeNode{Dynamics: []interface{}{"id4", "Name 4"}}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2Old, item3}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item2New, item4}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if ops == nil {
		t.Fatal("Expected differential ops, got nil-return")
	}
	got := opTypeCounts(ops)
	if got["r"] != 2 {
		t.Errorf("Expected 2 remove ops (id1, id3), got %d: %v", got["r"], ops)
	}
	if got["a"] != 1 {
		t.Errorf("Expected 1 append op (id4), got %d: %v", got["a"], ops)
	}
	if !hasUpdateOp(ops, "id2", "New Name") {
		t.Errorf("Expected ['u','id2',{'1':'New Name'}], got: %v", ops)
	}
}

// opTypeCounts tallies operations by their op-type string ("r","u","a","i","o","p").
func opTypeCounts(ops []interface{}) map[string]int {
	counts := map[string]int{}
	for _, op := range ops {
		if arr, ok := op.([]interface{}); ok && len(arr) >= 1 {
			if t, ok := arr[0].(string); ok {
				counts[t]++
			}
		}
	}
	return counts
}

// hasUpdateOp reports whether ops contains ["u", key, {"1": value}] — position 1
// is the name/title dynamic in every test item ([id, name]).
func hasUpdateOp(ops []interface{}, key, value string) bool {
	for _, op := range ops {
		arr, ok := op.([]interface{})
		if !ok || len(arr) < 3 || arr[0] != "u" || arr[1] != key {
			continue
		}
		payload, ok := arr[2].(map[string]interface{})
		if ok && payload["1"] == value {
			return true
		}
	}
	return false
}

// TestGenerateRangeDifferentialOperations_EmptyToItems tests transition from empty to items.
func TestGenerateRangeDifferentialOperations_EmptyToItems(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{}, // Empty
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
		Metadata: &TreeMetadata{IDKey: "id"},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have append operation with statics and metadata
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d: %v", len(ops), ops)
	}

	opArray, ok := ops[0].([]interface{})
	if !ok || len(opArray) < 3 {
		t.Fatalf("Expected ['a', items, statics, ...], got %v", ops[0])
	}

	if opArray[0] != "a" {
		t.Errorf("Expected 'a' operation, got %v", opArray[0])
	}

	// Check statics are included
	if opArray[2] == nil {
		t.Error("Statics should be included in empty-to-items transition")
	}
}

// TestGenerateRangeDifferentialOperations_ItemsToEmpty tests transition from items to empty.
func TestGenerateRangeDifferentialOperations_ItemsToEmpty(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{}, // Empty
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one removal operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) == 2 && opArray[0] == "r" && opArray[1] == "id1" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected removal operation ['r', 'id1'], operations: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_StripStatics tests static stripping when stripStatics=true.
func TestGenerateRangeDifferentialOperations_StripStatics(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	item2 := &TreeNode{
		Dynamics: []interface{}{"id2", "Name 2"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item2}, // item2 appended
		},
	}

	// With stripStatics=false
	opsWithStatics := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should contain statics when stripStatics=false
	foundStatics := false
	for _, op := range opsWithStatics {
		opArray, ok := op.([]interface{})
		if !ok {
			continue
		}
		// Check if operation is an append/prepend/insert with statics
		if len(opArray) >= 3 {
			switch opArray[0] {
			case "a", "p":
				// Third element should be statics array for append/prepend
				if reflect.DeepEqual(opArray[2], statics) {
					foundStatics = true
				}
			}
		}
	}

	if !foundStatics {
		t.Error("Expected operations to contain statics when stripStatics=false")
	}

	// With stripStatics=true - just verify it doesn't panic/error
	// The actual stripping is handled by PrepareTreeForClient which is tested separately
	opsStripped := GenerateRangeDifferentialOperations(oldValue, newValue, true)
	if opsStripped == nil {
		t.Error("Expected non-nil operations even with stripStatics=true")
	}
}

// TestExtractRangeData_TreeNode tests extracting range data from TreeNode.
func TestExtractRangeData_TreeNode(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: []interface{}{"id1"},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
		Metadata: &TreeMetadata{IDKey: "id"},
	}

	oldItems, newItems, extractedStatics, metadata := extractRangeData(oldValue, newValue)

	if oldItems == nil || newItems == nil {
		t.Fatal("extractRangeData should return non-nil items")
	}

	if len(oldItems) != 0 {
		t.Errorf("oldItems length = %d, want 0", len(oldItems))
	}

	if len(newItems) != 1 {
		t.Errorf("newItems length = %d, want 1", len(newItems))
	}

	// For empty-to-items transition, statics should come from newValue
	if extractedStatics == nil {
		t.Error("Statics should not be nil for empty-to-items transition")
	}

	if metadata == nil {
		t.Error("Metadata should not be nil")
	} else if metadata["idKey"] != "id" {
		t.Errorf("metadata['idKey'] = %v, want 'id'", metadata["idKey"])
	}
}

// TestExtractRangeData_EmptyToItems tests statics handling in empty-to-items transition.
func TestExtractRangeData_EmptyToItems(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: []string{}, // Empty statics
		Range: &RangeData{
			Items: []interface{}{},
		},
	}

	newValue := &TreeNode{
		Statics: statics, // New statics for items
		Range: &RangeData{
			Items: []interface{}{
				&TreeNode{Dynamics: []interface{}{"id1"}},
			},
		},
	}

	_, _, extractedStatics, _ := extractRangeData(oldValue, newValue)

	// Should use newValue's statics for empty-to-items transition
	if !reflect.DeepEqual(extractedStatics, statics) {
		t.Errorf("Extracted statics = %v, want %v", extractedStatics, statics)
	}
}

// TestGenerateRemovalOperations tests removal operation generation.
func TestGenerateRemovalOperations(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id2"}},
		&TreeNode{Dynamics: []interface{}{"id3"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id3"}},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}
	operations := []interface{}{}

	ctx := newRangeContext(oldItems, newItems, statics, nil)
	ops := generateRemovalOps(ctx, operations)

	// Should generate one removal for id2
	if len(ops) != 1 {
		t.Fatalf("Expected 1 removal operation, got %d: %v", len(ops), ops)
	}

	opArray, ok := ops[0].([]interface{})
	if !ok || len(opArray) != 2 {
		t.Fatalf("Expected ['r', 'id2'], got %v", ops[0])
	}

	if opArray[0] != "r" || opArray[1] != "id2" {
		t.Errorf("Operation = %v, want ['r', 'id2']", opArray)
	}
}

// TestGenerateInsertionOperations_Prepend tests prepend insertion.
func TestGenerateInsertionOperations_Prepend(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id2"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id2"}},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}
	operations := []interface{}{}

	ops := generateInsertionOps(newRangeContext(oldItems, newItems, statics, nil), operations)

	// Should generate prepend operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) >= 2 && opArray[0] == "p" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected prepend operation ['p', ...], got: %v", ops)
	}
}

// TestGenerateInsertionOperations_Append tests append insertion.
func TestGenerateInsertionOperations_Append(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id2"}},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}
	operations := []interface{}{}

	ops := generateInsertionOps(newRangeContext(oldItems, newItems, statics, nil), operations)

	// Should generate append operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) >= 2 && opArray[0] == "a" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected append operation ['a', ...], got: %v", ops)
	}
}

// TestGenerateInsertionOperations_Complex tests complex insertion pattern.
func TestGenerateInsertionOperations_Complex(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id3"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1"}},
		&TreeNode{Dynamics: []interface{}{"id2"}}, // Inserted in middle
		&TreeNode{Dynamics: []interface{}{"id3"}},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}
	operations := []interface{}{}

	ops := generateInsertionOps(newRangeContext(oldItems, newItems, statics, nil), operations)

	// Should generate individual insert operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) >= 3 && opArray[0] == "i" && opArray[1] == "id1" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected insert operation ['i', 'id1', ...], got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_NilInputs tests handling of nil inputs.
func TestGenerateRangeDifferentialOperations_NilInputs(t *testing.T) {
	tests := []struct {
		name     string
		oldValue interface{}
		newValue interface{}
	}{
		{"both nil", nil, nil},
		{"old nil", nil, &TreeNode{Range: &RangeData{Items: []interface{}{}}}},
		{"new nil", &TreeNode{Range: &RangeData{Items: []interface{}{}}}, nil},
		{"non-TreeNode old", "not a tree", &TreeNode{Range: &RangeData{Items: []interface{}{}}}},
		{"non-TreeNode new", &TreeNode{Range: &RangeData{Items: []interface{}{}}}, "not a tree"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := GenerateRangeDifferentialOperations(tt.oldValue, tt.newValue, false)
			if len(ops) != 0 {
				t.Errorf("Expected empty operations for invalid inputs, got %d: %v", len(ops), ops)
			}
		})
	}
}

// TestGenerateRangeDifferentialOperations_NoRangeField tests TreeNode without Range field.
func TestGenerateRangeDifferentialOperations_NoRangeField(t *testing.T) {
	oldValue := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		// No Range field
	}

	newValue := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		// No Range field
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	if len(ops) != 0 {
		t.Errorf("Expected no operations for TreeNodes without Range, got %d: %v", len(ops), ops)
	}
}

// TestGenerateRangeDifferentialOperations_NilRangeData tests TreeNode with nil Range.
func TestGenerateRangeDifferentialOperations_NilRangeData(t *testing.T) {
	oldValue := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range:   nil,
	}

	newValue := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range:   nil,
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	if len(ops) != 0 {
		t.Errorf("Expected no operations for nil Range, got %d: %v", len(ops), ops)
	}
}

// TestHasKeptItemChanged_NonTreeNodeReturnsTrue locks in the safety bias: if a
// kept-key resolves to a non-*TreeNode on either side, the function returns
// true (force fallback) rather than silently emit a no-op for unrenderable
// state. The path is unreachable through GenerateRangeDifferentialOperations
// today because key extraction itself requires *TreeNode, but the bias remains
// load-bearing for any future caller that constructs a context manually.
func TestHasKeptItemChanged_NonTreeNodeReturnsTrue(t *testing.T) {
	ctx := &rangeContext{
		newKeys:  []string{"id1"},
		oldByKey: map[string]interface{}{"id1": "raw-string"},
		newByKey: map[string]interface{}{"id1": &TreeNode{Dynamics: []interface{}{"id1", "x"}}},
	}
	if !hasKeptItemChanged(ctx) {
		t.Error("Expected true for non-*TreeNode old value (safety bias), got false")
	}
}

// TestGenerateRangeDifferentialOperations_KeptItemContentChange_EmitsUpdateOp
// covers a single item present on both renders whose Dynamics changed. The diff
// engine encodes it in place as ["u", key, {changed dynamics}] rather than
// falling back to a full-tree replacement.
func TestGenerateRangeDifferentialOperations_KeptItemContentChange_EmitsUpdateOp(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{&TreeNode{Dynamics: []interface{}{"id1", "Old"}}}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{&TreeNode{Dynamics: []interface{}{"id1", "New"}}}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if len(ops) != 1 || !hasUpdateOp(ops, "id1", "New") {
		t.Errorf("Expected [['u','id1',{'1':'New'}]], got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_StructuralAndContent_EmitsOps covers a
// render that adds an item AND mutates an existing item's content: both are
// encoded differentially — a per-item ["u"] for the kept item plus an append for
// the new one.
func TestGenerateRangeDifferentialOperations_StructuralAndContent_EmitsOps(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{&TreeNode{Dynamics: []interface{}{"id1", "Old"}}}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id1", "New"}},
			&TreeNode{Dynamics: []interface{}{"id2", "Fresh"}},
		}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if !hasUpdateOp(ops, "id1", "New") {
		t.Errorf("Expected a ['u','id1',{'1':'New'}] op, got: %v", ops)
	}
	if opTypeCounts(ops)["a"] != 1 {
		t.Errorf("Expected 1 append op (id2), got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_NestedRangeKeptItem_EmitsNestedUpdate is
// the headline nested case (the flat-scalar tests above don't exercise it): an
// outer range item that itself contains a nested range, where one nested leaf's
// content changes. The outer range must emit a single ["u", outerKey, payload]
// whose payload carries a nested ["u", innerKey, …] scoped to the changed leaf —
// not re-send the whole outer item or its unchanged nested sibling.
func TestGenerateRangeDifferentialOperations_NestedRangeKeptItem_EmitsNestedUpdate(t *testing.T) {
	inner := []string{`<li data-key="`, `">`, `</li>`}
	outer := []string{`<div data-key="`, `">`, `</div>`}
	mkItem := func(innerTitle string) *TreeNode {
		return &TreeNode{
			Statics: outer,
			Dynamics: []interface{}{"cat1", &TreeNode{
				Statics: inner,
				Range: &RangeData{
					Statics: inner,
					Items: []interface{}{
						&TreeNode{Statics: inner, Dynamics: []interface{}{"i1", "A"}},
						&TreeNode{Statics: inner, Dynamics: []interface{}{"i2", innerTitle}},
					},
				},
			}},
		}
	}
	oldValue := &TreeNode{Statics: outer, Range: &RangeData{Statics: outer, Items: []interface{}{mkItem("B")}}}
	newValue := &TreeNode{Statics: outer, Range: &RangeData{Statics: outer, Items: []interface{}{mkItem("RENAMED")}}}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, true)
	if len(ops) != 1 {
		t.Fatalf("expected a single outer ['u'] op, got: %v", ops)
	}
	op, _ := ops[0].([]interface{})
	if len(op) < 3 || op[0] != "u" || op[1] != "cat1" {
		t.Fatalf("expected ['u','cat1',payload], got: %v", ops[0])
	}
	js, _ := json.Marshal(op[2])
	s := string(js)
	if !strings.Contains(s, "RENAMED") {
		t.Errorf("nested ['u'] payload must carry the changed leaf 'RENAMED', got: %s", s)
	}
	if strings.Contains(s, `"s":[`) {
		t.Errorf("nested ['u'] payload must be statics-free (client has them cached), got: %s", s)
	}
	if !strings.Contains(s, `"u"`) || !strings.Contains(s, "i2") {
		t.Errorf("outer payload must carry a nested ['u','i2',…] op, got: %s", s)
	}
}

// TestGenerateRangeDifferentialOperations_ItemStructureChange_FallsBack covers a
// kept item whose STATICS changed (a conditional branch flipped in/out) — the
// per-item ["u"] payload would need statics the client lacks, which an update op
// must not carry, so the range falls back to a full-tree replacement.
func TestGenerateRangeDifferentialOperations_ItemStructureChange_FallsBack(t *testing.T) {
	oldValue := &TreeNode{
		Statics: []string{`<li data-key="`, `">`, `</li>`},
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Statics: []string{`<li data-key="`, `">`, `</li>`}, Dynamics: []interface{}{"id1", "A"}},
		}},
	}
	newValue := &TreeNode{
		Statics: []string{`<li data-key="`, `">`, `</li>`},
		Range: &RangeData{Items: []interface{}{
			// Same key, but the item's statics changed (extra slot / different shape).
			&TreeNode{Statics: []string{`<li data-key="`, `"><b>`, `</b></li>`}, Dynamics: []interface{}{"id1", "A"}},
		}},
	}

	if ops := GenerateRangeDifferentialOperations(oldValue, newValue, false); ops != nil {
		t.Errorf("Expected nil-return (full-tree fallback) for item structure change, got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_PureStructural_StillEmitsOps locks in
// the wire-size win for het ranges that change only structurally — pure-add of
// new items with no kept-item content drift must still emit a compact ['a']/['i']
// op carrying the new key.
func TestGenerateRangeDifferentialOperations_PureStructural_StillEmitsOps(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{&TreeNode{Dynamics: []interface{}{"id1", "A"}}}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id1", "A"}},
			&TreeNode{Dynamics: []interface{}{"id2", "B"}},
		}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if ops == nil {
		t.Fatal("Expected ops for pure-structural change, got nil-fallback")
	}

	foundID2 := false
	for _, op := range ops {
		opArr, ok := op.([]interface{})
		if !ok || len(opArr) < 2 {
			continue
		}
		switch opArr[0] {
		case "a", "p":
			items, ok := opArr[1].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				if node, ok := item.(*TreeNode); ok && len(node.Dynamics) > 0 && node.Dynamics[0] == "id2" {
					foundID2 = true
				}
			}
		case "i":
			if len(opArr) >= 3 {
				if node, ok := opArr[2].(*TreeNode); ok && len(node.Dynamics) > 0 && node.Dynamics[0] == "id2" {
					foundID2 = true
				}
			}
		}
	}
	if !foundID2 {
		t.Errorf("Expected an append/insert op carrying id2, got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_StaticsOnlyChange_FallsBack covers the
// conditional-branch flip pattern: a kept item's nested TreeNode flips Statics
// (e.g. {{if .Done}}<s>{{end}}) with no Dynamics change. The hash check alone
// would miss this (empty Dynamics → identical hash) — the statics fingerprint
// comparison catches it and forces full-tree fallback.
func TestGenerateRangeDifferentialOperations_StaticsOnlyChange_FallsBack(t *testing.T) {
	itemStatics := []string{`<li data-key="`, `">`, "</li>"}
	rangeStatics := []string{"<ul>", "</ul>"}
	oldValue := &TreeNode{
		Statics: rangeStatics,
		Range: &RangeData{
			Statics: itemStatics,
			Items: []interface{}{&TreeNode{
				Statics:  itemStatics,
				Dynamics: []interface{}{"task-1", &TreeNode{Statics: []string{"<s>", "</s>"}}},
			}},
		},
	}
	newValue := &TreeNode{
		Statics: rangeStatics,
		Range: &RangeData{
			Statics: itemStatics,
			Items: []interface{}{&TreeNode{
				Statics:  itemStatics,
				Dynamics: []interface{}{"task-1", &TreeNode{Statics: []string{""}}},
			}},
		},
	}

	if ops := GenerateRangeDifferentialOperations(oldValue, newValue, false); ops != nil {
		t.Errorf("Expected nil-return for statics-only kept-item change, got: %v", ops)
	}
}

// =============================================================================
// AUTO-KEY GENERATION INTEGRATION TESTS
// These tests verify that range diffing works correctly when templates don't
// have explicit key attributes (data-key, id, etc.).
// =============================================================================

// TestRangeDiff_WithoutExplicitKeys verifies that range diffing works correctly
// when no key attribute is present in the template statics.
func TestRangeDiff_WithoutExplicitKeys(t *testing.T) {
	// Statics WITHOUT key attribute - simulates {{range .Items}}<li>{{.}}</li>{{end}}
	statics := []string{"<li>", "</li>"}

	item1 := &TreeNode{Dynamics: []interface{}{"First"}}
	item2 := &TreeNode{Dynamics: []interface{}{"Second"}}
	item3 := &TreeNode{Dynamics: []interface{}{"Third"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2, item3}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one operation (append)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation (append), got %d: %v", len(ops), ops)
	}

	opArray, ok := ops[0].([]interface{})
	if !ok {
		t.Fatalf("Operation should be array, got %T", ops[0])
	}

	// Should be append operation
	if opArray[0] != "a" {
		t.Errorf("Expected append 'a', got %v", opArray[0])
	}
}

// TestRangeDiff_RemovalWithoutExplicitKeys verifies removal works with hash keys.
func TestRangeDiff_RemovalWithoutExplicitKeys(t *testing.T) {
	// Statics WITHOUT key attribute
	statics := []string{"<li>", "</li>"}

	item1 := &TreeNode{Dynamics: []interface{}{"First"}}
	item2 := &TreeNode{Dynamics: []interface{}{"Second"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1}}, // item2 removed
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one remove operation
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation (remove), got %d: %v", len(ops), ops)
	}

	opArray := ops[0].([]interface{})
	if opArray[0] != "r" {
		t.Errorf("Expected remove 'r', got %v", opArray[0])
	}

	// The key should be a hash (12 hex chars), not "0" or the content
	removedKey := opArray[1].(string)
	if len(removedKey) != 12 {
		t.Errorf("Expected 12-char hash key, got %d chars: %s", len(removedKey), removedKey)
	}
}

// TestRangeDiff_UpdateWithoutExplicitKeys verifies updates work with hash keys.
func TestRangeDiff_UpdateWithoutExplicitKeys(t *testing.T) {
	// Statics WITHOUT key attribute
	statics := []string{`<div class="`, `">`, `</div>`}

	item1Old := &TreeNode{Dynamics: []interface{}{"active", "Original content"}}
	item1New := &TreeNode{Dynamics: []interface{}{"active", "Updated content"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1Old}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1New}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Since the hash is based on ALL content, changing content changes the hash.
	// This means we'll get a remove + insert/prepend, not an update.
	// This is expected behavior for content-based hashing.
	if len(ops) == 0 {
		t.Fatal("Expected operations for content change, got none")
	}

	// Verify operations are valid
	for _, op := range ops {
		opArray := op.([]interface{})
		opType := opArray[0].(string)
		if opType != "r" && opType != "p" && opType != "i" && opType != "a" && opType != "u" {
			t.Errorf("Unexpected operation type: %s", opType)
		}
	}
}

// TestRangeDiff_ReorderWithoutExplicitKeys verifies reordering works with hash keys.
func TestRangeDiff_ReorderWithoutExplicitKeys(t *testing.T) {
	// Statics WITHOUT key attribute
	statics := []string{"<li>", "</li>"}

	item1 := &TreeNode{Dynamics: []interface{}{"First"}}
	item2 := &TreeNode{Dynamics: []interface{}{"Second"}}
	item3 := &TreeNode{Dynamics: []interface{}{"Third"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2, item3}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item3, item1, item2}}, // Reordered
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one reorder operation
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation (reorder), got %d: %v", len(ops), ops)
	}

	opArray := ops[0].([]interface{})
	if opArray[0] != "o" {
		t.Errorf("Expected order 'o', got %v", opArray[0])
	}

	// Keys should all be 12-char hashes
	order := opArray[1].([]string)
	for i, key := range order {
		if len(key) != 12 {
			t.Errorf("Key %d should be 12-char hash, got %d: %s", i, len(key), key)
		}
	}
}

// TestRangeDiff_SamePosition0DifferentItems verifies that items with the same
// value at position 0 are correctly differentiated using full content hash.
// This was the core bug before the fix.
func TestRangeDiff_SamePosition0DifferentItems(t *testing.T) {
	// Statics WITHOUT key attribute - simulates CSS class at position 0
	statics := []string{`<div class="`, `">`, `</div>`}

	// Two items with same CSS class but different names
	alice := &TreeNode{Dynamics: []interface{}{"active", "Alice"}}
	bob := &TreeNode{Dynamics: []interface{}{"active", "Bob"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{alice, bob}},
	}

	// Same items, just reordered
	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{bob, alice}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should be a pure reorder operation
	if len(ops) != 1 {
		t.Fatalf("Expected 1 reorder operation, got %d: %v", len(ops), ops)
	}

	opArray := ops[0].([]interface{})
	if opArray[0] != "o" {
		t.Errorf("Expected order 'o' (reorder), got %v. Items were incorrectly treated as different.", opArray[0])
	}

	order := opArray[1].([]string)
	if len(order) != 2 {
		t.Errorf("Expected 2 items in order, got %d", len(order))
	}

	// Keys should be unique (different hashes for Alice and Bob)
	if order[0] == order[1] {
		t.Errorf("BUG: Alice and Bob got same key: %s. Fix for same-position-0 bug failed!", order[0])
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// Tests for HasReordering and sameKeySet helper functions.
// =============================================================================

// TestHasReordering tests the HasReordering helper function.
func TestHasReordering(t *testing.T) {
	tests := []struct {
		name     string
		oldKeys  []string
		newKeys  []string
		expected bool
	}{
		{
			name:     "same order",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "different order",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"b", "a", "c"},
			expected: true,
		},
		{
			name:     "completely reversed",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"c", "b", "a"},
			expected: true,
		},
		{
			name:     "different lengths - fewer",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"a", "b"},
			expected: false,
		},
		{
			name:     "different lengths - more",
			oldKeys:  []string{"a", "b"},
			newKeys:  []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "empty slices",
			oldKeys:  []string{},
			newKeys:  []string{},
			expected: false,
		},
		{
			name:     "single item same",
			oldKeys:  []string{"a"},
			newKeys:  []string{"a"},
			expected: false,
		},
		{
			name:     "two items swapped",
			oldKeys:  []string{"a", "b"},
			newKeys:  []string{"b", "a"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasReordering(tt.oldKeys, tt.newKeys)
			if result != tt.expected {
				t.Errorf("HasReordering(%v, %v) = %v, want %v", tt.oldKeys, tt.newKeys, result, tt.expected)
			}
		})
	}
}

// TestSameKeySet tests the sameKeySet helper function.
func TestSameKeySet(t *testing.T) {
	tests := []struct {
		name     string
		oldKeys  []string
		newKeys  []string
		expected bool
	}{
		{
			name:     "same set same order",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "same set different order",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "different sets - one different",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "different lengths",
			oldKeys:  []string{"a", "b", "c"},
			newKeys:  []string{"a", "b"},
			expected: false,
		},
		{
			name:     "empty slices",
			oldKeys:  []string{},
			newKeys:  []string{},
			expected: true,
		},
		{
			name:     "single item same",
			oldKeys:  []string{"a"},
			newKeys:  []string{"a"},
			expected: true,
		},
		{
			name:     "single item different",
			oldKeys:  []string{"a"},
			newKeys:  []string{"b"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sameKeySet(tt.oldKeys, tt.newKeys)
			if result != tt.expected {
				t.Errorf("sameKeySet(%v, %v) = %v, want %v", tt.oldKeys, tt.newKeys, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// UPDATE + REORDER COMBINED TESTS
// Tests for the bug fix where items both update AND reorder simultaneously.
// This was the core bug discovered during Phase 2 fuzz testing.
// =============================================================================

// TestGenerateRangeDifferentialOperations_UpdateAndReorder verifies that a
// combined content change + reorder is encoded differentially: a per-item ["u"]
// for the changed item plus a trailing ["o"] carrying the new key order. (Earlier
// this took the full-tree fallback; the per-item ["u"] producer is back.)
func TestGenerateRangeDifferentialOperations_UpdateAndReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id1", "Alpha"}},
			&TreeNode{Dynamics: []interface{}{"id2", "Beta"}},
			&TreeNode{Dynamics: []interface{}{"id3", "Charlie"}},
		}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id2", "Beta"}},
			&TreeNode{Dynamics: []interface{}{"id1", "Alpha Updated"}},
			&TreeNode{Dynamics: []interface{}{"id3", "Charlie"}},
		}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if !hasUpdateOp(ops, "id1", "Alpha Updated") {
		t.Errorf("Expected a ['u','id1',{'1':'Alpha Updated'}] op, got: %v", ops)
	}
	if !hasReorderOp(ops, []string{"id2", "id1", "id3"}) {
		t.Errorf("Expected a trailing ['o',['id2','id1','id3']] op, got: %v", ops)
	}
}

// hasReorderOp reports whether ops contains ["o", wantKeys].
func hasReorderOp(ops []interface{}, wantKeys []string) bool {
	for _, op := range ops {
		arr, ok := op.([]interface{})
		if !ok || len(arr) < 2 || arr[0] != "o" {
			continue
		}
		gotKeys, ok := arr[1].([]string)
		if !ok || len(gotKeys) != len(wantKeys) {
			continue
		}
		match := true
		for i := range wantKeys {
			if gotKeys[i] != wantKeys[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestGenerateRangeDifferentialOperations_MultipleUpdatesAndReorder same shape as
// _UpdateAndReorder but with multiple kept-item content changes — each emits its
// own ["u"] alongside the trailing ["o"].
func TestGenerateRangeDifferentialOperations_MultipleUpdatesAndReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id1", "One"}},
			&TreeNode{Dynamics: []interface{}{"id2", "Two"}},
			&TreeNode{Dynamics: []interface{}{"id3", "Three"}},
		}},
	}
	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{Items: []interface{}{
			&TreeNode{Dynamics: []interface{}{"id3", "Three"}},
			&TreeNode{Dynamics: []interface{}{"id2", "Two Updated"}},
			&TreeNode{Dynamics: []interface{}{"id1", "One Updated"}},
		}},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if !hasUpdateOp(ops, "id1", "One Updated") || !hasUpdateOp(ops, "id2", "Two Updated") {
		t.Errorf("Expected ['u'] ops for id1 and id2, got: %v", ops)
	}
	if !hasReorderOp(ops, []string{"id3", "id2", "id1"}) {
		t.Errorf("Expected a trailing ['o',['id3','id2','id1']] op, got: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_RemovalNoReorder tests that no spurious
// reorder is generated when items are removed (key sets differ).
func TestGenerateRangeDifferentialOperations_RemovalNoReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1 := &TreeNode{Dynamics: []interface{}{"id1", "One"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Two"}}
	item3 := &TreeNode{Dynamics: []interface{}{"id3", "Three"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2, item3}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item3, item1}}, // id2 removed, order changed
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have removal, but NO reorder (key sets differ)
	hasRemoval := false
	hasReorder := false

	for _, op := range ops {
		opArray := op.([]interface{})
		switch opArray[0] {
		case "r":
			hasRemoval = true
		case "o":
			hasReorder = true
		}
	}

	if !hasRemoval {
		t.Error("Expected removal operation")
	}
	if hasReorder {
		t.Error("Should NOT have reorder when key sets differ (items were removed)")
	}
}

// TestGenerateRangeDifferentialOperations_InsertionNoReorder tests that no spurious
// reorder is generated when items are inserted (key sets differ).
func TestGenerateRangeDifferentialOperations_InsertionNoReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1 := &TreeNode{Dynamics: []interface{}{"id1", "One"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Two"}}
	item3 := &TreeNode{Dynamics: []interface{}{"id3", "Three"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1, item2}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item2, item3, item1}}, // id3 added, order changed
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have insertion, but NO reorder (key sets differ)
	hasInsertion := false
	hasReorder := false

	for _, op := range ops {
		opArray := op.([]interface{})
		switch opArray[0] {
		case "a", "p", "i":
			hasInsertion = true
		case "o":
			hasReorder = true
		}
	}

	if !hasInsertion {
		t.Error("Expected insertion operation")
	}
	if hasReorder {
		t.Error("Should NOT have reorder when key sets differ (items were added)")
	}
}

// streamStateFor builds a RangeStreamState snapshot from a slice of items, as
// the Phase 3 producer will do. ASSUMES homogeneous items: the helper takes
// the first item's statics fingerprint as canonical and never re-checks. Tests
// that need heterogeneous setups must build the streamState manually. Keys
// are read from dynamic position 0, matching how all current callers use it.
func streamStateFor(items []*TreeNode) *RangeStreamState {
	state := &RangeStreamState{
		Keys:   make([]string, len(items)),
		Hashes: make([]uint64, len(items)),
	}
	if len(items) > 0 {
		state.Fingerprint = build.CalculateStaticsFingerprint(items[0])
	}
	for i, item := range items {
		if len(item.Dynamics) > 0 {
			if keyStr, ok := item.Dynamics[0].(string); ok {
				state.Keys[i] = keyStr
			}
		}
		state.Hashes[i] = keys.ItemHashUint64(item.Dynamics)
	}
	return state
}

func TestGenerateRangeStreamOperations_NoChange(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}

	streamState := streamStateFor([]*TreeNode{item1, item2})
	newItems := []interface{}{item1, item2}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 0 {
		t.Errorf("Expected no operations, got %d: %v", len(ops), ops)
	}
}

func TestGenerateRangeStreamOperations_Update_FullDynamicsPayload(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldItem := &TreeNode{Dynamics: []interface{}{"id1", "Old Name"}}
	newItem := &TreeNode{Dynamics: []interface{}{"id1", "New Name"}}

	streamState := streamStateFor([]*TreeNode{oldItem})
	newItems := []interface{}{newItem}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 update op, got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "u" || opArr[1] != "id1" {
		t.Fatalf("Expected ['u', 'id1', payload], got %v", opArr)
	}
	payload, ok := opArr[2].(map[string]interface{})
	if !ok {
		t.Fatalf("Payload should be map[string]interface{}, got %T", opArr[2])
	}
	if payload["1"] != "New Name" {
		t.Errorf("payload['1'] = %v, want 'New Name'", payload["1"])
	}
	// Key position must be excluded from payload (key is in op[1]).
	if _, hasKey := payload["0"]; hasKey {
		t.Errorf("Key position 0 should not appear in payload, got %v", payload)
	}
}

func TestGenerateRangeStreamOperations_UpdateClearsAbsentPositions(t *testing.T) {
	// 3 dynamic positions: key, name, status.
	statics := []string{`<li data-key="`, `">`, `:`, `</li>`}
	oldItem := &TreeNode{Dynamics: []interface{}{"id1", "Name", "active"}}
	// Position 2 (status) clears to nil — wire payload must encode "" per §5c.
	newItem := &TreeNode{Dynamics: []interface{}{"id1", "Name", nil}}

	streamState := streamStateFor([]*TreeNode{oldItem})
	newItems := []interface{}{newItem}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 update op, got %d: %v", len(ops), ops)
	}
	payload := ops[0].([]interface{})[2].(map[string]interface{})
	if payload["2"] != "" {
		t.Errorf("Cleared position should encode as '', got %v (type %T)", payload["2"], payload["2"])
	}
}

func TestGenerateRangeStreamOperations_NilToEmptyStringPhantomUpdate(t *testing.T) {
	// Test plan item 11: nil→"" hashes-mismatch (nil is skipped; "" hashes as `1:""`),
	// so an update IS emitted even though the rendered output is identical.
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldItem := &TreeNode{Dynamics: []interface{}{"id1", nil}}
	newItem := &TreeNode{Dynamics: []interface{}{"id1", ""}}

	streamState := streamStateFor([]*TreeNode{oldItem})
	newItems := []interface{}{newItem}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 update op (nil→\"\" is a hash mismatch), got %d: %v", len(ops), ops)
	}
	payload := ops[0].([]interface{})[2].(map[string]interface{})
	if payload["1"] != "" {
		t.Errorf("payload['1'] = %v, want \"\"", payload["1"])
	}
}

func TestGenerateRangeStreamOperations_Removal(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}
	item3 := &TreeNode{Dynamics: []interface{}{"id3", "Name 3"}}

	streamState := streamStateFor([]*TreeNode{item1, item2, item3})
	newItems := []interface{}{item1, item3} // item2 removed

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	found := false
	for _, op := range ops {
		opArr, ok := op.([]interface{})
		if ok && len(opArr) == 2 && opArr[0] == "r" && opArr[1] == "id2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected removal op ['r', 'id2'], ops: %v", ops)
	}
}

func TestGenerateRangeStreamOperations_TailAppend4Items(t *testing.T) {
	// 5 → 9 items via 4 tail appends; expect SINGLE ['a', 4items, statics] op,
	// NOT 4 individual ['i'] ops, NOT a full-tree fallback.
	// Regression for areAllItemsAtEndCtx via oldKeySet (proposal §11 test plan).
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldItems := make([]*TreeNode, 5)
	for i := 0; i < 5; i++ {
		oldItems[i] = &TreeNode{Dynamics: []interface{}{
			fmt.Sprintf("id%d", i),
			fmt.Sprintf("Name %d", i),
		}}
	}
	streamState := streamStateFor(oldItems)

	newItems := make([]interface{}, 9)
	for i := 0; i < 5; i++ {
		newItems[i] = oldItems[i]
	}
	for i := 5; i < 9; i++ {
		newItems[i] = &TreeNode{Dynamics: []interface{}{
			fmt.Sprintf("id%d", i),
			fmt.Sprintf("Name %d", i),
		}}
	}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected SINGLE 'a' op for tail-append, got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "a" {
		t.Fatalf("Expected ['a', items, statics], got %v", opArr)
	}
	items := opArr[1].([]interface{})
	if len(items) != 4 {
		t.Errorf("Expected 4 appended items, got %d", len(items))
	}
}

func TestGenerateRangeStreamOperations_HeadPrepend(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}
	item0 := &TreeNode{Dynamics: []interface{}{"id0", "Name 0"}}

	streamState := streamStateFor([]*TreeNode{item1, item2})
	newItems := []interface{}{item0, item1, item2}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected SINGLE 'p' op for head-prepend, got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "p" {
		t.Errorf("Expected ['p', ...], got %v", opArr)
	}
}

func TestGenerateRangeStreamOperations_MidInsert(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}
	itemMid := &TreeNode{Dynamics: []interface{}{"idMid", "Mid"}}

	streamState := streamStateFor([]*TreeNode{item1, item2})
	newItems := []interface{}{item1, itemMid, item2}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	found := false
	for _, op := range ops {
		opArr, ok := op.([]interface{})
		if ok && len(opArr) >= 2 && opArr[0] == "i" && opArr[1] == "id1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected ['i', 'id1', item], ops: %v", ops)
	}
}

func TestGenerateRangeStreamOperations_PureReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}

	streamState := streamStateFor([]*TreeNode{item1, item2})
	newItems := []interface{}{item2, item1} // reordered, identical content

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) != 1 {
		t.Fatalf("Expected SINGLE 'o' op (pure-reorder fast path), got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "o" {
		t.Errorf("Expected ['o', keys], got %v", opArr)
	}
	gotKeys := opArr[1].([]string)
	if !reflect.DeepEqual(gotKeys, []string{"id2", "id1"}) {
		t.Errorf("Keys = %v, want [id2 id1]", gotKeys)
	}
}

func TestGenerateRangeStreamOperations_MixedUpdateAndReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1Old := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2Old := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}
	item1New := &TreeNode{Dynamics: []interface{}{"id1", "Name 1 UPDATED"}}

	streamState := streamStateFor([]*TreeNode{item1Old, item2Old})
	newItems := []interface{}{item2Old, item1New} // reordered AND id1 content changed

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if len(ops) < 2 {
		t.Fatalf("Expected at least 2 ops (update + reorder), got %d: %v", len(ops), ops)
	}

	uIdx, oIdx := -1, -1
	for i, op := range ops {
		opArr := op.([]interface{})
		switch opArr[0] {
		case "u":
			uIdx = i
		case "o":
			oIdx = i
		}
	}
	if uIdx == -1 || oIdx == -1 {
		t.Fatalf("Expected both 'u' and 'o' ops, got %v", ops)
	}
	if uIdx >= oIdx {
		t.Errorf("Per §5b ordering, 'u' must precede 'o'; got u@%d, o@%d", uIdx, oIdx)
	}
}

func TestGenerateRangeStreamOperations_ScatteredInsertFallback(t *testing.T) {
	// ≥4 distinct insertion points triggers complex-pattern fallback (returns nil).
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldItems := []*TreeNode{
		{Dynamics: []interface{}{"a", "A"}},
		{Dynamics: []interface{}{"b", "B"}},
		{Dynamics: []interface{}{"c", "C"}},
		{Dynamics: []interface{}{"d", "D"}},
	}
	streamState := streamStateFor(oldItems)

	// Insert N1 before a, N2 between a&b, N3 between b&c, N4 between c&d (4 unique points).
	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"n1", "New 1"}},
		oldItems[0],
		&TreeNode{Dynamics: []interface{}{"n2", "New 2"}},
		oldItems[1],
		&TreeNode{Dynamics: []interface{}{"n3", "New 3"}},
		oldItems[2],
		&TreeNode{Dynamics: []interface{}{"n4", "New 4"}},
		oldItems[3],
	}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if ops != nil {
		t.Errorf("Expected nil (complex-insertion fallback), got: %v", ops)
	}
}

func TestGenerateRangeStreamOperations_AllItemsRemoved(t *testing.T) {
	// Common production scenario: clearing a list. Every old key emits ['r'],
	// no insertions, no reorder. Result should be a non-nil ops slice.
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2 := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}

	streamState := streamStateFor([]*TreeNode{item1, item2})
	newItems := []interface{}{} // all removed

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if ops == nil {
		t.Fatal("Expected non-nil ops (every removal emitted), got nil (fallback)")
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 'r' ops (one per removed key), got %d: %v", len(ops), ops)
	}
	for _, op := range ops {
		opArr := op.([]interface{})
		if opArr[0] != "r" {
			t.Errorf("Expected ['r', key], got %v", opArr)
		}
	}
}

func TestGenerateRangeStreamOperations_KeyHashLengthMismatch_ReturnsNil(t *testing.T) {
	// Defensive guard: RangeStreamState documents Keys/Hashes as parallel slices
	// but doesn't enforce it as an invariant. A mismatch (e.g., from a
	// deserialisation bug) must return nil (legacy fallback), not panic.
	statics := []string{`<li data-key="`, `">`, `</li>`}
	streamState := &RangeStreamState{
		Keys:        []string{"id1", "id2"},
		Hashes:      []uint64{42}, // shorter than Keys
		Fingerprint: "anything",
	}
	newItems := []interface{}{
		&TreeNode{Dynamics: []interface{}{"id1", "Name 1"}},
	}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if ops != nil {
		t.Errorf("Expected nil (mismatched Keys/Hashes lengths), got: %v", ops)
	}
}

func TestGenerateRangeStreamOperations_HetRangeFallback(t *testing.T) {
	// One new item has a divergent structural fingerprint (different Statics) → return nil.
	statics := []string{`<li data-key="`, `">`, `</li>`}
	oldItem := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	streamState := streamStateFor([]*TreeNode{oldItem})

	// New item has Statics → different fingerprint than the empty-Statics baseline.
	newItem := &TreeNode{
		Statics:  []string{`<DIFFERENT>`, `</DIFFERENT>`},
		Dynamics: []interface{}{"id1", "Name 1"},
	}
	newItems := []interface{}{newItem}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, false)
	if ops != nil {
		t.Errorf("Expected nil (het-range fingerprint fallback), got: %v", ops)
	}
}

func TestGenerateRangeStreamOperations_StripStatics(t *testing.T) {
	// stripStatics=true should remove the trailing statics from 'a' ops.
	statics := []string{`<li data-key="`, `">`, `</li>`}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	item2New := &TreeNode{Dynamics: []interface{}{"id2", "Name 2"}}

	streamState := streamStateFor([]*TreeNode{item1})
	newItems := []interface{}{item1, item2New}

	ops := GenerateRangeStreamOperations(streamState, newItems, statics, nil, true)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 'a' op, got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "a" {
		t.Fatalf("Expected ['a', ...], got %v", opArr)
	}
	if len(opArr) != 2 {
		t.Errorf("Expected stripped 'a' op to have len 2 (no statics), got len %d: %v", len(opArr), opArr)
	}
}

func TestGenerateRangeDifferentialOperations_EmptyToItems_LegacyOldKeySetEquivalent(t *testing.T) {
	// Regression guard: legacy path with len(oldItems)==0 still dispatches to
	// handleEmptyToItemsTransition after the predicate swap to len(oldKeySet)==0.
	// Locks in the equivalence so a future change can't silently flip behavior.
	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{},
		},
	}
	item1 := &TreeNode{Dynamics: []interface{}{"id1", "Name 1"}}
	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1},
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)
	if len(ops) != 1 {
		t.Fatalf("Expected single 'a' op for empty-to-items transition, got %d: %v", len(ops), ops)
	}
	opArr := ops[0].([]interface{})
	if opArr[0] != "a" {
		t.Errorf("Expected ['a', items, statics], got %v", opArr)
	}
}
