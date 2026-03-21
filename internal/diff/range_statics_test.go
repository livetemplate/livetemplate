package diff

import (
	"testing"
)

// TestExtractRangeData_EmptyToItems_RangeStatics verifies that extractRangeData
// properly uses Range.Statics when TreeNode.Statics is nil (update context)
func TestExtractRangeData_EmptyToItems_RangeStatics(t *testing.T) {
	// Simulate empty range (oldNode)
	oldNode := &TreeNode{
		Statics: []string{""},
		Range: &RangeData{
			Items:   []interface{}{},
			Statics: nil,
		},
	}

	// Simulate new range with items (newNode) during update
	// TreeNode.Statics is nil (context says don't include)
	// but Range.Statics should be populated
	itemStatics := []string{"<li id=\"", "\">", "</li>"}
	newNode := &TreeNode{
		Statics: nil, // Not included during updates
		Range: &RangeData{
			Items: []interface{}{
				&TreeNode{
					Dynamics: []interface{}{"item-1", "Text 1"},
				},
			},
			Statics: itemStatics, // Should be populated by my fix
		},
		Metadata: &TreeMetadata{IDKey: "0"},
	}

	oldItems, newItems, statics, metadata := extractRangeData(oldNode, newNode)

	t.Logf("oldItems: %d, newItems: %d", len(oldItems), len(newItems))
	t.Logf("statics: %v (type: %T)", statics, statics)
	t.Logf("metadata: %v", metadata)

	if len(oldItems) != 0 {
		t.Errorf("Expected 0 old items, got %d", len(oldItems))
	}

	if len(newItems) != 1 {
		t.Errorf("Expected 1 new item, got %d", len(newItems))
	}

	// CRITICAL: statics should be the item template from Range.Statics
	staticsSlice, ok := statics.([]string)
	if !ok {
		t.Fatalf("Expected statics to be []string, got %T", statics)
	}

	if len(staticsSlice) == 0 {
		t.Error("statics should not be empty - need item template for rendering!")
	}

	if len(staticsSlice) != 3 {
		t.Errorf("Expected 3 statics, got %d: %v", len(staticsSlice), staticsSlice)
	}

	if metadata == nil {
		t.Error("metadata should not be nil")
	}
}

// TestGenerateRangeDifferentialOperations_EmptyToItems_HasStatics verifies
// that the generated 'a' (append) operation includes proper statics
func TestGenerateRangeDifferentialOperations_EmptyToItems_HasStatics(t *testing.T) {
	// Simulate empty range (oldNode)
	oldNode := &TreeNode{
		Statics: []string{""},
		Range: &RangeData{
			Items:   []interface{}{},
			Statics: nil,
		},
	}

	// Simulate new range with items (newNode) during update
	itemStatics := []string{"<li id=\"", "\">", "</li>"}
	newNode := &TreeNode{
		Statics: nil, // Not included during updates
		Range: &RangeData{
			Items: []interface{}{
				&TreeNode{
					Dynamics: []interface{}{"item-1", "Text 1"},
				},
			},
			Statics: itemStatics,
		},
		Metadata: &TreeMetadata{IDKey: "0"},
	}

	// stripStatics=false because client hasn't seen this structure yet
	operations := GenerateRangeDifferentialOperations(oldNode, newNode, false)

	t.Logf("Operations: %v", operations)

	if len(operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(operations))
	}

	op, ok := operations[0].([]interface{})
	if !ok {
		t.Fatalf("Expected operation to be []interface{}, got %T", operations[0])
	}

	// Format: ['a', items, statics, metadata]
	if op[0] != "a" {
		t.Errorf("Expected 'a' operation, got %v", op[0])
	}

	// Check statics at position 2
	if len(op) < 3 {
		t.Fatalf("Operation too short, expected at least 3 elements: %v", op)
	}

	statics := op[2]
	t.Logf("statics in operation: %v (type: %T)", statics, statics)

	if statics == nil {
		t.Error("statics should NOT be nil in append operation - breaks client rendering!")
	}

	staticsSlice, ok := statics.([]string)
	if !ok {
		t.Errorf("Expected statics to be []string, got %T", statics)
	}

	if len(staticsSlice) != 3 {
		t.Errorf("Expected 3 statics, got %d: %v", len(staticsSlice), staticsSlice)
	}
}
