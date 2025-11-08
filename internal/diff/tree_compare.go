package diff

import "reflect"

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
	if handleTopLevelRange(oldTree, newTree, currentPath, rangeMatches, changes) {
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
	changes *TreeNode,
) bool {
	// CRITICAL FIX: Check if both trees ARE range constructs (top-level range template)
	// Example: {{range .Items}}<div>...</div>{{end}} where the entire tree is a range
	// OR if newTree is a range but oldTree isn't (range appearing for first time, e.g., from {{else}} clause)
	if oldTree != nil && newTree != nil && newTree.HasRange() && newTree.HasStatics() {
		// Case 1: Both are ranges and matched
		if oldTree.HasRange() && oldTree.HasStatics() {
			if _, isMatched := rangeMatches[currentPath]; isMatched {
				return handleMatchedRanges(oldTree, newTree, changes)
			}
		} else {
			// Case 2: newTree is a range but oldTree isn't (range appearing for first time)
			// This happens when going from {{else}} clause to range content
			// Return the full new tree so client can replace the else content with range items
			*changes = *newTree
			return true
		}
	}
	return false
}

// handleMatchedRanges generates differential operations for matched range constructs.
func handleMatchedRanges(oldTree, newTree *TreeNode, changes *TreeNode) bool {
	// Generate differential operations for the entire range
	// Never strip statics - they're needed for rendering new items
	diffOps := GenerateRangeDifferentialOperations(oldTree, newTree, false)

	if len(diffOps) > 0 {
		// Return the operations directly - the entire tree is the range
		// Always include statics - they're needed for prepend/append rendering
		changes.Dynamics["d"] = diffOps
		changes.Statics = newTree.Statics
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
// Returns true if the value is an empty map or empty string.
func isStrippedValueEmpty(stripped interface{}) bool {
	if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
		return true
	}
	if strippedStr, ok := stripped.(string); ok && strippedStr == "" {
		return true
	}
	return false
}

// handleStructureValue handles TreeNode or map values with registry tracking.
// This function implements the core optimization: stripping statics when the client
// already has them cached.
//
// Parameters:
//   - newValue: The structure value (TreeNode or map) to process
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
func handleStructureValue(
	newValue interface{},
	clientHasStructure bool,
) (valueToSet interface{}, shouldTrack bool) {
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
		handleRangeMatch(k, oldValue, newValue, changes)
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

	// At least one is a primitive value or type changed - send new value as-is
	changes.SetDynamic(k, newValue)
}

// handleRangeMatch handles changes in matched range constructs.
func handleRangeMatch(k string, oldValue, newValue interface{}, changes *TreeNode) {
	// Generate differential operations for matched range constructs
	// Never strip statics - they're needed for rendering new items in prepend/append operations
	diffOps := GenerateRangeDifferentialOperations(oldValue, newValue, false)

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
	// Check if this is a fundamental structure change (not part of a range match)
	_, isRangeMatch := rangeMatches[fieldPath]
	structureChanged := !isRangeMatch && !AreStructuresSimilar(oldTreeNodePtr, newTreeNodePtr)

	// Check if both contain ranges
	oldHasRange := ContainsRangeConstruct(oldTreeNodePtr)
	newHasRange := ContainsRangeConstruct(newTreeNodePtr)

	if structureChanged && !(oldHasRange && newHasRange) {
		// Structure changed and this isn't just range item updates
		changes.SetDynamic(k, newTreeNodePtr)
	} else {
		// Structure similar, do normal diff
		nestedChanges := CompareTreesAndGetChangesWithPath(
			oldTreeNodePtr, newTreeNodePtr,
			insideNewStructure || structureChanged,
			fieldPath,
			rangeMatches,
			registry,
		)

		if nestedChanges.HasDynamics() {
			// Use nested changes as-is
			changes.SetDynamic(k, nestedChanges)
		} else {
			// No dynamic changes detected, check static-only changes
			handleStaticOnlyChanges(k, oldTreeNodePtr, newTreeNodePtr, changes)
		}
	}
}

// handleStaticOnlyChanges handles cases where only statics changed.
func handleStaticOnlyChanges(k string, oldTreeNodePtr, newTreeNodePtr *TreeNode, changes *TreeNode) {
	oldStripped := PrepareTreeForClient(oldTreeNodePtr, true)
	newStripped := PrepareTreeForClient(newTreeNodePtr, true)
	oldIsEmpty := IsEmpty(oldStripped)
	newIsEmpty := IsEmpty(newStripped)

	// If both strip to empty (both static-only) but the originals aren't equal,
	// the statics changed - send empty string to indicate change
	if oldIsEmpty && newIsEmpty && !DeepEqual(oldTreeNodePtr, newTreeNodePtr) {
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
