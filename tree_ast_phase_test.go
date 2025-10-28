package livetemplate

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
)

func TestParseTemplateToTree_HandlesCommentOnly(t *testing.T) {
	tree, err := parseTemplateToTree("{{/* nothing */}}", nil, newKeyGenerator())
	if err != nil {
		t.Fatalf("parseTemplateToTree returned error: %v", err)
	}
	if tree == nil {
		t.Fatal("expected tree, got nil")
	}
	if len(tree.Statics) != 1 {
		t.Fatalf("expected 1 static entry, got %d", len(tree.Statics))
	}
	if tree.Statics[0] != "" {
		t.Fatalf("expected empty static string, got %q", tree.Statics[0])
	}
	if tree.HasDynamics() {
		t.Fatalf("expected no dynamics, got %v", tree.Dynamics)
	}
}

func TestParseTemplateToTree_WithFuncMapRange(t *testing.T) {
	tmplStr := `<ul>{{range split .CSV ","}}<li>{{.}}</li>{{end}}</ul>`
	data := map[string]string{"CSV": "alpha,beta,gamma"}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = template.FuncMap{
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
	}

	tree, err := parseTemplateToTree(tmplStr, data, newKeyGenerator(), ctx)
	if err != nil {
		t.Fatalf("parseTemplateToTree returned error: %v", err)
	}
	if !reflect.DeepEqual(tree.Statics, []string{"<ul>", "</ul>"}) {
		t.Fatalf("unexpected statics: %#v", tree.Statics)
	}

	dynamic, ok := tree.Dynamics["0"]
	if !ok {
		t.Fatalf("expected dynamic range at position 0")
	}

	rangeNode, ok := dynamic.(*TreeNode)
	if !ok {
		t.Fatalf("expected *TreeNode for range dynamic, got %T", dynamic)
	}
	if !rangeNode.HasRange() {
		t.Fatalf("expected range node to have range data")
	}
	if rangeNode.Range == nil || len(rangeNode.Range.Items) != 3 {
		t.Fatalf("expected 3 range items, got %v", rangeNode.Range)
	}
}
