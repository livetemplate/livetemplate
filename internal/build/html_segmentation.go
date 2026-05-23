package build

import (
	"strings"

	"github.com/livetemplate/livetemplate/internal/render"
	"golang.org/x/net/html"
)

// htmlBlockTags defines block-level HTML tag names that create natural segment
// boundaries for tree-based HTML structure analysis. Stored as bare tag names
// (no leading "<") to match html.Tokenizer.TagName() output directly.
var htmlBlockTags = map[string]struct{}{
	"div": {}, "article": {}, "section": {}, "main": {},
	"aside": {}, "nav": {}, "ul": {}, "ol": {}, "table": {},
}

// findBlockTagBoundaries walks htmlDoc once with html.Tokenizer and returns
// ascending byte offsets of every StartTagToken whose tag name is in
// htmlBlockTags. The tokenizer correctly ignores block-tag-shaped substrings
// inside comments, RAWTEXT, RCDATA, and attribute values.
func findBlockTagBoundaries(htmlDoc string) []int {
	var boundaries []int
	z := html.NewTokenizer(strings.NewReader(htmlDoc))
	offset := 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return boundaries
		}
		raw := z.Raw()
		if tt == html.StartTagToken {
			name, _ := z.TagName()
			if _, ok := htmlBlockTags[string(name)]; ok {
				boundaries = append(boundaries, offset)
			}
		}
		offset += len(raw)
	}
}

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
func CreateHTMLStructureBasedTree(htmlDoc string) *TreeNode {
	// Boundaries are returned in document order by the single forward walk.
	boundaries := findBlockTagBoundaries(htmlDoc)

	if len(boundaries) > 0 {
		// Create segments based on boundaries
		const maxSegments = 8
		segmentSize := len(htmlDoc) / maxSegments

		var statics []string
		var dynamics []interface{}
		lastPos := 0

		for i, boundary := range boundaries {
			// Only create a segment if it's large enough
			if boundary-lastPos > segmentSize || i == len(boundaries)-1 {
				if lastPos == 0 {
					// First segment is typically more static (head, nav, etc)
					statics = append(statics, htmlDoc[lastPos:boundary])
				} else {
					// Create a dynamic segment
					statics = append(statics, "")
					dynamics = append(dynamics, htmlDoc[lastPos:boundary])
				}
				lastPos = boundary
			}
		}

		// Add the final segment
		if lastPos < len(htmlDoc) {
			statics = append(statics, "")
			dynamics = append(dynamics, htmlDoc[lastPos:])
		}

		// Build the tree
		tree := NewTreeNodeWithStatics(statics)
		for i, dyn := range dynamics {
			// Minify HTML content if it's a string containing HTML
			if strDyn, ok := dyn.(string); ok && strings.Contains(strDyn, "<") {
				dyn = render.MinifyHTML(strDyn)
			}
			tree.SetDynamic(i, dyn)
		}

		// If we got reasonable segmentation, use it
		if len(statics) > 2 && len(dynamics) > 0 {
			return tree
		}
	}

	// Fallback to single segment strategy
	fallback := NewTreeNodeWithStatics([]string{"", ""})
	fallback.SetDynamic(0, render.MinifyHTML(htmlDoc))
	return fallback
}
