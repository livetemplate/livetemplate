package livetemplate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Issue #547: a range item's key is the whole key attribute value, not just its
// first dynamic, so items whose first part repeats keep distinct keys and a
// live update lands on the item that changed.

type ckRow struct {
	Fold      bool
	Old, New  int
	Composing bool
	Draft     string
}

type ckItem struct {
	X, Y int
	K, V string
}

// ckTree renders data and returns the tree as generic JSON.
func ckTree(t *testing.T, tmpl *Template, data any) (*treeNode, map[string]any) {
	t.Helper()
	tree, err := tmpl.buildTree(data, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return tree, ckJSON(t, tree)
}

func ckJSON(t *testing.T, tree *treeNode) map[string]any {
	t.Helper()
	b, err := json.Marshal(tree.ToMap())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// ckRange finds the first range ({"d": [...]}) in a JSON tree.
func ckRange(v any) map[string]any {
	switch n := v.(type) {
	case map[string]any:
		if _, ok := n["d"].([]any); ok {
			return n
		}
		for _, c := range n {
			if r := ckRange(c); r != nil {
				return r
			}
		}
	case []any:
		for _, c := range n {
			if r := ckRange(c); r != nil {
				return r
			}
		}
	}
	return nil
}

// ckKeys lists a range's item keys the way the client reads them: _k, else the
// item's value at the idKey position.
func ckKeys(t *testing.T, tree map[string]any) []string {
	t.Helper()
	r := ckRange(tree)
	if r == nil {
		t.Fatalf("no range in %v", tree)
	}
	idKey := "_k"
	if m, ok := r["m"].(map[string]any); ok {
		if k, ok := m["idKey"].(string); ok {
			idKey = k
		}
	}
	var keys []string
	for _, it := range r["d"].([]any) {
		keys = append(keys, fmt.Sprint(it.(map[string]any)[idKey]))
	}
	return keys
}

// ckOps lists the range ops in an update ([op, key, …] arrays).
func ckOps(v any) [][]any {
	var ops [][]any
	switch n := v.(type) {
	case map[string]any:
		for _, c := range n {
			ops = append(ops, ckOps(c)...)
		}
	case []any:
		for _, c := range n {
			if op, ok := c.([]any); ok && len(op) >= 2 {
				if s, ok := op[0].(string); ok && len(s) == 1 {
					ops = append(ops, op)
					continue
				}
			}
			ops = append(ops, ckOps(c)...)
		}
	}
	return ops
}

const ckDiffSrc = `<div>{{range .Rows}}{{if .Fold}}<div data-key="F{{.Old}}-{{.New}}">fold</div>{{else}}<div data-key="row-L{{.Old}}-{{.New}}{{if .Composing}}-composing{{end}}">{{.New}}{{if .Composing}}<textarea>{{.Draft}}</textarea>{{end}}</div>{{end}}{{end}}</div>`

func ckDiffRows(draft string) []ckRow {
	rows := []ckRow{{Fold: true, Old: 1, New: 1}, {Old: 3, New: 3}}
	for n := 4; n <= 9; n++ {
		rows = append(rows, ckRow{Old: 0, New: n})
	}
	rows[len(rows)-1].Composing = true
	rows[len(rows)-1].Draft = draft
	return append(rows, ckRow{Old: 4, New: 10})
}

// TestCompositeKey_WrappedRange: the {{if}}-wrapped range (prereview's diff rows)
// keys each item by its whole data-key, including the conditional suffix.
func TestCompositeKey_WrappedRange(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(ckDiffSrc)
	if err != nil {
		t.Fatal(err)
	}
	_, m := ckTree(t, tmpl, map[string]any{"Rows": ckDiffRows("S")})
	want := []string{"F1-1", "row-L3-3", "row-L0-4", "row-L0-5", "row-L0-6", "row-L0-7", "row-L0-8", "row-L0-9-composing", "row-L4-10"}
	if got := ckKeys(t, m); !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// TestCompositeKey_UpdateTargetsTheChangedItem: one draft change is one ["u"]
// op, under the same key the initial tree gave that item.
func TestCompositeKey_UpdateTargetsTheChangedItem(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(ckDiffSrc)
	if err != nil {
		t.Fatal(err)
	}
	t1, m1 := ckTree(t, tmpl, map[string]any{"Rows": ckDiffRows("S")})
	t2, _ := ckTree(t, tmpl, map[string]any{"Rows": ckDiffRows("Surface")})
	tmpl.lastTree = t1
	changes := ckJSON(t, tmpl.compareTreesAndGetChanges(t1, t2))
	ops := ckOps(changes)
	if len(ops) != 1 || ops[0][0] != "u" {
		t.Fatalf("ops = %v, want one [\"u\", …]", ops)
	}
	keys := ckKeys(t, m1)
	if ops[0][1] != keys[7] {
		t.Fatalf("update key %v, want the composing row's initial key %q (keys %v)", ops[0][1], keys[7], keys)
	}
}

// TestCompositeKey_ExplicitKey: a key in the item's own statics (no wrapper)
// with a prefix and two parts.
func TestCompositeKey_ExplicitKey(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(`<ul>{{range .Items}}<li data-key="a-{{.X}}-{{.Y}}">{{.V}}</li>{{end}}</ul>`)
	if err != nil {
		t.Fatal(err)
	}
	items := []ckItem{{X: 0, Y: 1, V: "one"}, {X: 0, Y: 2, V: "two"}, {X: 1, Y: 1, V: "three"}}
	_, m := ckTree(t, tmpl, map[string]any{"Items": items})
	if got, want := ckKeys(t, m), []string{"a-0-1", "a-0-2", "a-1-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// TestCompositeKey_SimpleKeyUnchanged: a key that is exactly one dynamic keeps
// its position idKey and carries no _k, so the wire format doesn't move.
func TestCompositeKey_SimpleKeyUnchanged(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(`<ul>{{range .Items}}<li data-key="{{.K}}">{{.V}}</li>{{end}}</ul>`)
	if err != nil {
		t.Fatal(err)
	}
	_, m := ckTree(t, tmpl, map[string]any{"Items": []ckItem{{K: "a", V: "1"}, {K: "b", V: "2"}}})
	r := ckRange(m)
	if idKey := r["m"].(map[string]any)["idKey"]; idKey != "0" {
		t.Fatalf("idKey = %v, want \"0\"", idKey)
	}
	for _, it := range r["d"].([]any) {
		if _, ok := it.(map[string]any)["_k"]; ok {
			t.Fatalf("simple key item carries _k: %v", it)
		}
	}
}

// TestCompositeKey_RepeatedKeysResendTheRange: keys that genuinely repeat can't
// address one item, so a change is a full range resend, never a ["u"] op that
// the client would apply to the first item holding the key.
func TestCompositeKey_RepeatedKeysResendTheRange(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(`<ul>{{range .Items}}<li data-key="{{.K}}">{{.V}}</li>{{end}}</ul>`)
	if err != nil {
		t.Fatal(err)
	}
	t1, _ := ckTree(t, tmpl, map[string]any{"Items": []ckItem{{K: "x", V: "first"}, {K: "x", V: "second"}}})
	t2, _ := ckTree(t, tmpl, map[string]any{"Items": []ckItem{{K: "x", V: "first"}, {K: "x", V: "changed"}}})
	tmpl.lastTree = t1
	changes := ckJSON(t, tmpl.compareTreesAndGetChanges(t1, t2))
	if ops := ckOps(changes); len(ops) != 0 {
		t.Fatalf("repeated keys produced keyed ops %v, want a full range resend", ops)
	}
	if r := ckRange(changes); r == nil || len(r["d"].([]any)) != 2 {
		t.Fatalf("want the whole range resent, got %v", changes)
	}
}

// TestCompositeKey_ConditionalOnlyKey: a key that is one {{if}} (a nested node,
// not a string) is flattened like any composite key, so the client never reads
// an object as the key.
func TestCompositeKey_ConditionalOnlyKey(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(`<ul>{{range .Items}}<li data-key="{{if .K}}on-{{.V}}{{else}}off-{{.V}}{{end}}">{{.V}}</li>{{end}}</ul>`)
	if err != nil {
		t.Fatal(err)
	}
	_, m := ckTree(t, tmpl, map[string]any{"Items": []ckItem{{K: "y", V: "a"}, {V: "b"}}})
	if got, want := ckKeys(t, m), []string{"on-a", "off-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// TestCompositeKey_MixedBranches: in an {{if}}-wrapped range, a branch with a
// simple key and a branch with a composite one both key through _k, so one
// range never mixes key kinds.
func TestCompositeKey_MixedBranches(t *testing.T) {
	tmpl, err := Must(New("test")).Parse(`<ul>{{range .Items}}{{if .K}}<li data-key="{{.V}}">a</li>{{else}}<li data-key="n-{{.X}}-{{.Y}}">b</li>{{end}}{{end}}</ul>`)
	if err != nil {
		t.Fatal(err)
	}
	_, m := ckTree(t, tmpl, map[string]any{"Items": []ckItem{{K: "y", V: "plain"}, {X: 0, Y: 1}, {X: 0, Y: 2}}})
	if got, want := ckKeys(t, m), []string{"plain", "n-0-1", "n-0-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}
