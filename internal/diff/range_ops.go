// Package diff provides tree comparison and differential update generation for LiveTemplate.
// It generates minimal operations (insert, update, remove, reorder) to transform one tree into another.
package diff

import (
	"fmt"
	"sort"
)

// GenerateRangeDifferentialOperations generates differential operations for range constructs.
// stripStatics: if true, removes "s" keys from operations (client has cached them)
// if false, keeps "s" keys (client hasn't seen this structure yet)
// This is the main orchestrator (30 lines).
func GenerateRangeDifferentialOperations(oldValue, newValue interface{}, stripStatics bool) []interface{} {
	var operations []interface{}

	// Extract range data from old and new values
	oldItems, newItems, statics, metadata := extractRangeData(oldValue, newValue)
	if oldItems == nil || newItems == nil {
		return operations
	}

	// Get item keys for comparison
	oldKeys := ExtractItemKeys(oldItems, statics)
	newKeys := ExtractItemKeys(newItems, statics)

	// Check for pure reordering
	if IsPureReordering(oldItems, newItems, oldKeys, newKeys, statics) {
		return []interface{}{[]interface{}{"o", newKeys}}
	}

	// Generate operations for removals, updates, and insertions
	operations = generateRemovalOperations(oldItems, newItems, statics, operations)
	operations = generateUpdateOperations(oldItems, newItems, statics, operations)
	operations = generateInsertionOperations(oldItems, newItems, statics, metadata, operations)

	// Strip statics from all operations if requested
	if stripStatics {
		operations = stripStaticsFromOperations(operations)
	}

	return operations
}

// extractRangeData extracts items, statics, and metadata from old and new range values.
func extractRangeData(oldValue, newValue interface{}) (
	oldItems, newItems []interface{},
	statics interface{},
	metadata map[string]interface{},
) {
	// Try to extract TreeNode first
	oldNode, ok := oldValue.(*TreeNode)
	if !ok {
		return nil, nil, nil, nil
	}

	if !oldNode.HasRange() || oldNode.Range == nil {
		return nil, nil, nil, nil
	}

	oldItems = oldNode.Range.Items
	statics = oldNode.Statics

	newNode, ok := newValue.(*TreeNode)
	if !ok {
		return nil, nil, nil, nil
	}

	if !newNode.HasRange() || newNode.Range == nil {
		return nil, nil, nil, nil
	}

	newItems = newNode.Range.Items

	// IMPORTANT: For empty→items transition, we need proper item statics.
	// oldNode.Statics will be minimal (e.g., [""]) for empty ranges.
	// newNode.Statics may be nil if ShouldIncludeStatics() returned false.
	// In that case, check newNode.Range.Statics which should have the item template.
	if len(oldItems) == 0 && len(newItems) > 0 {
		// Try newNode.Statics first (set if ShouldIncludeStatics was true)
		if newNode.Statics != nil && len(newNode.Statics) > 0 {
			statics = newNode.Statics
		} else if newNode.Range != nil && len(newNode.Range.Statics) > 0 {
			// Fall back to Range.Statics which should always have item template
			statics = newNode.Range.Statics
		}
		// If all are empty/nil, statics remains as oldNode.Statics (minimal)
	} else if staticsSlice, ok := statics.([]string); ok && len(staticsSlice) == 0 {
		// Fallback if old statics empty
		if newNode.Statics != nil && len(newNode.Statics) > 0 {
			statics = newNode.Statics
		} else if newNode.Range != nil && len(newNode.Range.Statics) > 0 {
			statics = newNode.Range.Statics
		}
	}

	// Extract metadata for empty→items transitions
	if newNode.Metadata != nil {
		metadata = map[string]interface{}{
			"idKey": newNode.Metadata.IDKey,
		}
	}

	return oldItems, newItems, statics, metadata
}

// generateRemovalOperations finds and generates removal operations for items that were deleted.
func generateRemovalOperations(
	oldItems, newItems []interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	// Create map for easy lookup
	newItemsByKey := createItemKeyMap(newItems, statics)

	// Find removed items (in old but not in new)
	// Extract and sort keys to ensure deterministic order
	sortedOldKeys := make([]string, 0, len(oldItems))
	for _, item := range oldItems {
		if key, ok := GetItemKey(item, statics); ok {
			sortedOldKeys = append(sortedOldKeys, key)
		}
	}
	sort.Strings(sortedOldKeys)

	for _, key := range sortedOldKeys {
		if _, exists := newItemsByKey[key]; !exists {
			operations = append(operations, []interface{}{"r", key})
		}
	}

	return operations
}

// generateUpdateOperations finds and generates update operations for changed items.
func generateUpdateOperations(
	oldItems, newItems []interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	// Create maps for easy lookup
	oldItemsByKey := createItemKeyMap(oldItems, statics)
	newItemsByKey := createItemKeyMap(newItems, statics)

	// Find updated items (in both, but changed)
	// Extract and sort keys to ensure deterministic order
	sortedNewKeys := make([]string, 0, len(newItems))
	for _, item := range newItems {
		if key, ok := GetItemKey(item, statics); ok {
			sortedNewKeys = append(sortedNewKeys, key)
		}
	}
	sort.Strings(sortedNewKeys)

	for _, key := range sortedNewKeys {
		newItem := newItemsByKey[key]
		if oldItem, exists := oldItemsByKey[key]; exists {
			// Compare items and generate update operation if different
			changes := CompareRangeItemsForChanges(oldItem, newItem, statics)
			if len(changes) > 0 {
				// Always include changes, even if they're all empty strings.
				// Empty string changes indicate that a field should be cleared
				// (e.g., removing "checked" attribute when toggling a checkbox off).
				// The client needs to know about these changes to update the DOM.
				operations = append(operations, []interface{}{"u", key, changes})
			}
		}
	}

	return operations
}

// generateInsertionOperations finds and generates insertion operations for new items.
func generateInsertionOperations(
	oldItems, newItems []interface{},
	statics interface{},
	metadata map[string]interface{},
	operations []interface{},
) []interface{} {
	// Find new items
	addedKeys := FindNewItems(oldItems, newItems, statics)
	if len(addedKeys) == 0 {
		return operations
	}

	// Check if it's a complex pattern that should fall back to full state
	if IsComplexInsertionPattern(addedKeys, oldItems, newItems, statics) {
		return operations
	}

	// SPECIAL CASE: If old range was empty, use 'a' (append) with statics and metadata
	if len(oldItems) == 0 {
		return handleEmptyToItemsTransition(newItems, statics, metadata, operations)
	}

	// Range has existing items - detect append/prepend/insert patterns
	return handleIncrementalInsertions(addedKeys, oldItems, newItems, statics, operations)
}

// handleEmptyToItemsTransition handles the transition from empty range to items.
func handleEmptyToItemsTransition(
	newItems []interface{},
	statics interface{},
	metadata map[string]interface{},
	operations []interface{},
) []interface{} {
	// Build array of items to append, KEEPING nested statics
	// The client hasn't seen these items before, so they need full structure
	itemsToAppend := make([]interface{}, 0, len(newItems))
	for _, item := range newItems {
		itemsToAppend = append(itemsToAppend, PrepareTreeForClient(item, false))
	}

	// Use 'a' operation with statics and metadata so client can initialize range state
	// Format: ['a', items, statics, metadata]
	if metadata != nil {
		operations = append(operations, []interface{}{"a", itemsToAppend, statics, metadata})
	} else {
		operations = append(operations, []interface{}{"a", itemsToAppend, statics})
	}

	return operations
}

// handleIncrementalInsertions handles insertions when range already has items.
func handleIncrementalInsertions(
	addedKeys []string,
	oldItems, newItems []interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	newItemsByKey := createItemKeyMap(newItems, statics)

	// Check if all new items are at the start (prepend)
	if AreAllItemsAtStart(addedKeys, newItems, statics) {
		return handlePrependOperation(addedKeys, newItemsByKey, statics, operations)
	}

	// Check if all new items are at the end (append)
	if AreAllItemsAtEnd(addedKeys, oldItems, newItems, statics) {
		return handleAppendOperation(addedKeys, newItemsByKey, statics, operations)
	}

	// Individual insertions at specific positions
	return handleIndividualInsertions(addedKeys, newItems, newItemsByKey, statics, operations)
}

// handlePrependOperation generates prepend operations for items at the start.
func handlePrependOperation(
	addedKeys []string,
	newItemsByKey map[string]interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	itemsToPrepend := make([]interface{}, 0, len(addedKeys))
	for _, key := range addedKeys {
		if item, exists := newItemsByKey[key]; exists {
			// Strip nested statics from items (client has cached them)
			itemsToPrepend = append(itemsToPrepend, PrepareTreeForClient(item, true))
		}
	}
	// Use 'p' operation for prepending (O(1) on client)
	// Format: ['p', items, statics] - statics describe how to render items
	operations = append(operations, []interface{}{"p", itemsToPrepend, statics})
	return operations
}

// handleAppendOperation generates append operations for items at the end.
func handleAppendOperation(
	addedKeys []string,
	newItemsByKey map[string]interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	itemsToAppend := make([]interface{}, 0, len(addedKeys))
	for _, key := range addedKeys {
		if item, exists := newItemsByKey[key]; exists {
			// Strip nested statics from items (client has cached them)
			itemsToAppend = append(itemsToAppend, PrepareTreeForClient(item, true))
		}
	}
	// Use 'a' operation for appending (O(1) on client)
	// Format: ['a', items, statics] - statics describe how to render items
	operations = append(operations, []interface{}{"a", itemsToAppend, statics})
	return operations
}

// handleIndividualInsertions generates insert operations for items at specific positions.
func handleIndividualInsertions(
	addedKeys []string,
	newItems []interface{},
	newItemsByKey map[string]interface{},
	statics interface{},
	operations []interface{},
) []interface{} {
	for _, key := range addedKeys {
		if newItem, exists := newItemsByKey[key]; exists {
			// Find position for this specific item
			for i, item := range newItems {
				if itemKey, ok := GetItemKey(item, statics); ok && itemKey == key {
					if i == 0 {
						// Item at start - use prepend for single item
						strippedItem := PrepareTreeForClient(newItem, true)
						operations = append(operations, []interface{}{"p", []interface{}{strippedItem}, statics})
					} else {
						// Find the item before this one and use simplified insert
						if prevKey, ok := GetItemKey(newItems[i-1], statics); ok {
							// Simplified insert: ['i', afterId, data] (no position param)
							strippedItem := PrepareTreeForClient(newItem, true)
							operations = append(operations, []interface{}{"i", prevKey, strippedItem})
						}
					}
					break
				}
			}
		}
	}
	return operations
}

// CompareRangeItemsForChanges compares two range items and returns a map of field changes.
// For heterogeneous ranges, uses the item's _sk field to look up its specific statics.
func CompareRangeItemsForChanges(oldItem, newItem interface{}, statics interface{}) map[string]interface{} {
	changes := make(map[string]interface{})

	oldItemNode, ok1 := oldItem.(*TreeNode)
	newItemNode, ok2 := newItem.(*TreeNode)

	if !ok1 || !ok2 {
		return changes
	}

	// Get effective statics for the new item (handles both homogeneous and heterogeneous)
	effectiveStatics := getItemStatics(newItemNode, statics)

	// Find key position to skip it
	keyPos := FindKeyPositionFromStatics(effectiveStatics)
	keyPosStr := fmt.Sprintf("%d", keyPos)

	// Compare each field (except the key field)
	for fieldKey, newValue := range newItemNode.Dynamics {
		if fieldKey == keyPosStr {
			continue // Skip the key field
		}

		oldValue, exists := oldItemNode.GetDynamic(fieldKey)
		if !exists || !DeepEqual(oldValue, newValue) {
			// Strip statics from nested tree nodes since client already has them cached
			if newTreeNode, ok := newValue.(*TreeNode); ok {
				handleNestedTreeNodeChange(fieldKey, oldValue, newTreeNode, exists, changes)
			} else {
				changes[fieldKey] = newValue
			}
		}
	}

	// Also check for fields that were removed (in old but not in new).
	// This handles cases like unchecking a checkbox: the "checked" attribute
	// field exists in old but is absent from new (or is empty string).
	for fieldKey, oldValue := range oldItemNode.Dynamics {
		if fieldKey == keyPosStr {
			continue // Skip the key field
		}
		if _, exists := newItemNode.Dynamics[fieldKey]; !exists {
			// Field was removed - send empty string to indicate removal
			// Only report if old value was meaningful (not empty string, nil, etc.)
			if isMeaningfulValue(oldValue) {
				changes[fieldKey] = ""
			}
		}
	}

	return changes
}

// handleNestedTreeNodeChange handles changes in nested TreeNode fields.
// Uses fingerprint comparison to detect static structure changes.
func handleNestedTreeNodeChange(
	fieldKey string,
	oldValue interface{},
	newTreeNode *TreeNode,
	exists bool,
	changes map[string]interface{},
) {
	// Check if old value is also a TreeNode
	oldTreeNode, oldIsTree := oldValue.(*TreeNode)

	// If old value is NOT a TreeNode (e.g., empty string "", nil, or non-existent),
	// but new value IS a TreeNode, we need to send the full new TreeNode WITH statics,
	// because the client doesn't have these statics cached for this field.
	// This handles transitions like:
	// - "" -> {"s":["checked"]} (empty string to TreeNode)
	// - nil -> {"s":["checked"]} (non-existent field to TreeNode)
	if !oldIsTree {
		// Transition from non-TreeNode (or non-existent) to TreeNode - send full new value with statics
		changes[fieldKey] = PrepareTreeForClient(newTreeNode, false)
		return
	}

	stripped := PrepareTreeForClient(newTreeNode, true)

	// If stripping results in empty, check if this is a meaningful change
	if IsEmpty(stripped) {
		// Check if old value would also strip to empty
		if exists && oldIsTree {
			oldStripped := PrepareTreeForClient(oldTreeNode, true)
			if IsEmpty(oldStripped) {
				// Both old and new strip to empty (static-only).
				// Use fingerprint comparison to detect if statics changed.
				// e.g., old: {"s":["checked"]} vs new: {"s":[]}
				// Both strip to empty but the visual output is different.
				if ClientNeedsStatics(oldTreeNode, newTreeNode) {
					// Structure fingerprints differ - statics changed.
					// Send empty string to indicate the field should be cleared/re-rendered.
					changes[fieldKey] = ""
				}
				// If fingerprints are the same, truly no change - skip it
				return
			}
		}
		// Old doesn't exist or had dynamics, send empty string to indicate removal of dynamics
		changes[fieldKey] = ""
	} else {
		changes[fieldKey] = stripped
	}
}

// Helper functions

// createItemKeyMap creates a map of items indexed by their keys.
func createItemKeyMap(items []interface{}, statics interface{}) map[string]interface{} {
	itemsByKey := make(map[string]interface{})
	for _, item := range items {
		if key, ok := GetItemKey(item, statics); ok {
			itemsByKey[key] = item
		}
	}
	return itemsByKey
}

// stripStaticsFromOperations removes statics from all operations.
// Range operations have format: ['a'/'p'/'i', items, statics?, metadata?]
// We strip the statics (index 2) when client has already seen them.
func stripStaticsFromOperations(operations []interface{}) []interface{} {
	result := make([]interface{}, len(operations))
	for i, op := range operations {
		opArr, ok := op.([]interface{})
		if !ok || len(opArr) < 2 {
			result[i] = op
			continue
		}

		opType, _ := opArr[0].(string)
		switch opType {
		case "a", "p": // append/prepend: ['a'/'p', items, statics?, metadata?]
			if len(opArr) >= 3 {
				// Strip statics at index 2, keep metadata at index 3 if present
				strippedOp := []interface{}{opArr[0], opArr[1]}
				if len(opArr) >= 4 {
					// Keep metadata (index 3)
					strippedOp = append(strippedOp, nil, opArr[3])
				}
				result[i] = strippedOp
			} else {
				result[i] = opArr
			}
		case "i": // insert: ['i', afterId, data, statics?]
			if len(opArr) >= 4 {
				// Strip statics at index 3
				result[i] = []interface{}{opArr[0], opArr[1], opArr[2]}
			} else {
				result[i] = opArr
			}
		default:
			// Other operations (r, u, o) don't have statics
			result[i] = op
		}
	}
	return result
}
