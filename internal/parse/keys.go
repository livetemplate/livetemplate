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
