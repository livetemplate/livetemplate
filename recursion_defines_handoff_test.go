package livetemplate

import (
	"strings"
	"testing"
)

// TestRecursiveTemplate_BodyContentCarriesDefines is the rehomed half of the
// guarantee that used to live in internal/build's TrailingDefines test.
//
// Body extraction is now a pure <body>…</body> slice, so the recursion
// {{define}} blocks — which FlattenTemplate emits for cycle members and which
// sit past </html> — are re-attached by the caller from FlattenTemplate's second
// return rather than rescanned out of the template string (issue #496). The
// behaviour being pinned is unchanged: the content handed to the reactive parse
// must carry them, or a full-document recursive template silently degrades to
// HTML-string diffing.
func TestRecursiveTemplate_BodyContentCarriesDefines(t *testing.T) {
	const fullDoc = `<!DOCTYPE html><html><head><title>T</title></head><body>` +
		`{{define "node"}}<li data-key="{{.Path}}">{{.Name}}` +
		`{{if .IsDir}}<ul>{{range .Children}}{{template "node" .}}{{end}}</ul>{{end}}</li>{{end}}` +
		`<ul>{{template "node" .Root}}</ul></body></html>`

	tmpl, err := Must(New("handoff")).Parse(fullDoc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tmpl.mu.Lock()
	body := tmpl.getOrComputeBodyContent()
	tmpl.mu.Unlock()

	if !strings.Contains(body, `{{define "node"}}`) {
		t.Errorf("body content dropped the recursion define; the reactive parse would see an empty registry:\n%s", body)
	}
	// Exactly once — the block lives in templateStr's tail as well, so appending
	// it unconditionally rather than only when a body was sliced would duplicate
	// every definition.
	if got := strings.Count(body, `{{define "node"}}`); got != 1 {
		t.Errorf("expected the recursion define exactly once, got %d:\n%s", got, body)
	}
	// The <body> region itself must still be sliced — this is body content, not
	// the whole document.
	if strings.Contains(body, "<title>") {
		t.Errorf("body content leaked <head> content:\n%s", body)
	}
}

// TestRecursiveTemplate_CloneCarriesDefines guards the specific way this
// refactor could regress invisibly. recursionDefines is a parse-time field, and
// Clone copies fields by explicit enumeration rather than struct assignment — so
// a clone that lost it would recompute body content without the defines and
// silently fall back to HTML-string diffing. Production renders through
// per-session clones, never the master, so a master-only assertion would not
// catch it.
func TestRecursiveTemplate_CloneCarriesDefines(t *testing.T) {
	const fullDoc = `<!DOCTYPE html><html><body>` +
		`{{define "node"}}<li data-key="{{.Path}}">{{.Name}}` +
		`{{if .IsDir}}<ul>{{range .Children}}{{template "node" .}}{{end}}</ul>{{end}}</li>{{end}}` +
		`<ul>{{template "node" .Root}}</ul></body></html>`

	master, err := Must(New("handoff-clone")).Parse(fullDoc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	clone, err := master.Clone()
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	if clone.recursionDefines != master.recursionDefines {
		t.Fatalf("Clone dropped recursionDefines:\nmaster: %q\nclone:  %q",
			master.recursionDefines, clone.recursionDefines)
	}

	// Force the clone to recompute from scratch rather than inherit the cached
	// string, which is the path that actually depends on the field.
	clone.mu.Lock()
	clone.cachedBodyContent = ""
	clone.cachedBodyContentValid = false
	body := clone.getOrComputeBodyContent()
	clone.mu.Unlock()

	if !strings.Contains(body, `{{define "node"}}`) {
		t.Errorf("clone recomputed body content without the recursion define:\n%s", body)
	}

	// End to end: the clone must take the reactive AST path, not the fallback.
	data := struct{ Root fileNode }{Root: fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{{Name: "a.go", Path: "/a.go"}},
	}}
	if _, err := clone.buildTree(data, nil); err != nil {
		t.Fatalf("clone first render: %v", err)
	}
	if !clone.hasInitialTree {
		t.Fatal("clone fell back to HTML-structure diffing — the recursion registry was empty")
	}
}
