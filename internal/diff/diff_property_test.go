package diff

import (
	"testing"

	"pgregory.net/rapid"
)

// Property-based tests for diff correctness using rapid.
// These tests verify that the diff algorithm produces correct results
// by generating random trees and verifying invariants.

// genStatics generates a slice of static strings (HTML fragments).
func genStatics() *rapid.Generator[[]string] {
	return rapid.SliceOfN(
		rapid.StringMatching(`<[a-z]+>`), // Simple HTML-like tags
		1, 5,
	)
}

// genDynamics generates a map of dynamic values.
func genDynamics(depth int) *rapid.Generator[map[string]interface{}] {
	if depth <= 0 {
		// At max depth, only generate primitive values
		return rapid.Custom(func(t *rapid.T) map[string]interface{} {
			n := rapid.IntRange(0, 3).Draw(t, "numDynamics")
			result := make(map[string]interface{})
			for i := 0; i < n; i++ {
				key := rapid.StringMatching(`[0-9]`).Draw(t, "key")
				result[key] = rapid.StringMatching(`[a-zA-Z0-9 ]+`).Draw(t, "value")
			}
			return result
		})
	}

	return rapid.Custom(func(t *rapid.T) map[string]interface{} {
		n := rapid.IntRange(0, 3).Draw(t, "numDynamics")
		result := make(map[string]interface{})
		for i := 0; i < n; i++ {
			key := rapid.StringMatching(`[0-9]`).Draw(t, "key")
			// 30% chance of nested tree, 70% primitive
			if rapid.Float64Range(0, 1).Draw(t, "nestedChance") < 0.3 {
				result[key] = genTreeNode(depth - 1).Draw(t, "nestedTree")
			} else {
				result[key] = rapid.StringMatching(`[a-zA-Z0-9 ]+`).Draw(t, "value")
			}
		}
		return result
	})
}

// genTreeNode generates a random TreeNode with specified max depth.
func genTreeNode(depth int) *rapid.Generator[*TreeNode] {
	return rapid.Custom(func(t *rapid.T) *TreeNode {
		return &TreeNode{
			Statics:  genStatics().Draw(t, "statics"),
			Dynamics: genDynamics(depth).Draw(t, "dynamics"),
		}
	})
}

// TestClientNeedsStatics_Property_Deterministic tests that ClientNeedsStatics
// is deterministic - same inputs always produce same output.
func TestClientNeedsStatics_Property_Deterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tree1 := genTreeNode(2).Draw(t, "tree1")
		tree2 := genTreeNode(2).Draw(t, "tree2")

		// Call multiple times and verify same result
		result1 := ClientNeedsStatics(tree1, tree2)
		result2 := ClientNeedsStatics(tree1, tree2)
		result3 := ClientNeedsStatics(tree1, tree2)

		if result1 != result2 || result2 != result3 {
			t.Errorf("ClientNeedsStatics is not deterministic: got %v, %v, %v", result1, result2, result3)
		}
	})
}

// TestClientNeedsStatics_Property_NilOldAlwaysTrue tests that nil old tree
// always requires statics (first render case).
func TestClientNeedsStatics_Property_NilOldAlwaysTrue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tree := genTreeNode(2).Draw(t, "tree")

		if !ClientNeedsStatics(nil, tree) {
			t.Error("ClientNeedsStatics(nil, tree) should always return true")
		}
	})
}

// TestClientNeedsStatics_Property_NilNewAlwaysFalse tests that nil new tree
// never requires statics (removal case).
func TestClientNeedsStatics_Property_NilNewAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tree := genTreeNode(2).Draw(t, "tree")

		if ClientNeedsStatics(tree, nil) {
			t.Error("ClientNeedsStatics(tree, nil) should always return false")
		}
	})
}

// TestClientNeedsStatics_Property_SameTreeSameResult tests that
// identical trees produce same result regardless of pointer identity.
func TestClientNeedsStatics_Property_SameTreeSameResult(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		statics := genStatics().Draw(t, "statics")
		dynamics := genDynamics(1).Draw(t, "dynamics")

		// Create two trees with same content but different pointers
		tree1 := &TreeNode{Statics: statics, Dynamics: dynamics}
		tree2 := &TreeNode{Statics: statics, Dynamics: dynamics}

		// Both should produce the same result
		needsStatics := ClientNeedsStatics(tree1, tree2)
		if needsStatics {
			t.Error("Identical trees should not need statics re-sent")
		}
	})
}

// TestClientNeedsStatics_Property_SymmetryOnDifferent tests that when
// structures differ, both directions need statics.
func TestClientNeedsStatics_Property_SymmetryOnDifferent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tree1 := genTreeNode(2).Draw(t, "tree1")
		tree2 := genTreeNode(2).Draw(t, "tree2")

		// If tree1->tree2 needs statics, tree2->tree1 should too (or neither)
		needs1to2 := ClientNeedsStatics(tree1, tree2)
		needs2to1 := ClientNeedsStatics(tree2, tree1)

		// Note: This is only true when both trees are non-nil
		// If structures differ, both directions should need statics
		if needs1to2 != needs2to1 {
			t.Errorf("Asymmetric statics requirement: 1->2=%v, 2->1=%v", needs1to2, needs2to1)
		}
	})
}

// TestCompareTreesAndGetChanges_Property_NoFalseNegatives tests that
// the diff algorithm never misses actual changes - if dynamics differ,
// the diff should reflect that.
func TestCompareTreesAndGetChanges_Property_NoFalseNegatives(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate old tree with a known dynamic value
		oldTree := &TreeNode{
			Statics:  []string{"<div>", "</div>"},
			Dynamics: map[string]interface{}{"0": "old_value"},
		}

		// Generate new tree with a different dynamic value
		newValue := rapid.StringMatching(`new_[a-z]+`).Draw(t, "newValue")
		newTree := &TreeNode{
			Statics:  []string{"<div>", "</div>"},
			Dynamics: map[string]interface{}{"0": newValue},
		}

		// Compare
		changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil)

		// If values differ, there should be a change recorded
		if newValue != "old_value" {
			if changes == nil || changes.Dynamics == nil || changes.Dynamics["0"] != newValue {
				t.Errorf("Expected change for dynamic '0' from 'old_value' to '%s'", newValue)
			}
		}
	})
}

// TestCompareTreesAndGetChanges_Property_NoSpuriousChanges tests that
// the diff algorithm doesn't report changes when there are none.
func TestCompareTreesAndGetChanges_Property_NoSpuriousChanges(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a tree
		statics := genStatics().Draw(t, "statics")
		value := rapid.StringMatching(`[a-z]+`).Draw(t, "value")

		tree1 := &TreeNode{
			Statics:  statics,
			Dynamics: map[string]interface{}{"0": value},
		}
		tree2 := &TreeNode{
			Statics:  statics,
			Dynamics: map[string]interface{}{"0": value},
		}

		// Compare identical trees
		changes := CompareTreesAndGetChangesWithPath(tree1, tree2, false, "", nil)

		// Should have no changes in dynamics (statics might be included on first render)
		if changes != nil && len(changes.Dynamics) > 0 {
			t.Errorf("Identical trees should have no dynamic changes, got: %v", changes.Dynamics)
		}
	})
}
