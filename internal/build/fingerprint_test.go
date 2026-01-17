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

// TestCalculateFingerprint_ArrayPositionSensitivity tests that arrays with same elements
// in different positions produce different fingerprints.
// This verifies that the sequential iteration properly distinguishes element positions.
func TestCalculateFingerprint_ArrayPositionSensitivity(t *testing.T) {
	// Array with elements in one order
	tree1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{"a", "b", "c"},
		},
	}

	// Array with same elements in different order
	tree2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{"c", "b", "a"},
		},
	}

	// Array with same elements, different middle element
	tree3 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{"a", "c", "b"},
		},
	}

	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)
	fp3 := CalculateFingerprint(tree3)

	// Different orderings should produce different fingerprints
	if fp1 == fp2 {
		t.Error("Arrays with different element positions should produce different fingerprints (reversed)")
	}

	if fp1 == fp3 {
		t.Error("Arrays with different element positions should produce different fingerprints (swapped)")
	}

	if fp2 == fp3 {
		t.Error("Different array orderings should all produce unique fingerprints")
	}

	// Same order should produce same fingerprint
	tree4 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": []interface{}{"a", "b", "c"},
		},
	}

	fp4 := CalculateFingerprint(tree4)
	if fp1 != fp4 {
		t.Error("Same array order should produce same fingerprint")
	}
}

// TestCalculateFingerprint_NilTree tests that nil tree returns empty string.
func TestCalculateFingerprint_NilTree(t *testing.T) {
	fp := CalculateFingerprint(nil)
	if fp != "" {
		t.Errorf("Expected empty string for nil tree, got %q", fp)
	}
}

// TestCalculateFingerprint_StructuralSharing tests that the same node can appear
// in different branches (legitimate structural sharing) without being treated as circular.
func TestCalculateFingerprint_StructuralSharing(t *testing.T) {
	// Create a shared subtree
	sharedNode := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "shared"},
	}

	// Use the same node in two different positions
	tree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": sharedNode, // First occurrence
			"1": sharedNode, // Second occurrence - should NOT be treated as circular
		},
	}

	// Should not panic and should produce a valid fingerprint
	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("Structural sharing should produce non-empty fingerprint")
	}

	// The fingerprint should be deterministic
	fp2 := CalculateFingerprint(tree)
	if fp != fp2 {
		t.Error("Structural sharing should produce consistent fingerprint")
	}
}

// TestCalculateFingerprint_ActualCircular tests actual circular references are detected.
func TestCalculateFingerprint_ActualCircular(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}

	// Create actual circular reference (tree points to itself)
	tree.Dynamics["0"] = tree

	// Should handle circular reference without infinite loop
	fp := CalculateFingerprint(tree)
	if fp == "" {
		t.Error("Circular reference should produce non-empty fingerprint")
	}

	// Should be deterministic
	fp2 := CalculateFingerprint(tree)
	if fp != fp2 {
		t.Error("Circular reference should produce consistent fingerprint")
	}
}

// TestCalculateFingerprint_DelimiterConsistency tests that delimiter usage is consistent.
func TestCalculateFingerprint_DelimiterConsistency(t *testing.T) {
	// Test various edge cases that could cause collisions with inconsistent delimiters
	testCases := []struct {
		name  string
		tree1 *TreeNode
		tree2 *TreeNode
	}{
		{
			name: "string boundary collision",
			tree1: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": "abc",
					"1": "def",
				},
			},
			tree2: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": "abcdef",
				},
			},
		},
		{
			name: "int boundary collision",
			tree1: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": 12,
					"1": 34,
				},
			},
			tree2: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": 1234,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fp1 := CalculateFingerprint(tc.tree1)
			fp2 := CalculateFingerprint(tc.tree2)

			// Different structures should produce different fingerprints
			if fp1 == fp2 {
				t.Errorf("Different structures should not collide: %q == %q", fp1, fp2)
			}
		})
	}
}

// TestCalculateFingerprint_ErrorHandling tests unmarshalable types use type info.
func TestCalculateFingerprint_ErrorHandling(t *testing.T) {
	// Create tree with unmarshalable type (channel)
	tree1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": make(chan int),
		},
	}

	fp1 := CalculateFingerprint(tree1)
	if fp1 == "" {
		t.Error("Unmarshalable type should produce non-empty fingerprint")
	}

	// Same type should produce same fingerprint
	tree2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": make(chan int),
		},
	}

	fp2 := CalculateFingerprint(tree2)
	if fp1 != fp2 {
		t.Error("Same unmarshalable type should produce same fingerprint")
	}
}

// TestCalculateFingerprint_ComplexStructuralSharing tests multiple levels of sharing.
func TestCalculateFingerprint_ComplexStructuralSharing(t *testing.T) {
	// Create shared nodes
	leaf := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "leaf"},
	}

	branch1 := &TreeNode{
		Statics:  []string{"<p>", "</p>"},
		Dynamics: map[string]interface{}{"0": leaf},
	}

	branch2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": leaf}, // Same leaf as branch1
	}

	// Root uses both branches, which share the same leaf
	root := &TreeNode{
		Statics: []string{"<body>", "</body>"},
		Dynamics: map[string]interface{}{
			"0": branch1,
			"1": branch2,
		},
	}

	fp := CalculateFingerprint(root)
	if fp == "" {
		t.Error("Complex structural sharing should produce non-empty fingerprint")
	}

	// Should be deterministic
	fp2 := CalculateFingerprint(root)
	if fp != fp2 {
		t.Error("Complex structural sharing should produce consistent fingerprint")
	}
}

// =============================================================================
// CalculateStructureFingerprint Tests
// =============================================================================

// TestCalculateStructureFingerprint_SameStaticsDifferentDynamics tests that
// trees with the same statics but different dynamic values produce the SAME
// structure fingerprint (this is the key difference from CalculateFingerprint).
func TestCalculateStructureFingerprint_SameStaticsDifferentDynamics(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "hello"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "world"},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Same statics with different dynamics should produce same structure fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}

	// For contrast, regular fingerprint SHOULD be different
	fp1 := CalculateFingerprint(tree1)
	fp2 := CalculateFingerprint(tree2)

	if fp1 == fp2 {
		t.Error("Regular fingerprint should be different for different dynamic values")
	}
}

// TestCalculateStructureFingerprint_DifferentStatics tests that trees with
// different statics produce different structure fingerprints.
func TestCalculateStructureFingerprint_DifferentStatics(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different statics should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_DifferentDynamicPositions tests that
// trees with different dynamic positions produce different structure fingerprints.
func TestCalculateStructureFingerprint_DifferentDynamicPositions(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "", "</div>"},
		Dynamics: map[string]interface{}{"0": "a", "1": "b"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "", "</div>"},
		Dynamics: map[string]interface{}{"0": "x"}, // Only one dynamic
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different dynamic positions should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_NestedTreesSameStructure tests that nested
// trees with the same structure produce the same structure fingerprint.
func TestCalculateStructureFingerprint_NestedTreesSameStructure(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "value1"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "different_value"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Nested trees with same structure should produce same fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}
}

// TestCalculateStructureFingerprint_NestedTreesDifferentStructure tests that nested
// trees with different structures produce different structure fingerprints.
func TestCalculateStructureFingerprint_NestedTreesDifferentStructure(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "value"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<p>", "</p>"}, // Different nested statics
				Dynamics: map[string]interface{}{"0": "value"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Nested trees with different structures should produce different fingerprints")
	}
}

// TestCalculateStructureFingerprint_WithRange tests that range statics are
// included in the structure fingerprint.
func TestCalculateStructureFingerprint_WithRange(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li class=\"new\">", "</li>"}, // Different range statics
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different range statics should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_RangeSameStaticsDifferentItems tests that
// range with same statics but different items produces same structure fingerprint.
func TestCalculateStructureFingerprint_RangeSameStaticsDifferentItems(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items: []interface{}{
				map[string]interface{}{"0": "different1"},
				map[string]interface{}{"0": "different2"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Range with same statics but different items should produce same structure fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}
}

// TestCalculateStructureFingerprint_Deterministic tests that the function is deterministic.
func TestCalculateStructureFingerprint_Deterministic(t *testing.T) {
	tree := &TreeNode{
		Statics: []string{"<div>", "<span>", "</span>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": "value1",
			"1": &TreeNode{
				Statics:  []string{"<p>", "</p>"},
				Dynamics: map[string]interface{}{"0": "nested"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree)
	sfp2 := CalculateStructureFingerprint(tree)
	sfp3 := CalculateStructureFingerprint(tree)

	if sfp1 != sfp2 || sfp2 != sfp3 {
		t.Errorf("Structure fingerprint should be deterministic.\nFP1: %s\nFP2: %s\nFP3: %s", sfp1, sfp2, sfp3)
	}
}

// TestCalculateStructureFingerprint_NilTree tests that nil tree returns empty string.
func TestCalculateStructureFingerprint_NilTree(t *testing.T) {
	sfp := CalculateStructureFingerprint(nil)
	if sfp != "" {
		t.Errorf("Nil tree should return empty string, got: %s", sfp)
	}
}

// TestCalculateStructureFingerprint_EmptyTree tests that empty tree produces valid fingerprint.
func TestCalculateStructureFingerprint_EmptyTree(t *testing.T) {
	tree := &TreeNode{}
	sfp := CalculateStructureFingerprint(tree)
	if sfp == "" {
		t.Error("Empty tree should produce non-empty fingerprint")
	}
}

// TestCalculateStructureFingerprint_CircularReference tests circular reference handling.
func TestCalculateStructureFingerprint_CircularReference(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}
	// Create circular reference
	tree.Dynamics["0"] = tree

	// Should not panic and should produce valid fingerprint
	sfp := CalculateStructureFingerprint(tree)
	if sfp == "" {
		t.Error("Circular reference should still produce non-empty fingerprint")
	}
}

// =============================================================================
// Lexicographic Sorting Tests (10+ Keys)
// =============================================================================

// TestCalculateStructureFingerprint_LexicographicSorting10Keys tests that
// lexicographic sorting handles 10+ keys correctly.
// Issue: Keys "0"-"9" sort correctly, but "10", "11" may sort before "2" lexicographically.
// This tests that the fingerprint is deterministic regardless of map iteration order.
func TestCalculateStructureFingerprint_LexicographicSorting10Keys(t *testing.T) {
	// Create tree with 15 dynamic keys (0-14)
	dynamics := make(map[string]interface{})
	for i := 0; i < 15; i++ {
		key := string(rune('0' + i))
		if i >= 10 {
			key = "1" + string(rune('0'+i-10))
		}
		dynamics[key] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "nested"},
		}
	}

	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: dynamics,
	}

	// Calculate fingerprint multiple times to verify determinism
	fingerprints := make([]string, 10)
	for i := 0; i < 10; i++ {
		fingerprints[i] = CalculateStructureFingerprint(tree)
	}

	// All fingerprints should be identical
	for i := 1; i < 10; i++ {
		if fingerprints[i] != fingerprints[0] {
			t.Errorf("Fingerprint %d differs from fingerprint 0: %s vs %s", i, fingerprints[i], fingerprints[0])
		}
	}
}

// TestCalculateStructureFingerprint_LexicographicOrder validates correct ordering.
// Keys: "0", "1", "10", "11", "2", "3"... should sort consistently.
func TestCalculateStructureFingerprint_LexicographicOrder(t *testing.T) {
	// Tree with keys that lexicographically sort differently than numerically
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0":  "val0",
			"1":  "val1",
			"10": "val10",
			"11": "val11",
			"2":  "val2",
			"9":  "val9",
		},
	}

	// Same tree, constructed in different order (should produce same fingerprint)
	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"9":  "val9",
			"2":  "val2",
			"11": "val11",
			"10": "val10",
			"1":  "val1",
			"0":  "val0",
		},
	}

	fp1 := CalculateStructureFingerprint(tree1)
	fp2 := CalculateStructureFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Same tree with different construction order should produce same fingerprint")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

// Helper functions for benchmarks

func createBenchTreeSmall() *TreeNode {
	return &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}
}

func createBenchTreeMedium() *TreeNode {
	dynamics := make(map[string]interface{})
	for i := 0; i < 20; i++ {
		dynamics[string(rune('a'+i))] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "nested"},
		}
	}
	return &TreeNode{
		Statics:  []string{"<div class=\"container\">", "</div>"},
		Dynamics: dynamics,
	}
}

func createBenchTreeLarge() *TreeNode {
	dynamics := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		nested := make(map[string]interface{})
		for j := 0; j < 5; j++ {
			nested[string(rune('0'+j))] = "value"
		}
		dynamics[string(rune('a'+i%26))+string(rune('0'+i/26))] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: nested,
		}
	}
	return &TreeNode{
		Statics:  []string{"<div class=\"large-container\">", "</div>"},
		Dynamics: dynamics,
	}
}

func createBenchTreeDeepNested(depth int) *TreeNode {
	if depth == 0 {
		return &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "leaf"},
		}
	}
	return &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": createBenchTreeDeepNested(depth - 1),
		},
	}
}

func createBenchRangeTree(itemCount int) *TreeNode {
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = &TreeNode{
			Statics:  []string{"<li>", "</li>"},
			Dynamics: map[string]interface{}{"0": "item", "_k": i},
		}
	}
	return &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Items:   items,
			Statics: []string{"<li>", "</li>"},
		},
	}
}

// BenchmarkCalculateFingerprint_Small benchmarks fingerprinting a small tree.
func BenchmarkCalculateFingerprint_Small(b *testing.B) {
	tree := createBenchTreeSmall()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateFingerprint(tree)
	}
}

// BenchmarkCalculateFingerprint_Medium benchmarks fingerprinting a medium tree.
func BenchmarkCalculateFingerprint_Medium(b *testing.B) {
	tree := createBenchTreeMedium()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateFingerprint(tree)
	}
}

// BenchmarkCalculateFingerprint_Large benchmarks fingerprinting a large tree.
func BenchmarkCalculateFingerprint_Large(b *testing.B) {
	tree := createBenchTreeLarge()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_Small benchmarks structure fingerprinting.
func BenchmarkCalculateStructureFingerprint_Small(b *testing.B) {
	tree := createBenchTreeSmall()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_Medium benchmarks structure fingerprinting.
func BenchmarkCalculateStructureFingerprint_Medium(b *testing.B) {
	tree := createBenchTreeMedium()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_Large benchmarks structure fingerprinting.
func BenchmarkCalculateStructureFingerprint_Large(b *testing.B) {
	tree := createBenchTreeLarge()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_DeepNested benchmarks deeply nested trees.
func BenchmarkCalculateStructureFingerprint_DeepNested(b *testing.B) {
	tree := createBenchTreeDeepNested(20)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_Range100 benchmarks range trees.
func BenchmarkCalculateStructureFingerprint_Range100(b *testing.B) {
	tree := createBenchRangeTree(100)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkCalculateStructureFingerprint_Range1000 benchmarks large range trees.
func BenchmarkCalculateStructureFingerprint_Range1000(b *testing.B) {
	tree := createBenchRangeTree(1000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// BenchmarkFingerprintComparison compares old vs new approach.
// Old: CalculateFingerprint (includes dynamics)
// New: CalculateStructureFingerprint (statics only)
func BenchmarkFingerprintComparison(b *testing.B) {
	tree := createBenchTreeMedium()

	b.Run("Full_Fingerprint", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = CalculateFingerprint(tree)
		}
	})

	b.Run("Structure_Fingerprint", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = CalculateStructureFingerprint(tree)
		}
	})
}
