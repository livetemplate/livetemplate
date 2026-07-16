package parse

import (
	"errors"
	"html/template"
	"strings"
	"testing"
)

// mustParse builds an associated template set from src. The root template is
// named "main"; any {{define}} blocks in src become associated templates, the
// same shape parseInternal produces before it calls FlattenTemplate.
func mustParse(t *testing.T, src string) *template.Template {
	t.Helper()
	tmpl, err := template.New("main").Parse(src)
	if err != nil {
		t.Fatalf("stdlib Parse failed (fixture bug, not the code under test): %v", err)
	}
	return tmpl
}

// TestFlattenTemplate_Diamond is the discriminating test for the cycle guard:
// the same template invoked more than once on non-nested paths is NOT a cycle
// and must still flatten. "leaf" is invoked three times (twice as siblings in
// "row", once directly in "page") with no invocation ever nested inside its own
// body. A correct active-path guard pops each invocation on return, so this
// flattens cleanly; a global visited-set would wrongly reject the second "leaf".
func TestFlattenTemplate_Diamond(t *testing.T) {
	src := `{{define "leaf"}}<span>{{.}}</span>{{end}}` +
		`{{define "row"}}{{template "leaf" .A}}{{template "leaf" .B}}{{end}}` +
		`{{define "page"}}{{template "row" .}}{{template "leaf" .C}}{{end}}` +
		`{{template "page" .}}`

	out, err := FlattenTemplate(mustParse(t, src))
	if err != nil {
		t.Fatalf("diamond composition must flatten without error, got: %v", err)
	}
	// leaf's statics appear once per invocation (3×): proves each was inlined.
	if got := strings.Count(out, "<span>"); got != 3 {
		t.Errorf("expected leaf inlined 3 times, got %d in:\n%s", got, out)
	}
}

// TestFlattenTemplate_Cycles covers the self-referential invocation graphs that
// stack-overflowed during Parse before this guard. Each must come back as a
// *ParseError naming the cycle — never a panic and never a silent success.
func TestFlattenTemplate_Cycles(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantCycle string // substring expected in the reported cycle
	}{
		{
			name: "direct recursion",
			src: `{{define "treeNode"}}<li>{{.Name}}<ul>` +
				`{{range .Children}}{{template "treeNode" .}}{{end}}` +
				`</ul></li>{{end}}` +
				`{{template "treeNode" .}}`,
			wantCycle: "treeNode -> treeNode",
		},
		{
			name: "mutual recursion",
			src: `{{define "a"}}<div>{{template "b" .}}</div>{{end}}` +
				`{{define "b"}}<span>{{template "a" .}}</span>{{end}}` +
				`{{template "a" .}}`,
			wantCycle: "a -> b -> a",
		},
		{
			name:      "self-referential entry point",
			src:       `<div>{{template "main" .}}</div>`,
			wantCycle: "main -> main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FlattenTemplate(mustParse(t, tt.src))
			if err == nil {
				t.Fatal("expected a ParseError for recursive template, got nil")
			}

			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("expected *ParseError, got %T: %v", err, err)
			}
			if pe.Phase != "parse" {
				t.Errorf("expected Phase %q, got %q", "parse", pe.Phase)
			}
			if !strings.Contains(err.Error(), tt.wantCycle) {
				t.Errorf("error should report cycle %q, got: %v", tt.wantCycle, err)
			}
			if !strings.Contains(err.Error(), "recursive") {
				t.Errorf("error should mention %q, got: %v", "recursive", err)
			}
		})
	}
}

// TestFlattenTemplate_NonRecursiveUnaffected guards against the guard: ordinary
// (acyclic) composition must keep flattening byte-for-byte as before.
func TestFlattenTemplate_NonRecursiveUnaffected(t *testing.T) {
	src := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}` +
		`{{define "footer"}}<footer>{{.Year}}</footer>{{end}}` +
		`{{template "header" .}}<main>{{.Body}}</main>{{template "footer" .}}`

	out, err := FlattenTemplate(mustParse(t, src))
	if err != nil {
		t.Fatalf("non-recursive composition must flatten cleanly, got: %v", err)
	}
	for _, want := range []string{"<h1>", "<main>", "<footer>"} {
		if !strings.Contains(out, want) {
			t.Errorf("flattened output missing %q:\n%s", want, out)
		}
	}
}
