package build

import (
	"strings"

	"github.com/livetemplate/livetemplate/internal/util"
)

// AnalyzeChangeAndCreateTree determines the best tree structure based on the type of change
// between old and new HTML content.
//
// The function employs a multi-strategy approach:
// 1. Analyzes common prefix/suffix to identify stable regions
// 2. Creates tree with static boundaries when possible
// 3. Falls back to structure-based segmentation for major changes
//
// This optimizes tree generation by preserving static content when only a portion
// of the HTML has changed.
func AnalyzeChangeAndCreateTree(oldHTML, newHTML string) (*TreeNode, error) {
	// Find common prefix and suffix to understand change patterns
	commonPrefix := util.FindCommonPrefix(oldHTML, newHTML)
	commonSuffix := util.FindCommonSuffix(oldHTML, newHTML)

	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)

	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		// Use the same segmentation strategy as the HTML fallback to ensure
		// updates remain structurally consistent with initial renders.
		return CreateHTMLStructureBasedTree(newHTML), nil
	}

	staticOverlap := len(commonPrefix) + len(commonSuffix)
	if staticOverlap <= 2 {
		hasMarkupFragment := strings.Contains(commonPrefix, "<") || strings.Contains(commonPrefix, ">") ||
			strings.Contains(commonSuffix, "<") || strings.Contains(commonSuffix, ">")

		if hasMarkupFragment {
			return CreateHTMLStructureBasedTree(newHTML), nil
		}
	}

	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		// Dynamic HTML is passed through verbatim (not minified). Text-only
		// dynamics too — the former normalizeWhitespace side-effect was equally
		// unsafe for whitespace-significant content. See #467.
		dynamicPart := newHTML[changeStart:changeEnd]
		tree := NewTreeNodeWithStatics([]string{commonPrefix, commonSuffix})
		tree.SetDynamic(0, dynamicPart)
		return tree, nil
	}

	// Default to full dynamic content
	return CreateHTMLStructureBasedTree(newHTML), nil
}
