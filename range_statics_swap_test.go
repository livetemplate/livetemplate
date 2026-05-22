package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// Tests in this file cover issue #413:
// "{{range}} statics not cached across full content swap — resends per-row scaffolding".
//
// Goal: lock in the contract that per-row static scaffolding is NOT re-emitted
// across full content swaps of the same {{range}} (matching the
// tree-update-specification §3.4 "stripped from operations" rule for stripped
// statics when the structure fingerprint is unchanged).

// rangeFileLine mirrors the per-row shape used by the prereview app from
// issue #413: kind, line number, and a (template-safe) highlighted-content
// blob. Keeping the shape ASCII-simple makes wire-byte assertions stable.
type rangeFileLine struct {
	ID                 string
	Kind               string
	LineNo             int
	HighlightedContent template.HTML
}

type rangeFileState struct {
	Filename string
	Lines    []rangeFileLine
}

// makeRangeLines builds N homogeneous lines whose content varies with `i`,
// so a swap from set A to set B produces fully different dynamic values
// (matching the file-switch scenario from issue #413).
func makeRangeLines(n int, prefix string) []rangeFileLine {
	out := make([]rangeFileLine, n)
	for i := 0; i < n; i++ {
		out[i] = rangeFileLine{
			ID:                 fmt.Sprintf("%s-%d", prefix, i),
			Kind:               []string{"add", "rem", "ctx"}[i%3],
			LineNo:             i + 1,
			HighlightedContent: template.HTML(fmt.Sprintf("<span>%s line %d %s</span>", prefix, i, strings.Repeat("x", 16))),
		}
	}
	return out
}

// countStaticsKey returns how many times "s":[ appears in the wire payload.
// (Counts the JSON key, not the bytes inside any literal strings — payloads
// here don't embed `"s":[` inside content.)
func countStaticsKey(payload []byte) int {
	return bytes.Count(payload, []byte(`"s":`))
}

// renderUpdate renders d1 to prime the template (initial render) and returns
// the wire payload of the subsequent ExecuteUpdates(d2) — the update path
// these tests care about.
func renderUpdate(t *testing.T, tmplStr string, d1, d2 interface{}) []byte {
	t.Helper()
	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, d1); err != nil {
		t.Fatalf("Initial ExecuteUpdates failed: %v", err)
	}
	var update bytes.Buffer
	if err := tmpl.ExecuteUpdates(&update, d2); err != nil {
		t.Fatalf("Update ExecuteUpdates failed: %v", err)
	}
	return update.Bytes()
}

// TestRangeStatics_InitialRender_IncludesStatics is the baseline: the initial
// render of a {{range}} MUST include the item template under "s" so the client
// can cache the scaffold. Spec §5.1.
func TestRangeStatics_InitialRender_IncludesStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	d := rangeFileState{Filename: "a.go", Lines: makeRangeLines(5, "A")}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, d); err != nil {
		t.Fatalf("ExecuteUpdates failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte(`"s":`)) {
		t.Errorf("initial render MUST include statics; payload=%s", buf.String())
	}

	// The item template ("line-row") must appear (this is the per-row scaffold).
	if !bytes.Contains(buf.Bytes(), []byte(`line-row`)) {
		t.Errorf("initial render missing the per-row template scaffold; payload=%s", buf.String())
	}
}

// TestRangeStatics_FullContentSwap_NoPerRowStatics is the core assertion from
// issue #413: when the {{range}}'s backing slice is fully replaced with a new
// set of items (different keys, same range item template), the update payload
// MUST NOT re-emit per-row static scaffolding. The client has the item
// template cached from the initial render.
func TestRangeStatics_FullContentSwap_NoPerRowStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(8, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(8, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	// The wire payload must NOT contain any "s":[ — the item template is
	// cached on the client from the initial render.
	if n := countStaticsKey(update); n != 0 {
		t.Errorf("full content swap MUST NOT re-emit per-row statics; got %d \"s\":[ occurrences\n  payload=%s", n, update)
	}

	// The update should still contain the new line content under dynamics.
	if !bytes.Contains(update, []byte(`B line 0`)) {
		t.Errorf("update missing new dynamic content (B line 0); payload=%s", update)
	}
}

// TestRangeStatics_GrowCountDelta_NoPerRowStatics: growing the list (e.g.,
// switching from a 5-row file to a 50-row file) must not re-emit per-row
// statics. The 'p'/'a' insertion op may carry the item template once at
// the op envelope, but it gets stripped when the structure fingerprint
// matches the previous render — so the wire must not contain ANY "s":[.
func TestRangeStatics_GrowCountDelta_NoPerRowStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(5, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(50, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("count-delta GROW MUST NOT re-emit per-row statics; got %d \"s\":[ occurrences\n  first 600 bytes=%s", n, truncate(update, 600))
	}
}

// TestRangeStatics_ShrinkCountDelta_NoPerRowStatics: shrinking the list must
// not re-emit per-row statics either.
func TestRangeStatics_ShrinkCountDelta_NoPerRowStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(50, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(5, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("count-delta SHRINK MUST NOT re-emit per-row statics; got %d \"s\":[ occurrences\n  payload=%s", n, update)
	}
}

// TestRangeStatics_EmptyToNonEmpty_IncludesItemTemplateOnce: empty → items
// is the only update path where the item template can legitimately appear on
// the wire (the client received an empty range on first render and never saw
// the per-row scaffold). The item template must appear in the append op's
// statics slot ONCE — not N times.
func TestRangeStatics_EmptyToNonEmpty_IncludesItemTemplateOnce(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "empty.go", Lines: nil}
	d2 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(10, "A")}
	update := renderUpdate(t, tmplStr, d1, d2)

	// The item template ("line-row") must appear once (the op-level statics).
	if count := bytes.Count(update, []byte(`line-row`)); count != 1 {
		t.Errorf("empty→items MUST include item template exactly ONCE (op-level statics), got %d occurrences\n  payload=%s", count, update)
	}
}

// TestRangeStatics_NonEmptyToEmpty: populated → empty must clear/remove items
// but not include any per-row statics on the wire.
func TestRangeStatics_NonEmptyToEmpty(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(10, "A")}
	d2 := rangeFileState{Filename: "empty.go", Lines: nil}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("non-empty→empty MUST NOT emit any per-row statics; got %d \"s\":[\n  payload=%s", n, update)
	}
}

// TestRangeStatics_SingleItemSwap: a single-item range whose only item is
// replaced (different key, same template) must not re-emit the item template.
func TestRangeStatics_SingleItemSwap(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(1, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(1, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("single-item swap MUST NOT re-emit per-row statics; got %d \"s\":[\n  payload=%s", n, update)
	}
}

// TestRangeStatics_RepeatedSwap_PayloadStable: repeated A→B→A→B swaps must
// produce comparable payload sizes (no growth from accumulated static-resends).
// The third swap's payload size should be within a small factor of the first.
func TestRangeStatics_RepeatedSwap_PayloadStable(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span></div>{{end}}</div>`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Prime with A.
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, rangeFileState{Lines: makeRangeLines(50, "A1")}); err != nil {
		t.Fatalf("initial render failed: %v", err)
	}

	// Three full content swaps.
	var sizes [3]int
	prefixes := []string{"B", "A2", "B2"}
	for i, p := range prefixes {
		var buf bytes.Buffer
		if err := tmpl.ExecuteUpdates(&buf, rangeFileState{Lines: makeRangeLines(50, p)}); err != nil {
			t.Fatalf("swap %d failed: %v", i, err)
		}
		sizes[i] = buf.Len()
		if n := countStaticsKey(buf.Bytes()); n != 0 {
			t.Errorf("swap %d (%s) re-emitted statics %d times; payload=%s", i, p, n, buf.String())
		}
	}

	// Sanity-check sizes are within ~20% of each other — they should be near-
	// identical for homogeneous swaps of equal-sized lists.
	min, max := sizes[0], sizes[0]
	for _, s := range sizes[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	if max > min*12/10 {
		t.Errorf("repeated swap payload sizes drifted: %v (min=%d max=%d)", sizes, min, max)
	}
}

// TestRangeStatics_FullSwap_DynamicsPreserved sanity-checks that the swap
// payload — while omitting statics — still carries enough new-side data to
// rebuild every row. This is the wire-format contract: dynamics + op
// envelope + cached statics must be sufficient.
func TestRangeStatics_FullSwap_DynamicsPreserved(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Lines: makeRangeLines(20, "A")}
	d2 := rangeFileState{Lines: makeRangeLines(20, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	// Every new line's content must appear in the wire.
	for i := 0; i < 20; i++ {
		marker := fmt.Sprintf("B line %d ", i)
		if !bytes.Contains(update, []byte(marker)) {
			t.Errorf("update missing new row content for line %d (marker=%q)\n  payload=%s", i, marker, update)
		}
	}

	// JSON must parse.
	var parsed map[string]interface{}
	if err := json.Unmarshal(update, &parsed); err != nil {
		t.Errorf("update payload is not valid JSON: %v", err)
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}

// TestRangeStatics_AutoKey_AppendIncludesAutoKey is the flash-messages
// regression test for issue #413: a `{{range}}` over a slice of strings has
// no explicit data-key, so each item gets an auto-generated `_k` hash.
// Stream-mode inserts must carry the `_k` field so the client can track the
// new row for later remove/update operations. The earlier strip-on-insert
// attempt dropped AutoKey via PrepareTreeForClient and broke remove flow in
// examples/flash-messages.
func TestRangeStatics_AutoKey_AppendIncludesAutoKey(t *testing.T) {
	tmplStr := `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	d1 := map[string]interface{}{"Items": []string{"Apple", "Banana", "Cherry"}}
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, d1); err != nil {
		t.Fatalf("initial render failed: %v", err)
	}

	d2 := map[string]interface{}{"Items": []string{"Apple", "Banana", "Cherry", "Date"}}
	var update bytes.Buffer
	if err := tmpl.ExecuteUpdates(&update, d2); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	payload := update.String()
	if !strings.Contains(payload, `"_k"`) {
		t.Errorf("append payload missing auto-key (_k); client cannot track new row\n  payload=%s", payload)
	}
}

// TestRangeStatics_NestedBranch_AppendIncludesBranchStatics is the
// tinkerdown auto-tables regression test for issue #413: a `{{range}}` whose
// item template contains a nested `{{if}}` branch with dynamics must keep
// the branch's statics on insert. The client has no per-position branch
// statics cache — when it receives an item with stripped nested statics
// (numeric keys only on the nested object), it joins values without the
// HTML wrapper, losing the row's structure (`<span class="marker">…</span>`
// becomes just the dynamic text).
func TestRangeStatics_NestedBranch_AppendIncludesBranchStatics(t *testing.T) {
	tmplStr := `<ul>{{range .Items}}<li>{{if .HasMarker}}<span class="marker">{{.MarkerText}}</span>{{end}}{{.Name}}</li>{{end}}</ul>`

	type item struct {
		Name       string
		HasMarker  bool
		MarkerText string
	}

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	d1 := map[string]interface{}{"Items": []item{
		{Name: "Apple", HasMarker: true, MarkerText: "A"},
		{Name: "Banana", HasMarker: true, MarkerText: "B"},
	}}
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, d1); err != nil {
		t.Fatalf("initial render failed: %v", err)
	}

	d2 := map[string]interface{}{"Items": []item{
		{Name: "Apple", HasMarker: true, MarkerText: "A"},
		{Name: "Banana", HasMarker: true, MarkerText: "B"},
		{Name: "Cherry", HasMarker: true, MarkerText: "C"},
	}}
	var update bytes.Buffer
	if err := tmpl.ExecuteUpdates(&update, d2); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	payload := update.String()
	if !strings.Contains(payload, "marker") {
		t.Errorf("append payload missing nested branch statics (class=\"marker\"); client cannot render new row\n  payload=%s", payload)
	}
}
