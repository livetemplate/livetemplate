package diff

import (
	"reflect"
	"testing"
)

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
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items: []interface{}{item1, item2},
		},
	}

	newValue := &TreeNode{
		Statics: []string{"<li>", "</li>"},
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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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
	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}
	operations := []interface{}{}

	ops := generateRemovalOperations(oldItems, newItems, statics, operations)

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

	statics := []string{"<li>", "</li>"}
	operations := []interface{}{}

	ops := generateUpdateOperations(oldItems, newItems, statics, operations)

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

	statics := []string{"<li>", "</li>"}
	operations := []interface{}{}

	ops := generateInsertionOperations(oldItems, newItems, statics, nil, operations)

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

	statics := []string{"<li>", "</li>"}
	operations := []interface{}{}

	ops := generateInsertionOperations(oldItems, newItems, statics, nil, operations)

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

	statics := []string{"<li>", "</li>"}
	operations := []interface{}{}

	ops := generateInsertionOperations(oldItems, newItems, statics, nil, operations)

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

	statics := []string{"<li>", "</li>"}

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
	statics := []string{"<li>", "</li>"}

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

	statics := []string{"<li>", "</li>"}

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
			"1": "",         // Empty string (unchecked checkbox)
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

	// The change should be empty string to indicate field should be cleared
	change1, ok := changes["1"]
	if !ok {
		t.Fatal("Expected change for field '1'")
	}

	// When both strip to empty but statics differ, we send empty string
	if change1 != "" {
		t.Errorf("Expected empty string for change, got %T: %v", change1, change1)
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
