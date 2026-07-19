package parse

import (
	"bytes"
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

	out, defines, err := FlattenTemplate(mustParse(t, src))
	if err != nil {
		t.Fatalf("diamond composition must flatten without error, got: %v", err)
	}
	// A diamond is acyclic, so nothing is left un-inlined and no registry
	// {{define}} blocks are emitted.
	if defines != "" {
		t.Errorf("acyclic composition must emit no recursion defines, got:\n%s", defines)
	}
	// leaf's statics appear once per invocation (3×): proves each was inlined.
	if got := strings.Count(out, "<span>"); got != 3 {
		t.Errorf("expected leaf inlined 3 times, got %d in:\n%s", got, out)
	}
}

// TestFlattenTemplate_RecursionEmittedForRuntime covers the self-referential
// invocation graphs that once stack-overflowed during Parse. They must now
// flatten cleanly: each cycle member's {{template}} call is left verbatim (so it
// survives re-parse and is evaluated at build time by invokeTemplate) and its
// body is re-emitted once as a {{define}} block for the recursion registry.
func TestFlattenTemplate_RecursionEmittedForRuntime(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// verbatim invocations that must remain un-inlined, and the {{define}}
		// blocks that must be appended for the registry.
		wantVerbatim []string
		wantDefines  []string
	}{
		{
			name: "direct recursion",
			src: `{{define "treeNode"}}<li>{{.Name}}<ul>` +
				`{{range .Children}}{{template "treeNode" .}}{{end}}` +
				`</ul></li>{{end}}` +
				`{{template "treeNode" .}}`,
			wantVerbatim: []string{`{{template "treeNode"`},
			wantDefines:  []string{`{{define "treeNode"}}`},
		},
		{
			name: "mutual recursion",
			src: `{{define "a"}}<div>{{template "b" .}}</div>{{end}}` +
				`{{define "b"}}<span>{{template "a" .}}</span>{{end}}` +
				`{{template "a" .}}`,
			wantVerbatim: []string{`{{template "a"`, `{{template "b"`},
			wantDefines:  []string{`{{define "a"}}`, `{{define "b"}}`},
		},
		{
			name:         "self-referential entry point",
			src:          `<div>{{template "main" .}}</div>`,
			wantVerbatim: []string{`{{template "main"`},
			wantDefines:  []string{`{{define "main"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, defines, err := FlattenTemplate(mustParse(t, tt.src))
			if err != nil {
				t.Fatalf("recursive template must flatten for runtime invocation, got: %v", err)
			}
			// "Left un-inlined" is a property of the flattened result as a whole,
			// so assert against the assembly. Under mutual recursion the call to
			// "b" lives inside {{define "a"}} — i.e. in defines, not the
			// document — and pinning it to either half alone would encode which
			// cycle member happens to be the entry point.
			assembled := out + defines
			for _, want := range tt.wantVerbatim {
				if !strings.Contains(assembled, want) {
					t.Errorf("expected verbatim invocation %q (recursion left un-inlined) in:\n%s", want, assembled)
				}
			}
			// Registry blocks belong to the SECOND return, not the document —
			// asserting against defines is what keeps the split honest. Checking
			// out+defines instead would still pass if the two were re-merged.
			for _, want := range tt.wantDefines {
				if !strings.Contains(defines, want) {
					t.Errorf("expected registry define %q in the defines return, got:\n%s", want, defines)
				}
				if strings.Contains(out, want) {
					t.Errorf("registry define %q leaked into the document return:\n%s", want, out)
				}
			}
			// The assembled string must re-parse (callers concatenate the two and
			// feed that to parse.Parse), which also confirms the defines are
			// well-formed and that concatenation is a valid template.
			if _, err := template.New("verify").Parse(out + defines); err != nil {
				t.Errorf("document+defines must re-parse, got: %v\n%s", err, out+defines)
			}
		})
	}
}

// TestFlattenTemplate_CycleBackstop proves the active-path guard (checkFlattenCycle)
// still fires as a safety net: if detection ever under-identifies a cycle, the
// un-emitted {{template}} call re-enters walkAndFlatten and must produce a clean
// ParseError naming the cycle — never a stack overflow. Simulated by walking a
// self-referential template with an empty recursive set (detection "missed" it).
func TestFlattenTemplate_CycleBackstop(t *testing.T) {
	src := `{{define "treeNode"}}<li>{{.Name}}` +
		`{{range .Children}}{{template "treeNode" .}}{{end}}` +
		`</li>{{end}}` +
		`{{template "treeNode" .}}`
	tmpl := mustParse(t, src)
	templates := map[string]*template.Template{}
	for _, tm := range tmpl.Templates() {
		templates[tm.Name()] = tm
	}

	var buf bytes.Buffer
	err := walkAndFlatten(tmpl.Tree.Root, templates, &buf, []string{tmpl.Name()}, map[string]bool{})
	if err == nil {
		t.Fatal("empty recursive set must fall through to the cycle backstop, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "treeNode -> treeNode") || !strings.Contains(err.Error(), "recursive") {
		t.Errorf("backstop error should name the cycle, got: %v", err)
	}
}

// TestFlattenTemplate_NonRecursiveUnaffected guards against the guard: ordinary
// (acyclic) composition must keep flattening byte-for-byte as before.
func TestFlattenTemplate_NonRecursiveUnaffected(t *testing.T) {
	src := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}` +
		`{{define "footer"}}<footer>{{.Year}}</footer>{{end}}` +
		`{{template "header" .}}<main>{{.Body}}</main>{{template "footer" .}}`

	out, defines, err := FlattenTemplate(mustParse(t, src))
	if err != nil {
		t.Fatalf("non-recursive composition must flatten cleanly, got: %v", err)
	}
	if defines != "" {
		t.Errorf("non-recursive composition must emit no recursion defines, got:\n%s", defines)
	}
	for _, want := range []string{"<h1>", "<main>", "<footer>"} {
		if !strings.Contains(out, want) {
			t.Errorf("flattened output missing %q:\n%s", want, out)
		}
	}
}
