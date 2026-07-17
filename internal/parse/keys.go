package parse

import (
	"strings"

	"github.com/livetemplate/livetemplate/internal/keys"
)

// rangeItemWithStatics holds an item tree for range processing.
type rangeItemWithStatics struct {
	tree *TreeNode
}

// hasExplicitKeyAttribute checks if any key attribute is present in the statics.
func hasExplicitKeyAttribute(statics []string) bool {
	if len(statics) == 0 {
		return false
	}
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"",
		"v-key=\"",
	}
	for _, static := range statics {
		for _, attr := range keyAttrs {
			if strings.Contains(static, attr) {
				return true
			}
		}
	}
	return false
}

// detectIDKey detects which dynamic position contains the item ID
// by scanning statics for key attribute patterns.
// Returns the integer index of the dynamic position containing the key.
func detectIDKey(statics []string) int {
	if len(statics) == 0 {
		return 0
	}
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"",
		"v-key=\"",
	}
	for i, static := range statics {
		minPos := -1
		matchedIdx := -1
		for attrIdx, attr := range keyAttrs {
			if pos := strings.Index(static, attr); pos != -1 {
				if minPos == -1 || pos < minPos {
					minPos = pos
					matchedIdx = attrIdx
					if attrIdx == 0 {
						break
					}
				}
			}
		}
		if matchedIdx != -1 {
			return i
		}
	}
	return 0
}

// generateItemHash creates a stable hash for a range item based on its content.
func generateItemHash(item *TreeNode) string {
	if item == nil {
		return ""
	}
	return keys.GenerateItemHashFromSlice(item.Dynamics)
}

// wrappedItemKey returns the explicit data-key of a range item whose real keyed
// element is hidden one level down inside an invocation wrapper (the shape a
// recursive {{template}} range produces: createConditionalWrapper wraps the
// invoked body, so the item's own statics are empty and the <li data-key> lives
// in the single nested child). It reads the key value *through* the wrapper
// without restructuring the item, so the item keeps a stable identity across
// deep edits (the key is the item's path, not a content hash of its subtree).
//
// Returns ("", false) when the item is not this simple single-child wrapper or
// the child carries no key attribute — callers then fall back to content hashing.
func wrappedItemKey(item *TreeNode) (string, bool) {
	if item == nil || hasExplicitKeyAttribute(item.Statics) {
		return "", false
	}
	var child *TreeNode
	for _, d := range item.Dynamics {
		c, ok := d.(*TreeNode)
		if !ok {
			continue
		}
		if child != nil {
			return "", false // more than one nested child: not the simple wrapper
		}
		child = c
	}
	if child == nil || !hasExplicitKeyAttribute(child.Statics) {
		return "", false
	}
	if v, ok := child.GetDynamic(detectIDKey(child.Statics)); ok {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

// allWrappedItemKeys returns the through-wrapper data-key of every item, or
// (nil, false) if any item does not expose one. It is all-or-nothing so a range
// never mixes stable data-keys with content hashes.
func allWrappedItemKeys(items []rangeItemWithStatics) ([]string, bool) {
	out := make([]string, len(items))
	for i, item := range items {
		key, ok := wrappedItemKey(item.tree)
		if !ok {
			return nil, false
		}
		out[i] = key
	}
	return out, true
}
