package diff

import (
	"reflect"

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
//   - registry: Optional registry tracking which structures client has seen (may be nil)
//
// Returns:
//   - TreeNode containing only the changed fields (empty if no changes)
//
// The function handles:
//   - Top-level range constructs with differential operations
//   - Nil tree cases (tree removal)
//   - Deep comparison of dynamic fields
//   - Registry-based optimization (strips statics when client has them cached)
func CompareTreesAndGetChangesWithPath(
	oldTree, newTree *TreeNode,
	insideNewStructure bool,
	currentPath string,
	rangeMatches map[string]string,
	registry StructureRegistry,
) *TreeNode {
	changes := &TreeNode{
		Dynamics: make(map[string]interface{}),
	}

	// Handle top-level range constructs
	if handleTopLevelRange(oldTree, newTree, currentPath, rangeMatches, registry, changes) {
		return changes
	}

	// Handle nil trees
	if newTree == nil {
		return &TreeNode{}
	}

	// Compare dynamic segments
	compareDynamicSegments(oldTree, newTree, insideNewStructure, currentPath, rangeMatches, registry, changes)

	return changes
}

// handleTopLevelRange handles top-level range construct comparisons.
// Returns true if this was a range case and changes were set.
func handleTopLevelRange(
	oldTree, newTree *TreeNode,
	currentPath string,
	rangeMatches map[string]string,
	registry StructureRegistry,
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
				return handleMatchedRanges(oldTree, newTree, currentPath, registry, changes)
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
		// Also invalidate registry for this path since the range structure is gone
		registryUsable := registry != nil && !isNilRegistry(registry)
		if registryUsable {
			rangeStaticsPath := currentPath + ".__range_statics__"
			registry.InvalidatePath(rangeStaticsPath)
		}
		*changes = *newTree
		return true
	}

	return false
}

// handleMatchedRanges generates differential operations for matched range constructs.
// Uses fingerprint comparison to determine if client needs range statics.
func handleMatchedRanges(oldTree, newTree *TreeNode, currentPath string, registry StructureRegistry, changes *TreeNode) bool {
	// Use fingerprint comparison to determine if client has range statics cached.
	// If the structure fingerprints match, client already has the statics.
	// This replaces the registry-based tracking with simpler fingerprint comparison.
	clientHasRangeStatics := !ClientNeedsStatics(oldTree, newTree)

	// Strip statics if client already has them cached
	diffOps := GenerateRangeDifferentialOperations(oldTree, newTree, clientHasRangeStatics)

	if len(diffOps) > 0 {
		// Return the operations directly - the entire tree is the range
		changes.Dynamics["d"] = diffOps

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
	registry StructureRegistry,
	changes *TreeNode,
) {
	for k, newValue := range newTree.Dynamics {
		// Build full path for this field
		fieldPath := buildFieldPath(currentPath, k)

		var oldValue interface{}
		var exists bool
		if oldTree != nil {
			oldValue, exists = oldTree.GetDynamic(k)
		}

		if !exists {
			// Field is NEW compared to last update
			handleNewField(k, newValue, fieldPath, insideNewStructure, registry, changes)
		} else if !DeepEqual(oldValue, newValue) {
			// Field exists but changed
			handleChangedField(
				k, oldValue, newValue,
				fieldPath,
				insideNewStructure,
				rangeMatches,
				registry,
				oldTree, newTree,
				changes,
			)
		}
	}
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
	k string,
	newValue interface{},
	fieldPath string,
	insideNewStructure bool,
	registry StructureRegistry,
	changes *TreeNode,
) {
	// If we're inside a new structure, client has never seen this, so include statics
	if insideNewStructure {
		changes.SetDynamic(k, newValue)
		return
	}

	// Check if registry is usable (cache the check to avoid repeated reflection)
	registryUsable := registry != nil && !isNilRegistry(registry)

	// Check if client has seen THIS EXACT structure at this path (Phase 2: Registry)
	clientHasStructure := false
	if registryUsable {
		clientHasStructure = registry.HasSeen(fieldPath, newValue)
	}

	// Handle tree node values
	if handleNewTreeNodeField(k, newValue, clientHasStructure, fieldPath, registry, registryUsable, changes) {
		return
	}

	// Handle map values
	if handleNewMapField(k, newValue, clientHasStructure, fieldPath, registry, registryUsable, changes) {
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

// handleStructureValue handles TreeNode or map values with registry tracking.
// This function implements the core optimization: stripping statics when the client
// already has them cached.
//
// Parameters:
//   - newValue: The structure value (TreeNode or map) to process (must not be nil)
//   - clientHasStructure: True if client has already seen this structure
//
// Returns:
//   - valueToSet: The value to send to client (stripped or full, or empty string)
//   - shouldTrack: True if this structure should be tracked in registry
//
// Behavior:
//   - If client has structure: returns stripped value (dynamics only), shouldTrack=false
//   - If client doesn't have structure: returns full value (with statics), shouldTrack=true
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
	if isStrippedValueEmpty(stripped) {
		return "", false
	}
	return newValue, true
}

// handleNewTreeNodeField handles new TreeNode field values.
func handleNewTreeNodeField(
	k string,
	newValue interface{},
	clientHasStructure bool,
	fieldPath string,
	registry StructureRegistry,
	registryUsable bool,
	changes *TreeNode,
) bool {
	newTreeNode, ok := newValue.(*TreeNode)
	if !ok {
		return false
	}

	valueToSet, shouldTrack := handleStructureValue(newTreeNode, clientHasStructure)
	changes.SetDynamic(k, valueToSet)

	if shouldTrack && registryUsable {
		registry.MarkSeen(fieldPath, newValue)
	}
	return true
}

// handleNewMapField handles new map[string]interface{} field values.
func handleNewMapField(
	k string,
	newValue interface{},
	clientHasStructure bool,
	fieldPath string,
	registry StructureRegistry,
	registryUsable bool,
	changes *TreeNode,
) bool {
	m, ok := newValue.(map[string]interface{})
	if !ok {
		return false
	}

	valueToSet, shouldTrack := handleStructureValue(m, clientHasStructure)
	changes.SetDynamic(k, valueToSet)

	if shouldTrack && registryUsable {
		registry.MarkSeen(fieldPath, newValue)
	}
	return true
}

// handleChangedField processes a field that exists but changed value.
func handleChangedField(
	k string,
	oldValue, newValue interface{},
	fieldPath string,
	insideNewStructure bool,
	rangeMatches map[string]string,
	registry StructureRegistry,
	oldTree, newTree *TreeNode,
	changes *TreeNode,
) {
	// Check if this field has a range construct match
	if _, isRangeMatch := rangeMatches[fieldPath]; isRangeMatch {
		handleRangeMatch(k, oldValue, newValue, fieldPath, registry, changes)
		return
	}

	// Check if both old and new values are TreeNodes (nested structures)
	oldTreeNodePtr, newTreeNodePtr, bothAreTreeNodes := extractTreeNodePair(oldValue, newValue)
	if bothAreTreeNodes {
		handleNestedTreeNodes(
			k, oldTreeNodePtr, newTreeNodePtr,
			fieldPath, insideNewStructure,
			rangeMatches, registry,
			changes,
		)
		return
	}

	// New value is a tree node but old wasn't
	if newTreeNodePtr != nil {
		// Cache registry usability check
		registryUsable := registry != nil && !isNilRegistry(registry)
		handleNewTreeNodeFromPrimitive(k, newTreeNodePtr, fieldPath, registry, registryUsable, changes)
		return
	}

	// Old value was a tree node but new value is primitive (e.g., conditional becoming empty)
	// Invalidate registry entries for this path so statics are re-sent when tree node returns
	if oldTreeNodePtr != nil {
		registryUsable := registry != nil && !isNilRegistry(registry)
		if registryUsable {
			registry.InvalidatePath(fieldPath)
		}
	}

	// At least one is a primitive value or type changed - send new value as-is
	changes.SetDynamic(k, newValue)
}

// handleRangeMatch handles changes in matched range constructs.
// Uses fingerprint comparison to determine if client needs range statics.
func handleRangeMatch(k string, oldValue, newValue interface{}, fieldPath string, registry StructureRegistry, changes *TreeNode) {
	// Use fingerprint comparison to determine if client has range statics cached.
	// Extract TreeNodes for fingerprint comparison.
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
func handleEmptyRangeDiff(k string, oldValue, newValue interface{}, changes *TreeNode) {
	// Check if both are empty ranges (no change needed)
	if IsRangeConstruct(newValue) && !HasRangeItems(newValue) &&
		IsRangeConstruct(oldValue) && !HasRangeItems(oldValue) {
		// Both empty ranges, no update needed
		return
	}

	// Check if new value is an empty range (items→empty transition)
	// Send the empty range structure so client knows to clear items
	if IsRangeConstruct(newValue) && !HasRangeItems(newValue) {
		// Send empty range with statics (client will clear items and keep structure)
		changes.SetDynamic(k, newValue)
	} else {
		// Regular fallback with statics included
		changes.SetDynamic(k, newValue)
	}
}

// extractTreeNodePair extracts TreeNode pointers from old and new values.
func extractTreeNodePair(oldValue, newValue interface{}) (*TreeNode, *TreeNode, bool) {
	oldTreeNodePtr, oldIsTree := oldValue.(*TreeNode)
	newTreeNodePtr, newIsTree := newValue.(*TreeNode)
	return oldTreeNodePtr, newTreeNodePtr, oldIsTree && newIsTree
}

// handleNestedTreeNodes handles comparison of nested TreeNode structures.
func handleNestedTreeNodes(
	k string,
	oldTreeNodePtr, newTreeNodePtr *TreeNode,
	fieldPath string,
	insideNewStructure bool,
	rangeMatches map[string]string,
	registry StructureRegistry,
	changes *TreeNode,
) {
	// Check if this is a fundamental structure change using fingerprint comparison.
	// This implements the simplified diff architecture: fingerprint comparison replaces
	// complex AreStructuresSimilar checks with O(1) fingerprint comparison.
	_, isRangeMatch := rangeMatches[fieldPath]

	// Use fingerprint-based structure comparison for non-range matches
	structureChanged := !isRangeMatch && ClientNeedsStatics(oldTreeNodePtr, newTreeNodePtr)

	// Check if both contain ranges
	oldHasRange := ContainsRangeConstruct(oldTreeNodePtr)
	newHasRange := ContainsRangeConstruct(newTreeNodePtr)

	if structureChanged && !(oldHasRange && newHasRange) {
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
			registry,
		)

		if nestedChanges.HasDynamics() {
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
func handleStaticOnlyChanges(k string, oldTreeNodePtr, newTreeNodePtr *TreeNode, changes *TreeNode) {
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
	k string,
	newTreeNodePtr *TreeNode,
	fieldPath string,
	registry StructureRegistry,
	registryUsable bool,
	changes *TreeNode,
) {
	// Use registry to check if client has seen THIS EXACT structure at this path
	clientHasStructure := false
	if registryUsable {
		clientHasStructure = registry.HasSeen(fieldPath, newTreeNodePtr)
	}

	valueToSet, shouldTrack := handleStructureValue(newTreeNodePtr, clientHasStructure)
	changes.SetDynamic(k, valueToSet)

	if shouldTrack && registryUsable {
		registry.MarkSeen(fieldPath, newTreeNodePtr)
	}
}

// isNilRegistry checks if the registry interface contains a nil pointer.
// This handles the Go interface gotcha where an interface can be non-nil but contain a nil pointer.
//
// Background:
// In Go, an interface value is nil only if both its type and value are nil.
// A common bug is:
//
//	var ptr *ConcreteRegistry // ptr is nil
//	var iface StructureRegistry = ptr // iface is non-nil (has type), but contains nil value
//	if iface != nil { /* this is true! */ }
//
// This function uses reflection to detect this case and return true when the
// underlying pointer is nil, even if the interface wrapper is non-nil.
//
// Performance note: Uses reflection, so should be cached by callers to avoid
// repeated checks in hot paths.
func isNilRegistry(registry StructureRegistry) bool {
	if registry == nil {
		return true
	}
	// Use reflection to check if the underlying value is nil
	v := reflect.ValueOf(registry)
	return v.Kind() == reflect.Ptr && v.IsNil()
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

	// Compare structure fingerprints
	oldFingerprint := build.CalculateStructureFingerprint(oldTree)
	newFingerprint := build.CalculateStructureFingerprint(newTree)

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
