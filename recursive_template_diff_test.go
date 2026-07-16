package livetemplate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestRecursiveTemplate_MinimalUpdate_AddChild is the correctness proof for the
// UPDATE path (not just first render): adding one child to a recursive tree must
// produce a minimal update — a granular range op for the new node — not a
// full-subtree resend of the unchanged siblings. Range items here are each a
// {{template "treeNode" .}} invocation (a nested wrapper subtree), so this also
// verifies data-key extraction descends through the invocation wrapper.
func TestRecursiveTemplate_MinimalUpdate_AddChild(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	t1Data := fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{
			{Name: "a.go", Path: "/a.go"},
			{Name: "b.go", Path: "/b.go"},
		},
	}
	t2Data := fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{
			{Name: "a.go", Path: "/a.go"},
			{Name: "b.go", Path: "/b.go"},
			{Name: "c.go", Path: "/c.go"}, // added at end
		},
	}

	t1, err := tmpl.buildTree(t1Data, nil)
	if err != nil {
		t.Fatalf("build t1: %v", err)
	}
	t2, err := tmpl.buildTree(t2Data, nil)
	if err != nil {
		t.Fatalf("build t2: %v", err)
	}

	tmpl.lastTree = t1
	changes := tmpl.compareTreesAndGetChanges(t1, t2)
	js, _ := json.Marshal(changes.ToMap())
	update := string(js)
	t.Logf("update JSON:\n%s", update)

	// The new node must be present in the update.
	if !strings.Contains(update, "c.go") {
		t.Errorf("update must carry the added node c.go, got:\n%s", update)
	}
	// The unchanged siblings' text content must NOT be resent (minimal update).
	// If a.go/b.go appear, the whole list was re-rendered instead of a granular op.
	if strings.Contains(update, "a.go") || strings.Contains(update, "b.go") {
		t.Errorf("unchanged siblings should not be resent (non-minimal update):\n%s", update)
	}
}

// TestRecursiveTemplate_MinimalUpdate_DepthGrows verifies the second half of the
// diff contract: when the data grows a NEW nesting level (a leaf becomes a
// directory with children), the newly-appearing structure must carry its statics
// ("s") — the client has never seen that level, so it can't have them cached.
func TestRecursiveTemplate_MinimalUpdate_DepthGrows(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// T1: a single leaf (IsDir=false) — the {{if .IsDir}} branch is empty, so no
	// <ul>/child structure exists on the client yet.
	t1Data := fileNode{Name: "root", Path: "/", IsDir: false}
	// T2: same node becomes a directory with one child — a new nested level.
	t2Data := fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{{Name: "a.go", Path: "/a.go"}},
	}

	t1, err := tmpl.buildTree(t1Data, nil)
	if err != nil {
		t.Fatalf("build t1: %v", err)
	}
	t2, err := tmpl.buildTree(t2Data, nil)
	if err != nil {
		t.Fatalf("build t2: %v", err)
	}

	tmpl.lastTree = t1
	changes := tmpl.compareTreesAndGetChanges(t1, t2)
	m := changes.ToMap()
	js, _ := json.Marshal(m)
	update := string(js)
	t.Logf("depth-grows update JSON:\n%s", update)

	// The new child must be present.
	if !strings.Contains(update, "a.go") {
		t.Errorf("update must carry the new child a.go, got:\n%s", update)
	}

	// The newly-appearing nesting level (the {{if .IsDir}} branch at position 2,
	// previously empty) must include its own statics — the client has never seen
	// this structure, so it can't have them cached. Inspect the map structurally
	// rather than string-matching JSON-escaped HTML.
	root, ok := m["0"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested root node at \"0\", got: %s", update)
	}
	cond, ok := root["2"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the newly-populated {{if}} branch at \"0\".\"2\", got: %s", update)
	}
	if _, hasStatics := cond["s"]; !hasStatics {
		t.Errorf("newly-appearing level must resend its statics (\"s\"), got:\n%s", update)
	}
}

// TestRecursiveTemplate_ExecuteInitialHTML covers the initial page-load path:
// html/template Execute (t.tmpl, which carries the appended {{define}} blocks)
// must render a recursive template correctly. This is the HTML the browser gets
// on first load, before the reactive tree takes over.
func TestRecursiveTemplate_ExecuteInitialHTML(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	data := fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{
			{Name: "a.go", Path: "/a.go"},
			{Name: "sub", Path: "/sub", IsDir: true, Children: []fileNode{
				{Name: "b.go", Path: "/sub/b.go"},
			}},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute on a recursive template must succeed, got: %v", err)
	}
	html := buf.String()

	// Every node must be present, nested correctly (root contains sub contains b.go).
	for _, want := range []string{
		`data-key="/"`, `<span>root</span>`,
		`data-key="/a.go"`, `<span>a.go</span>`,
		`data-key="/sub"`, `<span>sub</span>`,
		`data-key="/sub/b.go"`, `<span>b.go</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Execute output missing %q in:\n%s", want, html)
		}
	}
	// The deepest node must appear after its parent (nesting order preserved).
	if strings.Index(html, "/sub/b.go") < strings.Index(html, `data-key="/sub"`) {
		t.Errorf("nesting order wrong — /sub/b.go should follow /sub:\n%s", html)
	}
}
