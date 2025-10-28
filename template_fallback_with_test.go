package livetemplate

import (
	"html/template"
	"reflect"
	"testing"

	"iter"
)

func TestTemplateGenerateInitialTreeFallsBackForWithIterSeq(t *testing.T) {
	tmpl := New("with-iter-seq")
	tmpl.Funcs(template.FuncMap{
		"seq": func() iter.Seq[string] {
			return func(yield func(string) bool) {
				if !yield("alpha") {
					return
				}
				yield("beta")
			}
		},
	})

	templateStr := `{{with seq}}<ul>{{range .}}<li>{{.}}</li>{{end}}</ul>{{else}}<p>empty</p>{{end}}`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := NewTreeGenerationContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := parseTemplateToTree(templateStr, nil, newKeyGenerator(), ctx); err == nil {
		t.Fatalf("expected AST parser to error for with pipeline returning iter.Seq")
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
