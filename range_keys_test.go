package livetemplate

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strings"
	"testing"
)

// Tests for #554: nested ranges over plain values and repeated plain values.
// The and/or and map-order tests are in internal/parse, because Execute falls
// back to html/template when livetemplate's evaluator fails.

const wordsTmpl = `<p>{{range split .Words " "}}<b>{{.}}</b>{{end}}</p>`

func parseWithSplit(t *testing.T, name, src string) *Template {
	t.Helper()
	tmpl := Must(New(name))
	tmpl.Funcs(template.FuncMap{"split": strings.Split})
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tmpl
}

// updates renders each state in turn and returns each render's update as JSON.
func updates(t *testing.T, tmpl *Template, states ...interface{}) []map[string]any {
	t.Helper()
	out := make([]map[string]any, len(states))
	for i, s := range states {
		var buf bytes.Buffer
		if err := tmpl.ExecuteUpdates(&buf, s); err != nil {
			t.Fatalf("render %d: %v", i, err)
		}
		out[i] = map[string]any{}
		if buf.Len() > 0 {
			if err := json.Unmarshal(buf.Bytes(), &out[i]); err != nil {
				t.Fatalf("render %d: %v in %s", i, err, buf.String())
			}
		}
	}
	return out
}

func words(s string) map[string]string { return map[string]string{"Words": s} }

// hasOp reports whether ops holds an op of kind with the given key.
func hasOp(ops [][]any, kind, key string) bool {
	for _, op := range ops {
		if op[0] == kind && (key == "" || op[1] == key) {
			return true
		}
	}
	return false
}

const nestedOnlyTmpl = `<code>{{range $w := split .Line " "}}{{range $p := split $w "|"}}{{$p}};{{end}}{{end}}</code>`

// A range item whose whole body is another range used to lose its content:
// every item rendered empty under the empty-input hash cbf29ce48422.
func TestRange_BodyIsOnlyANestedRange(t *testing.T) {
	tmpl := parseWithSplit(t, "nested-only", nestedOnlyTmpl)
	data := map[string]string{"Line": "a b|c d"}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "a;b;c;d;") {
		t.Fatalf("rendered %q, want it to contain a;b;c;d;", buf.String())
	}

	fresh := parseWithSplit(t, "nested-only-keys", nestedOnlyTmpl)
	keys := ckKeys(t, updates(t, fresh, data)[0])
	seen := map[string]bool{}
	for _, k := range keys {
		if k == "cbf29ce48422" || seen[k] {
			t.Fatalf("outer keys %v: empty-input hash or a repeat", keys)
		}
		seen[k] = true
	}
	if len(keys) != 3 {
		t.Fatalf("outer keys %v, want 3", keys)
	}
}

// Repeated plain values get distinct keys, and the first copy keeps its hash.
func TestRange_RepeatedValuesGetDistinctKeys(t *testing.T) {
	base := ckKeys(t, updates(t, parseWithSplit(t, "single", wordsTmpl), words("a b"))[0])
	got := ckKeys(t, updates(t, parseWithSplit(t, "repeats", wordsTmpl), words("a a b"))[0])
	want := []string{base[0], base[0] + "-2", base[1]}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// With distinct keys, a later update is sent as item ops, not the whole range.
func TestRange_RepeatedValuesUpdateInPlace(t *testing.T) {
	second := updates(t, parseWithSplit(t, "repeat-updates", wordsTmpl), words("a a b"), words("a a b c"))[1]
	if ckRange(second) != nil {
		t.Fatalf("appending to a range with repeated values resent the whole range: %v", second)
	}
	if ops := ckOps(second); !hasOp(ops, "a", "") && !hasOp(ops, "i", "") {
		t.Fatalf("expected an append or insert op, got %v", ops)
	}
}

// Removing the first of two equal items renames the survivor, which is sent
// as one remove and one add; other items are untouched.
func TestRange_RemovingARepeatedValueRenamesTheSurvivor(t *testing.T) {
	out := updates(t, parseWithSplit(t, "repeat-remove", wordsTmpl), words("a b a"), words("b a"))
	keys := ckKeys(t, out[0])
	hA, hB := keys[0], keys[1]
	if ckRange(out[1]) != nil {
		t.Fatalf("removing a repeated value resent the whole range: %v", out[1])
	}
	ops := ckOps(out[1])
	if !hasOp(ops, "r", hA+"-2") {
		t.Errorf("expected the renamed survivor %q to be removed, got %v", hA+"-2", ops)
	}
	for _, op := range ops {
		if op[1] == hB {
			t.Errorf("the untouched item %q has an op: %v", hB, op)
		}
	}
}

// A stream-mode range whose rows hold a nested range over a map still sends
// append ops, and an unchanged row gives no diff on a second render.
func TestRange_NestedMapRangeKeepsStreamMode(t *testing.T) {
	type row struct {
		Seq    string
		Fields map[string]interface{}
	}
	type state struct{ Rows []row }
	tmpl := Must(New("nested-map"))
	if _, err := tmpl.Parse(`<ol>{{range .Rows}}<li data-key="{{.Seq}}"><span>{{range $k, $v := .Fields}}<i>{{$k}}={{$v}}</i>{{end}}</span></li>{{end}}</ol>`); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Enough fields that Go's random map order can't repeat by chance.
	r1 := row{Seq: "1", Fields: map[string]interface{}{
		"to": "x@example.com", "subject": "hi", "n": 3, "a": 1, "b": 2, "c": 3, "d": 4, "e": 5}}
	r2 := row{Seq: "2", Fields: map[string]interface{}{"body": "http://localhost/a"}}

	states := []interface{}{state{Rows: []row{r1}}}
	for i := 0; i < 5; i++ {
		states = append(states, state{Rows: []row{r1}})
	}
	states = append(states, state{Rows: []row{r1, r2}})
	out := updates(t, tmpl, states...)

	for i, u := range out[1 : len(out)-1] {
		if len(u) != 0 {
			t.Fatalf("re-render %d of an unchanged row produced a diff: %v", i+1, u)
		}
	}
	ops := ckOps(out[len(out)-1])
	if !hasOp(ops, "a", "") {
		t.Fatalf("expected an append op for the new row, got %v", ops)
	}
	if hasOp(ops, "u", "1") {
		t.Errorf("the unchanged row was sent again: %v", ops)
	}
}
