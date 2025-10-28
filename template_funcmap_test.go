package livetemplate

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestTemplateGenerateTreeWithFuncMap(t *testing.T) {
	tmpl := New("funcMap").Funcs(template.FuncMap{
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
	})

	if _, err := tmpl.Parse(`<ul>{{range split .CSV ","}}<li>{{.}}</li>{{end}}</ul>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Render once to exercise the helper paths used in production.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"CSV": "one,two"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	tree, err := tmpl.generateTreeInternalWithErrors(map[string]string{"CSV": "one,two"}, nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	dynamic, ok := tree.Dynamics["0"]
	if !ok {
		t.Fatalf("expected dynamic range at position 0")
	}

	rangeNode, ok := dynamic.(*TreeNode)
	if !ok {
		t.Fatalf("expected *TreeNode for dynamic, got %T", dynamic)
	}

	if !rangeNode.HasRange() {
		t.Fatalf("expected range node to have range data")
	}

	if rangeNode.Range == nil || len(rangeNode.Range.Items) != 2 {
		t.Fatalf("expected 2 items in range, got %v", rangeNode.Range)
	}

	if tmpl.initialTree == nil {
		t.Fatalf("expected initial tree to be cached")
	}
}
