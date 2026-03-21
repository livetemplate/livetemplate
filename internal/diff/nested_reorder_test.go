package diff

import (
	"testing"
)

// TestIsPureReordering_WithNestedTreeNodes tests pure reordering with nested TreeNodes.
func TestIsPureReordering_WithNestedTreeNodes(t *testing.T) {
	// Create items with nested TreeNodes (like checkbox indicators)
	item1 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}}, // Status indicator
			"todo-1",                          // ID
			"",                                // Empty field
			"Learn Go templates",              // Title
			&TreeNode{Statics: []string{"✓"}}, // Checkbox
			&TreeNode{ // Priority
				Statics:  []string{" (Priority: ", ")"},
				Dynamics: []interface{}{"High"},
			},
		},
	}

	item2 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}},  // Not completed
			"todo-3",                          // ID
			"",                                // Empty field
			"Write documentation",             // Title
			&TreeNode{Statics: []string{"○"}}, // Unchecked
			&TreeNode{
				Statics:  []string{" (Priority: ", ")"},
				Dynamics: []interface{}{"Low"},
			},
		},
	}

	// For reordering: old has [item1, item2], new has [item2, item1]
	// But items are exact copies (same pointer)
	oldItems := []interface{}{item1, item2}
	newItems := []interface{}{item2, item1}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"} // Statics with key

	result := IsPureReordering(oldItems, newItems, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for same items in different order, got false")
	}
}

// TestIsPureReordering_WithDifferentNestedTreeNodeValues tests that when the same
// item key has different nested TreeNode content, it's detected as NOT pure reordering.
func TestIsPureReordering_WithDifferentNestedTreeNodeValues(t *testing.T) {
	// Old item1 has "completed" status
	item1Old := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}},
			"todo-1",
		},
	}

	// New item1 has "" (not completed) status - DIFFERENT content, same key
	item1New := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}}, // Changed from completed to not
			"todo-1",
		},
	}

	oldItems := []interface{}{item1Old}
	newItems := []interface{}{item1New}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, statics)

	if result {
		t.Errorf("Expected IsPureReordering to return false when item content changed, got true")
	}
}

// TestIsPureReordering_WithIdenticalNestedTreeNodes tests that when items are recreated
// with identical content (different pointers, same values), reordering is detected.
func TestIsPureReordering_WithIdenticalNestedTreeNodes(t *testing.T) {
	// Create old items
	oldItem1 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}},
			"todo-1",
			"Task A",
		},
	}
	oldItem2 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}},
			"todo-2",
			"Task B",
		},
	}

	// Create NEW items with IDENTICAL content but DIFFERENT pointers (as would happen
	// when rebuilding the tree from template execution)
	newItem1 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}}, // Same content, new pointer
			"todo-1",
			"Task A",
		},
	}
	newItem2 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}}, // Same content, new pointer
			"todo-2",
			"Task B",
		},
	}

	// Old order: [item1, item2], New order: [item2, item1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1} // Reordered

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for identical content in different order, got false")
	}
}

// TestIsPureReordering_WithAutoKeyField tests that the _k auto-generated key field
// is correctly skipped during comparison, preventing false negatives when one tree
// has _k fields but another doesn't.
func TestIsPureReordering_WithAutoKeyField(t *testing.T) {
	// OLD items: from initial render, have AutoKey field
	oldItem1 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}},
			"todo-1",
			"Task A",
		},
		AutoKey: "todo-1",
	}
	oldItem2 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}},
			"todo-2",
			"Task B",
		},
		AutoKey: "todo-2",
	}

	// NEW items: from re-render, also have AutoKey but may have been regenerated
	newItem1 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"completed"}},
			"todo-1",
			"Task A",
		},
		AutoKey: "todo-1",
	}
	newItem2 := &TreeNode{
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{""}},
			"todo-2",
			"Task B",
		},
		AutoKey: "todo-2",
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true when items have AutoKey field, got false")
	}
}

// TestIsPureReordering_CachedFingerprint tests that items with different
// cachedStructureFingerprint values are still detected as equal when content matches.
// This is important because old trees may have cached fingerprints while new trees don't.
func TestIsPureReordering_CachedFingerprint(t *testing.T) {
	// Create old items and trigger fingerprint caching by calling GetStructureFingerprint
	oldNestedNode1 := &TreeNode{Statics: []string{"completed"}}
	_ = oldNestedNode1.GetStructureFingerprint() // Cache the fingerprint
	oldNestedNode2 := &TreeNode{Statics: []string{""}}
	_ = oldNestedNode2.GetStructureFingerprint() // Cache the fingerprint

	oldItem1 := &TreeNode{
		Dynamics: []interface{}{oldNestedNode1, "todo-1"},
	}
	oldItem2 := &TreeNode{
		Dynamics: []interface{}{oldNestedNode2, "todo-2"},
	}

	// Create new items WITHOUT cached fingerprint (fresh from build)
	newItem1 := &TreeNode{
		Dynamics: []interface{}{&TreeNode{Statics: []string{"completed"}}, "todo-1"},
	}
	newItem2 := &TreeNode{
		Dynamics: []interface{}{&TreeNode{Statics: []string{""}}, "todo-2"},
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true even with different cached fingerprints, got false")
	}
}

// TestIsPureReordering_AutoKeyFieldVariations tests that the AutoKey field
// variations don't break reordering detection:
// - Both items have AutoKey with same value
// - Both items have AutoKey with different values (shouldn't matter if actual content matches)
func TestIsPureReordering_AutoKeyFieldVariations(t *testing.T) {
	// OLD items: Have AutoKey field
	oldItem1 := &TreeNode{
		Dynamics: []interface{}{
			"todo-1", // Key at position 0
			&TreeNode{Statics: []string{"completed"}},
			"Task A",
		},
		AutoKey: "todo-1",
	}
	oldItem2 := &TreeNode{
		Dynamics: []interface{}{
			"todo-2", // Key at position 0
			&TreeNode{Statics: []string{""}},
			"Task B",
		},
		AutoKey: "todo-2",
	}

	// NEW items: Also have AutoKey field
	newItem1 := &TreeNode{
		Dynamics: []interface{}{
			"todo-1",
			&TreeNode{Statics: []string{"completed"}},
			"Task A",
		},
		AutoKey: "todo-1",
	}
	newItem2 := &TreeNode{
		Dynamics: []interface{}{
			"todo-2",
			&TreeNode{Statics: []string{""}},
			"Task B",
		},
		AutoKey: "todo-2",
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	// Statics with key at position 0 (after first static)
	statics := []string{`<tr data-lvt-key="`, `">`, `<td>`, `</td>`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for items with AutoKey field, got false")
	}
}
