// Package diff provides tree comparison and differential update generation for LiveTemplate.
// It generates minimal operations (insert, update, remove, reorder) to transform one tree into another.
package diff

import (
	"sort"
	"strconv"
)

// rangeContext holds pre-computed data for a single range diff operation,
// avoiding redundant key extraction, map creation, and statics parsing.
type rangeContext struct {
	oldItems  []interface{}
	newItems  []interface{}
	statics   interface{}
	metadata  map[string]interface{}
	oldKeys   []string
	newKeys   []string
	oldByKey  map[string]interface{}
	newByKey  map[string]interface{}
	keyPos    int
	keyPosStr string
	addedKeys []string
}

func newRangeContext(oldItems, newItems []interface{}, statics interface{}, metadata map[string]interface{}) *rangeContext {
	ctx := &rangeContext{
		oldItems: oldItems,
		newItems: newItems,
		statics:  statics,
		metadata: metadata,
		oldByKey: make(map[string]interface{}, len(oldItems)),
		newByKey: make(map[string]interface{}, len(newItems)),
		oldKeys:  make([]string, 0, len(oldItems)),
		newKeys:  make([]string, 0, len(newItems)),
	}
	ctx.keyPos = FindKeyPositionFromStatics(statics)
	ctx.keyPosStr = strconv.Itoa(ctx.keyPos)

	for _, item := range oldItems {
		if key, ok := getItemKeyWithPos(item, ctx.keyPos, ctx.keyPosStr); ok {
			ctx.oldKeys = append(ctx.oldKeys, key)
			ctx.oldByKey[key] = item
		}
	}
	for _, item := range newItems {
		if key, ok := getItemKeyWithPos(item, ctx.keyPos, ctx.keyPosStr); ok {
			ctx.newKeys = append(ctx.newKeys, key)
			ctx.newByKey[key] = item
		}
	}

	// Compute added keys (in new but not in old)
	oldKeySet := make(map[string]struct{}, len(ctx.oldKeys))
	for _, k := range ctx.oldKeys {
		oldKeySet[k] = struct{}{}
	}
	for _, k := range ctx.newKeys {
		if _, exists := oldKeySet[k]; !exists {
			ctx.addedKeys = append(ctx.addedKeys, k)
		}
	}

	return ctx
}

func (ctx *rangeContext) getItemKey(item interface{}) (string, bool) {
	return getItemKeyWithPos(item, ctx.keyPos, ctx.keyPosStr)
}

// GenerateRangeDifferentialOperations generates differential operations for range constructs.
// stripStatics: if true, removes "s" keys from operations (client has cached them)
// if false, keeps "s" keys (client hasn't seen this structure yet)
// This is the main orchestrator (30 lines).
//
// Returns nil when differential operations cannot fully express the change,
// signaling that the caller should fall back to full tree replacement.
func GenerateRangeDifferentialOperations(oldValue, newValue interface{}, stripStatics bool) []interface{} {
	oldItems, newItems, statics, metadata := extractRangeData(oldValue, newValue)
	if oldItems == nil || newItems == nil {
		return nil
	}

	ctx := newRangeContext(oldItems, newItems, statics, metadata)

	if isPureReorderingCtx(ctx) {
		return []interface{}{[]interface{}{"o", ctx.newKeys}}
	}

	// FIX for issue #111: Check for complex insertion patterns BEFORE generating
	// any differential operations.
	if len(ctx.addedKeys) > 0 && isComplexInsertionPatternCtx(ctx) {
		return nil
	}

	operations := make([]interface{}, 0, 4)
	operations = generateRemovalOps(ctx, operations)
	operations = generateUpdateOps(ctx, operations)
	operations = generateInsertionOps(ctx, operations)

	if sameKeySet(ctx.oldKeys, ctx.newKeys) && HasReordering(ctx.oldKeys, ctx.newKeys) {
		operations = append(operations, []interface{}{"o", ctx.newKeys})
	}

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
	// Use Range.Statics (item template) for key extraction, not Statics (outer wrapper).
	// oldNode.Statics is the outer wrapper (e.g., ["<ul>", "</ul>"]),
	// but oldNode.Range.Statics is the item template (e.g., ["<li id=\"", "...", "</li>"])
	// which contains the key attribute (id=", data-key=", etc.) needed for item matching.
	if oldNode.Range.Statics != nil {
		statics = oldNode.Range.Statics
	} else {
		statics = oldNode.Statics // Fallback if Range.Statics is nil
	}

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
		if len(newNode.Statics) > 0 {
			statics = newNode.Statics
		} else if newNode.Range != nil && len(newNode.Range.Statics) > 0 {
			// Fall back to Range.Statics which should always have item template
			statics = newNode.Range.Statics
		}
		// If all are empty/nil, statics remains as oldNode.Statics (minimal)
	} else if staticsSlice, ok := statics.([]string); ok && len(staticsSlice) == 0 {
		// Fallback if old statics empty
		if len(newNode.Statics) > 0 {
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

func generateRemovalOps(ctx *rangeContext, operations []interface{}) []interface{} {
	sortedOldKeys := make([]string, len(ctx.oldKeys))
	copy(sortedOldKeys, ctx.oldKeys)
	sort.Strings(sortedOldKeys)

	for _, key := range sortedOldKeys {
		if _, exists := ctx.newByKey[key]; !exists {
			operations = append(operations, []interface{}{"r", key})
		}
	}
	return operations
}

func generateUpdateOps(ctx *rangeContext, operations []interface{}) []interface{} {
	sortedNewKeys := make([]string, len(ctx.newKeys))
	copy(sortedNewKeys, ctx.newKeys)
	sort.Strings(sortedNewKeys)

	for _, key := range sortedNewKeys {
		newItem := ctx.newByKey[key]
		if oldItem, exists := ctx.oldByKey[key]; exists {
			changes := compareRangeItemsWithKeyPos(oldItem, newItem, ctx.keyPos, ctx.keyPosStr)
			if len(changes) > 0 {
				// Include all changes, even empty strings — they signal field removal
				// (e.g., removing "checked" attribute when toggling a checkbox off).
				operations = append(operations, []interface{}{"u", key, changes})
			}
		}
	}
	return operations
}

func generateInsertionOps(ctx *rangeContext, operations []interface{}) []interface{} {
	if len(ctx.addedKeys) == 0 {
		return operations
	}

	if len(ctx.oldItems) == 0 {
		return handleEmptyToItemsTransition(ctx.newItems, ctx.statics, ctx.metadata, operations)
	}

	return handleIncrementalInsertionsCtx(ctx, operations)
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

func handleIncrementalInsertionsCtx(ctx *rangeContext, operations []interface{}) []interface{} {
	if areAllItemsAtStartCtx(ctx) {
		return handlePrependOperation(ctx.addedKeys, ctx.newByKey, ctx.statics, operations)
	}

	if areAllItemsAtEndCtx(ctx) {
		return handleAppendOperation(ctx.addedKeys, ctx.newByKey, ctx.statics, operations)
	}

	return handleIndividualInsertionsCtx(ctx, operations)
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
			// Keep nested statics for new items - client hasn't seen these items before
			// so nested TreeNode structures (like conditionals) need their statics.
			itemsToPrepend = append(itemsToPrepend, PrepareTreeForClient(item, false))
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
			// Keep nested statics for new items - client hasn't seen these items before
			// so nested TreeNode structures (like conditionals) need their statics.
			itemsToAppend = append(itemsToAppend, PrepareTreeForClient(item, false))
		}
	}
	// Use 'a' operation for appending (O(1) on client)
	// Format: ['a', items, statics] - statics describe how to render items
	operations = append(operations, []interface{}{"a", itemsToAppend, statics})
	return operations
}

func handleIndividualInsertionsCtx(ctx *rangeContext, operations []interface{}) []interface{} {
	for _, key := range ctx.addedKeys {
		if newItem, exists := ctx.newByKey[key]; exists {
			for i, item := range ctx.newItems {
				if itemKey, ok := ctx.getItemKey(item); ok && itemKey == key {
					if i == 0 {
						preparedItem := PrepareTreeForClient(newItem, false)
						operations = append(operations, []interface{}{"p", []interface{}{preparedItem}, ctx.statics})
					} else {
						if prevKey, ok := ctx.getItemKey(ctx.newItems[i-1]); ok {
							preparedItem := PrepareTreeForClient(newItem, false)
							operations = append(operations, []interface{}{"i", prevKey, preparedItem})
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
func CompareRangeItemsForChanges(oldItem, newItem interface{}, statics interface{}) map[string]interface{} {
	keyPos := FindKeyPositionFromStatics(statics)
	keyPosStr := strconv.Itoa(keyPos)
	return compareRangeItemsWithKeyPos(oldItem, newItem, keyPos, keyPosStr)
}

func compareRangeItemsWithKeyPos(oldItem, newItem interface{}, keyPos int, keyPosStr string) map[string]interface{} {
	changes := make(map[string]interface{})

	oldItemNode, ok1 := oldItem.(*TreeNode)
	newItemNode, ok2 := newItem.(*TreeNode)

	if !ok1 || !ok2 {
		return changes
	}

	for fieldKey, newValue := range newItemNode.Dynamics {
		if keyPos >= 0 && fieldKey == keyPosStr {
			continue
		}

		oldValue, exists := oldItemNode.GetDynamic(fieldKey)
		if !exists || !DeepEqual(oldValue, newValue) {
			if newTreeNode, ok := newValue.(*TreeNode); ok {
				handleNestedTreeNodeChange(fieldKey, oldValue, newTreeNode, exists, changes)
			} else {
				changes[fieldKey] = newValue
			}
		}
	}

	// Check for fields removed (in old but not in new), e.g. unchecking a checkbox.
	for fieldKey, oldValue := range oldItemNode.Dynamics {
		if keyPos >= 0 && fieldKey == keyPosStr {
			continue
		}
		if _, exists := newItemNode.Dynamics[fieldKey]; !exists {
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
					// Send the full new TreeNode WITH statics so client can update structure.
					// This handles cases like conditional branch changes within range items.
					changes[fieldKey] = PrepareTreeForClient(newTreeNode, false)
				}
				// If fingerprints are the same, truly no change - skip it
				return
			}
		}
		// Old doesn't exist or had dynamics, send empty string to indicate removal of dynamics
		changes[fieldKey] = ""
	} else {
		// Check if structure changed (different statics needed) even when dynamics exist.
		// This handles conditional branch changes within range items where both the
		// structure (statics) AND content (dynamics) change simultaneously.
		// e.g., {{if .HasError}}<span class="error-message">{{.Error}}</span>{{else}}<span class="status">Pending</span>{{end}}
		// When HasError changes, we need to send new statics, not just new dynamics.
		if oldIsTree && ClientNeedsStatics(oldTreeNode, newTreeNode) {
			// Statics changed - send full tree with new statics
			changes[fieldKey] = PrepareTreeForClient(newTreeNode, false)
		} else {
			changes[fieldKey] = stripped
		}
	}
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
