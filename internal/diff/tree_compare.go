package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
)

// CompareTreesAndGetChangesWithPath compares two tree structures and returns minimal changes.
// This is the main orchestrator function that coordinates the comparison process.
//
// Parameters:
//   - oldTree: The previous tree state (may be nil)
//   - newTree: The current tree state (may be nil)
//   - insideNewStructure: True if we're inside a structure the client hasn't seen
//   - currentPath: Dot-separated path to current location in tree (e.g., "field.nested")
//   - rangeMatches: Map of paths to matched range constructs (for differential operations)
//
// Returns:
//   - TreeNode containing only the changed fields (empty if no changes)
//
// The function handles:
//   - Top-level range constructs with differential operations
//   - Nil tree cases (tree removal)
//   - Deep comparison of dynamic fields
//   - Fingerprint-based optimization (strips statics when client has them cached)
func CompareTreesAndGetChangesWithPath(
	oldTree, newTree *TreeNode,
	insideNewStructure bool,
	currentPath string,
	rangeMatches map[string]string,
) *TreeNode {
	changes := &TreeNode{}

	// Handle top-level range constructs
	if handleTopLevelRange(oldTree, newTree, currentPath, rangeMatches, changes) {
		return changes
	}

	// Handle nil trees
	if newTree == nil {
		return &TreeNode{}
	}

	// Check for structural changes that involve dynamic field changes (like range↔else).
	// When the set of dynamic keys differs AND structure changed, client needs the full new tree.
	// This handles cases like:
	//   - Range→Else: old={0: range_data}, new={statics_only} (field "0" removed)
	//   - Else→Range: old={statics_only}, new={0: range_data} (field "0" added)
	//   - Conditional branch change where dynamics appear or disappear
	//
	// We only do this when:
	// 1. Structure fingerprint changed (ClientNeedsStatics)
	// 2. The dynamic field keys differ (added or removed)
	// This avoids triggering for normal range operations (where field keys are the same).
	if oldTree != nil && ClientNeedsStatics(oldTree, newTree) && hasDynamicsChanged(oldTree, newTree) {
		return newTree
	}

	// Compare dynamic segments
	compareDynamicSegments(oldTree, newTree, insideNewStructure, currentPath, rangeMatches, changes)

	return changes
}

// handleTopLevelRange handles top-level range construct comparisons.
// Returns true if this was a range case and changes were set.
func handleTopLevelRange(
	oldTree, newTree *TreeNode,
	currentPath string,
	rangeMatches map[string]string,
	changes *TreeNode,
) bool {
	if oldTree == nil || newTree == nil {
		return false
	}

	// Case 1: newTree is a range (with or without items)
	// This covers: range→range transitions and else→range transitions
	if newTree.HasRange() && newTree.HasStatics() {
		if oldTree.HasRange() && oldTree.HasStatics() {
			// Both are ranges - use differential operations if matched
			if _, isMatched := rangeMatches[currentPath]; isMatched {
				return handleMatchedRanges(oldTree, newTree, changes)
			}
		} else {
			// else→range: newTree is a range but oldTree isn't
			// Return full new tree so client can replace the else content with range items
			*changes = *newTree
			return true
		}
	}

	// Case 2: range→else transition
	// oldTree was a range (had items) but newTree is NOT a range (it's the else clause content)
	// This happens when collection becomes empty: {{range .Items}}...{{else}}No items{{end}}
	if oldTree.HasRange() && oldTree.HasStatics() && !newTree.HasRange() {
		// Return full new tree so client can replace range items with else content
		*changes = *newTree
		return true
	}

	return false
}

// handleMatchedRanges generates differential operations for matched range constructs.
// Uses fingerprint comparison to determine if client needs range statics.
func handleMatchedRanges(oldTree, newTree *TreeNode, changes *TreeNode) bool {
	// Use fingerprint comparison to determine if client has range statics cached.
	// If the structure fingerprints match, client already has the statics.
	clientHasRangeStatics := !ClientNeedsStatics(oldTree, newTree)

	// Strip statics if client already has them cached
	diffOps := GenerateRangeDifferentialOperations(oldTree, newTree, clientHasRangeStatics)

	if len(diffOps) > 0 {
		// Return the operations directly - the entire tree is the range
		changes.Range = &RangeData{Items: diffOps}

		// Only include root-level statics if client needs them (structure changed)
		if !clientHasRangeStatics {
			changes.Statics = newTree.Statics
		}
		return true
	} else {
		// No operations generated - check for empty range cases
		if (newTree.Range == nil || len(newTree.Range.Items) == 0) &&
			(oldTree.Range == nil || len(oldTree.Range.Items) == 0) {
			// Both empty, no change
			return true
		}
		// Fallback: return the new tree
		*changes = *newTree
		return true
	}
}

// compareDynamicSegments compares all dynamic fields between old and new trees.
func compareDynamicSegments(
	oldTree, newTree *TreeNode,
	insideNewStructure bool,
	currentPath string,
	rangeMatches map[string]string,
	changes *TreeNode,
) {
	// Pre-allocate changes slice to avoid repeated growth
	changes.GrowDynamics(len(newTree.Dynamics))
	for i, newValue := range newTree.Dynamics {
		if newValue == nil {
			continue
		}

		// Build full path for this field
		posKey := build.PositionKey(i)
		fieldPath := buildFieldPath(currentPath, posKey)

		var oldValue interface{}
		var exists bool
		if oldTree != nil {
			oldValue, exists = oldTree.GetDynamic(i)
		}

		if !exists {
			// Field is NEW compared to last update
			handleNewField(i, newValue, insideNewStructure, changes)
		} else if !DeepEqual(oldValue, newValue) {
			// Field exists but changed
			handleChangedField(
				i, oldValue, newValue,
				fieldPath,
				insideNewStructure,
				rangeMatches,
				changes,
			)
		}
	}
	// Remove trailing nil entries from pre-allocated changes
	changes.TrimDynamics()
}

// buildFieldPath constructs the full path for a field.
func buildFieldPath(currentPath, k string) string {
	if currentPath != "" {
		return currentPath + "." + k
	}
	return k
}

// handleNewField processes a field that is new (not in old tree).
func handleNewField(
	k int,
	newValue interface{},
	insideNewStructure bool,
	changes *TreeNode,
) {
	// If we're inside a new structure, client has never seen this, so include statics
	if insideNewStructure {
		changes.SetDynamic(k, newValue)
		return
	}

	// Handle tree node values - send full value with statics for new fields
	if newTreeNode, ok := newValue.(*TreeNode); ok {
		valueToSet, _ := handleStructureValue(newTreeNode, false)
		changes.SetDynamic(k, valueToSet)
		return
	}

	// Handle map values
	if m, ok := newValue.(map[string]interface{}); ok {
		valueToSet, _ := handleStructureValue(m, false)
		changes.SetDynamic(k, valueToSet)
		return
	}

	// Primitive value
	changes.SetDynamic(k, newValue)
}

// isStrippedValueEmpty checks if a stripped value is considered empty.
// Returns true if the value is an empty map, empty string, or empty TreeNode.
//
// This function handles all possible return types from PrepareTreeForClient:
//   - map[string]interface{} (serialized form)
//   - *TreeNode (when passed a TreeNode directly)
//   - string (empty indicator)
func isStrippedValueEmpty(stripped interface{}) bool {
	// Check for empty map (most common case for serialized form)
	if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
		return true
	}
	// Check for empty string (explicit empty indicator)
	if strippedStr, ok := stripped.(string); ok && strippedStr == "" {
		return true
	}
	// Check for empty TreeNode (when PrepareTreeForClient returns TreeNode directly)
	if treeNode, ok := stripped.(*TreeNode); ok {
		return !treeNode.HasDynamics() && !treeNode.HasStatics()
	}
	return false
}

// handleStructureValue handles TreeNode or map values.
// This function implements the core optimization: stripping statics when the client
// already has them cached.
//
// Parameters:
//   - newValue: The structure value (TreeNode or map) to process (must not be nil)
//   - clientHasStructure: True if client has already seen this structure
//
// Returns:
//   - valueToSet: The value to send to client (stripped or full, or empty string)
//   - shouldTrack: True if this structure should be tracked (always false now, kept for compatibility)
//
// Behavior:
//   - If client has structure: returns stripped value (dynamics only)
//   - If client doesn't have structure: returns full value (with statics)
//   - If stripped value is empty: returns empty string to signal static-only change
//
// Preconditions:
//   - newValue must not be nil (caller's responsibility to ensure)
func handleStructureValue(
	newValue interface{},
	clientHasStructure bool,
) (valueToSet interface{}, shouldTrack bool) {
	// Defensive check: newValue should never be nil, but handle gracefully
	if newValue == nil {
		return "", false
	}

	// Prepare the value for client (strip statics if appropriate)
	stripped := PrepareTreeForClient(newValue, true)

	if clientHasStructure {
		// Client already has structure - use stripped version
		if isStrippedValueEmpty(stripped) {
			return "", false
		}
		return stripped, false
	}

	// Client doesn't have structure - send full value WITH statics
	// Even if there are no dynamics (stripped is empty), we still need to
	// send the tree with statics so the client knows what to render.
	// Only return "" if the newValue itself has no content.
	if treeNode, ok := newValue.(*TreeNode); ok {
		if !treeNode.HasStatics() && !treeNode.HasDynamics() && !treeNode.HasRange() {
			return "", false
		}
		return newValue, false
	}
	// For non-TreeNode values, fall back to stripped check
	if isStrippedValueEmpty(stripped) {
		return "", false
	}
	return newValue, false
}

// handleChangedField processes a field that exists but changed value.
func handleChangedField(
	k int,
	oldValue, newValue interface{},
	fieldPath string,
	insideNewStructure bool,
	rangeMatches map[string]string,
	changes *TreeNode,
) {
	// Check if this field has a range construct match
	if _, isRangeMatch := rangeMatches[fieldPath]; isRangeMatch {
		handleRangeMatch(k, oldValue, newValue, changes)
		return
	}

	// Check if both old and new values are TreeNodes (nested structures)
	oldTreeNodePtr, newTreeNodePtr, bothAreTreeNodes := extractTreeNodePair(oldValue, newValue)
	if bothAreTreeNodes {
		handleNestedTreeNodes(
			k, oldTreeNodePtr, newTreeNodePtr,
			fieldPath, insideNewStructure,
			rangeMatches,
			changes,
		)
		return
	}

	// New value is a tree node but old wasn't (primitive→TreeNode transition)
	if newTreeNodePtr != nil {
		handleNewTreeNodeFromPrimitive(k, newTreeNodePtr, changes)
		return
	}

	// At least one is a primitive value or type changed - send new value as-is
	changes.SetDynamic(k, newValue)
}

// handleRangeMatch handles changes in matched range constructs.
// Uses fingerprint comparison to determine if client needs range statics.
func handleRangeMatch(k int, oldValue, newValue interface{}, changes *TreeNode) {
	// Use fingerprint comparison to determine if client has range statics cached.
	clientHasRangeStatics := !clientNeedsStaticsForValue(oldValue, newValue)

	// Strip statics if client already has them cached
	diffOps := GenerateRangeDifferentialOperations(oldValue, newValue, clientHasRangeStatics)

	if len(diffOps) > 0 {
		// For nested ranges, set operations directly (not wrapped in TreeNode)
		changes.SetDynamic(k, diffOps)
	} else {
		// No diff operations generated - use fallback
		handleEmptyRangeDiff(k, oldValue, newValue, changes)
	}
}

// handleEmptyRangeDiff handles the case when no diff operations were generated.
func handleEmptyRangeDiff(k int, oldValue, newValue interface{}, changes *TreeNode) {
	// Both empty ranges means no change needed
	if IsRangeConstruct(newValue) && !HasRangeItems(newValue) &&
		IsRangeConstruct(oldValue) && !HasRangeItems(oldValue) {
		return
	}

	// Send new value so client can update (covers both items->empty and other transitions)
	changes.SetDynamic(k, newValue)
}

// extractTreeNodePair extracts TreeNode pointers from old and new values.
func extractTreeNodePair(oldValue, newValue interface{}) (*TreeNode, *TreeNode, bool) {
	oldTreeNodePtr, oldIsTree := oldValue.(*TreeNode)
	newTreeNodePtr, newIsTree := newValue.(*TreeNode)
	return oldTreeNodePtr, newTreeNodePtr, oldIsTree && newIsTree
}

// handleNestedTreeNodes handles comparison of nested TreeNode structures.
func handleNestedTreeNodes(
	k int,
	oldTreeNodePtr, newTreeNodePtr *TreeNode,
	fieldPath string,
	insideNewStructure bool,
	rangeMatches map[string]string,
	changes *TreeNode,
) {
	// Check if this is a fundamental structure change using fingerprint comparison.
	// This implements the simplified diff architecture: fingerprint comparison replaces
	// complex structure checks with O(1) fingerprint comparison.
	_, isRangeMatch := rangeMatches[fieldPath]

	// Use fingerprint-based structure comparison for non-range matches
	structureChanged := !isRangeMatch && ClientNeedsStatics(oldTreeNodePtr, newTreeNodePtr)

	// Check if both contain ranges
	oldHasRange := ContainsRangeConstruct(oldTreeNodePtr)
	newHasRange := ContainsRangeConstruct(newTreeNodePtr)

	if structureChanged && (!oldHasRange || !newHasRange) {
		// Structure changed and this isn't just range item updates
		// Send full tree with statics since client needs the new structure
		changes.SetDynamic(k, newTreeNodePtr)
	} else {
		// Structure unchanged (same fingerprint) or both have ranges, do normal diff
		nestedChanges := CompareTreesAndGetChangesWithPath(
			oldTreeNodePtr, newTreeNodePtr,
			insideNewStructure || structureChanged,
			fieldPath,
			rangeMatches,
		)

		if nestedChanges.HasDynamics() || nestedChanges.HasRange() {
			// Use nested changes as-is (statics stripped since structure unchanged)
			changes.SetDynamic(k, nestedChanges)
		} else {
			// No dynamic changes detected, check static-only changes
			handleStaticOnlyChanges(k, oldTreeNodePtr, newTreeNodePtr, changes)
		}
	}
}

// handleStaticOnlyChanges handles cases where only statics changed.
// Uses fingerprint comparison to detect static structure changes.
func handleStaticOnlyChanges(k int, oldTreeNodePtr, newTreeNodePtr *TreeNode, changes *TreeNode) {
	oldStripped := PrepareTreeForClient(oldTreeNodePtr, true)
	newStripped := PrepareTreeForClient(newTreeNodePtr, true)
	oldIsEmpty := IsEmpty(oldStripped)
	newIsEmpty := IsEmpty(newStripped)

	// If both strip to empty (both static-only) but structures differ,
	// the statics changed - use fingerprint comparison for efficient detection
	if oldIsEmpty && newIsEmpty && ClientNeedsStatics(oldTreeNodePtr, newTreeNodePtr) {
		// Structure fingerprints differ, so statics changed - send empty string
		// to indicate the field should be re-rendered with new statics
		changes.SetDynamic(k, "")
	}
}

// handleNewTreeNodeFromPrimitive handles when new value is TreeNode but old wasn't.
func handleNewTreeNodeFromPrimitive(
	k int,
	newTreeNodePtr *TreeNode,
	changes *TreeNode,
) {
	// New tree appearing where primitive was - client needs full structure with statics
	valueToSet, _ := handleStructureValue(newTreeNodePtr, false)
	changes.SetDynamic(k, valueToSet)
}

// ClientNeedsStatics determines whether the client needs statics by comparing structure fingerprints.
// This implements the core optimization from the simplified diff architecture:
// - If old and new trees have the same structure fingerprint, client already has statics cached
// - If fingerprints differ (or old is nil), client needs statics sent
//
// This fingerprint-based approach replaces complex path-based registry tracking with a simple
// O(1) comparison after fingerprint calculation.
//
// Parameters:
//   - oldTree: The previous tree state (may be nil for first render)
//   - newTree: The current tree state (may be nil)
//
// Returns:
//   - true if client needs statics (first render or structure changed)
//   - false if client already has statics cached (same structure)
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
	// First render or new structure appearing - client needs statics
	if oldTree == nil {
		return true
	}

	// Tree being removed - no statics needed
	if newTree == nil {
		return false
	}

	// Compare structure fingerprints using cached values
	// GetStructureFingerprint computes and caches the fingerprint on first call
	oldFingerprint := oldTree.GetStructureFingerprint()
	newFingerprint := newTree.GetStructureFingerprint()

	// If fingerprints match, client already has the static structure
	return oldFingerprint != newFingerprint
}

// clientNeedsStaticsForValue determines if client needs statics for a dynamic value.
// This is used when comparing individual dynamic values that may be TreeNodes.
func clientNeedsStaticsForValue(oldValue, newValue interface{}) bool {
	oldTree, oldIsTree := oldValue.(*TreeNode)
	newTree, newIsTree := newValue.(*TreeNode)

	// If new value isn't a tree, no statics to send
	if !newIsTree {
		return false
	}

	// If old value wasn't a tree, client needs statics for this new tree
	if !oldIsTree {
		return true
	}

	// Both are trees - compare fingerprints
	return ClientNeedsStatics(oldTree, newTree)
}
