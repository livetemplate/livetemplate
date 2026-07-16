package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"testing"
)

// These benchmarks characterize the runtime-invocation recursion path (C8)
// against the status quo it replaces — a whole recursive subtree rendered as one
// opaque html/template string and shipped as {{.FileBrowserHTML}}. They exist to
// keep the value story honest: recursion support's unconditional win is
// *correctness* (a recursive {{template}} renders at all, and does so as part of
// the page's reactive tree instead of an innerHTML blob), NOT a smaller update
// payload. The update-size numbers below show why the "minimal update" claim does
// not yet hold for deep, narrow trees — see BenchmarkRecursiveUpdate.

// benchTree builds a balanced directory tree of the given depth and branching
// factor — a stand-in for a file browser / comment thread / org chart, the shapes
// recursive {{template}} exists to serve.
func benchTree(depth, branch int, prefix string) fileNode {
	n := fileNode{Name: fmt.Sprintf("dir%s", prefix), Path: "/d" + prefix, IsDir: true}
	if depth == 0 {
		return fileNode{Name: fmt.Sprintf("file%s.go", prefix), Path: "/f" + prefix + ".go"}
	}
	for i := 0; i < branch; i++ {
		n.Children = append(n.Children, benchTree(depth-1, branch, fmt.Sprintf("%s-%d", prefix, i)))
	}
	return n
}

// BenchmarkRecursiveRender measures a first render of a recursive tree through the
// reactive tree path (parse → buildTree), the cost paid once per initial page load.
// This is the expensive leg: building the full nested TreeNode with per-level
// statics costs ~20-30x an opaque html/template Execute of the same tree (compare
// ns/op and allocs/op against BenchmarkOpaqueHTMLBaseline). That CPU buys the
// reactive tree — subsequent edits re-render in place with DOM-state preservation
// rather than replacing an opaque innerHTML blob — but it is a real cost, not free.
func BenchmarkRecursiveRender(b *testing.B) {
	parsed, err := Must(New("bench")).Parse(recursiveTreeSrc)
	if err != nil {
		b.Fatalf("parse: %v", err)
	}
	data := benchTree(5, 3, "") // 3^5 leaves ≈ 364 nodes

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parsed.buildTree(data, nil); err != nil {
			b.Fatalf("buildTree: %v", err)
		}
	}
}

// BenchmarkRecursiveUpdate measures the per-update cost after a rename deep in the
// tree: rebuild and diff against the previous render (what the framework does on
// every action), reporting the wire payload as "update_bytes".
//
// The honest finding this benchmark exists to surface: the update is NOT scoped to
// the changed leaf. Recursive range items are keyed by a *deep content hash* (the
// item's real <li data-key> is hidden one level down inside the invocation
// wrapper, so range keying can't see it — see the keying caveat in
// docs/proposals/recursive-templates-proposal.md §6). Renaming any node changes
// every ancestor's hash, so the diff re-sends the item's whole ENCLOSING TOP-LEVEL
// BRANCH as ["r", oldkey],["p", [full branch with statics]] — its sibling branches
// are spared, but the branch itself comes back whole.
//
// Concretely, for this depth-5 branch-3 shape a deep-leaf edit's update_bytes
// (~24KB, one of three top-level branches) lands ESSENTIALLY EQUAL to
// BenchmarkOpaqueHTMLBaseline's full_html_bytes (~23.5KB, the whole tree) — and
// costs ~25x the CPU to produce. So for a deep, narrow tree the reactive update is
// no smaller on the wire than shipping the entire opaque string, and much dearer to
// compute. The update-size win only materializes when the enclosing branch is a
// small fraction of the tree (a WIDE tree, many top-level branches) or once the
// data-key-through-the-wrapper follow-up lands and deep edits scope to a single
// per-leaf ["u", key, {…}]. Recursion support's win here is correctness and
// reactive DOM-state preservation, not payload size.
func BenchmarkRecursiveUpdate(b *testing.B) {
	parsed, err := Must(New("bench")).Parse(recursiveTreeSrc)
	if err != nil {
		b.Fatalf("parse: %v", err)
	}
	base := benchTree(5, 3, "")
	mutated := benchTree(5, 3, "")
	// Rename one leaf deep in the tree.
	mutated.Children[0].Children[0].Children[0].Children[0].Children[0].Name = "renamed.go"

	first, err := parsed.buildTree(base, nil)
	if err != nil {
		b.Fatalf("build base: %v", err)
	}

	var updateBytes int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed.lastTree = first
		next, err := parsed.buildTree(mutated, nil)
		if err != nil {
			b.Fatalf("build mutated: %v", err)
		}
		changes := parsed.compareTreesAndGetChanges(first, next)
		js, _ := json.Marshal(changes.ToMap())
		updateBytes = len(js)
	}
	b.ReportMetric(float64(updateBytes), "update_bytes")
}

// BenchmarkOpaqueHTMLBaseline is the status-quo comparison: rendering the same tree
// as a single opaque html/template string (the {{.FileBrowserHTML}} escape hatch
// recursion replaces). Every change re-executes and re-sends the ENTIRE string,
// which the client applies by replacing innerHTML — no tree diff, and every DOM
// node in the region is destroyed and rebuilt (focus, scroll, and event state in
// the subtree are lost). Its full_html_bytes is the number BenchmarkRecursiveUpdate's
// update_bytes should be read against: for the deep-narrow shape here they are
// ~equal in bytes, but this baseline is ~25x cheaper in CPU. Recursion support does
// not beat it on payload or CPU for this shape — it beats it on *what the client can
// do with the payload* (in-place reconciliation) and on rendering correctly at all.
func BenchmarkOpaqueHTMLBaseline(b *testing.B) {
	// A standalone recursive html/template (the workaround: recursion works, but
	// the output is one opaque string with no tree/diff).
	src := `{{define "treeNode"}}<li data-key="{{.Path}}"><span>{{.Name}}</span>` +
		`{{if .IsDir}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul>{{end}}` +
		`</li>{{end}}<ul>{{template "treeNode" .}}</ul>`
	ht, err := template.New("baseline").Parse(src)
	if err != nil {
		b.Fatalf("parse baseline: %v", err)
	}
	data := benchTree(5, 3, "")

	var fullBytes int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := ht.Execute(&buf, data); err != nil {
			b.Fatalf("execute baseline: %v", err)
		}
		fullBytes = buf.Len()
	}
	b.ReportMetric(float64(fullBytes), "full_html_bytes")
}
