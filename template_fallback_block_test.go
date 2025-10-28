package livetemplate

import (
	"reflect"
	"testing"
)

func TestTemplateGenerateInitialTreeFallsBackForBlockWithDynamicTemplate(t *testing.T) {
	tmpl := New("block-dynamic-template")

	staticTemplateStr := `{{define "layout"}}<main>{{block "region" .}}{{template "content" .}}{{end}}</main>{{end}}{{define "content"}}<p>{{.Message}}</p>{{end}}{{template "layout" .}}`
	if _, err := tmpl.Parse(staticTemplateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dynamicTemplateStr := `{{define "layout"}}<main>{{block "region" .}}{{template (printf "%s" .PartialName) .}}{{end}}</main>{{end}}{{define "content"}}<p>{{.Message}}</p>{{end}}{{template "layout" .}}`
	tmpl.templateStr = dynamicTemplateStr

	data := map[string]interface{}{
		"PartialName": "content",
		"Message":     "hello",
	}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(dynamicTemplateStr, data, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for block with dynamic template invocation")
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
