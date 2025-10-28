package livetemplate

import (
	"reflect"
	"testing"
)

func TestTemplateGenerateInitialTreeFallsBackForRangeBreak(t *testing.T) {
	tmpl := New("range-break-fallback")
	templateStr := `<ul>{{range .Items}}{{if eq . "stop"}}{{break}}{{end}}<li>{{.}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Items": []string{"alpha", "stop", "gamma"}}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(templateStr, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for range with break")
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

func TestTemplateGenerateInitialTreeFallsBackForRangeContinue(t *testing.T) {
	tmpl := New("range-continue-fallback")
	templateStr := `<ul>{{range $i, $item := .Items}}{{if eq $item "skip"}}{{continue}}{{end}}<li>{{$i}}-{{$item}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Items": []string{"alpha", "skip", "gamma"}}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(templateStr, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for range with continue")
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
