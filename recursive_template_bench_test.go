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
// opaque html/template string and shipped as {{.FileBrowserHTML}}. Recursion
// support renders a recursive {{template}} at all (the flatten path overflows) AND
// keeps it inside the page's reactive tree. With per-leaf recursive range diffing,
// a deep edit is scoped to a nested ["u", key, …] chain down to the single changed
// node — see BenchmarkRecursiveUpdate for the wire-size win over the opaque baseline.

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
// The update is scoped to the changed leaf. Recursive range items are keyed by
// their real data-key (the <li data-key>, read through the invocation wrapper), so
// a deep edit leaves every ancestor's key stable and the differential range diff
// emits a nested ["u", key, …] chain down to the single renamed node — statics-free
// and carrying no unaffected sibling.
//
// Concretely, for this depth-5 branch-3 shape (~364 nodes) a deep-leaf edit's
// update_bytes is ~200 B versus BenchmarkOpaqueHTMLBaseline's full_html_bytes
// (~23.5 KB, the whole tree re-sent) — a ~100x wire-size reduction, the win
// recursion-in-the-reactive-tree delivers over the opaque {{.FileBrowserHTML}}
// escape hatch. The trade is CPU: the reactive path rebuilds + diffs the tree
// (~20x an opaque Execute), buying the minimal payload and in-place DOM updates.
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
// update_bytes should be read against: the reactive per-leaf update is ~100x smaller
// on the wire and preserves DOM state, at the cost of ~20x the CPU to compute the
// diff. The baseline is cheaper per render but re-sends everything on every change.
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
