package invariants

import (
	"strings"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TreeToMap converts a TreeNode to a map for JSON serialization.
// This is used for cross-validation with the TypeScript oracle.
func TreeToMap(tree *build.TreeNode) map[string]any {
	if tree == nil {
		return nil
	}

	result := make(map[string]any)

	if tree.HasStatics() {
		result["s"] = tree.Statics
	}

	for k, v := range tree.GetDynamics() {
		result[k] = convertValue(v)
	}

	if tree.HasRange() && tree.Range != nil {
		// Convert range items recursively
		convertedItems := make([]any, len(tree.Range.Items))
		for i, item := range tree.Range.Items {
			convertedItems[i] = convertValue(item)
		}
		result["d"] = convertedItems
		if tree.Range.Statics != nil {
			result["s"] = tree.Range.Statics
		}
		if tree.Metadata != nil && tree.Metadata.IDKey != "" {
			result["m"] = map[string]any{"idKey": tree.Metadata.IDKey}
		}
	}

	return result
}

// convertValue converts any value, handling nested TreeNodes and maps.
func convertValue(v any) any {
	switch val := v.(type) {
	case *build.TreeNode:
		return TreeToMap(val)
	case map[string]any:
		result := make(map[string]any)
		for k, v2 := range val {
			result[k] = convertValue(v2)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v2 := range val {
			result[i] = convertValue(v2)
		}
		return result
	default:
		return v
	}
}

// normalizeHTML normalizes HTML for comparison.
func normalizeHTML(html string) string {
	// Remove extra whitespace
	html = strings.TrimSpace(html)
	// Normalize newlines
	html = strings.ReplaceAll(html, "\r\n", "\n")
	// Remove whitespace between tags
	for strings.Contains(html, "> <") {
		html = strings.ReplaceAll(html, "> <", "><")
	}
	for strings.Contains(html, ">\n<") {
		html = strings.ReplaceAll(html, ">\n<", "><")
	}
	for strings.Contains(html, ">\t<") {
		html = strings.ReplaceAll(html, ">\t<", "><")
	}
	return html
}
