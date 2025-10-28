package livetemplate

import (
	"reflect"
	"strings"
	"testing"
)

func TestTemplateGenerateInitialTreeFallsBackForChannelRange(t *testing.T) {
	tmpl := New("channel-range")
	if _, err := tmpl.Parse(`<ul>{{range .Events}}<li>{{.}}</li>{{end}}</ul>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	events := make(chan string, 2)
	events <- "alpha"
	events <- "beta"
	close(events)
	var data map[string]interface{} = map[string]interface{}{"Events": (<-chan string)(events)}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(`<ul>{{range .Events}}<li>{{.}}</li>{{end}}</ul>`, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for channel range")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data, nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := tmpl.createHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.initialTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.initialTree)
	}
}

func TestTemplateGenerateInitialTreeFallsBackForChannelRangeWithDecls(t *testing.T) {
	tmpl := New("channel-range-with-vars")
	templateStr := `<ul>{{range $i, $event := .Events}}<li>{{$i}}-{{$event}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	events := make(chan string, 2)
	events <- "alpha"
	events <- "beta"
	close(events)
	data := map[string]interface{}{"Events": (<-chan string)(events)}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(templateStr, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for channel range with declarations")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data, nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := tmpl.createHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.initialTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.initialTree)
	}
}

func TestTemplateGenerateInitialTreeFallsBackForIntegerRange(t *testing.T) {
	tmpl := New("range-integer")
	templateStr := `<ol>{{range 3}}<li>#{{.}}</li>{{end}}</ol>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(templateStr, nil, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for integer range")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(nil, nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := tmpl.createHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.initialTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.initialTree)
	}
}

func TestCreateHTMLStructureBasedTreeSegmentsBlockBoundaries(t *testing.T) {
	tmpl := New("fallback-html")
	html := `<div>header</div><main><p>body</p></main><div>footer</div>`

	tree := tmpl.createHTMLStructureBasedTree(html)
	if tree == nil {
		t.Fatalf("expected fallback tree")
	}

	expectedStatics := []string{"<div>header</div>", "", ""}
	if !reflect.DeepEqual(tree.Statics, expectedStatics) {
		t.Fatalf("unexpected statics: %#v", tree.Statics)
	}

	if len(tree.Dynamics) != 2 {
		t.Fatalf("expected 2 dynamic segments, got %d", len(tree.Dynamics))
	}

	segmentZero, ok := tree.Dynamics["0"].(string)
	if !ok {
		t.Fatalf("expected dynamic segment 0 to be string, got %T", tree.Dynamics["0"])
	}
	if !strings.Contains(segmentZero, "<main") || !strings.Contains(segmentZero, "body") {
		t.Fatalf("dynamic segment 0 missing expected content: %q", segmentZero)
	}

	segmentOne, ok := tree.Dynamics["1"].(string)
	if !ok {
		t.Fatalf("expected dynamic segment 1 to be string, got %T", tree.Dynamics["1"])
	}
	if !strings.Contains(segmentOne, "<div") || !strings.Contains(segmentOne, "footer") {
		t.Fatalf("dynamic segment 1 missing expected content: %q", segmentOne)
	}

	if tree.HasRange() {
		t.Fatalf("fallback segmentation should not introduce range metadata")
	}
}
