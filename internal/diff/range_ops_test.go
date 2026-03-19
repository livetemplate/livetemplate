package diff

import (
	"reflect"
	"testing"
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
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Name 2",
		},
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
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Name 2",
		},
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
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Name 2",
		},
	}
	item3 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id3",
			"1": "Name 3",
		},
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

// TestGenerateRangeDifferentialOperations_Update tests item update operations.
func TestGenerateRangeDifferentialOperations_Update(t *testing.T) {
	item1Old := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Old Name",
		},
	}
	item1New := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "New Name",
		},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1Old},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1New},
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have one update operation
	found := false
	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if ok && len(opArray) >= 2 && opArray[0] == "u" && opArray[1] == "id1" {
			found = true
			// Check that changes are included
			if len(opArray) > 2 {
				changes, ok := opArray[2].(map[string]interface{})
				if !ok {
					t.Errorf("Changes should be map, got %T", opArray[2])
				} else if changes["1"] != "New Name" {
					t.Errorf("Changes['1'] = %v, want 'New Name'", changes["1"])
				}
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected update operation ['u', 'id1', changes], operations: %v", ops)
	}
}

// TestGenerateRangeDifferentialOperations_Insertion tests item insertion (append).
func TestGenerateRangeDifferentialOperations_Insertion(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Name 2",
		},
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

// TestGenerateRangeDifferentialOperations_Mixed tests mixed operations (removal + update + insertion).
func TestGenerateRangeDifferentialOperations_Mixed(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2Old := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Old Name",
		},
	}
	item2New := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "New Name",
		},
	}
	item3 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id3",
			"1": "Name 3",
		},
	}
	item4 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id4",
			"1": "Name 4",
		},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1, item2Old, item3},
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item2New, item4}, // id1 removed, id2 updated, id3 removed, id4 added
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have: 2 removals, 1 update, 1 append/insert
	hasRemoval := false
	hasUpdate := false
	hasInsertion := false

	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if !ok {
			continue
		}
		if len(opArray) < 2 {
			continue
		}

		switch opArray[0] {
		case "r":
			hasRemoval = true
		case "u":
			hasUpdate = true
		case "a", "p", "i":
			hasInsertion = true
		}
	}

	if !hasRemoval {
		t.Error("Expected at least one removal operation")
	}
	if !hasUpdate {
		t.Error("Expected update operation")
	}
	if !hasInsertion {
		t.Error("Expected insertion operation")
	}
}

// TestGenerateRangeDifferentialOperations_EmptyToItems tests transition from empty to items.
func TestGenerateRangeDifferentialOperations_EmptyToItems(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
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
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
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
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name 1",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Name 2",
		},
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
		Dynamics: map[string]interface{}{
			"0": "id1",
		},
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
				&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
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
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3"}},
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

// TestGenerateUpdateOperations tests update operation generation.
func TestGenerateUpdateOperations(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "Old"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "New"}},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}
	operations := []interface{}{}

	ctx := newRangeContext(oldItems, newItems, statics, nil)
	ops := generateUpdateOps(ctx, operations)

	// Should generate one update operation
	if len(ops) != 1 {
		t.Fatalf("Expected 1 update operation, got %d: %v", len(ops), ops)
	}

	opArray, ok := ops[0].([]interface{})
	if !ok || len(opArray) < 3 {
		t.Fatalf("Expected ['u', 'id1', changes], got %v", ops[0])
	}

	if opArray[0] != "u" || opArray[1] != "id1" {
		t.Errorf("Operation = %v, want ['u', 'id1', ...]", opArray)
	}

	changes, ok := opArray[2].(map[string]interface{})
	if !ok {
		t.Fatalf("Changes should be map, got %T", opArray[2])
	}

	if changes["1"] != "New" {
		t.Errorf("changes['1'] = %v, want 'New'", changes["1"])
	}
}

// TestGenerateInsertionOperations_Prepend tests prepend insertion.
func TestGenerateInsertionOperations_Prepend(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}},
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
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}},
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
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}}, // Inserted in middle
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3"}},
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

// TestCompareRangeItemsForChanges_NoDiff tests no changes between items.
func TestCompareRangeItemsForChanges_NoDiff(t *testing.T) {
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name",
		},
	}

	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Name",
		},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	if len(changes) != 0 {
		t.Errorf("Expected no changes, got: %v", changes)
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

// TestCompareRangeItemsForChanges_NilInputs tests nil input handling.
func TestCompareRangeItemsForChanges_NilInputs(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	tests := []struct {
		name     string
		oldItem  interface{}
		newItem  interface{}
		expected int
	}{
		{"both nil", nil, nil, 0},
		{"old nil", nil, &TreeNode{Dynamics: map[string]interface{}{"0": "id1"}}, 0},
		{"new nil", &TreeNode{Dynamics: map[string]interface{}{"0": "id1"}}, nil, 0},
		{"non-TreeNode old", "not a tree", &TreeNode{Dynamics: map[string]interface{}{"0": "id1"}}, 0},
		{"non-TreeNode new", &TreeNode{Dynamics: map[string]interface{}{"0": "id1"}}, "not a tree", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := CompareRangeItemsForChanges(tt.oldItem, tt.newItem, statics)
			if len(changes) != tt.expected {
				t.Errorf("Expected %d changes, got %d: %v", tt.expected, len(changes), changes)
			}
		})
	}
}

// TestCompareRangeItemsForChanges_Changed tests detecting changes between items.
func TestCompareRangeItemsForChanges_Changed(t *testing.T) {
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Old Name",
			"2": "Old Value",
		},
	}

	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "New Name",
			"2": "Old Value", // Unchanged
		},
	}

	statics := []string{`<li data-key="`, `">`, `</li>`}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should detect change in position "1" only
	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	if changes["1"] != "New Name" {
		t.Errorf("changes['1'] = %v, want 'New Name'", changes["1"])
	}

	// Position "0" (key) should be skipped
	if _, hasKey := changes["0"]; hasKey {
		t.Error("Key field should not be in changes")
	}

	// Position "2" (unchanged) should not be in changes
	if _, has2 := changes["2"]; has2 {
		t.Error("Unchanged field should not be in changes")
	}
}

// TestCompareRangeItemsForChanges_NonTreeNodeToTreeNode tests transition from
// non-TreeNode value (e.g., empty string) to TreeNode with statics.
// This is the case for checkbox toggles: "" -> {"s":["checked"]}
func TestCompareRangeItemsForChanges_NonTreeNodeToTreeNode(t *testing.T) {
	// Old item has an empty string for the checkbox field
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": "", // Empty string (unchecked checkbox)
			"2": "Task text",
		},
	}

	// New item has a TreeNode with statics for the checkbox field
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}}, // TreeNode with statics (checked)
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "<input ", ">", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1"
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	// The change should include the full TreeNode with statics (not stripped)
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// PrepareTreeForClient returns *TreeNode when clientHasStatics=false
	changeTree, ok := change1.(*TreeNode)
	if !ok {
		t.Fatalf("Expected *TreeNode for change, got %T: %v", change1, change1)
	}

	// Statics should be preserved (["checked"])
	if len(changeTree.Statics) != 1 || changeTree.Statics[0] != "checked" {
		t.Errorf("Expected statics ['checked'], got %v", changeTree.Statics)
	}
}

// TestCompareRangeItemsForChanges_NonExistentToTreeNode tests transition from
// non-existent field to TreeNode with statics.
func TestCompareRangeItemsForChanges_NonExistentToTreeNode(t *testing.T) {
	// Old item doesn't have the field at all
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			// Field "1" doesn't exist
		},
	}

	// New item has a TreeNode with statics for the field
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}}, // TreeNode with statics
		},
	}

	statics := []string{"<li>", "<input ", ">", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1"
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	// The change should include the full TreeNode with statics
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// PrepareTreeForClient returns *TreeNode when clientHasStatics=false
	changeTree, ok := change1.(*TreeNode)
	if !ok {
		t.Fatalf("Expected *TreeNode for change, got %T: %v", change1, change1)
	}

	// Statics should be preserved (["checked"])
	if len(changeTree.Statics) != 1 || changeTree.Statics[0] != "checked" {
		t.Errorf("Expected statics ['checked'], got %v", changeTree.Statics)
	}
}

// TestCompareRangeItemsForChanges_StaticsDiffer tests detecting changes when
// both old and new TreeNodes strip to empty but have different statics.
// This is the core checkbox toggle scenario:
// Old: {"s":["checked"]} -> strips to empty
// New: {"s":[]} -> strips to empty
// But they are visually different, so this IS a meaningful change.
func TestCompareRangeItemsForChanges_StaticsDiffer(t *testing.T) {
	// Old item has a checked checkbox (TreeNode with statics ["checked"])
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}}, // Checked state
			"2": "Task text",
		},
	}

	// New item has unchecked checkbox (TreeNode with empty statics)
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{}}, // Unchecked state - empty statics
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "<input ", ">", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1" - the statics differ!
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change (statics differ), got %d: %v", len(changes), changes)
	}

	// The change should be a TreeNode with the new statics so client can update structure
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// When statics differ, we send the full TreeNode with new statics so client can update
	// This is the correct behavior - the client needs to see the new statics to render correctly
	changeTree, isTree := change1.(*TreeNode)
	if !isTree {
		t.Fatalf("Expected *TreeNode for change, got %T: %v", change1, change1)
	}

	// The new statics should be empty (unchecked state)
	if len(changeTree.Statics) != 0 {
		t.Errorf("Expected empty statics for unchecked state, got %v", changeTree.Statics)
	}
}

// TestCompareRangeItemsForChanges_StaticsSameNoChange tests that no change is
// detected when both old and new TreeNodes have identical statics.
func TestCompareRangeItemsForChanges_StaticsSameNoChange(t *testing.T) {
	// Both have the same statics - no change should be detected
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}},
			"2": "Task text",
		},
	}

	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}}, // Same statics
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "<input ", ">", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have no changes - statics are identical
	if len(changes) != 0 {
		t.Errorf("Expected no changes (statics same), got %d: %v", len(changes), changes)
	}
}

// TestCompareRangeItemsForChanges_FieldRemoved tests detecting when a field
// exists in old item but is removed from new item.
// This handles cases like unchecking a checkbox where the "checked" attribute
// field exists in old but is absent from new.
func TestCompareRangeItemsForChanges_FieldRemoved(t *testing.T) {
	// Old item has a field "1" with a TreeNode value
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{Statics: []string{"checked"}}, // Field exists
			"2": "Task text",
		},
	}

	// New item doesn't have field "1" at all
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			// Field "1" is absent - removed
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "<input ", ">", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1" - it was removed
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change (field removed), got %d: %v", len(changes), changes)
	}

	// The change should be empty string to indicate removal
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	if change1 != "" {
		t.Errorf("Expected empty string for removed field, got %T: %v", change1, change1)
	}
}

// TestCompareRangeItemsForChanges_FieldRemovedWithStringValue tests detecting
// when a field with a string value is removed from new item.
func TestCompareRangeItemsForChanges_FieldRemovedWithStringValue(t *testing.T) {
	// Old item has a field "1" with a string value
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": "some value", // String value exists
			"2": "Task text",
		},
	}

	// New item doesn't have field "1" at all
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			// Field "1" is absent - removed
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "", "", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1" - it was removed
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change (field removed), got %d: %v", len(changes), changes)
	}

	// The change should be empty string to indicate removal
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	if change1 != "" {
		t.Errorf("Expected empty string for removed field, got %T: %v", change1, change1)
	}
}

// TestCompareRangeItemsForChanges_EmptyFieldNotReported tests that removing
// an already-empty field doesn't generate a spurious change.
func TestCompareRangeItemsForChanges_EmptyFieldNotReported(t *testing.T) {
	// Old item has a field "1" with empty string
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": "", // Empty string
			"2": "Task text",
		},
	}

	// New item doesn't have field "1" at all
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			// Field "1" is absent
			"2": "Task text",
		},
	}

	statics := []string{"<li>", "", "", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have no changes - empty string to absent is not meaningful
	if len(changes) != 0 {
		t.Errorf("Expected no changes (empty to absent), got %d: %v", len(changes), changes)
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

	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "First"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "Second"}}
	item3 := &TreeNode{Dynamics: map[string]interface{}{"0": "Third"}}

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

	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "First"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "Second"}}

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

	item1Old := &TreeNode{Dynamics: map[string]interface{}{"0": "active", "1": "Original content"}}
	item1New := &TreeNode{Dynamics: map[string]interface{}{"0": "active", "1": "Updated content"}}

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

	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "First"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "Second"}}
	item3 := &TreeNode{Dynamics: map[string]interface{}{"0": "Third"}}

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
	alice := &TreeNode{Dynamics: map[string]interface{}{"0": "active", "1": "Alice"}}
	bob := &TreeNode{Dynamics: map[string]interface{}{"0": "active", "1": "Bob"}}

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

// TestGenerateRangeDifferentialOperations_UpdateAndReorder tests the case where
// items change content AND change order simultaneously. This is the core bug
// that was discovered during Phase 2 fuzz testing - previously only updates
// were generated, missing the reorder operation.
func TestGenerateRangeDifferentialOperations_UpdateAndReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1Old := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Alpha",
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id2",
			"1": "Beta",
		},
	}
	item3 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id3",
			"1": "Charlie",
		},
	}
	item1New := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "id1",
			"1": "Alpha Updated", // Content changed
		},
	}

	oldValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item1Old, item2, item3}, // Order: id1, id2, id3
		},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range: &RangeData{
			Items: []interface{}{item2, item1New, item3}, // Order: id2, id1, id3 (reordered + id1 updated)
		},
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should have BOTH update AND reorder operations
	hasUpdate := false
	hasReorder := false

	for _, op := range ops {
		opArray, ok := op.([]interface{})
		if !ok {
			continue
		}
		switch opArray[0] {
		case "u":
			hasUpdate = true
			// Verify update is for id1
			if opArray[1] != "id1" {
				t.Errorf("Expected update for 'id1', got %v", opArray[1])
			}
		case "o":
			hasReorder = true
			// Verify reorder has correct new order
			newOrder := opArray[1].([]string)
			expectedOrder := []string{"id2", "id1", "id3"}
			if !reflect.DeepEqual(newOrder, expectedOrder) {
				t.Errorf("Reorder keys = %v, want %v", newOrder, expectedOrder)
			}
		}
	}

	if !hasUpdate {
		t.Error("Expected update operation for changed content")
	}
	if !hasReorder {
		t.Error("Expected reorder operation for changed order - THIS IS THE BUG FIX")
	}
}

// TestGenerateRangeDifferentialOperations_MultipleUpdatesAndReorder tests
// multiple items updating while also reordering.
func TestGenerateRangeDifferentialOperations_MultipleUpdatesAndReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1Old := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "One"}}
	item2Old := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Two"}}
	item3Old := &TreeNode{Dynamics: map[string]interface{}{"0": "id3", "1": "Three"}}

	item1New := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "One Updated"}}
	item2New := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Two Updated"}}
	item3New := &TreeNode{Dynamics: map[string]interface{}{"0": "id3", "1": "Three"}} // Unchanged

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1Old, item2Old, item3Old}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item3New, item2New, item1New}}, // Reversed order
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Count operation types
	updateCount := 0
	hasReorder := false

	for _, op := range ops {
		opArray := op.([]interface{})
		switch opArray[0] {
		case "u":
			updateCount++
		case "o":
			hasReorder = true
		}
	}

	// Should have 2 updates (id1 and id2 changed) and 1 reorder
	if updateCount != 2 {
		t.Errorf("Expected 2 update operations, got %d", updateCount)
	}
	if !hasReorder {
		t.Error("Expected reorder operation - THIS IS THE BUG FIX")
	}
}

// TestGenerateRangeDifferentialOperations_UpdateNoReorder tests that no reorder
// is generated when items update but order stays the same.
func TestGenerateRangeDifferentialOperations_UpdateNoReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1Old := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "Old"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Same"}}

	item1New := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "New"}}

	oldValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1Old, item2}},
	}

	newValue := &TreeNode{
		Statics: statics,
		Range:   &RangeData{Items: []interface{}{item1New, item2}}, // Same order
	}

	ops := GenerateRangeDifferentialOperations(oldValue, newValue, false)

	// Should only have update, no reorder
	hasUpdate := false
	hasReorder := false

	for _, op := range ops {
		opArray := op.([]interface{})
		switch opArray[0] {
		case "u":
			hasUpdate = true
		case "o":
			hasReorder = true
		}
	}

	if !hasUpdate {
		t.Error("Expected update operation")
	}
	if hasReorder {
		t.Error("Should NOT have reorder operation when order is unchanged")
	}
}

// TestGenerateRangeDifferentialOperations_RemovalNoReorder tests that no spurious
// reorder is generated when items are removed (key sets differ).
func TestGenerateRangeDifferentialOperations_RemovalNoReorder(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}

	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "One"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Two"}}
	item3 := &TreeNode{Dynamics: map[string]interface{}{"0": "id3", "1": "Three"}}

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

	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "One"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Two"}}
	item3 := &TreeNode{Dynamics: map[string]interface{}{"0": "id3", "1": "Three"}}

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

// =============================================================================
// CONDITIONAL BRANCH CHANGE TESTS
// Tests for the Phase 6 bug fix where conditionals change within range items,
// requiring both statics AND dynamics to be sent.
// =============================================================================

// TestCompareRangeItemsForChanges_ConditionalBranchChange tests the case where
// a conditional branch changes within a range item, causing both statics AND
// dynamics to change. The old behavior sent only dynamics (bug), the fix sends
// full tree with statics.
//
// Example template:
// {{if .HasError}}<span class="error-message">{{.Error}}</span>{{else}}<span class="status">Pending</span>{{end}}
//
// When HasError changes from false to true:
// Old: {"s":["<span class=\"status\">","</span>"], "0":"Pending"}
// New: {"s":["<span class=\"error-message\">","</span>"], "0":"Error message"}
func TestCompareRangeItemsForChanges_ConditionalBranchChange(t *testing.T) {
	// Old item: showing status (HasError=false)
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{
				Statics:  []string{`<span class="status">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "Pending"},
			},
		},
	}

	// New item: showing error (HasError=true) - DIFFERENT statics AND dynamics
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{
				Statics:  []string{`<span class="error-message">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "Permission denied"},
			},
		},
	}

	statics := []string{"<li>", "", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1"
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// The change should be a TreeNode WITH statics (not stripped)
	changeTree, isTree := change1.(*TreeNode)
	if !isTree {
		t.Fatalf("Expected *TreeNode for conditional branch change, got %T: %v", change1, change1)
	}

	// Verify statics are included (the new error message wrapper)
	if len(changeTree.Statics) != 2 {
		t.Errorf("Expected 2 statics (error-message wrapper), got %d: %v", len(changeTree.Statics), changeTree.Statics)
	}

	if changeTree.Statics[0] != `<span class="error-message">` {
		t.Errorf("Expected error-message statics, got %v", changeTree.Statics)
	}

	// Verify dynamics are also included
	if changeTree.Dynamics["0"] != "Permission denied" {
		t.Errorf("Expected dynamics with error message, got %v", changeTree.Dynamics)
	}
}

// TestCompareRangeItemsForChanges_SameStructureDifferentContent tests that when
// structure is the same (same statics) but content differs, only dynamics are sent.
// This is the normal case - no statics needed because client has them cached.
func TestCompareRangeItemsForChanges_SameStructureDifferentContent(t *testing.T) {
	// Old item: status showing "Pending"
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{
				Statics:  []string{`<span class="status">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "Pending"},
			},
		},
	}

	// New item: status showing "Done" - SAME statics, different dynamics
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "task1",
			"1": &TreeNode{
				Statics:  []string{`<span class="status">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "Done"},
			},
		},
	}

	statics := []string{"<li>", "", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1"
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// The change should be stripped (dynamics only, no statics)
	// PrepareTreeForClient with clientHasStatics=true returns map[string]interface{}
	changeMap, isMap := change1.(map[string]interface{})
	if !isMap {
		// Could also be *TreeNode with no statics - check that
		if changeTree, isTree := change1.(*TreeNode); isTree {
			if len(changeTree.Statics) > 0 {
				t.Errorf("Expected NO statics when structure unchanged, got %v", changeTree.Statics)
			}
		} else {
			t.Fatalf("Expected map or TreeNode without statics, got %T: %v", change1, change1)
		}
	} else {
		// Verify it's stripped (no "s" key for statics)
		if _, hasStatics := changeMap["s"]; hasStatics {
			t.Errorf("Expected NO statics in change when structure unchanged, got %v", changeMap)
		}
		// Verify dynamics are present
		if changeMap["0"] != "Done" {
			t.Errorf("Expected dynamics with new content, got %v", changeMap)
		}
	}
}

// TestCompareRangeItemsForChanges_ConditionalBranchChange_LoadingToError tests
// a more complex scenario: item transitions from loading spinner to error message.
// This is a realistic async loading pattern.
func TestCompareRangeItemsForChanges_ConditionalBranchChange_LoadingToError(t *testing.T) {
	// Old item: showing loading spinner
	oldItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "item-1",
			"1": &TreeNode{
				Statics:  []string{`<span class="spinner">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "⏳"},
			},
			"2": "Item Title",
		},
	}

	// New item: showing error (loading finished with error)
	newItem := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "item-1",
			"1": &TreeNode{
				Statics:  []string{`<span class="error">`, `</span>`},
				Dynamics: map[string]interface{}{"0": "Network timeout"},
			},
			"2": "Item Title",
		},
	}

	statics := []string{"<li id=\"", "\">", "", "", "</li>"}

	changes := CompareRangeItemsForChanges(oldItem, newItem, statics)

	// Should have 1 change for field "1" (the conditional part)
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d: %v", len(changes), changes)
	}

	// Field "0" (key) should not be in changes
	if _, hasKey := changes["0"]; hasKey {
		t.Error("Key field should not be in changes")
	}

	// Field "2" (title) should not be in changes
	if _, hasTitle := changes["2"]; hasTitle {
		t.Error("Unchanged field should not be in changes")
	}

	change1 := changes["1"]

	// The change should be a TreeNode WITH statics
	changeTree, isTree := change1.(*TreeNode)
	if !isTree {
		t.Fatalf("Expected *TreeNode with new statics, got %T: %v", change1, change1)
	}

	// Verify new statics are included (error wrapper)
	if len(changeTree.Statics) < 2 || changeTree.Statics[0] != `<span class="error">` {
		t.Errorf("Expected error statics, got %v", changeTree.Statics)
	}
}
