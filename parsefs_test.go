package livetemplate

import (
	"bytes"
	"embed"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

//go:embed testdata/simple.html testdata/layout.html testdata/content.html
var parseFSTestData embed.FS

//go:embed testdata/project/main.gotemplate
var parseFSProjectData embed.FS

// wrapperIDPattern matches the random per-parse wrapper token so two renders of
// the same template (which differ only in that token) can be compared for parity.
var wrapperIDPattern = regexp.MustCompile(`data-lvt-id="[^"]*"`)

func renderNormalized(t *testing.T, tmpl *Template, data interface{}) string {
	t.Helper()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	return wrapperIDPattern.ReplaceAllString(buf.String(), `data-lvt-id="X"`)
}

// TestParseFS_RendersSameAsParseFiles is the golden-parity check: the same
// template parsed from disk (ParseFiles) and from an fs.FS (ParseFS) must render
// identically once the random wrapper id is normalized.
func TestParseFS_RendersSameAsParseFiles(t *testing.T) {
	data := map[string]interface{}{"Title": "Hi", "Content": "Body text"}

	fromFiles := Must(New("test"))
	if _, err := fromFiles.ParseFiles("testdata/simple.html"); err != nil {
		t.Fatalf("ParseFiles() failed: %v", err)
	}

	fromFS := Must(New("test"))
	if _, err := fromFS.ParseFS(parseFSTestData, "testdata/simple.html"); err != nil {
		t.Fatalf("ParseFS() failed: %v", err)
	}

	if got, want := renderNormalized(t, fromFS, data), renderNormalized(t, fromFiles, data); got != want {
		t.Errorf("ParseFS render != ParseFiles render\n ParseFS:   %s\n ParseFiles: %s", got, want)
	}
}

// TestWithParseFS_ViaNew exercises the New() precedence wiring: WithParseFS parses
// straight from the embed.FS, no temp-file staging.
func TestWithParseFS_ViaNew(t *testing.T) {
	tmpl := Must(New("app", WithParseFS(parseFSTestData, "testdata/simple.html")))
	out := renderNormalized(t, tmpl, map[string]interface{}{"Title": "Embedded", "Content": "works"})
	for _, want := range []string{"Embedded", "works"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got: %s", want, out)
		}
	}
}

// TestWithParseFS_PrecedenceOverParseFiles verifies WithParseFS wins when both it
// and WithParseFiles are configured.
func TestWithParseFS_PrecedenceOverParseFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"tpl/main.html": &fstest.MapFile{Data: []byte(`<div>from-fs {{.V}}</div>`)},
	}
	tmpl := Must(New("app",
		WithParseFiles("testdata/simple.html"),
		WithParseFS(fsys, "tpl/*.html"),
	))
	out := renderNormalized(t, tmpl, map[string]interface{}{"V": "value"})
	if !strings.Contains(out, "from-fs value") {
		t.Errorf("expected the fs.FS template to win, got: %s", out)
	}
}

// TestParseFS_Composition parses a main template plus a {{define}} partial from
// separate fs.FS entries into one set.
func TestParseFS_Composition(t *testing.T) {
	fromFiles := Must(New("test"))
	if _, err := fromFiles.ParseFiles("testdata/layout.html", "testdata/content.html"); err != nil {
		t.Fatalf("ParseFiles() failed: %v", err)
	}

	fromFS := Must(New("test"))
	if _, err := fromFS.ParseFS(parseFSTestData, "testdata/layout.html", "testdata/content.html"); err != nil {
		t.Fatalf("ParseFS() failed: %v", err)
	}

	data := map[string]interface{}{"Title": "T", "Heading": "H", "Body": "B"}
	if got, want := renderNormalized(t, fromFS, data), renderNormalized(t, fromFiles, data); got != want {
		t.Errorf("ParseFS composition render != ParseFiles\n ParseFS:    %s\n ParseFiles: %s", got, want)
	}
}

// TestWithParseFS_WithComponentTemplates exercises the clone-and-extend branch in
// parseSources via the WithParseFS path: when component templates are already
// parsed, the main template (loaded from an fs.FS) must compose with them, same as
// the WithParseFiles path. main.gotemplate invokes {{template "test:greeting:v1"}}.
// TestParseFS_SingleGlobLexicalMain confirms the order-sensitive rule: with one
// wildcard pattern matching several files, fs.Glob's lexical order makes the
// lexically-first match the main template (rendered by Execute), and the rest
// compose in as {{define}} partials.
func TestParseFS_SingleGlobLexicalMain(t *testing.T) {
	fsys := fstest.MapFS{
		// "a-home" sorts before "b-widget", so it must be the main template.
		"tpl/a-home.html":   &fstest.MapFile{Data: []byte(`<div>home {{.V}} {{template "widget" .}}</div>`)},
		"tpl/b-widget.html": &fstest.MapFile{Data: []byte(`{{define "widget"}}<span>widget</span>{{end}}`)},
	}
	tmpl := Must(New("app", WithParseFS(fsys, "tpl/*.html")))
	out := renderNormalized(t, tmpl, map[string]interface{}{"V": "value"})
	if !strings.Contains(out, "home value") {
		t.Errorf("lexically-first match should be the main template; got: %s", out)
	}
	if !strings.Contains(out, "<span>widget</span>") {
		t.Errorf("later matches should compose in as partials; got: %s", out)
	}
}

func TestWithParseFS_WithComponentTemplates(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
	}
	tmpl, err := New("test",
		WithComponentTemplates(set),
		WithParseFS(parseFSProjectData, "testdata/project/main.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if tmpl.tmpl.Lookup("test:greeting:v1") == nil {
		t.Error("component 'test:greeting:v1' not found via WithParseFS path — clone-and-extend branch not hit")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Main Template")) {
		t.Errorf("expected main template composed with component, got: %s", buf.String())
	}
}

func TestParseFS_Errors(t *testing.T) {
	fsys := fstest.MapFS{
		"a.html": &fstest.MapFile{Data: []byte(`<div>{{.X}}</div>`)},
	}
	tests := []struct {
		name     string
		patterns []string
	}{
		{"no patterns", nil},
		{"pattern matches nothing", []string{"nope/*.html"}},
		{"bad glob pattern", []string{"[invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := Must(New("test"))
			if _, err := tmpl.ParseFS(fsys, tt.patterns...); err == nil {
				t.Errorf("ParseFS(%v) expected error, got nil", tt.patterns)
			}
		})
	}
}
