package build

import (
	"testing"
)

// TestCalculateFingerprint_Simple tests simple tree fingerprinting.
func TestCalculateFingerprint_Simple(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	fp := CalculateFingerprint(tree)

	if fp == "" {
		t.Error("CalculateFingerprint should return non-empty fingerprint")
	}
}

// TestCalculateFingerprint_Deterministic tests that same input produces same fingerprint.
func TestCalculateFingerprint_Deterministic(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Same tree structure should produce same fingerprint")
	}
}

// TestCalculateFingerprint_Different tests that different inputs produce different fingerprints.
func TestCalculateFingerprint_Different(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value1"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value2"},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	if fp1 == fp2 {
		t.Error("Different tree structures should produce different fingerprints")
	}
}

// TestCalculateFingerprint_Nested tests fingerprinting with nested trees.
func TestCalculateFingerprint_Nested(t *testing.T) {
	tree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "nested"},
			},
		},
	}

	fp := CalculateFingerprint(tree)

	if fp == "" {
		t.Error("Nested tree should produce non-empty fingerprint")
	}
}

// TestCalculateFingerprint_Range tests fingerprinting with range data.
func TestCalculateFingerprint_Range(t *testing.T) {
	tree := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Items:   []interface{}{"item1", "item2"},
			Statics: []string{"<li>", "</li>"},
		},
	}

	fp := CalculateFingerprint(tree)

	if fp == "" {
		t.Error("Range tree should produce non-empty fingerprint")
	}
}

// TestHashTreeIncremental tests incremental tree hashing.
func TestHashTreeIncremental(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	// Test via CalculateFingerprint which uses the hash functions
	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("CalculateFingerprint should use hashing")
	}
}

// TestHashValueIncremental_AllTypes tests hashing different value types via CalculateFingerprint.
func TestHashValueIncremental_AllTypes(t *testing.T) {
	testCases := []*TreeNode{
		{Dynamics: map[string]interface{}{"0": "string"}},
		{Dynamics: map[string]interface{}{"0": 42}},
		{Dynamics: map[string]interface{}{"0": 3.14}},
		{Dynamics: map[string]interface{}{"0": true}},
		{Dynamics: map[string]interface{}{"0": []interface{}{"a", "b"}}},
		{Dynamics: map[string]interface{}{"0": map[string]interface{}{"key": "value"}}},
	}

	for _, tree := range testCases {
		// Ensure all value types can be hashed
		fp := CalculateFingerprint(tree)
		if fp == "" {
			t.Error("Should produce fingerprint for all value types")
		}
	}
}

// TestAddFingerprintToTree tests fingerprint function behavior.
func TestAddFingerprintToTree(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	result := AddFingerprintToTree(tree)

	// Function currently returns tree without modifying it (fingerprinting disabled)
	if result != tree {
		t.Error("AddFingerprintToTree should return the same tree")
	}
}

// TestAddFingerprintToTree_EmptyTree tests with empty tree.
func TestAddFingerprintToTree_EmptyTree(t *testing.T) {
	tree := &TreeNode{}

	result := AddFingerprintToTree(tree)

	// Should return tree without modification
	if result != tree {
		t.Error("Should return same tree for empty tree")
	}
}
