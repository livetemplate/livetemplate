package livetemplate

import (
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeChangeAndCreateTree_EntireContentFallbackParity(t *testing.T) {
	tmpl := &Template{}

	oldHTML := `<div><p>legacy</p></div>`
	newHTML := `<main>
  <section>
    <h1>Dynamic Title</h1>
    <article><p>Body content</p></article>
  </section>
</main>`

	tree, err := tmpl.analyzeChangeAndCreateTree(oldHTML, newHTML, nil, nil)
	if err != nil {
		t.Fatalf("analyzeChangeAndCreateTree returned error: %v", err)
	}

	fallbackTree := tmpl.createHTMLStructureBasedTree(newHTML)
	if !reflect.DeepEqual(tree, fallbackTree) {
		t.Fatalf("expected structural fallback parity\nwant: %#v\ngot:  %#v", fallbackTree, tree)
	}
}

func TestAnalyzeChangeAndCreateTree_PartialChangeKeepsStatics(t *testing.T) {
	tmpl := &Template{}

	oldHTML := `<div><p>Hello</p></div>`
	newHTML := `<div><p>Hello World</p></div>`

	tree, err := tmpl.analyzeChangeAndCreateTree(oldHTML, newHTML, nil, nil)
	if err != nil {
		t.Fatalf("analyzeChangeAndCreateTree returned error: %v", err)
	}

	if !reflect.DeepEqual(tree.Statics, []string{"<div><p>Hello", "</p></div>"}) {
		t.Fatalf("unexpected statics: %#v", tree.Statics)
	}

	dynamic, ok := tree.Dynamics["0"].(string)
	if !ok {
		t.Fatalf("expected string dynamic, got %#v", tree.Dynamics["0"])
	}

	if strings.TrimSpace(dynamic) != "World" {
		t.Fatalf("expected normalized dynamic \"World\", got %q", dynamic)
	}
}
