package livetemplate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/render"
)

// Keying note for recursive-template range items: each {{range}} item is a
// {{template …}} invocation, so its top-level tree is the invocation wrapper (empty
// statics) that hides the item's real <li data-key="…"> one level down.
// buildRangeTreeWithStatics reads that data-key *through* the wrapper
// (allWrappedItemKeys) and uses it as the item's stable key, so an item keeps its
// identity across deep edits instead of keying on a content hash of its subtree.
//
// Consequence, pinned by _DescendantScopesToLeaf: because the key is stable, a deep
// descendant edit leaves every ancestor's key unchanged, so the diff engine's
// per-item recursive diff scopes it to a nested chain of ["u", key, …] ops down to
// the single changed leaf — no ancestor branch, and no unaffected sibling (top-level
// or in-branch), is re-sent. Direct-child edits still emit granular i/r/o/a ops
// (proven below). Renders are always correct; this is the delivered update-size win.

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

// dirWith builds the standard root directory holding the given leaf children, so
// the keying tests below differ only in their child list.
func dirWith(children ...fileNode) fileNode {
	return fileNode{Name: "root", Path: "/", IsDir: true, Children: children}
}

// leaf is a childless file node keyed by its path.
func leaf(name, path string) fileNode {
	return fileNode{Name: name, Path: path}
}

// recursiveUpdateJSON parses recursiveTreeSrc, builds t1 then t2, and returns the
// diff between them as a JSON string. Each child is a {{template "treeNode" .}}
// invocation, so the range op the diff emits is the proof that range keying
// descends THROUGH the invocation wrapper.
func recursiveUpdateJSON(t *testing.T, t1, t2 fileNode) string {
	t.Helper()
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	first, err := tmpl.buildTree(t1, nil)
	if err != nil {
		t.Fatalf("build t1: %v", err)
	}
	second, err := tmpl.buildTree(t2, nil)
	if err != nil {
		t.Fatalf("build t2: %v", err)
	}
	tmpl.lastTree = first
	js, _ := json.Marshal(tmpl.compareTreesAndGetChanges(first, second).ToMap())
	return string(js)
}

// itemKeys returns the range-item keys (the "_k" values) of the direct children
// of a recursive tree's root, in order. It reaches through the invocation wrapper
// each child sits behind, so a non-empty, correctly-ordered result is itself proof
// that range keying descends through that wrapper.
func itemKeys(t *testing.T, tmpl *Template, data fileNode) []string {
	t.Helper()
	tree, err := tmpl.buildTree(data, nil)
	if err != nil {
		t.Fatalf("build for keys: %v", err)
	}
	m := tree.ToMap()
	root, _ := m["0"].(map[string]interface{})    // the treeNode subtree
	cond, _ := root["2"].(map[string]interface{}) // the {{if .IsDir}} branch
	rng, _ := cond["0"].(map[string]interface{})  // the {{range .Children}} node
	items, _ := rng["d"].([]interface{})          // range item list
	keys := make([]string, 0, len(items))
	for _, it := range items {
		if im, ok := it.(map[string]interface{}); ok {
			if k, ok := im["_k"].(string); ok {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// TestRecursiveTemplate_MinimalUpdate_InsertMiddle is the discriminating keying
// test that append (_AddChild) cannot exercise: inserting a child in the MIDDLE
// of the list forces the diff to emit an ["i", after-id, …] op, whose after-id is
// the key of the preceding sibling. That key only resolves if range keying reaches
// through the {{template}} invocation wrapper down to the item's identity —
// appending always lands at the tail (the ["a"] fast path) and never resolves an
// after-id, so it proves nothing about mid-list keying. (Keys are content hashes,
// not the raw data-key value, because the wrapper hides the item's top-level
// statics from hasExplicitKeyAttribute; identity is still exact since the data-key
// value is itself one of the hashed dynamics — see the file-level note.)
func TestRecursiveTemplate_MinimalUpdate_InsertMiddle(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	before := dirWith(leaf("a.go", "/a.go"), leaf("c.go", "/c.go"))
	after := dirWith(leaf("a.go", "/a.go"), leaf("b.go", "/b.go"), leaf("c.go", "/c.go"))

	// a.go's key from the "before" tree is the after-id the insert must anchor on.
	wantAfter := itemKeys(t, tmpl, before)[0]

	first, err := tmpl.buildTree(before, nil)
	if err != nil {
		t.Fatalf("build before: %v", err)
	}
	next, err := tmpl.buildTree(after, nil)
	if err != nil {
		t.Fatalf("build after: %v", err)
	}
	tmpl.lastTree = first
	js, _ := json.Marshal(tmpl.compareTreesAndGetChanges(first, next).ToMap())
	update := string(js)
	t.Logf("insert-middle update JSON:\n%s", update)

	if !strings.Contains(update, `"i"`) {
		t.Errorf("mid-list insert must emit an [\"i\", after-id, …] op, got:\n%s", update)
	}
	// The after-id must be a.go's key resolved through the wrapper — not empty,
	// not a wrong sibling. This is the airtight wrapper-descent proof.
	if !strings.Contains(update, `"`+wantAfter+`"`) {
		t.Errorf("insert op must anchor on the preceding sibling's key %q, got:\n%s", wantAfter, update)
	}
	if !strings.Contains(update, "b.go") {
		t.Errorf("update must carry the inserted node b.go, got:\n%s", update)
	}
	// The stable siblings' text content must NOT be resent — only the new item.
	if strings.Contains(update, ">a.go<") || strings.Contains(update, ">c.go<") {
		t.Errorf("stable siblings must not be re-rendered on a granular insert:\n%s", update)
	}
}

// TestRecursiveTemplate_MinimalUpdate_Reorder proves the marquee keying promise
// for recursive lists: reordering children (no content change) emits a single
// ["o", [ids…]] permutation carrying only the keys — not a teardown/rebuild of
// every invoked subtree. A correct 3-distinct-key permutation is itself proof that
// range keying descends through the invocation wrapper (the keys are content
// hashes, not the raw data-key value — see the file-level note).
func TestRecursiveTemplate_MinimalUpdate_Reorder(t *testing.T) {
	before := dirWith(leaf("a.go", "/a.go"), leaf("b.go", "/b.go"), leaf("c.go", "/c.go"))
	after := dirWith(leaf("c.go", "/c.go"), leaf("a.go", "/a.go"), leaf("b.go", "/b.go"))

	update := recursiveUpdateJSON(t, before, after)
	t.Logf("reorder update JSON:\n%s", update)

	if !strings.Contains(update, `"o"`) {
		t.Errorf("a pure reorder must emit an [\"o\", [ids]] op, got:\n%s", update)
	}
	// A reorder moves keys, it does not re-render item bodies — none of the
	// unchanged <span> contents should reappear in the update.
	if strings.Contains(update, ">a.go<") || strings.Contains(update, ">b.go<") || strings.Contains(update, ">c.go<") {
		t.Errorf("reorder must not re-render item bodies (keys only):\n%s", update)
	}
}

// TestRecursiveTemplate_MinimalUpdate_Remove proves removal emits a granular
// ["r", id] op keyed by the removed item's data-key, leaving the surviving
// siblings untouched (not resent).
func TestRecursiveTemplate_MinimalUpdate_Remove(t *testing.T) {
	before := dirWith(leaf("a.go", "/a.go"), leaf("b.go", "/b.go"), leaf("c.go", "/c.go"))
	after := dirWith(leaf("a.go", "/a.go"), leaf("c.go", "/c.go"))

	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// b.go is the middle child of the "before" tree; its key must be the id named
	// by the ["r", id] op.
	wantRemoved := itemKeys(t, tmpl, before)[1]

	first, err := tmpl.buildTree(before, nil)
	if err != nil {
		t.Fatalf("build before: %v", err)
	}
	next, err := tmpl.buildTree(after, nil)
	if err != nil {
		t.Fatalf("build after: %v", err)
	}
	tmpl.lastTree = first
	js, _ := json.Marshal(tmpl.compareTreesAndGetChanges(first, next).ToMap())
	update := string(js)
	t.Logf("remove update JSON:\n%s", update)

	if !strings.Contains(update, `"r"`) {
		t.Errorf("removal must emit an [\"r\", id] op, got:\n%s", update)
	}
	if !strings.Contains(update, `"`+wantRemoved+`"`) {
		t.Errorf("remove op must name the removed item's key %q, got:\n%s", wantRemoved, update)
	}
	// Surviving siblings must not be re-rendered.
	if strings.Contains(update, ">a.go<") || strings.Contains(update, ">c.go<") {
		t.Errorf("surviving siblings must not be resent on a granular remove:\n%s", update)
	}
}

// TestRecursiveTemplate_DescendantScopesToLeaf pins the delivered per-leaf update
// for DEEP edits: renaming a grandchild deep under a nested directory is encoded as
// a nested chain of ["u", key, …] ops down to the single changed leaf. Neither the
// unaffected top-level sibling branch NOR an unaffected sibling grandchild in the
// SAME branch is re-sent — the churn is scoped to the changed leaf, not its
// enclosing branch. This is the payload win from keying recursive items by their
// data-key through the invocation wrapper plus the differential per-item diff.
func TestRecursiveTemplate_DescendantScopesToLeaf(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mk := func(grandchild string) fileNode {
		return dirWith(
			fileNode{Name: "sub", Path: "/sub", IsDir: true, Children: []fileNode{
				leaf(grandchild, "/sub/g.go"),
				leaf("g-sibling.go", "/sub/g-sibling.go"), // sibling IN the edited branch
			}},
			leaf("sibling.go", "/sibling.go"), // unaffected top-level branch
		)
	}
	first, err := tmpl.buildTree(mk("g.go"), nil)
	if err != nil {
		t.Fatalf("build first: %v", err)
	}
	next, err := tmpl.buildTree(mk("g-RENAMED.go"), nil)
	if err != nil {
		t.Fatalf("build next: %v", err)
	}
	tmpl.lastTree = first
	js, _ := json.Marshal(tmpl.compareTreesAndGetChanges(first, next).ToMap())
	update := string(js)
	t.Logf("descendant-mutation update JSON:\n%s", update)

	// The renamed grandchild is carried (the change reaches the wire).
	if !strings.Contains(update, "g-RENAMED.go") {
		t.Errorf("update must carry the renamed grandchild, got:\n%s", update)
	}
	// The unaffected top-level sibling branch is NOT re-sent.
	if strings.Contains(update, "sibling.go") && !strings.Contains(update, "g-sibling.go") {
		t.Errorf("unaffected top-level sibling branch must not be re-sent:\n%s", update)
	}
	// Per-leaf scoping: the sibling grandchild IN the edited branch is NOT re-sent
	// either — the update reaches only the changed leaf, not its enclosing branch.
	if strings.Contains(update, "g-sibling.go") {
		t.Errorf("sibling grandchild in the edited branch must not be re-sent (per-leaf scope):\n%s", update)
	}
	// No statics in the wire — a deep content edit is a pure dynamics-only chain.
	if strings.Contains(update, `"s":[`) {
		t.Errorf("deep content edit must not re-send statics:\n%s", update)
	}
}

// TestRecursiveTemplate_FullDocument_Reactive is a regression guard for a silent
// C8 bug: FlattenTemplate appends the recursion cycle members as {{define}} blocks
// AFTER the document, and body extraction for a FULL HTML document used to drop
// them — leaving the recursion registry empty at serve time, so the whole template
// degraded to HTML-string diffing (hasInitialTree=false) with none of the reactive
// tree or per-leaf machinery running. A fragment kept the defines and worked, so
// the gap was invisible to every fragment-based test. This asserts a full document
// takes the reactive AST path AND scopes a deep edit to a per-leaf ["u"] chain.
func TestRecursiveTemplate_FullDocument_Reactive(t *testing.T) {
	const fullDoc = `<!DOCTYPE html><html><head><title>T</title></head><body>` +
		`{{define "treeNode"}}<li data-key="{{.Path}}"><span>{{.Name}}</span>` +
		`{{if .IsDir}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul>{{end}}</li>{{end}}` +
		`<ul id="tree">{{template "treeNode" .Root}}</ul></body></html>`
	type fullDocState struct{ Root fileNode }

	tmpl, err := Must(New("fulldoc")).Parse(fullDoc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mk := func(deepName string) fullDocState {
		return fullDocState{Root: dirWith(
			fileNode{Name: "sub", Path: "/sub", IsDir: true, Children: []fileNode{
				leaf(deepName, "/sub/g.go"),
			}},
			leaf("sibling.go", "/sibling.go"),
		)}
	}
	if _, err := tmpl.buildTree(mk("g.go"), nil); err != nil {
		t.Fatalf("first render: %v", err)
	}
	// The reactive AST path must be active — a fallback to HTML-string diffing
	// (hasInitialTree=false) is the exact silent regression this test guards.
	if !tmpl.hasInitialTree {
		t.Fatal("full-document recursive template fell back to HTML-structure diffing (registry drop regressed)")
	}
	update, _ := tmpl.buildTree(mk("g-RENAMED.go"), nil)
	js, _ := json.Marshal(update.ToMap())
	s := string(js)
	t.Logf("full-doc deep-edit update:\n%s", s)
	if !strings.Contains(s, "g-RENAMED.go") {
		t.Errorf("update must carry the renamed deep node:\n%s", s)
	}
	if strings.Contains(s, "sibling.go") {
		t.Errorf("unaffected sibling must not be re-sent (per-leaf scope):\n%s", s)
	}
	if strings.Contains(s, `"s":[`) {
		t.Errorf("deep content edit must not re-send statics:\n%s", s)
	}
}

// TestRecursiveTemplate_FirstRenderOverLimit documents the first-render leg of the
// depth-guard asymmetry (the update leg is covered by _ConfigurableDepth): a
// finite tree deeper than the configured cap does NOT error on first render — it
// degrades to the html/template structural fallback, still yielding a usable page
// (just without the reactive tree for the over-limit levels). This pins the
// documented "working-but-non-reactive first render, rejected update" behavior.
func TestRecursiveTemplate_FirstRenderOverLimit(t *testing.T) {
	tmpl, err := Must(New("test", WithMaxTemplateDepth(3))).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// depth-6 tree > cap of 3, but finite. First render must not error.
	tree, err := tmpl.buildTree(chainDir(6), nil)
	if err != nil {
		t.Fatalf("first render over the depth cap must degrade, not error, got: %v", err)
	}
	html, err := render.TreeToHTML(tree.ToMap())
	if err != nil {
		t.Fatalf("fallback tree must still reconstruct to HTML, got: %v", err)
	}
	// The page is usable: at least the shallow levels rendered.
	if !strings.Contains(html, "data-key=") {
		t.Errorf("degraded first render must still produce a usable page, got:\n%s", html)
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
