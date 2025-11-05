package signature_test

import (
	"sync"
	"testing"

	"github.com/livetemplate/livetemplate"
	"github.com/livetemplate/livetemplate/internal/signature"
)

func TestClientStructureRegistry_SinglePath(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Test HasSeen on empty registry
	if registry.HasSeen("0", "test") {
		t.Error("Empty registry should return false for HasSeen")
	}

	// Mark a scalar value as seen
	registry.MarkSeen("0", "test")

	// Verify HasSeen returns true for same structure type
	if !registry.HasSeen("0", "test") {
		t.Error("Registry should return true for marked scalar")
	}

	// Verify different scalar value at same path ALSO returns true (same structure)
	// Note: Registry tracks structure type, not content
	if !registry.HasSeen("0", "different") {
		t.Error("Registry should return true for any scalar at same path")
	}
}

func TestClientStructureRegistry_NestedPaths(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Mark nested paths
	registry.MarkSeen("0", "root value")
	registry.MarkSeen("0.1", "nested value")
	registry.MarkSeen("0.1.2", "deeply nested")

	// Verify all paths tracked independently
	if !registry.HasSeen("0", "root value") {
		t.Error("Root path should be tracked")
	}
	if !registry.HasSeen("0.1", "nested value") {
		t.Error("Nested path should be tracked")
	}
	if !registry.HasSeen("0.1.2", "deeply nested") {
		t.Error("Deeply nested path should be tracked")
	}

	// Verify Size
	if registry.Size() != 3 {
		t.Errorf("Registry should have 3 paths, got %d", registry.Size())
	}
}

func TestClientStructureRegistry_StructureChange(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Create a scalar value
	scalar := "test"
	registry.MarkSeen("0", scalar)

	if !registry.HasSeen("0", scalar) {
		t.Error("Scalar should be seen")
	}

	// Create a TreeNode (different structure)
	node := &livetemplate.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}

	// Same path, different structure - should return false
	if registry.HasSeen("0", node) {
		t.Error("Different structure at same path should return false")
	}

	// Mark the new structure
	registry.MarkSeen("0", node)

	// Now it should be seen
	if !registry.HasSeen("0", node) {
		t.Error("Marked TreeNode should be seen")
	}

	// Original scalar should no longer match
	if registry.HasSeen("0", scalar) {
		t.Error("Old scalar should not match after structure change")
	}
}

func TestClientStructureRegistry_RangeTransitions(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Start with empty range
	emptyRange := &livetemplate.TreeNode{
		Statics:  []string{"<tr>", "</tr>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items:   []interface{}{},
			Statics: []string{"<td>", "</td>"},
		},
	}

	registry.MarkSeen("0", emptyRange)

	if !registry.HasSeen("0", emptyRange) {
		t.Error("Empty range should be seen")
	}

	// Add items to range (structure signature changes)
	rangeWithItems := &livetemplate.TreeNode{
		Statics:  []string{"<tr>", "</tr>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "item1"},
			},
			Statics: []string{"<td>", "</td>"},
		},
	}

	// Should return false - signature changed (empty → items)
	if registry.HasSeen("0", rangeWithItems) {
		t.Error("Range with items should not match empty range")
	}

	// Mark the new state
	registry.MarkSeen("0", rangeWithItems)

	// Now it should be seen
	if !registry.HasSeen("0", rangeWithItems) {
		t.Error("Range with items should be seen after marking")
	}

	// Empty range should no longer match
	if registry.HasSeen("0", emptyRange) {
		t.Error("Empty range should not match after transition to items")
	}
}

func TestClientStructureRegistry_GetSignature(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Test non-existent path
	_, exists := registry.GetSignature("0")
	if exists {
		t.Error("Non-existent path should return false")
	}

	// Mark a value
	registry.MarkSeen("0", "test")

	// Get signature
	sig, exists := registry.GetSignature("0")
	if !exists {
		t.Error("Existing path should return true")
	}
	if sig != signature.SigScalar {
		t.Errorf("Expected SigScalar, got %s", sig)
	}

	// Mark a conditional
	node := &livetemplate.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}
	registry.MarkSeen("1", node)

	sig, exists = registry.GetSignature("1")
	if !exists {
		t.Error("Conditional path should exist")
	}
	if sig != signature.SigConditional {
		t.Errorf("Expected SigConditional, got %s", sig)
	}
}

func TestClientStructureRegistry_HasPath(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	if registry.HasPath("0") {
		t.Error("Empty registry should not have path")
	}

	registry.MarkSeen("0", "test")

	if !registry.HasPath("0") {
		t.Error("Registry should have marked path")
	}

	if registry.HasPath("1") {
		t.Error("Registry should not have unmarked path")
	}
}

func TestClientStructureRegistry_Clear(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Add multiple paths
	registry.MarkSeen("0", "test1")
	registry.MarkSeen("1", "test2")
	registry.MarkSeen("2", "test3")

	if registry.Size() != 3 {
		t.Errorf("Registry should have 3 paths, got %d", registry.Size())
	}

	// Clear registry
	registry.Clear()

	if registry.Size() != 0 {
		t.Errorf("Cleared registry should have 0 paths, got %d", registry.Size())
	}

	// Verify paths are gone
	if registry.HasPath("0") || registry.HasPath("1") || registry.HasPath("2") {
		t.Error("Cleared registry should not have any paths")
	}
}

func TestClientStructureRegistry_ThreadSafety(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Number of concurrent goroutines
	numGoroutines := 100
	numOperations := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // readers + writers + clearers

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				path := string(rune(id % 10))
				registry.MarkSeen(path, "test")
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				path := string(rune(id % 10))
				registry.HasSeen(path, "test")
				registry.GetSignature(path)
				registry.HasPath(path)
			}
		}(i)
	}

	// Concurrent clearers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations/10; j++ {
				registry.Size()
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we got here without deadlock or race conditions, test passes
	// Note: Run with -race flag to detect race conditions
}

func TestClientStructureRegistry_RangeStaticsChange(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Range with one set of statics
	range1 := &livetemplate.TreeNode{
		Statics:  []string{"<tr>", "</tr>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "item1"},
			},
			Statics: []string{"<td>", "</td>"},
		},
	}

	registry.MarkSeen("0", range1)

	// Range with different statics (different template)
	range2 := &livetemplate.TreeNode{
		Statics:  []string{"<tr>", "</tr>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "item1"},
			},
			Statics: []string{"<th>", "</th>"}, // Different statics
		},
	}

	// Should return false - different statics hash
	if registry.HasSeen("0", range2) {
		t.Error("Range with different statics should not match")
	}

	// But range with same statics should match
	range1Copy := &livetemplate.TreeNode{
		Statics:  []string{"<tr>", "</tr>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "different item"},
			},
			Statics: []string{"<td>", "</td>"}, // Same statics
		},
	}

	if !registry.HasSeen("0", range1Copy) {
		t.Error("Range with same statics should match")
	}
}

func TestClientStructureRegistry_ComplexScenario(t *testing.T) {
	registry := signature.NewClientStructureRegistry()

	// Simulate a realistic scenario: conditional with nested range

	// Step 1: Empty state (conditional showing "No items")
	emptyMessage := "No items"
	registry.MarkSeen("11", emptyMessage)

	// Step 2: First item appears (conditional switches to range)
	firstItem := &livetemplate.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "item1"},
			},
			Statics: []string{"<span>", "</span>"},
		},
	}

	// Registry should detect structure change
	if registry.HasSeen("11", firstItem) {
		t.Error("First item should not match empty message")
	}

	registry.MarkSeen("11", firstItem)

	// Step 3: More items added (same structure, different data)
	moreItems := &livetemplate.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
		Range: &livetemplate.RangeData{
			Items: []interface{}{
				map[string]interface{}{"0": "item1"},
				map[string]interface{}{"0": "item2"},
			},
			Statics: []string{"<span>", "</span>"},
		},
	}

	// Should match - same signature (statics unchanged)
	if !registry.HasSeen("11", moreItems) {
		t.Error("More items with same statics should match")
	}

	// Step 4: All items removed (back to empty message)
	backToEmpty := "No items"

	// Should not match - different structure
	if registry.HasSeen("11", backToEmpty) {
		t.Error("Empty message should not match range structure")
	}

	registry.MarkSeen("11", backToEmpty)

	// Now it should match
	if !registry.HasSeen("11", backToEmpty) {
		t.Error("Marked empty message should match")
	}
}
