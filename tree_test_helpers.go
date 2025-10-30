package livetemplate

import (
	"fmt"
	"strings"
)

// reconstructHTML rebuilds HTML string from tree structure
// Used by tree testing to verify tree structure produces correct output
func reconstructHTML(tree *TreeNode) string {
	if tree == nil {
		return ""
	}

	if !tree.HasStatics() {
		return ""
	}

	// Check if this is a range comprehension
	if tree.HasRange() {
		if tree.Range == nil || len(tree.Range.Items) == 0 {
			// Debug: Empty range
			return ""
		}

		var result strings.Builder
		for _, itemDynamics := range tree.Range.Items {
			// Items are *TreeNode
			itemNode, ok := itemDynamics.(*TreeNode)
			if !ok {
				// Skip non-TreeNode items
				continue
			}

			// Reconstruct each item using statics and item dynamics
			for i, static := range tree.Statics {
				result.WriteString(static)
				if i < len(tree.Statics)-1 {
					key := fmt.Sprintf("%d", i)
					if val, exists := itemNode.GetDynamic(key); exists {
						if nestedTree, ok := val.(*TreeNode); ok {
							result.WriteString(reconstructHTML(nestedTree))
						} else {
							result.WriteString(fmt.Sprintf("%v", val))
						}
					}
				}
			}
		}
		return result.String()
	}

	var result strings.Builder

	// Interleave statics and dynamics
	for i, static := range tree.Statics {
		result.WriteString(static)

		// Add dynamic value if exists
		if i < len(tree.Statics)-1 {
			key := fmt.Sprintf("%d", i)
			if val, exists := tree.GetDynamic(key); exists {
				// Check if value is nested tree with its own range
				if nestedTree, ok := val.(*TreeNode); ok {
					result.WriteString(reconstructHTML(nestedTree))
				} else {
					result.WriteString(fmt.Sprintf("%v", val))
				}
			}
		}
	}

	return result.String()
}
