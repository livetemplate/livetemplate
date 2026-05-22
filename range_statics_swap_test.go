package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// Per-row static scaffolding must not be re-emitted across full content swaps of the same {{range}}.

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

// countStaticsKey counts occurrences of the statics key. Matches the `[` suffix to skip the rare false positive of `"s":` appearing inside a string value.
func countStaticsKey(payload []byte) int {
	return bytes.Count(payload, []byte(`"s":[`))
}

// renderUpdate primes the template with d1 then returns the wire payload of ExecuteUpdates(d2).
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

// Initial render of a {{range}} must include the item template under "s" so the client can cache it.
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

// Full slice replacement on the same range template must not re-emit per-row statics; client has them cached.
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

// Growing the list must not re-emit per-row statics. n == 0: this template has only string dynamics, so PrepareTreeForClient strips nothing at the item level; op-envelope statics get stripped separately by fingerprint match.
func TestRangeStatics_GrowCountDelta_NoPerRowStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(5, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(50, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("count-delta GROW MUST NOT re-emit per-row statics; got %d \"s\":[ occurrences\n  first 600 bytes=%s", n, truncate(update, 600))
	}
}

// Shrinking the list must not re-emit per-row statics.
func TestRangeStatics_ShrinkCountDelta_NoPerRowStatics(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(50, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(5, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("count-delta SHRINK MUST NOT re-emit per-row statics; got %d \"s\":[ occurrences\n  payload=%s", n, update)
	}
}

// Empty → items is the only update path where the item template legitimately appears on the wire — exactly once.
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

// Populated → empty must remove items without emitting any per-row statics.
func TestRangeStatics_NonEmptyToEmpty(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(10, "A")}
	d2 := rangeFileState{Filename: "empty.go", Lines: nil}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("non-empty→empty MUST NOT emit any per-row statics; got %d \"s\":[\n  payload=%s", n, update)
	}
}

// Single-item swap (different key, same template) must not re-emit the item template.
func TestRangeStatics_SingleItemSwap(t *testing.T) {
	tmplStr := `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span></div>{{end}}</div>`

	d1 := rangeFileState{Filename: "a.go", Lines: makeRangeLines(1, "A")}
	d2 := rangeFileState{Filename: "b.go", Lines: makeRangeLines(1, "B")}
	update := renderUpdate(t, tmplStr, d1, d2)

	if n := countStaticsKey(update); n != 0 {
		t.Errorf("single-item swap MUST NOT re-emit per-row statics; got %d \"s\":[\n  payload=%s", n, update)
	}
}

// Repeated swaps of equal-sized lists must produce stable payload sizes (no accumulated static-resend growth).
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

	min, max := sizes[0], sizes[0]
	for _, s := range sizes[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	// Allow up to 20% size drift across repeated same-size swaps.
	if max > min*12/10 {
		t.Errorf("repeated swap payload sizes drifted: %v (min=%d max=%d)", sizes, min, max)
	}
}

// Swap payload omits statics but must still carry every new row's dynamic content.
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

// Stream-mode insert must carry auto-generated _k so the client can later remove/update the new row.
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

// Range items with a nested {{if}} branch carrying dynamics must keep their branch statics on insert.
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
