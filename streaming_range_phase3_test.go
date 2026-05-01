package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// homoTodo is a homogeneous range item (no conditional inside).
type homoTodo struct {
	ID   string
	Text string
}

type homoTodoState struct {
	Todos []homoTodo
}

const homoTodoTemplate = `<ul>{{range .Todos}}<li data-key="{{.ID}}">{{.Text}}</li>{{end}}</ul>`

// renderTwice renders twice and returns the second render's JSON.
func renderTwice(t *testing.T, tmpl *Template, s1, s2 interface{}) string {
	t.Helper()
	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, s1); err != nil {
		t.Fatalf("first render: %v", err)
	}
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, s2); err != nil {
		t.Fatalf("second render: %v", err)
	}
	return buf.String()
}

// TestStreamMode_FiresOnSecondRender — after a homogeneous first render, the second render must route through stream mode.
func TestStreamMode_FiresOnSecondRender(t *testing.T) {
	tmpl := Must(New("stream-fires"))
	if _, err := tmpl.Parse(homoTodoTemplate); err != nil {
		t.Fatalf("parse: %v", err)
	}
	s1 := homoTodoState{Todos: []homoTodo{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
	}}
	s2 := homoTodoState{Todos: []homoTodo{
		{ID: "1", Text: "FIRST"}, // text changed
		{ID: "2", Text: "second"},
	}}

	out := renderTwice(t, tmpl, s1, s2)

	// Stream-mode emits whole-item dynamics; legacy emits partial deltas.
	if !strings.Contains(out, `["u","1",`) {
		t.Fatalf("expected stream-mode [\"u\", \"1\", ...] op, got: %s", out)
	}
	// Regression gate (test plan item 8): a stream-mode no-content-add render
	// must NOT emit ["a", ...] — that would mean dispatch fell through to
	// extractRangeData → handleEmptyToItemsTransition on a nil Items tree.
	if strings.Contains(out, `["a",`) {
		t.Errorf("unexpected [\"a\", ...] op — extractRangeData fall-through regression; output: %s", out)
	}
}

// TestStreamMode_ReconnectResyncEmitsFullTree (item 4) — clear lastTree, render again, expect full tree (not stream ops).
func TestStreamMode_ReconnectResyncEmitsFullTree(t *testing.T) {
	tmpl := Must(New("reconnect"))
	if _, err := tmpl.Parse(homoTodoTemplate); err != nil {
		t.Fatalf("parse: %v", err)
	}
	state := homoTodoState{Todos: []homoTodo{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
	}}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("initial render: %v", err)
	}
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, state); err != nil {
		t.Fatalf("second render: %v", err)
	}

	// Simulate reconnect: drop lastTree + reset hasInitialTree.
	tmpl.mu.Lock()
	tmpl.lastTree = nil
	tmpl.hasInitialTree = false
	tmpl.mu.Unlock()

	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("post-reconnect render: %v", err)
	}
	resync := buf.String()

	// First render after reconnect MUST include statics — the client lost them.
	if !strings.Contains(resync, `"s":[`) {
		t.Errorf("post-reconnect render missing statics; got: %s", resync)
	}
	// MUST NOT contain stream-mode update ops — those would assume the client
	// has retained item state, but a reconnected client has nothing.
	if strings.Contains(resync, `["u","1",`) || strings.Contains(resync, `["u","2",`) {
		t.Errorf("post-reconnect render should NOT contain stream-mode update ops; got: %s", resync)
	}
}

// TestStreamMode_FullDynamicsMapInvariant (test plan item 9) — every stream-mode
// ["u"] payload must include every dynamic position (with "" for absent / nil),
// not just the changed positions. Proposal §5c.
func TestStreamMode_FullDynamicsMapInvariant(t *testing.T) {
	// Template with multiple dynamic positions per item — Done is intentionally
	// kept as a scalar boolean (no {{if}}) so items stay homogeneous and stream
	// mode actually fires.
	const tmplSrc = `<ul>{{range .Items}}<li data-key="{{.ID}}">{{.Text}} {{.Author}} {{.Tag}}</li>{{end}}</ul>`
	type item struct {
		ID, Text, Author, Tag string
	}
	type state struct {
		Items []item
	}

	tmpl := Must(New("full-dyn-map"))
	if _, err := tmpl.Parse(tmplSrc); err != nil {
		t.Fatalf("parse: %v", err)
	}

	s1 := state{Items: []item{
		{ID: "1", Text: "alpha", Author: "alice", Tag: "x"},
		{ID: "2", Text: "beta", Author: "bob", Tag: "y"},
	}}
	s2 := state{Items: []item{
		{ID: "1", Text: "ALPHA", Author: "alice", Tag: "x"}, // only Text changed
		{ID: "2", Text: "beta", Author: "bob", Tag: "y"},
	}}

	out := renderTwice(t, tmpl, s1, s2)

	// Locate the ["u","1",{...}] op and verify the payload map carries every
	// non-key position (1, 2, 3 — position 0 is the key column, omitted).
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse failed: %v\noutput: %s", err, out)
	}
	rangeOps := findRangeOps(parsed)
	if rangeOps == nil {
		t.Fatalf("no range ops found in output: %s", out)
	}
	var updateOp []interface{}
	for _, op := range rangeOps {
		if arr, ok := op.([]interface{}); ok && len(arr) >= 3 && arr[0] == "u" && arr[1] == "1" {
			updateOp = arr
			break
		}
	}
	if updateOp == nil {
		t.Fatalf(`no ["u", "1", ...] op found in: %s`, out)
	}
	payload, ok := updateOp[2].(map[string]interface{})
	if !ok {
		t.Fatalf("update op payload is not a map: %T", updateOp[2])
	}
	// Per §5c: every non-key position present. Item template positions: 0=ID
	// (key, omitted), 1=Text, 2=Author, 3=Tag. Payload must have 1, 2, 3 — not
	// just position 1 (the changed one).
	for _, pos := range []string{"1", "2", "3"} {
		if _, present := payload[pos]; !present {
			t.Errorf("payload missing position %q (full-dynamics-map invariant); payload: %v", pos, payload)
		}
	}
}

// TestStreamMode_HetRangeFallback — once item structures diverge ({{if}} inside range), §5d fallback emits the full new range tree.
func TestStreamMode_HetRangeFallback(t *testing.T) {
	const tmplSrc = `<ul>{{range .Items}}<li data-key="{{.ID}}">{{.Text}}{{if .Flagged}}<span>!</span>{{end}}</li>{{end}}</ul>`
	type item struct {
		ID, Text string
		Flagged  bool
	}
	type state struct {
		Items []item
	}

	tmpl := Must(New("het-fallback"))
	if _, err := tmpl.Parse(tmplSrc); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Render 1: one item, all flagged → trivially homogeneous → transitions.
	s1 := state{Items: []item{{ID: "1", Text: "a", Flagged: true}}}
	// Render 2: add an unflagged item → heterogeneous (flagged vs non-flagged
	// items have different child-tree structure → divergent statics fingerprints).
	s2 := state{Items: []item{
		{ID: "1", Text: "a", Flagged: true},
		{ID: "2", Text: "b", Flagged: false},
	}}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, s1); err != nil {
		t.Fatalf("render 1: %v", err)
	}
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, s2); err != nil {
		t.Fatalf("render 2: %v", err)
	}
	out := buf.String()

	// Het-fallback wire shape: the changes tree carries the new range as a
	// nested tree at the range's dynamic position — NOT granular `["a"]`/`["u"]`
	// ops. The retained tree's range position must hold a *TreeNode (the full
	// new range) whose Range.Items includes an entry with key "2" and Text "b".
	rangeNode := tmpl.lastTree.Dynamics[0].(*build.TreeNode)
	if rangeNode.Range == nil || len(rangeNode.Range.Items) != 2 {
		t.Fatalf("retained range should have 2 items after het-fallback; got: %#v", rangeNode.Range)
	}
	item2 := rangeNode.Range.Items[1].(*build.TreeNode)
	if got := item2.Dynamics[0]; got != "2" {
		t.Errorf("item[1] key should be %q, got %q", "2", got)
	}
	if got := item2.Dynamics[1]; got != "b" {
		t.Errorf("item[1] text should be %q, got %q", "b", got)
	}
	// Wire output: no granular range ops emitted.
	if strings.Contains(out, `["u",`) || strings.Contains(out, `["a",`) || strings.Contains(out, `["i",`) {
		t.Errorf("het-fallback must NOT emit granular range ops; got: %s", out)
	}
}

// findRangeOps walks the parsed wire tree looking for a position whose value
// is an op-array (each element is itself a []interface{} op tuple). Skips
// reserved wire keys ("s" statics, "f" fingerprint, "m" metadata) — Go map
// iteration is randomized, so the candidate must be shape-validated.
func findRangeOps(node map[string]interface{}) []interface{} {
	for k, v := range node {
		if k == "s" || k == "f" || k == "m" {
			continue
		}
		if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			if _, isOpTuple := arr[0].([]interface{}); isOpTuple {
				return arr
			}
		}
		if nested, ok := v.(map[string]interface{}); ok {
			if found := findRangeOps(nested); found != nil {
				return found
			}
		}
	}
	return nil
}

// TestStreamMode_MemoryRegression (test plan item 2) — measures the heap delta
// from nilling Range.Items after transition. Renders the SAME state twice;
// render 2's no-change diff hits the early-return so t.lastTree stays the
// (now-transitioned) render-1 tree, observable in the assertions.
func TestStreamMode_MemoryRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("memory regression test skipped in -short")
	}

	for _, n := range []int{10, 100, 1000, 10000} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			tmpl := Must(New("mem-" + fmt.Sprint(n)))
			if _, err := tmpl.Parse(homoTodoTemplate); err != nil {
				t.Fatalf("parse: %v", err)
			}
			items := make([]homoTodo, n)
			for i := 0; i < n; i++ {
				items[i] = homoTodo{ID: fmt.Sprintf("k%d", i), Text: fmt.Sprintf("text-%d", i)}
			}
			state := homoTodoState{Todos: items}

			// First render: builds initial tree (Items populated, no transition yet).
			if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, state); err != nil {
				t.Fatalf("render 1: %v", err)
			}

			// Snapshot heap with items retained in lastTree.
			runtime.GC()
			runtime.GC()
			var withItems runtime.MemStats
			runtime.ReadMemStats(&withItems)

			// Second render: top-of-buildTree transition fires — Items becomes nil
			// on lastTree's range, replaced by StreamState.
			if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, state); err != nil {
				t.Fatalf("render 2: %v", err)
			}

			// Verify transition fired.
			rangeNode := tmpl.lastTree.Dynamics[0].(*build.TreeNode)
			if rangeNode.Range.StreamState == nil {
				t.Fatalf("expected stream-mode after second render, but StreamState is nil")
			}
			if rangeNode.Range.Items != nil {
				t.Fatalf("expected Items=nil after transition, got len=%d", len(rangeNode.Range.Items))
			}

			runtime.GC()
			runtime.GC()
			var afterTransition runtime.MemStats
			runtime.ReadMemStats(&afterTransition)

			// Heap accounting is noisy but the delta should be substantial at
			// large N. We log the numbers (audit-checkpoint capture) and assert
			// only a sanity floor — the proposal's ≥4× claim is the design
			// target, not a CI gate (per-allocator variance in Go can swing
			// individual measurements by 20-30%).
			delta := int64(withItems.HeapAlloc) - int64(afterTransition.HeapAlloc)
			perItem := float64(delta) / float64(n)
			t.Logf("N=%d: heap with items=%d, after transition=%d, delta=%d (%.1f bytes/item)",
				n, withItems.HeapAlloc, afterTransition.HeapAlloc, delta, perItem)

			// Sanity floor at large N: transition must shrink the heap, not grow it.
			// Small N can be dominated by allocator noise, so only assert at N>=1000.
			if n >= 1000 && delta <= 0 {
				t.Errorf("expected heap to shrink after transition at N=%d; delta=%d", n, delta)
			}
		})
	}
}
