package build

import (
	"fmt"
	"strings"

	"github.com/livetemplate/livetemplate/internal/render"
)

// htmlBlockTags defines block-level HTML elements that create natural segment boundaries
// for tree-based HTML structure analysis and segmentation.
var htmlBlockTags = []string{"<div", "<article", "<section", "<main", "<aside", "<nav", "<ul", "<ol", "<table"}

// CreateHTMLStructureBasedTree implements deterministic segmentation strategies for HTML content.
// It analyzes HTML structure and creates a tree representation with static and dynamic segments.
//
// The function uses a multi-strategy approach:
// 1. Identifies block-level HTML tag boundaries
// 2. Creates segments at natural HTML structure points
// 3. Falls back to single-segment strategy if segmentation is insufficient
//
// This is used as a fallback when template parsing fails or for initial tree generation
// without template metadata.
func CreateHTMLStructureBasedTree(html string) *TreeNode {
	// Find the positions of block elements
	var boundaries []int
	for _, tag := range htmlBlockTags {
		idx := 0
		for {
			pos := strings.Index(html[idx:], tag)
			if pos == -1 {
				break
			}
			boundaries = append(boundaries, idx+pos)
			idx = idx + pos + len(tag)
		}
	}

	// Sort boundaries
	if len(boundaries) > 0 {
		// Simple sort
		for i := 0; i < len(boundaries)-1; i++ {
			for j := i + 1; j < len(boundaries); j++ {
				if boundaries[i] > boundaries[j] {
					boundaries[i], boundaries[j] = boundaries[j], boundaries[i]
				}
			}
		}

		// Create segments based on boundaries
		const maxSegments = 8
		segmentSize := len(html) / maxSegments

		var statics []string
		var dynamics []interface{}
		lastPos := 0
		dynamicIndex := 0

		for i, boundary := range boundaries {
			// Only create a segment if it's large enough
			if boundary-lastPos > segmentSize || i == len(boundaries)-1 {
				if lastPos == 0 {
					// First segment is typically more static (head, nav, etc)
					statics = append(statics, html[lastPos:boundary])
				} else {
					// Create a dynamic segment
					statics = append(statics, "")
					dynamics = append(dynamics, html[lastPos:boundary])
					dynamicIndex++
				}
				lastPos = boundary
			}
		}

		// Add the final segment
		if lastPos < len(html) {
			statics = append(statics, "")
			dynamics = append(dynamics, html[lastPos:])
		}

		// Build the tree
		tree := NewTreeNodeWithStatics(statics)
		for i, dyn := range dynamics {
			// Minify HTML content if it's a string containing HTML
			if strDyn, ok := dyn.(string); ok && strings.Contains(strDyn, "<") {
				dyn = render.MinifyHTML(strDyn)
			}
			tree.SetDynamic(fmt.Sprintf("%d", i), dyn)
		}

		// If we got reasonable segmentation, use it
		if len(statics) > 2 && len(dynamics) > 0 {
			return tree
		}
	}

	// Fallback to single segment strategy
	fallback := NewTreeNodeWithStatics([]string{"", ""})
	fallback.SetDynamic("0", render.MinifyHTML(html))
	return fallback
}
