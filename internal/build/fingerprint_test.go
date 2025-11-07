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

// TestCalculateFingerprint_CircularReference tests circular reference detection.
func TestCalculateFingerprint_CircularReference(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}

	// Create circular reference
	tree.Dynamics["0"] = tree

	// Should not panic or infinite loop
	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("Should produce fingerprint even with circular reference")
	}
}

// TestCalculateFingerprint_FloatPrecision tests float hashing with exact precision.
func TestCalculateFingerprint_FloatPrecision(t *testing.T) {
	// Test with values that definitely have different binary representations
	tree1 := &TreeNode{
		Dynamics: map[string]interface{}{"0": 1.23456789},
	}

	tree2 := &TreeNode{
		Dynamics: map[string]interface{}{"0": 1.23456790},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	// These should be different due to binary representation differences
	if fp1 == fp2 {
		t.Error("Different float values should produce different fingerprints")
	}

	// Same value should produce same hash
	tree3 := &TreeNode{
		Dynamics: map[string]interface{}{"0": 1.23456789},
	}
	fp3 := CalculateFingerprint(tree3)

	if fp1 != fp3 {
		t.Error("Same float value should produce same fingerprint")
	}
}

// TestCalculateFingerprint_KeyOrdering tests consistent hashing regardless of insertion order.
func TestCalculateFingerprint_KeyOrdering(t *testing.T) {
	// Create two trees with same content but different insertion order
	tree1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "a",
			"1": "b",
			"2": "c",
		},
	}

	tree2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"2": "c",
			"0": "a",
			"1": "b",
		},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Key order should not affect fingerprint")
	}
}

// TestCalculateFingerprint_MarshalErrors tests handling of unmarshalable types.
func TestCalculateFingerprint_MarshalErrors(t *testing.T) {
	// Channels and functions cannot be marshaled to JSON
	tree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": make(chan int), // Channels cannot be marshaled
		},
	}

	// Should not panic, should handle error gracefully
	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("Should produce fingerprint even with unmarshalable types")
	}
}

// TestCalculateFingerprint_DeepNesting tests deeply nested structures.
func TestCalculateFingerprint_DeepNesting(t *testing.T) {
	// Create deeply nested tree
	innermost := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "deep"},
	}

	middle := &TreeNode{
		Statics:  []string{"<p>", "</p>"},
		Dynamics: map[string]interface{}{"0": innermost},
	}

	outer := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": middle},
	}

	fp := CalculateFingerprint(outer)
	if fp == "" {
		t.Error("Should handle deeply nested structures")
	}
}

// TestCalculateFingerprint_NilValues tests nil value handling.
func TestCalculateFingerprint_NilValues(t *testing.T) {
	tree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": nil,
			"1": "value",
		},
	}

	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("Should handle nil values")
	}
}

// TestCalculateFingerprint_EmptyCollections tests empty arrays and maps.
func TestCalculateFingerprint_EmptyCollections(t *testing.T) {
	tree1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{},
			"1": map[string]interface{}{},
		},
	}

	tree2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{},
			"1": map[string]interface{}{},
		},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Empty collections should produce consistent fingerprints")
	}
}
