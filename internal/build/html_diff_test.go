package build

import "testing"

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
