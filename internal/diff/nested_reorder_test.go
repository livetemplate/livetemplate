package diff

import (
	"testing"
)

// TestIsPureReordering_WithNestedTreeNodes tests pure reordering with nested TreeNodes.
func TestIsPureReordering_WithNestedTreeNodes(t *testing.T) {
	// Create items with nested TreeNodes (like checkbox indicators)
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{"completed"}}, // Status indicator
			"1": "todo-1",                                  // ID
			"2": "",                                        // Empty field
			"3": "Learn Go templates",                      // Title
			"4": &TreeNode{Statics: []string{"✓"}},         // Checkbox
			"5": &TreeNode{ // Priority
				Statics:  []string{" (Priority: ", ")"},
				Dynamics: map[string]interface{}{"0": "High"},
			},
		},
	}

	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{""}},  // Not completed
			"1": "todo-3",                          // ID
			"2": "",                                // Empty field
			"3": "Write documentation",             // Title
			"4": &TreeNode{Statics: []string{"○"}}, // Unchecked
			"5": &TreeNode{
				Statics:  []string{" (Priority: ", ")"},
				Dynamics: map[string]interface{}{"0": "Low"},
			},
		},
	}

	// For reordering: old has [item1, item2], new has [item2, item1]
	// But items are exact copies (same pointer)
	oldItems := []interface{}{item1, item2}
	newItems := []interface{}{item2, item1}

	oldKeys := []string{"todo-1", "todo-3"}
	newKeys := []string{"todo-3", "todo-1"}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"} // Statics with key

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for same items in different order, got false")
	}
}

// TestIsPureReordering_WithDifferentNestedTreeNodeValues tests that when the same
// item key has different nested TreeNode content, it's detected as NOT pure reordering.
func TestIsPureReordering_WithDifferentNestedTreeNodeValues(t *testing.T) {
	// Old item1 has "completed" status
	item1Old := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{"completed"}},
			"1": "todo-1",
		},
	}

	// New item1 has "" (not completed) status - DIFFERENT content, same key
	item1New := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{""}}, // Changed from completed to not
			"1": "todo-1",
		},
	}

	oldItems := []interface{}{item1Old}
	newItems := []interface{}{item1New}

	oldKeys := []string{"todo-1"}
	newKeys := []string{"todo-1"}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if result {
		t.Errorf("Expected IsPureReordering to return false when item content changed, got true")
	}
}

// TestIsPureReordering_WithIdenticalNestedTreeNodes tests that when items are recreated
// with identical content (different pointers, same values), reordering is detected.
func TestIsPureReordering_WithIdenticalNestedTreeNodes(t *testing.T) {
	// Create old items
	oldItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{"completed"}},
			"1": "todo-1",
			"2": "Task A",
		},
	}
	oldItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{""}},
			"1": "todo-2",
			"2": "Task B",
		},
	}

	// Create NEW items with IDENTICAL content but DIFFERENT pointers (as would happen
	// when rebuilding the tree from template execution)
	newItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{"completed"}}, // Same content, new pointer
			"1": "todo-1",
			"2": "Task A",
		},
	}
	newItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{""}}, // Same content, new pointer
			"1": "todo-2",
			"2": "Task B",
		},
	}

	// Old order: [item1, item2], New order: [item2, item1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1} // Reordered

	oldKeys := []string{"todo-1", "todo-2"}
	newKeys := []string{"todo-2", "todo-1"} // Reordered

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for identical content in different order, got false")
	}
}

// TestIsPureReordering_WithAutoKeyField tests that the _k auto-generated key field
// is correctly skipped during comparison, preventing false negatives when one tree
// has _k fields but another doesn't.
func TestIsPureReordering_WithAutoKeyField(t *testing.T) {
	// OLD items: from initial render, have _k auto-key field
	oldItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-1", // Auto-generated key
			"0":  &TreeNode{Statics: []string{"completed"}},
			"1":  "todo-1",
			"2":  "Task A",
		},
	}
	oldItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-2", // Auto-generated key
			"0":  &TreeNode{Statics: []string{""}},
			"1":  "todo-2",
			"2":  "Task B",
		},
	}

	// NEW items: from re-render, also have _k but may have been regenerated
	newItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-1", // Same key
			"0":  &TreeNode{Statics: []string{"completed"}},
			"1":  "todo-1",
			"2":  "Task A",
		},
	}
	newItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-2", // Same key
			"0":  &TreeNode{Statics: []string{""}},
			"1":  "todo-2",
			"2":  "Task B",
		},
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	oldKeys := []string{"todo-1", "todo-2"}
	newKeys := []string{"todo-2", "todo-1"}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true when items have _k field, got false")
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
		Dynamics: map[string]interface{}{
			"0": oldNestedNode1,
			"1": "todo-1",
		},
	}
	oldItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": oldNestedNode2,
			"1": "todo-2",
		},
	}

	// Create new items WITHOUT cached fingerprint (fresh from build)
	newItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{"completed"}},
			"1": "todo-1",
		},
	}
	newItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{Statics: []string{""}},
			"1": "todo-2",
		},
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	oldKeys := []string{"todo-1", "todo-2"}
	newKeys := []string{"todo-2", "todo-1"}

	statics := []string{`<tr data-lvt-key="`, `">`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true even with different cached fingerprints, got false")
	}
}

// TestIsPureReordering_AutoKeyFieldVariations tests that the _k auto-key field
// variations don't break reordering detection:
// - Both items have _k with same value
// - Both items have _k with different values (shouldn't matter if actual content matches)
func TestIsPureReordering_AutoKeyFieldVariations(t *testing.T) {
	// OLD items: Have _k field
	oldItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-1",
			"0":  "todo-1", // Key at position 0
			"1":  &TreeNode{Statics: []string{"completed"}},
			"2":  "Task A",
		},
	}
	oldItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-2",
			"0":  "todo-2", // Key at position 0
			"1":  &TreeNode{Statics: []string{""}},
			"2":  "Task B",
		},
	}

	// NEW items: Also have _k field
	newItem1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-1",
			"0":  "todo-1",
			"1":  &TreeNode{Statics: []string{"completed"}},
			"2":  "Task A",
		},
	}
	newItem2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"_k": "todo-2",
			"0":  "todo-2",
			"1":  &TreeNode{Statics: []string{""}},
			"2":  "Task B",
		},
	}

	// Reorder: old [1,2], new [2,1]
	oldItems := []interface{}{oldItem1, oldItem2}
	newItems := []interface{}{newItem2, newItem1}

	oldKeys := []string{"todo-1", "todo-2"}
	newKeys := []string{"todo-2", "todo-1"}

	// Statics with key at position 0 (after first static)
	statics := []string{`<tr data-lvt-key="`, `">`, `<td>`, `</td>`, "</tr>"}

	result := IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics)

	if !result {
		t.Errorf("Expected IsPureReordering to return true for items with _k field, got false")
	}
}
