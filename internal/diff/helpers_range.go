package diff

import (
	"fmt"
	"strconv"

	"github.com/livetemplate/livetemplate/internal/build"
)

const (
	// maxInsertionPoints is the threshold for determining if an insertion pattern
	// is too complex for individual insert operations.
	maxInsertionPoints = 3
)

// nodeContainsNestedRange reports whether any TreeNode reachable through node's
// dynamics is a range construct. A range whose items contain nested ranges is
// kept off the stream-mode transition and diffed item-by-item, so a deep edit
// scopes to a nested ["u", key, …] instead of re-sending the branch.
func nodeContainsNestedRange(node *TreeNode) bool {
	if node == nil {
		return false
	}
	for _, d := range node.Dynamics {
		child, ok := d.(*TreeNode)
		if !ok {
			continue
		}
		if child.HasRange() || nodeContainsNestedRange(child) {
			return true
		}
	}
	return false
}

// HasReordering returns true if the order of keys differs between old and new.
// Returns false if lengths differ (which indicates insertions/removals).
func HasReordering(oldKeys, newKeys []string) bool {
	if len(oldKeys) != len(newKeys) {
		return false
	}
	for i := range oldKeys {
		if oldKeys[i] != newKeys[i] {
			return true
		}
	}
	return false
}

// sameKeySet returns true if oldKeys and newKeys contain the same set of keys.
func sameKeySet(oldKeys, newKeys []string) bool {
	if len(oldKeys) != len(newKeys) {
		return false
	}
	oldSet := make(map[string]struct{}, len(oldKeys))
	for _, k := range oldKeys {
		oldSet[k] = struct{}{}
	}
	for _, k := range newKeys {
		if _, exists := oldSet[k]; !exists {
			return false
		}
	}
	return true
}

// IsPureReordering checks if the items are the same but just in different order.
func IsPureReordering(oldItems, newItems []interface{}, statics interface{}) bool {
	ctx := newRangeContext(oldItems, newItems, statics, nil)
	return isPureReorderingCtx(ctx)
}

// FindNewItems returns keys of items that exist in new but not in old.
func FindNewItems(oldItems, newItems []interface{}, statics interface{}) []string {
	ctx := newRangeContext(oldItems, newItems, statics, nil)
	return ctx.addedKeys
}

// AreAllItemsAtStart checks if all new items are at the beginning of the list (prepend).
func AreAllItemsAtStart(newKeys []string, newItems []interface{}, statics interface{}) bool {
	ctx := newRangeContext(nil, newItems, statics, nil)
	ctx.addedKeys = newKeys
	return areAllItemsAtStartCtx(ctx)
}

// AreAllItemsAtEnd checks if all new items are at the end of the list (append).
func AreAllItemsAtEnd(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	ctx := newRangeContext(oldItems, newItems, statics, nil)
	ctx.addedKeys = newKeys
	return areAllItemsAtEndCtx(ctx)
}

// IsComplexInsertionPattern checks if the insertion pattern is too complex for simple operations.
func IsComplexInsertionPattern(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	ctx := newRangeContext(oldItems, newItems, statics, nil)
	ctx.addedKeys = newKeys
	return isComplexInsertionPatternCtx(ctx)
}

// Context-aware internal variants that use pre-computed data from rangeContext.

func isPureReorderingCtx(ctx *rangeContext) bool {
	if len(ctx.oldKeys) != len(ctx.newKeys) {
		return false
	}

	if !sameKeySet(ctx.oldKeys, ctx.newKeys) {
		return false
	}

	positionField := DetectPositionField(ctx.oldByKey)
	positionFieldInt := -1
	if positionField != "" {
		positionFieldInt, _ = strconv.Atoi(positionField)
	}

	for key, oldItem := range ctx.oldByKey {
		newItem, exists := ctx.newByKey[key]
		if !exists {
			return false
		}

		oldItemNode, ok1 := oldItem.(*TreeNode)
		newItemNode, ok2 := newItem.(*TreeNode)

		if !ok1 || !ok2 {
			if !DeepEqual(oldItem, newItem) {
				return false
			}
			continue
		}

		for i, oldValue := range oldItemNode.Dynamics {
			if oldValue == nil {
				continue
			}
			if i == positionFieldInt || (ctx.keyPos >= 0 && i == ctx.keyPos) {
				continue
			}
			newValue, exists := newItemNode.GetDynamic(i)
			if !exists || !DeepEqual(oldValue, newValue) {
				return false
			}
		}

		for i, newValue := range newItemNode.Dynamics {
			if newValue == nil {
				continue
			}
			if i == positionFieldInt || (ctx.keyPos >= 0 && i == ctx.keyPos) {
				continue
			}
			if _, exists := oldItemNode.GetDynamic(i); !exists {
				return false
			}
		}
	}

	for i := range ctx.oldKeys {
		if ctx.oldKeys[i] != ctx.newKeys[i] {
			return true
		}
	}

	return false
}

func areAllItemsAtStartCtx(ctx *rangeContext) bool {
	if len(ctx.addedKeys) == 0 {
		return false
	}
	for i, key := range ctx.addedKeys {
		if i >= len(ctx.newItems) {
			return false
		}
		if itemKey, ok := ctx.getItemKey(ctx.newItems[i]); ok {
			if itemKey != key {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func areAllItemsAtEndCtx(ctx *rangeContext) bool {
	if len(ctx.addedKeys) == 0 || len(ctx.oldKeySet) == 0 {
		return false
	}

	startIndex := len(ctx.newItems) - len(ctx.addedKeys)

	for i := 0; i < startIndex; i++ {
		if i >= len(ctx.newItems) {
			return false
		}
		if itemKey, ok := ctx.getItemKey(ctx.newItems[i]); ok {
			if _, exists := ctx.oldKeySet[itemKey]; !exists {
				return false
			}
		} else {
			return false
		}
	}

	for i, key := range ctx.addedKeys {
		index := startIndex + i
		if index >= len(ctx.newItems) {
			return false
		}
		if itemKey, ok := ctx.getItemKey(ctx.newItems[index]); ok {
			if itemKey != key {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func isComplexInsertionPatternCtx(ctx *rangeContext) bool {
	if len(ctx.addedKeys) == 0 {
		return false
	}

	if areAllItemsAtEndCtx(ctx) {
		return false
	}
	if areAllItemsAtStartCtx(ctx) {
		return false
	}

	insertionPoints := make(map[string]bool, len(ctx.addedKeys))
	for i, item := range ctx.newItems {
		if keyStr, ok := ctx.getItemKey(item); ok {
			if _, inOld := ctx.oldKeySet[keyStr]; !inOld {
				var insertionPoint string
				if i > 0 {
					if prevKeyStr, ok := ctx.getItemKey(ctx.newItems[i-1]); ok {
						insertionPoint = prevKeyStr + ":after"
					}
				} else {
					insertionPoint = "start"
				}
				insertionPoints[insertionPoint] = true
			}
		}
	}

	return len(insertionPoints) > maxInsertionPoints
}

// GetRangeSignature creates a signature for a range construct based on its static template structure.
func GetRangeSignature(rangeValue interface{}) string {
	if node, ok := rangeValue.(*TreeNode); ok {
		if node.HasStatics() {
			return fmt.Sprintf("%v", node.Statics)
		}
		return ""
	}

	rangeMap, ok := rangeValue.(map[string]interface{})
	if !ok {
		return ""
	}
	staticParts, exists := rangeMap["s"]
	if !exists {
		return ""
	}
	return fmt.Sprintf("%v", staticParts)
}

// FindRangeConstructs finds all range constructs in a tree, recursively searching nested structures.
func FindRangeConstructs(tree *TreeNode) map[string]interface{} {
	result := make(map[string]interface{})
	if tree == nil {
		return result
	}
	findRangeConstructsRecursive(tree, "", result)
	return result
}

func findRangeConstructsRecursive(tree *TreeNode, path string, result map[string]interface{}) {
	if tree == nil {
		return
	}

	if tree.HasRange() && tree.HasStatics() {
		result[path] = tree
		return
	}

	for i, value := range tree.Dynamics {
		if value == nil {
			continue
		}
		field := build.PositionKey(i)
		fieldPath := field
		if path != "" {
			fieldPath = path + "." + field
		}
		if IsRangeConstruct(value) {
			result[fieldPath] = value
		} else if nestedTree, ok := value.(*TreeNode); ok {
			findRangeConstructsRecursive(nestedTree, fieldPath, result)
		}
	}
}

// FindRangeConstructMatches finds matching range constructs between old and new trees.
func FindRangeConstructMatches(oldTree, newTree *TreeNode) map[string]string {
	matches := make(map[string]string)
	if oldTree == nil || newTree == nil {
		return matches
	}

	oldRanges := FindRangeConstructs(oldTree)
	newRanges := FindRangeConstructs(newTree)

	for newField, newRange := range newRanges {
		newSignature := GetRangeSignature(newRange)

		matched := false
		for oldField, oldRange := range oldRanges {
			oldSignature := GetRangeSignature(oldRange)
			if newSignature == oldSignature && newSignature != "" {
				matches[newField] = oldField
				matched = true
				break
			}
			oldEmpty := oldSignature == "" || oldSignature == "[]"
			newEmpty := newSignature == "" || newSignature == "[]"
			if oldField == newField && (oldEmpty || newEmpty) {
				matches[newField] = oldField
				matched = true
				break
			}
		}

		if !matched && len(newRanges) == 1 && len(oldRanges) == 1 {
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
