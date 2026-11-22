package parse

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// buildJSON builds src's tree for data and returns it as JSON, failing the
// test on any evaluation error.
func buildJSON(t *testing.T, src string, funcs template.FuncMap, data interface{}) string {
	t.Helper()
	tmpl, err := Parse(src, funcs)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	tree, err := BuildTree(tmpl, data, &Context{IncludeStatics: true})
	if err != nil {
		t.Fatalf("build %q: %v", src, err)
	}
	b, err := json.Marshal(tree.ToMap())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// and/or evaluate their arguments one at a time, as text/template does (#554).
func TestAndOrShortCircuit(t *testing.T) {
	const andURL = `<p>{{if and (ge (len .S) 8) (eq (slice .S 0 7) "http://")}}y{{else}}n{{end}}</p>`
	const orURL = `<p>{{if or (lt (len .S) 8) (eq (slice .S 0 7) "http://")}}y{{else}}n{{end}}</p>`
	tests := []struct {
		name, src string
		data      map[string]interface{}
		want      string
	}{
		{"and stops before a panicking argument", andURL, map[string]interface{}{"S": "ab"}, `{"s":["n"]}`},
		{"and evaluates every argument when all are true", andURL, map[string]interface{}{"S": "http://x.y"}, `{"s":["y"]}`},
		{"or stops at its first true argument", orURL, map[string]interface{}{"S": "ab"}, `{"s":["y"]}`},
		{"and returns the deciding value", `<p>{{and 1 0 2}}</p>`, nil, `"0":"0"`},
		{"and returns its last value", `<p>{{and 1 2 3}}</p>`, nil, `"0":"3"`},
		{"or returns the deciding value", `<p>{{or 0 "x" "y"}}</p>`, nil, `"0":"x"`},
		{"or with no true argument returns its last", `<p>{{or 0 "" 7}}</p>`, nil, `"0":"7"`},
		{"a nil first argument decides and",
			`<p>{{if and .Missing (slice .S 0 7)}}y{{else}}n{{end}}</p>`,
			map[string]interface{}{"Missing": nil, "S": "ab"}, `{"s":["n"]}`},
		{"a pipe argument comes last", `<p>{{"x" | and 1}}</p>`, nil, `"0":"x"`},
		{"or with a pipe argument stops before it", `<p>{{"x" | or 1}}</p>`, nil, `"0":"1"`},
		{"and with only a pipe argument", `<p>{{"x" | and}}</p>`, nil, `"0":"x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildJSON(t, tt.src, nil, tt.data); !strings.Contains(got, tt.want) {
				t.Errorf("tree %s, want it to contain %s", got, tt.want)
			}
		})
	}
}

// A page's own function named and is called normally, with every argument.
func TestAndOrPageFunctionGetsEveryArgument(t *testing.T) {
	var got []interface{}
	funcs := template.FuncMap{"and": func(args ...interface{}) string {
		got = args
		return "custom"
	}}
	tree := buildJSON(t, `<p>{{and false .X}}</p>`, funcs, map[string]interface{}{"X": "x"})
	if !strings.Contains(tree, `"0":"custom"`) {
		t.Fatalf("tree %s, want the page's own and", tree)
	}
	if len(got) != 2 {
		t.Fatalf("the page's and got %v, want both arguments", got)
	}
}

// itemValues lists the first dynamic of each item in the tree's first range.
func itemValues(t *testing.T, tree string) string {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(tree), &root); err != nil {
		t.Fatal(err)
	}
	rng, _ := root["0"].(map[string]any)
	items, _ := rng["d"].([]any)
	vals := make([]string, 0, len(items))
	for _, it := range items {
		vals = append(vals, fmt.Sprint(it.(map[string]any)["0"]))
	}
	return strings.Join(vals, " ")
}

// A range over a map visits its keys in sorted order, as text/template does,
// so the order is the same on every render.
func TestRangeMapKeysSorted(t *testing.T) {
	tmpl, err := Parse(`<p>{{range $k, $v := .M}}{{$k}};{{end}}</p>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		m    interface{}
		want string
	}{
		{"strings", map[string]int{"m": 1, "b": 2, "z": 3, "a": 4, "q": 5, "c": 6, "k": 7, "e": 8}, "a b c e k m q z"},
		{"ints by value", map[int]bool{10: true, 2: true, 33: true, -1: true, 7: true, 100: true, 0: true}, "-1 0 2 7 10 33 100"},
		{"floats by value", map[float64]int{1.5: 1, -2: 2, 10: 3, 0.25: 4, 3: 5}, "-2 0.25 1.5 3 10"},
		{"mixed kinds by kind, then value", map[interface{}]int{"9": 1, 10: 2, "a": 3, 2: 4, true: 5}, "true 2 10 9 a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 20; i++ {
				tree, err := BuildTree(tmpl, map[string]interface{}{"M": tt.m}, &Context{IncludeStatics: true})
				if err != nil {
					t.Fatal(err)
				}
				b, _ := json.Marshal(tree.ToMap())
				if got := itemValues(t, string(b)); got != tt.want {
					t.Fatalf("render %d: keys %q, want %q", i, got, tt.want)
				}
			}
		})
	}
}

// A range inside a tag already has its own slot, so the body-only fix leaves
// its tree byte-for-byte as it was. The expected JSON was captured from main
// before #554.
func TestRangeInsideATagUnchanged(t *testing.T) {
	const want = `{"0":{"d":[{"0":{"d":[{"0":"x","_k":"b5018057e8ee"},{"0":"y","_k":"b4fdd657e8eb"}],"m":{"idKey":"_k"},"s":["\u003cb\u003e","\u003c/b\u003e"]},"_k":"95db2c1eb343"},{"0":{"d":[{"0":"z","_k":"b4fa7057e8e8"}],"m":{"idKey":"_k"},"s":["\u003cb\u003e","\u003c/b\u003e"]},"_k":"3c6b2834e5cb"}],"m":{"idKey":"_k"},"s":["\u003cli\u003e","\u003c/li\u003e"]},"s":["\u003cul\u003e","\u003c/ul\u003e"]}`
	data := map[string]interface{}{"A": []map[string]interface{}{{"B": []string{"x", "y"}}, {"B": []string{"z"}}}}
	if got := buildJSON(t, `<ul>{{range .A}}<li>{{range .B}}<b>{{.}}</b>{{end}}</li>{{end}}</ul>`, nil, data); got != want {
		t.Fatalf("tree changed:\n got %s\nwant %s", got, want)
	}
}
