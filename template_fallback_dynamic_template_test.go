package livetemplate

import (
	"reflect"
	"testing"
)

func TestTemplateGenerateInitialTreeFallsBackForDynamicTemplateInvocation(t *testing.T) {
	tmpl := New("dynamic-template")

	staticTemplateStr := `{{define "content"}}<p>{{.Message}}</p>{{end}}<section>{{template "content" .}}</section>`
	if _, err := tmpl.Parse(staticTemplateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dynamicTemplateStr := `{{define "content"}}<p>{{.Message}}</p>{{end}}<section>{{template (printf "%s" .PartialName) .}}</section>`
	// Override the template source to mimic a runtime-selected partial name.
	// html/template rejects this construct during parsing, which is exactly
	// what triggers the fallback path we want to guard.
	tmpl.templateStr = dynamicTemplateStr

	data := map[string]interface{}{
		"PartialName": "content",
		"Message":     "hello",
	}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(dynamicTemplateStr, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for dynamic template invocation")
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

	// The initial tree must match HTML segmentation fallback.
	expected := tmpl.createHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.initialTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.initialTree)
	}
}
