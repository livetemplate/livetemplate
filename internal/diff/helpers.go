package diff

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const (
	// hashPrefixLength is the number of characters to use from the generated hash
	// for compact item identifiers. 12 characters provide sufficient uniqueness
	// (48 bits of entropy) while keeping identifiers short.
	//
	// Collision probability: Using birthday paradox, ~16 million items would be
	// needed for a 1% collision probability. This is well beyond typical range
	// sizes in templates, making 12 characters sufficient for practical use.
	hashPrefixLength = 12

	// maxInsertionPoints is the threshold for determining if an insertion pattern
	// is too complex for individual insert operations. Patterns with more than
	// this many separate insertion points use full replace strategy instead.
	maxInsertionPoints = 3
)

// IsEmpty checks if a value is considered empty (empty string, empty map, empty slice).
func IsEmpty(v interface{}) bool {
	switch val := v.(type) {
	case *TreeNode:
		return !val.HasStatics() && !val.HasDynamics() && !val.HasRange()
	case string:
		return val == ""
	case map[string]interface{}:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// isMeaningfulValue checks if a value is meaningful (not empty/nil) and should
// be reported when removed. This is used in CompareRangeItemsForChanges to
// determine if removing a field should generate a change notification.
//
// A value is meaningful if:
// - It's a non-empty string
// - It's a TreeNode (which has structure/statics)
// - It's a non-empty map or slice
// - It's a non-nil value of another type
func isMeaningfulValue(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case string:
		return val != ""
	case *TreeNode:
		// TreeNodes are always meaningful - they have structure
		return true
	case map[string]interface{}:
		return len(val) > 0
	case []interface{}:
		return len(val) > 0
	default:
		// Other types (int, bool, etc.) are meaningful
		return true
	}
}

// IsRangeConstruct checks if a value is a range construct (has Range and Statics).
func IsRangeConstruct(value interface{}) bool {
	// Check if value is a TreeNode with Range field
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && node.HasStatics()
	}

	// Fallback: check for map representation (for compatibility during migration)
	if valueMap, ok := value.(map[string]interface{}); ok {
		_, hasD := valueMap["d"]
		_, hasS := valueMap["s"]
		// Both "d" (data array) and "s" (statics array) must be present
		return hasD && hasS
	}

	return false
}

// HasRangeItems checks if a range construct has any items in its data array.
// Returns true only if value is a range AND has at least one item.
func HasRangeItems(value interface{}) bool {
	// Check if value is a TreeNode with Range and items
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && len(node.Range.Items) > 0
	}

	// Fallback: check for map representation
	if valueMap, ok := value.(map[string]interface{}); ok {
		if d, hasD := valueMap["d"]; hasD {
			if dArray, ok := d.([]interface{}); ok {
				return len(dArray) > 0
			}
		}
	}

	return false
}

// ContainsRangeConstruct recursively checks if a tree node or any of its children contains a range construct.
func ContainsRangeConstruct(value interface{}) bool {
	// Check if this value itself is a range
	if IsRangeConstruct(value) {
		return true
	}

	// Check TreeNode dynamics recursively
	if node, ok := value.(*TreeNode); ok {
		for _, v := range node.Dynamics {
			if ContainsRangeConstruct(v) {
				return true
			}
		}
		return false
	}

	// Fallback: check for map representation
	if valueMap, ok := value.(map[string]interface{}); ok {
		// Recursively check all children (skip "s" and "f" keys)
		for k, v := range valueMap {
			if k == "s" || k == "f" {
				continue
			}
			if ContainsRangeConstruct(v) {
				return true
			}
		}
	}

	return false
}

// DeepEqual compares two values deeply.
// For TreeNode pointers, it uses TreeNodeEqual to ignore internal cache fields.
// For other types, it uses reflect.DeepEqual.
func DeepEqual(a, b interface{}) bool {
	// Handle TreeNode comparisons specially
	if aNode, ok := a.(*TreeNode); ok {
		if bNode, ok := b.(*TreeNode); ok {
			return TreeNodeEqual(aNode, bNode)
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

// TreeNodeEqual compares two TreeNodes for equality, ignoring internal cache fields.
// This is necessary because cachedStructureFingerprint may differ between otherwise
// identical trees (old tree may have been cached, new tree hasn't).
func TreeNodeEqual(a, b *TreeNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare Statics
	if !reflect.DeepEqual(a.Statics, b.Statics) {
		return false
	}

	// Compare Dynamics size
	if len(a.Dynamics) != len(b.Dynamics) {
		return false
	}

	// Compare each dynamic value
	for key, aVal := range a.Dynamics {
		bVal, exists := b.Dynamics[key]
		if !exists {
			return false
		}
		if !DeepEqual(aVal, bVal) {
			return false
		}
	}

	// Compare Fingerprint (the computed hash)
	if a.Fingerprint != b.Fingerprint {
		return false
	}

	// Compare Range data if present
	if (a.Range == nil) != (b.Range == nil) {
		return false
	}
	if a.Range != nil {
		if !reflect.DeepEqual(a.Range.Items, b.Range.Items) {
			return false
		}
		if !reflect.DeepEqual(a.Range.Statics, b.Range.Statics) {
			return false
		}
	}

	// Compare Metadata if present
	if (a.Metadata == nil) != (b.Metadata == nil) {
		return false
	}
	if a.Metadata != nil && a.Metadata.IDKey != b.Metadata.IDKey {
		return false
	}

	// Note: cachedStructureFingerprint is intentionally NOT compared
	// It's an internal optimization field that may differ between trees

	return true
}

// findKeyAttrPosition searches for key attributes in a string slice.
// Returns the position where a key attribute is found, or -1 if not found.
func findKeyAttrPosition(statics []string, keyAttrs []string) int {
	for i, staticStr := range statics {
		// Check for any of the key attributes in priority order
		for _, keyAttr := range keyAttrs {
			if strings.Contains(staticStr, keyAttr) {
				// The next position after this static contains the key value
				return i
			}
		}
	}
	return -1 // Not found
}

// FindKeyPositionFromStatics parses the statics array to find which position contains the key.
// Supports both []string and []interface{} formats for backward compatibility.
func FindKeyPositionFromStatics(statics interface{}) int {
	// Priority order for key attributes (same as server-side)
	keyAttrs := []string{`data-lvt-key="`, `data-key="`, `key="`, `id="`}

	// Try []string first (most common case)
	if staticsArr, ok := statics.([]string); ok {
		return findKeyAttrPosition(staticsArr, keyAttrs)
	}

	// Try []interface{} with string conversion
	if staticsArr, ok := statics.([]interface{}); ok {
		// Convert []interface{} to []string
		stringSlice := make([]string, 0, len(staticsArr))
		for _, static := range staticsArr {
			if staticStr, ok := static.(string); ok {
				stringSlice = append(stringSlice, staticStr)
			} else {
				// Non-string element, append empty to maintain positions
				stringSlice = append(stringSlice, "")
			}
		}
		return findKeyAttrPosition(stringSlice, keyAttrs)
	}

	return -1 // Unknown type, no key attribute found
}

// GetItemKey extracts the key from a range item using the statics structure.
func GetItemKey(item interface{}, statics interface{}) (string, bool) {
	// Handle TreeNode items
	if itemNode, ok := item.(*TreeNode); ok {
		// First, check for reserved auto-generated key field
		if autoKey, exists := itemNode.GetDynamic("_k"); exists {
			if keyStr, ok := autoKey.(string); ok {
				return keyStr, true
			}
		}

		// Get the effective statics for this item
		effectiveStatics := getItemStatics(itemNode, statics)

		keyPos := FindKeyPositionFromStatics(effectiveStatics)

		// Only use position-based lookup if a key attribute was actually found
		// (keyPos >= 0 means a key attribute like data-key, id, etc. was detected)
		if keyPos >= 0 {
			keyPosStr := fmt.Sprintf("%d", keyPos)
			if key, exists := itemNode.GetDynamic(keyPosStr); exists {
				if keyStr, ok := key.(string); ok {
					return keyStr, true
				}
			}
		}

		// No explicit key attribute found, generate a content-based hash
		// This ensures items have stable keys even without template key attributes
		return GenerateItemHash(itemNode), true
	}

	return "", false
}

// getItemStatics returns the effective statics for an item.
// All ranges use shared statics (homogeneous approach).
// StaticsMap was removed in v0.8.0 - heterogeneous ranges use full replace via fingerprint diff.
func getItemStatics(itemNode *TreeNode, statics interface{}) interface{} {
	// Handle nil statics
	if statics == nil {
		return nil
	}

	// Return statics as-is - all ranges use shared statics
	return statics
}

// GenerateItemHash creates a stable hash for a range item based on its content.
// This is used when no explicit key attribute is provided in the template.
// Uses FNV-1a hash for fast, non-cryptographic content fingerprinting.
func GenerateItemHash(item interface{}) string {
	// Handle TreeNode
	if itemNode, ok := item.(*TreeNode); ok {
		// Create a canonical JSON representation for hashing
		// Sort keys to ensure deterministic ordering
		keys := make([]string, 0, len(itemNode.Dynamics))
		for k := range itemNode.Dynamics {
			// Skip internal/reserved fields
			if k != "_k" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		// Build canonical representation
		var parts []string
		for _, k := range keys {
			val, _ := itemNode.GetDynamic(k)
			valJSON, err := json.Marshal(val)
			if err != nil {
				// Fallback to string representation if marshaling fails.
				// This can occur for non-JSON-serializable types like channels,
				// functions, or complex structures with circular references.
				parts = append(parts, fmt.Sprintf("%s:%v", k, val))
			} else {
				parts = append(parts, fmt.Sprintf("%s:%s", k, string(valJSON)))
			}
		}

		// Hash the canonical representation using FNV-1a
		content := strings.Join(parts, "|")
		hasher := fnv.New64a()
		hasher.Write([]byte(content))
		hash := hex.EncodeToString(hasher.Sum(nil))

		// Return first hashPrefixLength characters for compactness
		if len(hash) >= hashPrefixLength {
			return hash[:hashPrefixLength]
		}
		return hash
	}

	return ""
}

// ExtractItemKeys extracts the keys from a slice of range items using the statics structure.
func ExtractItemKeys(items []interface{}, statics interface{}) []string {
	var keys []string
	for _, item := range items {
		// Items are now *TreeNode
		if itemNode, ok := item.(*TreeNode); ok {
			if key, ok := GetItemKey(itemNode, statics); ok {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// DetectPositionField finds the field containing positional display like "#0", "#1", etc.
// This is used to identify which field should be excluded from content comparison during
// reordering detection, as position fields change when items are reordered even if
// their actual content hasn't changed.
// Note: Only checks the first item as position fields are consistent across all items.
func DetectPositionField(itemsByKey map[string]interface{}) string {
	positionPattern := regexp.MustCompile(`^#\d+`)

	// Check first item only - position field pattern is consistent across all items
	for _, item := range itemsByKey {
		if itemNode, ok := item.(*TreeNode); ok {
			for field, value := range itemNode.Dynamics {
				if strValue, ok := value.(string); ok {
					if positionPattern.MatchString(strValue) {
						return field
					}
				}
			}
		}
		break // Intentional - only check first item
	}
	return ""
}

// IsPureReordering checks if the items are the same but just in different order.
func IsPureReordering(oldItems, newItems []interface{}, oldKeys, newKeys []string, statics interface{}) bool {
	// Must have same number of items
	if len(oldKeys) != len(newKeys) {
		return false
	}

	// Check if keys are the same (just different order)
	oldKeySet := make(map[string]bool)
	newKeySet := make(map[string]bool)

	for _, k := range oldKeys {
		oldKeySet[k] = true
	}
	for _, k := range newKeys {
		newKeySet[k] = true
	}

	// If key sets don't match, it's not pure reordering
	if len(oldKeySet) != len(newKeySet) {
		return false
	}
	for k := range oldKeySet {
		if !newKeySet[k] {
			return false
		}
	}

	// Now check if the items with same keys have identical content
	oldItemsByKey := make(map[string]interface{}, len(oldItems))
	newItemsByKey := make(map[string]interface{}, len(newItems))

	for _, item := range oldItems {
		if key, ok := GetItemKey(item, statics); ok {
			oldItemsByKey[key] = item
		}
	}

	for _, item := range newItems {
		if key, ok := GetItemKey(item, statics); ok {
			newItemsByKey[key] = item
		}
	}

	// Detect position field by finding field with pattern like "#0", "#1", etc.
	positionField := DetectPositionField(oldItemsByKey)

	// Compare each item's content (excluding position-dependent fields)
	for key, oldItem := range oldItemsByKey {
		newItem, exists := newItemsByKey[key]
		if !exists {
			return false
		}

		// Compare items excluding position field (field contains "#0:", "#1:", etc.)
		oldItemNode, ok1 := oldItem.(*TreeNode)
		newItemNode, ok2 := newItem.(*TreeNode)

		if !ok1 || !ok2 {
			// If we can't compare as TreeNodes, fall back to full comparison
			if !DeepEqual(oldItem, newItem) {
				return false
			}
			continue
		}

		// Find key position to skip it in comparison
		keyPos := FindKeyPositionFromStatics(statics)
		keyPosStr := fmt.Sprintf("%d", keyPos)

		// Compare all fields except position field, key field, and auto-key field
		for field, oldValue := range oldItemNode.Dynamics {
			// Skip position field (contains positional display like "#0:")
			// Skip key field (determined from statics)
			// Skip auto-generated key field "_k" (may be present in one tree but not the other)
			if field == positionField || field == keyPosStr || field == "_k" {
				continue
			}

			newValue, exists := newItemNode.GetDynamic(field)
			if !exists || !DeepEqual(oldValue, newValue) {
				return false
			}
		}

		// Also check that new item doesn't have extra fields (except position, key, and auto-key)
		for field := range newItemNode.Dynamics {
			if field == positionField || field == keyPosStr || field == "_k" {
				continue
			}
			if _, exists := oldItemNode.GetDynamic(field); !exists {
				return false
			}
		}
	}

	// Check if order actually changed
	for i := range oldKeys {
		if oldKeys[i] != newKeys[i] {
			return true // Same items, different order = pure reordering
		}
	}

	// Same items, same order = no change
	return false
}

// FindNewItems returns keys of items that exist in new but not in old.
func FindNewItems(oldItems, newItems []interface{}, statics interface{}) []string {
	oldKeys := make(map[string]bool, len(oldItems))
	for _, item := range oldItems {
		if key, ok := GetItemKey(item, statics); ok {
			oldKeys[key] = true
		}
	}

	var newKeys []string
	for _, item := range newItems {
		if key, ok := GetItemKey(item, statics); ok {
			if !oldKeys[key] {
				newKeys = append(newKeys, key)
			}
		}
	}

	return newKeys
}

// AreAllItemsAtStart checks if all new items are at the beginning of the list (prepend).
func AreAllItemsAtStart(newKeys []string, newItems []interface{}, statics interface{}) bool {
	if len(newKeys) == 0 {
		return false
	}

	// Check if all new keys are at the beginning of newItems
	for i, key := range newKeys {
		if i >= len(newItems) {
			return false
		}

		// Get key from item (supports both TreeNode and legacy map format)
		if itemKey, ok := GetItemKey(newItems[i], statics); ok {
			if itemKey != key {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// AreAllItemsAtEnd checks if all new items are at the end of the list (append).
func AreAllItemsAtEnd(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	if len(newKeys) == 0 || len(oldItems) == 0 {
		return false
	}

	// New items should be after all old items
	// Start index for new items should be len(oldItems)
	startIndex := len(newItems) - len(newKeys)

	// Verify that items before startIndex are all old items
	oldKeys := ExtractItemKeys(oldItems, statics)
	for i := 0; i < startIndex; i++ {
		if i >= len(newItems) {
			return false
		}
		if itemKey, ok := GetItemKey(newItems[i], statics); ok {
			// Check if this key exists in oldKeys
			found := false
			for _, oldKey := range oldKeys {
				if oldKey == itemKey {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else {
			return false
		}
	}

	// Check if all new keys are contiguous at the end
	for i, key := range newKeys {
		index := startIndex + i
		if index >= len(newItems) {
			return false
		}
		if itemKey, ok := GetItemKey(newItems[index], statics); ok {
			if itemKey != key {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// IsComplexInsertionPattern checks if the insertion pattern is too complex for simple operations.
func IsComplexInsertionPattern(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	if len(newKeys) == 0 {
		return false
	}

	// OPTIMIZATION: Check for simple append/prepend patterns first.
	// These are common patterns (e.g., load_more, real-time feeds) that should NOT
	// be considered complex regardless of how many items are added.
	if AreAllItemsAtEnd(newKeys, oldItems, newItems, statics) {
		return false // Append pattern - not complex
	}
	if AreAllItemsAtStart(newKeys, newItems, statics) {
		return false // Prepend pattern - not complex
	}

	// For scattered insertions, count unique insertion points
	insertionPoints := make(map[string]bool, len(newKeys))

	for i, item := range newItems {
		if keyStr, ok := GetItemKey(item, statics); ok {
			// Check if this is a new key
			for _, newKey := range newKeys {
				if newKey == keyStr {
					// Determine insertion point
					var insertionPoint string
					if i > 0 {
						if prevKeyStr, ok := GetItemKey(newItems[i-1], statics); ok {
							insertionPoint = prevKeyStr + ":after"
						}
					} else {
						insertionPoint = "start"
					}
					insertionPoints[insertionPoint] = true
					break
				}
			}
		}
	}

	return len(insertionPoints) > maxInsertionPoints
}

// GetRangeSignature creates a signature for a range construct based on its static template structure.
// This signature should be the same for the same template construct regardless of data.
func GetRangeSignature(rangeValue interface{}) string {
	// Check if value is a TreeNode with statics
	if node, ok := rangeValue.(*TreeNode); ok {
		if node.HasStatics() {
			return fmt.Sprintf("%v", node.Statics)
		}
		return ""
	}

	// Fallback: check for map representation (for compatibility during migration)
	rangeMap, ok := rangeValue.(map[string]interface{})
	if !ok {
		return ""
	}

	// Use the static parts ("s") as the signature since they represent the template structure
	staticParts, exists := rangeMap["s"]
	if !exists {
		return ""
	}

	// Convert static parts to a string signature
	return fmt.Sprintf("%v", staticParts)
}

// FindRangeConstructs finds all range constructs in a tree, recursively searching nested structures.
func FindRangeConstructs(tree *TreeNode) map[string]interface{} {
	if tree == nil {
		return make(map[string]interface{})
	}
	return findRangeConstructsRecursive(tree, "")
}

// findRangeConstructsRecursive finds range constructs with path tracking.
func findRangeConstructsRecursive(tree *TreeNode, path string) map[string]interface{} {
	ranges := make(map[string]interface{})

	if tree == nil {
		return ranges
	}

	// CRITICAL FIX: Check if the tree ITSELF is a range construct
	// This handles top-level ranges like: {{range .Items}}...{{end}}
	// where the entire tree has Range field set
	if tree.HasRange() && tree.HasStatics() {
		ranges[path] = tree
		// Don't recurse into range internals - treat the range as an atomic unit
		return ranges
	}

	// Tree is not a range, search for ranges as field values in dynamics
	for field, value := range tree.Dynamics {
		// Build the full path to this field
		fieldPath := field
		if path != "" {
			fieldPath = path + "." + field
		}

		if IsRangeConstruct(value) {
			ranges[fieldPath] = value
		} else {
			// Recursively search nested tree nodes
			if nestedTree, ok := value.(*TreeNode); ok {
				// Merge nested ranges into our map
				nestedRanges := findRangeConstructsRecursive(nestedTree, fieldPath)
				for k, v := range nestedRanges {
					ranges[k] = v
				}
			}
		}
	}

	return ranges
}

// FindRangeConstructMatches finds matching range constructs between old and new trees.
func FindRangeConstructMatches(oldTree, newTree *TreeNode) map[string]string {
	matches := make(map[string]string)

	// Handle nil trees
	if oldTree == nil || newTree == nil {
		return matches
	}

	// Find all range constructs in both trees
	oldRanges := FindRangeConstructs(oldTree)
	newRanges := FindRangeConstructs(newTree)

	// Match range constructs by their static template signature
	for newField, newRange := range newRanges {
		newSignature := GetRangeSignature(newRange)

		matched := false
		for oldField, oldRange := range oldRanges {
			oldSignature := GetRangeSignature(oldRange)

			// If signatures match, this is the same template construct
			if newSignature == oldSignature && newSignature != "" {
				matches[newField] = oldField
				matched = true
				break // Each new range should match at most one old range
			}
		}

		// FALLBACK: If no match found and one side has empty signature (empty range),
		// AND there's only one range in each tree at the same position, match by position
		if !matched && len(newRanges) == 1 && len(oldRanges) == 1 {
			// Single range in both trees at same position - must be the same construct
			for oldField := range oldRanges {
				if newField == oldField {
					matches[newField] = oldField
					break
				}
			}
		}
	}

	return matches
}
