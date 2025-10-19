package livetemplate

import (
	"fmt"
	"strings"
)

// reconstructHTML rebuilds HTML string from tree structure
// Used by tree testing to verify tree structure produces correct output
func reconstructHTML(tree TreeNode) string {
	if tree == nil {
		return ""
	}

	statics, ok := tree["s"].([]string)
	if !ok {
		return ""
	}

	// Check if this is a range comprehension (has "d" key)
	if dynamics, hasDynamics := tree["d"]; hasDynamics {
		dynamicsArray, ok := dynamics.([]interface{})
		if !ok || len(dynamicsArray) == 0 {
			return ""
		}

		var result strings.Builder
		for _, itemDynamics := range dynamicsArray {
			itemMap, ok := itemDynamics.(map[string]interface{})
			if !ok {
				continue
			}

			// Reconstruct each item using statics and item dynamics
			for i, static := range statics {
				result.WriteString(static)
				if i < len(statics)-1 {
					key := fmt.Sprintf("%d", i)
					if val, exists := itemMap[key]; exists {
						if nestedTree, ok := val.(TreeNode); ok {
							result.WriteString(reconstructHTML(nestedTree))
						} else if nestedMap, ok := val.(map[string]interface{}); ok {
							// Handle nested TreeNode represented as map[string]interface{}
							result.WriteString(reconstructHTML(TreeNode(nestedMap)))
						} else {
							result.WriteString(fmt.Sprintf("%v", val))
						}
					}
				}
			}
		}
		return result.String()
	}

	if len(statics) == 0 {
		return ""
	}

	var result strings.Builder

	// Interleave statics and dynamics
	for i, static := range statics {
		result.WriteString(static)

		// Add dynamic value if exists
		if i < len(statics)-1 {
			key := fmt.Sprintf("%d", i)
			if val, exists := tree[key]; exists {
				// Check if value is nested tree
				if nestedTree, ok := val.(TreeNode); ok {
					result.WriteString(reconstructHTML(nestedTree))
				} else if nestedMap, ok := val.(map[string]interface{}); ok {
					// Handle nested TreeNode represented as map[string]interface{}
					result.WriteString(reconstructHTML(TreeNode(nestedMap)))
				} else {
					result.WriteString(fmt.Sprintf("%v", val))
				}
			}
		}
	}

	return result.String()
}
