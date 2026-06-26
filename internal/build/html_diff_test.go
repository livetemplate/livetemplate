package build

import "testing"

// TestAnalyzeChangeAndCreateTree_PreservesDynamicWhitespace confirms the diff
// path (common prefix/suffix with a changed middle) passes the dynamic part
// through verbatim rather than minifying it.
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
