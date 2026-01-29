// Package invariants provides verification of LiveTemplate's core invariants.
package invariants

import (
	"encoding/json"
	"fmt"

	"github.com/livetemplate/livetemplate/internal/build"
)

// ApplyDiff applies a diff tree to an old tree to reconstruct the new tree.
// This is the "oracle" that verifies diff correctness.
//
// The key insight: if diff(oldTree, newTree) produces diffTree,
// then ApplyDiff(oldTree, diffTree) should produce a tree equivalent to newTree.
//
// Diff format:
//   - Simple value changes: {"0": "new value"} replaces dynamic at position "0"
//   - Nested tree changes: {"2": {nested TreeNode}} applied recursively
//   - Range operations: {"3": [["r", "key"], ["u", "key", changes], ...]}
//     Operations are stored at the SAME position as the range, not in a special "d" key
func ApplyDiff(oldTree, diffTree *build.TreeNode) *build.TreeNode {
	if diffTree == nil {
		return oldTree
	}
	if oldTree == nil {
		// First render - diff IS the new tree
		return diffTree
	}

	result := oldTree.Clone()

	// Apply dynamic changes
	for k, newValue := range diffTree.GetDynamics() {
		// Check if this is a range operation array (array of arrays starting with op code)
		if ops, ok := newValue.([]any); ok && len(ops) > 0 {
			if isRangeOperationArray(ops) {
				// Get the corresponding old value to find the range
				if oldValue, exists := result.GetDynamic(k); exists {
					if oldTreeNode, ok := oldValue.(*build.TreeNode); ok {
						// Apply range operations to the nested tree
						result.SetDynamic(k, applyRangeOps(oldTreeNode, ops))
						continue
					}
				}
				// Old value doesn't exist or isn't a tree - this is an empty->items transition
				// Create a new tree with the items from append operation
				newRangeTree := createRangeTreeFromOps(ops)
				if newRangeTree != nil {
					result.SetDynamic(k, newRangeTree)
				}
				continue
			}
		}

		// Handle nested tree changes
		if newTreeNode, ok := newValue.(*build.TreeNode); ok {
			if oldValue, exists := result.GetDynamic(k); exists {
				if oldTreeNode, ok := oldValue.(*build.TreeNode); ok {
					// Recursively apply diff to nested tree
					result.SetDynamic(k, ApplyDiff(oldTreeNode, newTreeNode))
					continue
				}
			}
			// New tree appearing where none existed before
			result.SetDynamic(k, newTreeNode)
			continue
		}

		// Direct value replacement
		result.SetDynamic(k, newValue)
	}

	return result
}

// isRangeOperationArray checks if an array looks like range operations.
// Range operations are arrays of arrays where each inner array starts with an op code.
func isRangeOperationArray(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	// Check first element - should be an array starting with a string op code
	firstOp, ok := arr[0].([]any)
	if !ok || len(firstOp) < 1 {
		return false
	}
	opCode, ok := firstOp[0].(string)
	if !ok {
		return false
	}
	// Valid op codes: r (remove), i (insert), u (update), o (reorder), a (append), p (prepend)
	switch opCode {
	case "r", "i", "u", "o", "a", "p":
		return true
	}
	return false
}

// createRangeTreeFromOps creates a new range tree from append/prepend operations.
// This handles the empty->items transition case.
func createRangeTreeFromOps(ops []any) *build.TreeNode {
	result := build.NewTreeNode()

	for _, op := range ops {
		opArray, ok := op.([]any)
		if !ok || len(opArray) < 2 {
			continue
		}

		opType, ok := opArray[0].(string)
		if !ok {
			continue
		}

		switch opType {
		case "a", "p": // Append or Prepend: ["a/p", items, statics, ?metadata]
			items, ok := opArray[1].([]any)
			if !ok {
				continue
			}

			// Create range data
			result.Range = &build.RangeData{
				Items: items,
			}

			// Extract statics if present
			if len(opArray) >= 3 {
				if statics, ok := opArray[2].([]any); ok {
					result.Statics = make([]string, len(statics))
					for i, s := range statics {
						if str, ok := s.(string); ok {
							result.Statics[i] = str
						}
					}
				} else if statics, ok := opArray[2].([]string); ok {
					result.Statics = statics
				}
				// Also set Range.Statics
				result.Range.Statics = result.Statics
			}

			// Extract metadata if present
			if len(opArray) >= 4 {
				if meta, ok := opArray[3].(map[string]any); ok {
					if idKey, ok := meta["idKey"].(string); ok {
						result.Metadata = &build.TreeMetadata{IDKey: idKey}
					}
				}
			}

			return result
		}
	}

	return nil
}

// applyRangeOps applies differential range operations to a tree.
//
// WIRE FORMAT (from range_ops.go - MUST MATCH EXACTLY):
//   - Remove:  ["r", key]
//   - Insert:  ["i", prevKey, data]           (3 args, NOT 4!)
//   - Update:  ["u", key, changes]
//   - Reorder: ["o", newKeyOrder]
//   - Append:  ["a", items, statics, ?meta]   (items at index 1!)
//   - Prepend: ["p", items, statics]          (was missing in original design!)
func applyRangeOps(tree *build.TreeNode, ops []any) *build.TreeNode {
	if !tree.HasRange() || tree.Range == nil {
		return tree
	}

	// Clone items to avoid modifying original
	items := make([]any, len(tree.Range.Items))
	copy(items, tree.Range.Items)

	// Build key-to-index map for efficient lookups
	keyIndex := buildKeyIndex(items)

	for _, op := range ops {
		opArray, ok := op.([]any)
		if !ok || len(opArray) < 1 {
			continue
		}

		opType, ok := opArray[0].(string)
		if !ok {
			continue
		}

		switch opType {
		case "r": // Remove: ["r", key]
			if len(opArray) < 2 {
				continue
			}
			key, ok := opArray[1].(string)
			if !ok {
				continue
			}
			items, keyIndex = removeItemByKey(items, key, keyIndex)

		case "i": // Insert: ["i", prevKey, data] (3 args!)
			if len(opArray) < 3 {
				continue
			}
			prevKey, ok := opArray[1].(string)
			if !ok {
				continue
			}
			data := opArray[2]
			items, keyIndex = insertAfterKey(items, prevKey, data, keyIndex)

		case "u": // Update: ["u", key, changes]
			if len(opArray) < 3 {
				continue
			}
			key, ok := opArray[1].(string)
			if !ok {
				continue
			}
			changes, ok := opArray[2].(map[string]any)
			if !ok {
				continue
			}
			items = updateItemByKey(items, key, changes, keyIndex)

		case "o": // Reorder: ["o", newKeyOrder]
			if len(opArray) < 2 {
				continue
			}
			newOrder, ok := opArray[1].([]any)
			if !ok {
				// Try []any as well (Go 1.18+ alias for interface{})
				newOrderIface, ok2 := opArray[1].([]any)
				if ok2 {
					newOrder = make([]any, len(newOrderIface))
					copy(newOrder, newOrderIface)
				} else {
					// Try []string (common when keys are strings)
					newOrderStr, ok3 := opArray[1].([]string)
					if ok3 {
						newOrder = make([]any, len(newOrderStr))
						for i, v := range newOrderStr {
							newOrder[i] = v
						}
					} else {
						continue
					}
				}
			}
			items = reorderItems(items, newOrder, keyIndex)
			keyIndex = buildKeyIndex(items) // Rebuild index after reorder

		case "a": // Append: ["a", items, statics, ?metadata]
			if len(opArray) < 2 {
				continue
			}
			newItems, ok := opArray[1].([]any)
			if !ok {
				continue
			}
			items = append(items, newItems...)
			keyIndex = buildKeyIndex(items) // Rebuild index

		case "p": // Prepend: ["p", items, statics]
			if len(opArray) < 2 {
				continue
			}
			newItems, ok := opArray[1].([]any)
			if !ok {
				continue
			}
			items = append(newItems, items...)
			keyIndex = buildKeyIndex(items) // Rebuild index
		}
	}

	result := tree.Clone()
	result.Range.Items = items
	return result
}

// buildKeyIndex creates a map from item keys to their indices.
func buildKeyIndex(items []any) map[string]int {
	index := make(map[string]int, len(items))
	for i, item := range items {
		if key := extractKey(item); key != "" {
			index[key] = i
		}
	}
	return index
}

// extractKey extracts the key from an item.
// Keys are typically at dynamics position "0" (the ID field).
func extractKey(item any) string {
	switch v := item.(type) {
	case *build.TreeNode:
		if val, exists := v.GetDynamic("0"); exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
	case map[string]any:
		if val, exists := v["0"]; exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
		// Also check for "ID" field (common pattern)
		if val, exists := v["ID"]; exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

// removeItemByKey removes an item by its key.
func removeItemByKey(items []any, key string, keyIndex map[string]int) ([]any, map[string]int) {
	idx, exists := keyIndex[key]
	if !exists {
		return items, keyIndex
	}

	// Remove item
	items = append(items[:idx], items[idx+1:]...)

	// Update index
	delete(keyIndex, key)
	for k, i := range keyIndex {
		if i > idx {
			keyIndex[k] = i - 1
		}
	}

	return items, keyIndex
}

// insertAfterKey inserts an item after the item with the given key.
// If prevKey is empty, inserts at the beginning.
func insertAfterKey(items []any, prevKey string, data any, keyIndex map[string]int) ([]any, map[string]int) {
	insertIdx := 0
	if prevKey != "" {
		if idx, exists := keyIndex[prevKey]; exists {
			insertIdx = idx + 1
		}
	}

	// Insert item
	newItems := make([]any, 0, len(items)+1)
	newItems = append(newItems, items[:insertIdx]...)
	newItems = append(newItems, data)
	newItems = append(newItems, items[insertIdx:]...)

	// Update index for items after insertion point
	newKey := extractKey(data)
	for k, i := range keyIndex {
		if i >= insertIdx {
			keyIndex[k] = i + 1
		}
	}
	if newKey != "" {
		keyIndex[newKey] = insertIdx
	}

	return newItems, keyIndex
}

// updateItemByKey updates fields in an item identified by key.
func updateItemByKey(items []any, key string, changes map[string]any, keyIndex map[string]int) []any {
	idx, exists := keyIndex[key]
	if !exists {
		return items
	}

	item := items[idx]
	switch v := item.(type) {
	case *build.TreeNode:
		for k, val := range changes {
			v.SetDynamic(k, val)
		}
	case map[string]any:
		for k, val := range changes {
			v[k] = val
		}
	}

	return items
}

// reorderItems reorders items according to the new key order.
func reorderItems(items []any, newOrder []any, keyIndex map[string]int) []any {
	if len(newOrder) == 0 {
		return items
	}

	reordered := make([]any, 0, len(newOrder))
	for _, keyRaw := range newOrder {
		key, ok := keyRaw.(string)
		if !ok {
			continue
		}
		if idx, exists := keyIndex[key]; exists && idx < len(items) {
			reordered = append(reordered, items[idx])
		}
	}

	return reordered
}

// TreeDynamicsEqual compares the dynamic content of two trees.
// Statics are NOT compared because the client caches them.
//
// This comparison is lenient about missing empty values:
// - If one tree has a dynamic that the other doesn't, and that value is empty
//   (empty string, nil, or static-only TreeNode), they're treated as equivalent.
// - This handles the case where diff operations don't include empty values for new items.
func TreeDynamicsEqual(a, b *build.TreeNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	aDyn := a.GetDynamics()
	bDyn := b.GetDynamics()

	// Check all dynamics in a
	for k, aVal := range aDyn {
		bVal, exists := bDyn[k]
		if !exists {
			// b doesn't have this dynamic - OK if a's value is empty
			if !isEmptyValue(aVal) {
				return false
			}
			continue
		}

		if !dynamicsEqual(aVal, bVal) {
			return false
		}
	}

	// Check for dynamics in b that aren't in a
	for k, bVal := range bDyn {
		if _, exists := aDyn[k]; !exists {
			// a doesn't have this dynamic - OK if b's value is empty
			if !isEmptyValue(bVal) {
				return false
			}
		}
	}

	// Compare ranges if present
	if a.HasRange() || b.HasRange() {
		if !rangesEqual(a, b) {
			return false
		}
	}

	return true
}

// dynamicsEqual compares two dynamic values.
func dynamicsEqual(aVal, bVal any) bool {
	// Handle nested TreeNodes
	aTree, aIsTree := aVal.(*build.TreeNode)
	bTree, bIsTree := bVal.(*build.TreeNode)

	if aIsTree && bIsTree {
		return TreeDynamicsEqual(aTree, bTree)
	}

	// Handle one tree, one non-tree
	if aIsTree {
		// a is tree, b is not - equal if a is empty and b is empty
		return isTreeEmpty(aTree) && isEmptyValue(bVal)
	}
	if bIsTree {
		// b is tree, a is not - equal if b is empty and a is empty
		return isTreeEmpty(bTree) && isEmptyValue(aVal)
	}

	// Compare using JSON for deep equality
	return jsonEqual(aVal, bVal)
}

// isEmptyValue checks if a value is considered "empty" for comparison purposes.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case *build.TreeNode:
		return isTreeEmpty(val)
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	}
	return false
}

// isTreeEmpty checks if a TreeNode is empty (no meaningful content).
func isTreeEmpty(t *build.TreeNode) bool {
	if t == nil {
		return true
	}
	// A tree with only statics (no dynamics, no range) is considered empty
	// This represents a false conditional branch
	if !t.HasDynamics() && !t.HasRange() {
		return true
	}
	// Check if all dynamics are empty
	for _, v := range t.GetDynamics() {
		if !isEmptyValue(v) {
			return false
		}
	}
	return !t.HasRange()
}

// rangesEqual compares range data between two trees.
func rangesEqual(a, b *build.TreeNode) bool {
	aHas := a.HasRange() && a.Range != nil
	bHas := b.HasRange() && b.Range != nil

	if aHas != bHas {
		return false
	}

	if !aHas {
		return true
	}

	aItems := a.Range.Items
	bItems := b.Range.Items

	if len(aItems) != len(bItems) {
		return false
	}

	for i := range aItems {
		aItem, aIsTree := aItems[i].(*build.TreeNode)
		bItem, bIsTree := bItems[i].(*build.TreeNode)

		if aIsTree && bIsTree {
			if !TreeDynamicsEqual(aItem, bItem) {
				return false
			}
			continue
		}

		if !jsonEqual(aItems[i], bItems[i]) {
			return false
		}
	}

	return true
}

// jsonEqual compares two values using JSON serialization.
func jsonEqual(a, b any) bool {
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)

	if err1 != nil || err2 != nil {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}

	return string(aJSON) == string(bJSON)
}
