package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

// treesFromString parses a template (including any associated {{define}} templates)
// and returns every parse tree, mirroring how the root package feeds
// t.tmpl.Templates() into CollectReferencedIdents.
func treesFromString(t *testing.T, src string) []*parse.Tree {
	t.Helper()
	tmpl, err := template.New("test").Parse(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	var trees []*parse.Tree
	for _, assoc := range tmpl.Templates() {
		if assoc.Tree != nil {
			trees = append(trees, assoc.Tree)
		}
	}
	return trees
}

func TestCollectReferencedIdents(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"plain field", `{{.Name}}`, []string{"Name"}},
		{"method chain", `{{.User.Name}}`, []string{"User", "Name"}},
		{"function arg", `{{len .Items}}`, []string{"Items"}},
		{"if branches", `{{if .Show}}{{.Yes}}{{else}}{{.No}}{{end}}`, []string{"Show", "Yes", "No"}},
		{"range branches", `{{range .Items}}{{.Label}}{{else}}{{.Empty}}{{end}}`, []string{"Items", "Label", "Empty"}},
		{"with", `{{with .Ctx}}{{.Inner}}{{end}}`, []string{"Ctx", "Inner"}},
		{"index string literal", `{{index . "Dynamic"}}`, []string{"Dynamic"}},
		{"variable skips dollar", `{{$x := .Foo}}{{$x.Bar}}`, []string{"Foo", "Bar"}},
		{"nested parenthesized pipe", `{{if gt (len .Items) 0}}ok{{end}}`, []string{"Items"}},
		{"associated templates union", `{{define "sub"}}{{.SubField}}{{end}}{{template "sub" .}}{{.MainField}}`, []string{"SubField", "MainField"}},
		{"empty template", `hello`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectReferencedIdents(treesFromString(t, tt.src)...)
			for _, want := range tt.want {
				if _, ok := got[want]; !ok {
					t.Errorf("missing %q in %v", want, identSet(got))
				}
			}
			if len(tt.want) == 0 && len(got) != 0 {
				t.Errorf("expected empty set, got %v", identSet(got))
			}
		})
	}
}

func TestCollectReferencedIdents_NilAndEmpty(t *testing.T) {
	if got := CollectReferencedIdents(); len(got) != 0 {
		t.Errorf("no trees: expected empty, got %v", identSet(got))
	}
	if got := CollectReferencedIdents(nil, nil); len(got) != 0 {
		t.Errorf("nil trees: expected empty, got %v", identSet(got))
	}
}

func identSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
