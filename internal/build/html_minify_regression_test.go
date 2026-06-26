package build

import "testing"

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
