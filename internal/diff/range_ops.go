// Package diff provides tree comparison and differential update generation for LiveTemplate.
// It generates minimal operations (insert, update, remove, reorder) to transform one tree into another.
package diff

import (
	"sort"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// rangeContext holds pre-computed data for a single range diff operation,
// avoiding redundant key extraction, map creation, and statics parsing.
type rangeContext struct {
	oldItems []interface{}
	newItems []interface{}
	statics  interface{}
	metadata map[string]interface{}
	oldKeys  []string
	newKeys  []string
	// INVARIANT: oldByKey is nil on the stream path. Helpers that need only
	// presence read oldKeySet instead. Phase 3+ helpers added here MUST NOT
	// assume oldByKey is populated when fromStream is conceptually true.
	oldByKey  map[string]interface{}
	newByKey  map[string]interface{}
	oldKeySet map[string]struct{}
	keyPos    int
	addedKeys []string
}

func newRangeContext(oldItems, newItems []interface{}, statics interface{}, metadata map[string]interface{}) *rangeContext {
	ctx := &rangeContext{
		oldItems:  oldItems,
		newItems:  newItems,
		statics:   statics,
		metadata:  metadata,
		oldByKey:  make(map[string]interface{}, len(oldItems)),
		newByKey:  make(map[string]interface{}, len(newItems)),
		oldKeys:   make([]string, 0, len(oldItems)),
		newKeys:   make([]string, 0, len(newItems)),
		oldKeySet: make(map[string]struct{}, len(oldItems)),
	}
	ctx.keyPos = FindKeyPositionFromStatics(statics)

	for _, item := range oldItems {
		if key, ok := getItemKeyWithPos(item, ctx.keyPos); ok {
			ctx.oldKeys = append(ctx.oldKeys, key)
			ctx.oldByKey[key] = item
			ctx.oldKeySet[key] = struct{}{}
		}
	}
	for _, item := range newItems {
		if key, ok := getItemKeyWithPos(item, ctx.keyPos); ok {
			ctx.newKeys = append(ctx.newKeys, key)
			ctx.newByKey[key] = item
		}
	}

	for _, k := range ctx.newKeys {
		if _, exists := ctx.oldKeySet[k]; !exists {
			ctx.addedKeys = append(ctx.addedKeys, k)
		}
	}

	return ctx
}

func newStreamRangeContext(oldKeys []string, newItems []interface{}, statics interface{}, metadata map[string]interface{}) *rangeContext {
	ctx := &rangeContext{
		newItems:  newItems,
		statics:   statics,
		metadata:  metadata,
		oldKeys:   oldKeys,
		newByKey:  make(map[string]interface{}, len(newItems)),
		newKeys:   make([]string, 0, len(newItems)),
		oldKeySet: make(map[string]struct{}, len(oldKeys)),
	}
	ctx.keyPos = FindKeyPositionFromStatics(statics)

	for _, k := range oldKeys {
		ctx.oldKeySet[k] = struct{}{}
	}
	for _, item := range newItems {
		if key, ok := getItemKeyWithPos(item, ctx.keyPos); ok {
			ctx.newKeys = append(ctx.newKeys, key)
			ctx.newByKey[key] = item
		}
	}
	for _, k := range ctx.newKeys {
		if _, exists := ctx.oldKeySet[k]; !exists {
			ctx.addedKeys = append(ctx.addedKeys, k)
		}
	}

	return ctx
}

func (ctx *rangeContext) getItemKey(item interface{}) (string, bool) {
	return getItemKeyWithPos(item, ctx.keyPos)
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

	// If items exist but none yield extractable keys (e.g., map items rather
	// than *TreeNode), the diff engine cannot produce ops — signal fallback.
	if (len(oldItems) > 0 && len(ctx.oldKeys) == 0) ||
		(len(newItems) > 0 && len(ctx.newKeys) == 0) {
		return nil
	}

	// Kept-item changes can't be encoded as differential ops; full-tree fallback (spec §5c/§5d).
	if hasKeptItemChanged(ctx) {
		return nil
	}

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
	operations = generateInsertionOps(ctx, operations)

	if sameKeySet(ctx.oldKeys, ctx.newKeys) && HasReordering(ctx.oldKeys, ctx.newKeys) {
		operations = append(operations, []interface{}{"o", ctx.newKeys})
	}

	if stripStatics {
		operations = stripStaticsFromOperations(operations)
	}

	return operations
}

// GenerateRangeStreamOperations generates differential operations for a range
// whose retained "old side" is a RangeStreamState snapshot (keys + per-item
// content hashes + structure-fingerprint), not a slice of cached item bodies.
//
// Return semantics: nil signals "stream mode not applicable, caller must fall
// back to GenerateRangeDifferentialOperations" (streamState is nil or has
// inconsistent Keys/Hashes lengths, an item lacks an extractable key, an
// item's statics fingerprint diverges from streamState.Fingerprint per §5d,
// or the insertion pattern is too scattered ≥4 distinct points). An empty
// non-nil slice signals "stream mode produced no ops" (every item unchanged).
// Phase 3 callers MUST check ops != nil before treating an empty slice as a
// no-op.
//
// Update payloads carry the full per-item dynamics map (with "" for absent
// positions) per proposal §5c; absence means "this position is empty/cleared",
// not "this position is unchanged".
func GenerateRangeStreamOperations(
	streamState *build.RangeStreamState,
	newItems []interface{},
	statics interface{},
	metadata map[string]interface{},
	stripStatics bool,
) []interface{} {
	if streamState == nil {
		return nil
	}
	if len(streamState.Hashes) != len(streamState.Keys) {
		return nil
	}

	keyPos := FindKeyPositionFromStatics(statics)
	newKeys := make([]string, 0, len(newItems))
	newHashes := make([]uint64, 0, len(newItems))

	for _, item := range newItems {
		itemNode, ok := item.(*TreeNode)
		if !ok {
			return nil
		}
		if build.CalculateStaticsFingerprint(itemNode) != streamState.Fingerprint {
			return nil
		}
		key, ok := getItemKeyWithPos(item, keyPos)
		if !ok {
			return nil
		}
		newKeys = append(newKeys, key)
		newHashes = append(newHashes, keys.ItemHashUint64(itemNode.Dynamics))
	}

	ctx := newStreamRangeContext(streamState.Keys, newItems, statics, metadata)

	oldHashByKey := make(map[string]uint64, len(streamState.Keys))
	for i, k := range streamState.Keys {
		oldHashByKey[k] = streamState.Hashes[i]
	}

	hasReorder := sameKeySet(streamState.Keys, newKeys) && HasReordering(streamState.Keys, newKeys)
	if hasReorder {
		anyHashChanged := false
		for i, k := range newKeys {
			if oldHashByKey[k] != newHashes[i] {
				anyHashChanged = true
				break
			}
		}
		if !anyHashChanged {
			return []interface{}{[]interface{}{"o", newKeys}}
		}
	}

	if len(ctx.addedKeys) > 0 && isComplexInsertionPatternCtx(ctx) {
		return nil
	}

	operations := make([]interface{}, 0, 4)
	operations = generateStreamRemovalOps(streamState.Keys, ctx.newByKey, operations)
	operations = generateStreamUpdateOps(newItems, newKeys, newHashes, oldHashByKey, ctx.keyPos, operations)
	operations = generateInsertionOps(ctx, operations)

	if hasReorder {
		operations = append(operations, []interface{}{"o", newKeys})
	}

	if stripStatics {
		operations = stripStaticsFromOperations(operations)
	}

	return operations
}

// generateStreamRemovalOps: sorted for determinism (mirrors generateRemovalOps).
func generateStreamRemovalOps(oldKeys []string, newByKey map[string]interface{}, operations []interface{}) []interface{} {
	sortedOldKeys := make([]string, len(oldKeys))
	copy(sortedOldKeys, oldKeys)
	sort.Strings(sortedOldKeys)

	for _, key := range sortedOldKeys {
		if _, exists := newByKey[key]; !exists {
			operations = append(operations, []interface{}{"r", key})
		}
	}
	return operations
}

// generateStreamUpdateOps emits whole-item dynamics on hash mismatch (proposal §5c).
func generateStreamUpdateOps(
	newItems []interface{},
	newKeys []string,
	newHashes []uint64,
	oldHashByKey map[string]uint64,
	keyPos int,
	operations []interface{},
) []interface{} {
	for i, key := range newKeys {
		oldHash, exists := oldHashByKey[key]
		if !exists || oldHash == newHashes[i] {
			continue
		}
		itemNode, ok := newItems[i].(*TreeNode)
		if !ok {
			continue
		}
		payload := dynamicsToUpdatePayload(itemNode.Dynamics, keyPos)
		operations = append(operations, []interface{}{"u", key, payload})
	}
	return operations
}

// dynamicsToUpdatePayload encodes per proposal §5c: nil → "", nested *TreeNode → stripped, key position omitted.
func dynamicsToUpdatePayload(dynamics []interface{}, keyPos int) map[string]interface{} {
	payload := make(map[string]interface{}, len(dynamics))
	for i, val := range dynamics {
		if i == keyPos {
			continue
		}
		fieldKey := build.PositionKey(i)
		switch v := val.(type) {
		case nil:
			payload[fieldKey] = ""
		case *TreeNode:
			// clientHasStatics=true: fingerprint check above guarantees homogeneous statics.
			payload[fieldKey] = PrepareTreeForClient(v, true)
		default:
			payload[fieldKey] = v
		}
	}
	return payload
}

// dispatchStreamMode runs the shared stream-mode diff. (nil, false) signals fallback; empty ops mean no-change.
func dispatchStreamMode(streamState *RangeStreamState, newValue interface{}, clientHasRangeStatics bool) ([]interface{}, bool) {
	newItems, statics, metadata := extractNewSideRange(newValue)
	if newItems == nil {
		return nil, false
	}
	ops := GenerateRangeStreamOperations(streamState, newItems, statics, metadata, clientHasRangeStatics)
	if ops == nil {
		return nil, false
	}
	return ops, true
}

func handleStreamModeRange(oldTree, newTree *TreeNode, changes *TreeNode) bool {
	clientHasRangeStatics := !ClientNeedsStatics(oldTree, newTree)
	diffOps, ok := dispatchStreamMode(oldTree.Range.StreamState, newTree, clientHasRangeStatics)
	if !ok {
		*changes = *newTree
		return true
	}
	if len(diffOps) > 0 {
		changes.Range = &RangeData{Items: diffOps}
		if !clientHasRangeStatics {
			changes.Statics = newTree.Statics
		}
	}
	return true
}

func handleStreamModeRangeMatch(k int, oldNode *TreeNode, newValue interface{}, changes *TreeNode) {
	clientHasRangeStatics := !clientNeedsStaticsForValue(oldNode, newValue)
	diffOps, ok := dispatchStreamMode(oldNode.Range.StreamState, newValue, clientHasRangeStatics)
	if !ok {
		// Het-fallback or extraction failure — emit full new value at position k,
		// mirroring the top-level path's *changes = *newTree fallback.
		changes.SetDynamic(k, newValue)
		return
	}
	if len(diffOps) > 0 {
		changes.SetDynamic(k, diffOps)
	}
}

// extractNewSideRange returns the new-side range data for stream-mode
// dispatch. Mirrors the new-side branch of extractRangeData. Returns nil
// items if newValue is not a valid range.
func extractNewSideRange(newValue interface{}) (
	newItems []interface{},
	statics interface{},
	metadata map[string]interface{},
) {
	newNode, ok := newValue.(*TreeNode)
	if !ok || !newNode.HasRange() || newNode.Range == nil {
		return nil, nil, nil
	}
	newItems = newNode.Range.Items
	if newNode.Range.Statics != nil {
		statics = newNode.Range.Statics
	} else {
		statics = newNode.Statics
	}
	if newNode.Metadata != nil {
		metadata = map[string]interface{}{
			"idKey": newNode.Metadata.IDKey,
		}
	}
	return
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

// hasKeptItemChanged reports whether any item present in both renders has a
// different statics fingerprint or content hash. Both checks matter: statics
// catch conditional-branch flips (e.g. {{if .Done}}<s>{{end}}) that leave
// Dynamics identical; hash catches content changes within the same structure.
// Non-*TreeNode items default to true (safety-biased fallback).
func hasKeptItemChanged(ctx *rangeContext) bool {
	for _, key := range ctx.newKeys {
		oldItem, exists := ctx.oldByKey[key]
		if !exists {
			continue
		}
		newItem := ctx.newByKey[key]
		oldNode, oldOk := oldItem.(*TreeNode)
		newNode, newOk := newItem.(*TreeNode)
		if !oldOk || !newOk {
			return true
		}
		if build.CalculateStaticsFingerprint(oldNode) != build.CalculateStaticsFingerprint(newNode) {
			return true
		}
		if keys.ItemHashUint64(oldNode.Dynamics) != keys.ItemHashUint64(newNode.Dynamics) {
			return true
		}
	}
	return false
}

func generateInsertionOps(ctx *rangeContext, operations []interface{}) []interface{} {
	if len(ctx.addedKeys) == 0 {
		return operations
	}

	if len(ctx.oldKeySet) == 0 {
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
