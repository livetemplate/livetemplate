package diff

import (
	"fmt"
	"testing"
)

// TestLoadMoreScenario tests the load_more scenario where statics should not be resent.
// This simulates: 20 items -> 50 items (load_more adds 30 more items)
func TestLoadMoreScenario(t *testing.T) {
	// Create item statics (what each row in the range looks like)
	itemStatics := []string{
		`<tr data-key="`,
		`"><td>`,
		`</td><td><button>Edit</button></td></tr>`,
	}

	// Create 20 items for oldTree (initial page)
	oldItems := make([]any, 20)
	for i := range 20 {
		oldItems[i] = &TreeNode{
			Dynamics: map[string]any{
				"0": fmt.Sprintf("id-%d", i),    // ID
				"1": fmt.Sprintf("Title %d", i), // Title
			},
		}
	}

	// Create 50 items for newTree (after load_more)
	newItems := make([]any, 50)
	for i := range 50 {
		newItems[i] = &TreeNode{
			Dynamics: map[string]any{
				"0": fmt.Sprintf("id-%d", i),
				"1": fmt.Sprintf("Title %d", i),
			},
		}
	}

	// Create old tree structure (simulating the template structure)
	// Path: root.15.0 is the range
	oldRange := &TreeNode{
		Statics: itemStatics,
		Range: &RangeData{
			Items:   oldItems,
			Statics: itemStatics,
		},
		Metadata: &TreeMetadata{IDKey: "0"},
	}

	// Wrap in conditional wrapper (field 15) -> field 0
	oldConditional := &TreeNode{
		Statics:  []string{"", ""}, // Conditional wrapper statics
		Dynamics: map[string]any{"0": oldRange},
	}

	oldTree := &TreeNode{
		Statics:  []string{"<html>", "</html>"},
		Dynamics: map[string]any{"15": oldConditional},
	}

	// Create new tree structure (after load_more)
	newRange := &TreeNode{
		Statics: itemStatics,
		Range: &RangeData{
			Items:   newItems,
			Statics: itemStatics,
		},
		Metadata: &TreeMetadata{IDKey: "0"},
	}

	newConditional := &TreeNode{
		Statics:  []string{"", ""},
		Dynamics: map[string]any{"0": newRange},
	}

	newTree := &TreeNode{
		Statics:  []string{"<html>", "</html>"},
		Dynamics: map[string]any{"15": newConditional},
	}

	// Step 1: Check if ranges are found
	t.Log("Step 1: Finding range constructs in both trees")
	oldRanges := FindRangeConstructs(oldTree)
	newRanges := FindRangeConstructs(newTree)

	oldPaths := make([]string, 0, len(oldRanges))
	for k := range oldRanges {
		oldPaths = append(oldPaths, k)
	}
	newPaths := make([]string, 0, len(newRanges))
	for k := range newRanges {
		newPaths = append(newPaths, k)
	}
	t.Logf("Old ranges found at paths: %v", oldPaths)
	t.Logf("New ranges found at paths: %v", newPaths)

	if len(oldRanges) == 0 {
		t.Fatal("No ranges found in old tree!")
	}
	if len(newRanges) == 0 {
		t.Fatal("No ranges found in new tree!")
	}

	// Step 2: Check if ranges are matched
	t.Log("Step 2: Matching range constructs")
	rangeMatches := FindRangeConstructMatches(oldTree, newTree)
	t.Logf("Range matches: %v", rangeMatches)

	if len(rangeMatches) == 0 {
		t.Fatal("No range matches found! This is the bug.")
	}

	// Verify the range at 15.0 is matched
	if _, exists := rangeMatches["15.0"]; !exists {
		t.Errorf("Range at path '15.0' not matched! Found matches: %v", rangeMatches)
	}

	// Step 3: Test the full diff
	t.Log("Step 3: Computing tree diff")
	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", rangeMatches)

	t.Logf("Changes: %+v", changes)

	// Check if changes contain differential operations instead of full tree
	if changes.HasDynamics() {
		field15, exists := changes.GetDynamic("15")
		if !exists {
			t.Log("No changes at field 15 - good if items were just appended")
		} else {
			t.Logf("Field 15 changes: %+v", field15)

			// If field 15 is a TreeNode, check its field 0
			if node, ok := field15.(*TreeNode); ok && node.HasDynamics() {
				field0, _ := node.GetDynamic("0")
				t.Logf("Field 15.0 changes: %+v", field0)

				// Check if this is differential operations (array) or full tree
				if ops, ok := field0.([]any); ok {
					t.Logf("Got differential operations: %d ops", len(ops))
				} else if tn, ok := field0.(*TreeNode); ok {
					// If it's a TreeNode, check if it has statics (BAD) or just ops
					if tn.HasStatics() {
						t.Error("BUG: Statics were included in the diff! They should not be resent on load_more.")
					}
					if tn.Range != nil {
						t.Errorf("BUG: Full range data sent instead of differential ops! Items count: %d", len(tn.Range.Items))
					}
				}
			}
		}
	}
}
