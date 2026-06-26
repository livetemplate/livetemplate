package build

import (
	"strings"
	"testing"
)

// preWrapDynamic is a CSS-whitespace-significant fragment: the doubled/odd
// spaces are meaningful because a class (not a tag name) makes the element
// white-space:pre-wrap. tdewolff's HTML minifier is tag-aware but CSS-blind,
// so it used to collapse this into "indented code", silently corrupting
// highlighted code, diffs, and ASCII art. See #467.
const preWrapDynamic = `  indented   code  `

// TestCreateHTMLStructureBasedTree_PreservesDynamicWhitespace confirms the
// fallback single-segment path passes dynamic HTML through verbatim rather
// than minifying it. Regression test for #467.
func TestCreateHTMLStructureBasedTree_PreservesDynamicWhitespace(t *testing.T) {
	// No block-level tags -> findBlockTagBoundaries returns empty -> the
	// fallback single-segment tree carries the whole input as one dynamic.
	input := `<span class="chroma">` + preWrapDynamic + `</span>`

	tree := CreateHTMLStructureBasedTree(input)
	got, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatalf("expected a dynamic at index 0, tree=%#v", tree)
	}
	if got != input {
		t.Errorf("dynamic HTML was not preserved verbatim:\n got: %q\nwant: %q", got, input)
	}
}

// TestCreateHTMLStructureBasedTree_MultiSegmentPreservesWhitespace confirms the
// *block-tag* multi-segment path (not the fallback) also stores dynamic
// segments verbatim. Real block tags make findBlockTagBoundaries return
// boundaries, exercising the `for i, dyn := range dynamics` loop where
// minification was removed. Regression test for #467.
func TestCreateHTMLStructureBasedTree_MultiSegmentPreservesWhitespace(t *testing.T) {
	// Several block-level <div>s spaced far enough apart to exceed segmentSize
	// (len/8), so the segmentation loop emits multiple dynamic segments. The
	// final segment carries the whitespace-significant content.
	pad := strings.Repeat("x", 80)
	chroma := `<span class="chroma">` + preWrapDynamic + `</span>`
	input := `<div>` + pad + `</div>` +
		`<div>` + pad + `</div>` +
		`<div>` + pad + `</div>` +
		`<div>` + chroma + `</div>`

	tree := CreateHTMLStructureBasedTree(input)

	// The fallback path produces exactly 2 statics; the multi-segment path
	// produces more. Guard that we are actually exercising the intended path.
	if len(tree.Statics) <= 2 {
		t.Fatalf("expected multi-segment tree (>2 statics), got %d — test no longer exercises the block-tag path", len(tree.Statics))
	}

	found := false
	for i := 0; i < tree.DynamicLen(); i++ {
		if v, ok := tree.GetDynamic(i); ok {
			if s, isStr := v.(string); isStr && strings.Contains(s, preWrapDynamic) {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("whitespace-significant content was collapsed or missing across multi-segment dynamics: %#v", tree.Dynamics)
	}
}

// TestAnalyzeChangeAndCreateTree_PreservesDynamicWhitespace confirms the diff
// path (common prefix/suffix with a changed middle) passes the dynamic part
// through verbatim. Regression test for #467.
func TestAnalyzeChangeAndCreateTree_PreservesDynamicWhitespace(t *testing.T) {
	oldHTML := `<div class="chroma">OLD</div>`
	newHTML := `<div class="chroma">` + preWrapDynamic + `</div>`

	tree, err := AnalyzeChangeAndCreateTree(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("AnalyzeChangeAndCreateTree returned error: %v", err)
	}
	got, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatalf("expected a dynamic at index 0, tree=%#v", tree)
	}
	if got != preWrapDynamic {
		t.Errorf("dynamic HTML was not preserved verbatim:\n got: %q\nwant: %q", got, preWrapDynamic)
	}
}
